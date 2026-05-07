// Package demosite — runner half. Ports
// archscope_engine.demo_site_runner. The orchestrator:
//
//  1. reads a JSON manifest describing a demo scenario,
//  2. for each `files[]` entry runs the matching analyzer,
//  3. writes JSON / HTML / PPTX outputs into `<output_root>/<data_source>/<scenario>/`,
//  4. emits a per-scenario `run-summary.json` and `index.html`.
//
// The Python source carries a few extra knobs we keep:
//   - `BaselineManifestPath` triggers a before/after diff against a
//     `normal-baseline` scenario (see baseline.go).
//   - `WritePPTX=false` short-circuits PPTX output (used by the
//     baseline run + tests that don't need slides).
//
// JSON-only constraint: the manifest loader uses encoding/json
// directly. YAML manifests are not supported in the Go port — convert
// with `yq -o=json` before invoking. The Python runner accepts both,
// but the canonical demo manifests in the repo are JSON.
package demosite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	enginejson "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/pptx"
)

// DemoAnalyzerRun mirrors the Python dataclass. Optional path fields
// use *string so a "no PPTX" run round-trips as null in JSON without
// the caller faking an empty path.
type DemoAnalyzerRun struct {
	AnalyzerType string   `json:"analyzer_type"`
	File         string   `json:"file"`
	Command      []string `json:"command"`
	JSONPath     *string  `json:"json_path,omitempty"`
	HTMLPath     *string  `json:"html_path,omitempty"`
	PPTXPath     *string  `json:"pptx_path,omitempty"`
	SkippedLines int      `json:"skipped_lines"`
	Failed       bool     `json:"failed"`
	Error        *string  `json:"error,omitempty"`
}

// DemoScenarioRun is the per-manifest result envelope.
type DemoScenarioRun struct {
	Scenario        string                   `json:"scenario"`
	DataSource      string                   `json:"data_source"`
	ManifestPath    string                   `json:"manifest_path"`
	OutputDir       string                   `json:"output_dir"`
	Runs            []DemoAnalyzerRun        `json:"runs"`
	SkippedFiles    []map[string]any         `json:"skipped_files"`
	ReferenceFiles  []map[string]any         `json:"reference_files"`
	ComparisonPaths []string                 `json:"comparison_paths"`
	IndexPath       *string                  `json:"index_path,omitempty"`
}

// FailedRuns mirrors the Python `failed_runs` property.
func (s DemoScenarioRun) FailedRuns() []DemoAnalyzerRun {
	out := []DemoAnalyzerRun{}
	for _, r := range s.Runs {
		if r.Failed {
			out = append(out, r)
		}
	}
	return out
}

// JSONPaths collects non-nil json paths across runs.
func (s DemoScenarioRun) JSONPaths() []string {
	out := []string{}
	for _, r := range s.Runs {
		if r.JSONPath != nil {
			out = append(out, *r.JSONPath)
		}
	}
	return out
}

// RunOptions corresponds to the Python keyword-arg surface.
type RunOptions struct {
	ManifestPath         string
	OutputRoot           string
	BaselineManifestPath string // empty = no baseline diff
	WritePPTX            bool
	MappingPath          string // empty = walk parents from manifest
}

// DiscoverDemoManifests returns the manifest paths under
// `manifestRoot`. If `manifestRoot` is itself a file, returns just
// that path. Otherwise globs `*/*/manifest.json` and sorts lex.
// Mirrors `discover_demo_manifests`.
func DiscoverDemoManifests(manifestRoot string) ([]string, error) {
	info, err := os.Stat(manifestRoot)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", manifestRoot, err)
	}
	if !info.IsDir() {
		return []string{manifestRoot}, nil
	}
	pattern := filepath.Join(manifestRoot, "*", "*", "manifest.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	sort.Strings(matches)
	return matches, nil
}

// RunDemoSiteManifest is the top-level orchestrator. Mirrors
// `run_demo_site_manifest`.
func RunDemoSiteManifest(opts RunOptions) (DemoScenarioRun, error) {
	manifestPath := opts.ManifestPath
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return DemoScenarioRun{}, err
	}
	scenario := stringOr(manifest["scenario"], filepath.Base(filepath.Dir(manifestPath)))
	dataSource := dataSourceOf(manifest, manifestPath)
	scenarioOutputDir := filepath.Join(opts.OutputRoot, dataSource, scenario)
	if err := os.MkdirAll(scenarioOutputDir, 0o755); err != nil {
		return DemoScenarioRun{}, fmt.Errorf("mkdir %s: %w", scenarioOutputDir, err)
	}

	mappingAnchor := opts.MappingPath
	if mappingAnchor == "" {
		mappingAnchor = manifestPath
	}
	mappings, err := LoadAnalyzerTypeMappings(mappingAnchor)
	if err != nil {
		return DemoScenarioRun{}, err
	}

	recommended := dictOf(manifest["recommended_archscope_options"])

	var (
		runs           []DemoAnalyzerRun
		skippedFiles   []map[string]any
		referenceFiles []map[string]any
	)

	for index, fileEntry := range manifestFiles(manifest) {
		merged := mergeDicts(recommended, fileEntry)
		analyzerType := strings.TrimSpace(stringOr(fileEntry["analyzer_type"], ""))
		relativeFile := strings.TrimSpace(stringOr(fileEntry["file"], ""))
		if analyzerType == "" || relativeFile == "" {
			skippedFiles = append(skippedFiles, map[string]any{
				"file":          relativeFile,
				"analyzer_type": analyzerType,
				"reason":        "missing file or analyzer_type",
			})
			continue
		}
		mapping, ok := mappings[analyzerType]
		if ok && mapping.Command == nil && len(mapping.FormatOverrides) == 0 {
			referenceFiles = append(referenceFiles, map[string]any{
				"file":          relativeFile,
				"analyzer_type": analyzerType,
				"description":   fileEntry["description"],
				"path":          filepath.Join(filepath.Dir(manifestPath), relativeFile),
			})
			continue
		}
		if !ok {
			skippedFiles = append(skippedFiles, map[string]any{
				"file":          relativeFile,
				"analyzer_type": analyzerType,
				"reason":        "unsupported analyzer_type",
			})
			continue
		}

		sourcePath := filepath.Join(filepath.Dir(manifestPath), relativeFile)
		stem := strings.TrimSuffix(filepath.Base(relativeFile), filepath.Ext(relativeFile))
		outputBase := fmt.Sprintf("%02d-%s-%s", index+1, stem, analyzerType)
		jsonPath := filepath.Join(scenarioOutputDir, outputBase+".json")
		htmlPath := filepath.Join(scenarioOutputDir, outputBase+".html")
		var pptxPath string
		if opts.WritePPTX {
			pptxPath = filepath.Join(scenarioOutputDir, outputBase+".pptx")
		}

		command, cmdErr := commandForEntry(merged, sourcePath, jsonPath, mapping)
		// We still record the (best-effort) command on failure rows so
		// downstream tooling can report what we tried to invoke.
		if cmdErr != nil {
			errStr := cmdErr.Error()
			runs = append(runs, DemoAnalyzerRun{
				AnalyzerType: analyzerType,
				File:         relativeFile,
				Command:      command,
				Failed:       true,
				Error:        &errStr,
			})
			continue
		}

		result, runErr := analyzeFile(merged, sourcePath)
		if runErr != nil {
			errStr := runErr.Error()
			runs = append(runs, DemoAnalyzerRun{
				AnalyzerType: analyzerType,
				File:         relativeFile,
				Command:      command,
				Failed:       true,
				Error:        &errStr,
			})
			continue
		}
		annotateDemoMetadata(&result, manifest, merged, manifestPath)

		if err := enginejson.Write(jsonPath, &result); err != nil {
			errStr := err.Error()
			runs = append(runs, DemoAnalyzerRun{
				AnalyzerType: analyzerType,
				File:         relativeFile,
				Command:      command,
				Failed:       true,
				Error:        &errStr,
			})
			continue
		}
		if err := html.WriteWithOptions(htmlPath, &result, html.Options{
			Title:      fmt.Sprintf("%s - %s", scenario, analyzerType),
			SourcePath: jsonPath,
		}); err != nil {
			errStr := err.Error()
			runs = append(runs, DemoAnalyzerRun{
				AnalyzerType: analyzerType,
				File:         relativeFile,
				Command:      command,
				Failed:       true,
				Error:        &errStr,
			})
			continue
		}
		if pptxPath != "" {
			if err := pptx.Write(pptxPath, &result); err != nil {
				errStr := err.Error()
				runs = append(runs, DemoAnalyzerRun{
					AnalyzerType: analyzerType,
					File:         relativeFile,
					Command:      command,
					Failed:       true,
					Error:        &errStr,
				})
				continue
			}
		}

		skipped := 0
		if result.Metadata.Diagnostics != nil {
			skipped = result.Metadata.Diagnostics.SkippedLines
		}
		runRow := DemoAnalyzerRun{
			AnalyzerType: analyzerType,
			File:         relativeFile,
			Command:      command,
			JSONPath:     strPtr(jsonPath),
			HTMLPath:     strPtr(htmlPath),
			SkippedLines: skipped,
		}
		if pptxPath != "" {
			runRow.PPTXPath = strPtr(pptxPath)
		}
		runs = append(runs, runRow)
	}

	// Baseline diff (no-op if scenario == "normal-baseline" or
	// BaselineManifestPath empty).
	comparisonPaths, err := writeBaselineComparison(scenario, scenarioOutputDir, opts.BaselineManifestPath, opts.OutputRoot, jsonPathsOf(runs))
	if err != nil {
		return DemoScenarioRun{}, err
	}

	summaryPath := filepath.Join(scenarioOutputDir, "run-summary.json")
	if err := enginejson.Write(summaryPath, scenarioSummary(manifest, manifestPath, runs, skippedFiles, referenceFiles, comparisonPaths)); err != nil {
		return DemoScenarioRun{}, fmt.Errorf("write summary: %w", err)
	}
	indexPath := filepath.Join(scenarioOutputDir, "index.html")
	if err := os.WriteFile(indexPath, []byte(RenderDemoIndex(scenario, dataSource, manifest, runs, skippedFiles, referenceFiles, comparisonPaths)), 0o644); err != nil {
		return DemoScenarioRun{}, fmt.Errorf("write index: %w", err)
	}

	if skippedFiles == nil {
		skippedFiles = []map[string]any{}
	}
	if referenceFiles == nil {
		referenceFiles = []map[string]any{}
	}
	if comparisonPaths == nil {
		comparisonPaths = []string{}
	}
	if runs == nil {
		runs = []DemoAnalyzerRun{}
	}
	return DemoScenarioRun{
		Scenario:        scenario,
		DataSource:      dataSource,
		ManifestPath:    manifestPath,
		OutputDir:       scenarioOutputDir,
		Runs:            runs,
		SkippedFiles:    skippedFiles,
		ReferenceFiles:  referenceFiles,
		ComparisonPaths: comparisonPaths,
		IndexPath:       strPtr(indexPath),
	}, nil
}

// readManifest loads + decodes a manifest JSON file.
func readManifest(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	return manifest, nil
}

// commandForEntry rebuilds the CLI command tuple a user could type.
// Mirrors `_command_for_entry` — same option ordering so docs stay
// in sync.
func commandForEntry(fileEntry map[string]any, sourcePath, outputPath string, mapping AnalyzerTypeMapping) ([]string, error) {
	analyzerType := stringOr(fileEntry["analyzer_type"], "")
	fileFormat := effectiveMappingFormat(fileEntry, sourcePath)
	cmd := CommandForMapping(mapping, fileFormat)
	inputOption := InputOptionForMapping(mapping, fileFormat)
	if cmd == nil || inputOption == "" {
		return nil, fmt.Errorf("Analyzer type is not executable: %s", analyzerType)
	}
	command := append([]string(nil), cmd...)
	command = append(command, inputOption, sourcePath)
	if analyzerType == "profiler_collapsed" && inputOption == "--wall" {
		if v, ok := fileEntry["wall_interval_ms"]; ok {
			command = append(command, "--wall-interval-ms", scalarString(v))
		}
	}
	if analyzerType == "profiler_collapsed" && inputOption == "--file" {
		if v, ok := fileEntry["interval_ms"]; ok {
			command = append(command, "--interval-ms", scalarString(v))
		}
	}
	if analyzerType == "profiler_collapsed" {
		if v, ok := fileEntry["elapsed_sec"]; ok {
			command = append(command, "--elapsed-sec", scalarString(v))
		}
	}
	if v, ok := fileEntry["format"]; ok && analyzerType == "access_log" {
		if s, ok := v.(string); ok && s != "" {
			command = append(command, "--format", s)
		}
	}
	if v, ok := fileEntry["top_n"]; ok && truthyInt(v) {
		command = append(command, "--top-n", scalarString(v))
	}
	command = append(command, "--out", outputPath)
	return command, nil
}

// jsonPathsOf extracts non-nil JSONPath values from the run set —
// helper for baseline comparison.
func jsonPathsOf(runs []DemoAnalyzerRun) []string {
	out := []string{}
	for _, r := range runs {
		if r.JSONPath != nil {
			out = append(out, *r.JSONPath)
		}
	}
	return out
}

// scenarioSummary mirrors `_scenario_summary` — emits the
// machine-readable run-summary.json the docs site indexes.
func scenarioSummary(
	manifest map[string]any,
	manifestPath string,
	runs []DemoAnalyzerRun,
	skippedFiles, referenceFiles []map[string]any,
	comparisonPaths []string,
) map[string]any {
	summaries := analysisSummaries(runs)
	totalSkipped := 0
	failedCount := 0
	okCount := 0
	for _, r := range runs {
		totalSkipped += r.SkippedLines
		if r.Failed {
			failedCount++
		} else {
			okCount++
		}
	}
	findingTotal := 0
	for _, s := range summaries {
		if v, ok := s["finding_count"].(int); ok {
			findingTotal += v
		}
	}
	analyzerRuns := make([]map[string]any, 0, len(runs))
	for _, r := range runs {
		analyzerRuns = append(analyzerRuns, map[string]any{
			"file":          r.File,
			"analyzer_type": r.AnalyzerType,
			"command":       r.Command,
			"json_path":     deref(r.JSONPath),
			"html_path":     deref(r.HTMLPath),
			"pptx_path":     deref(r.PPTXPath),
			"skipped_lines": r.SkippedLines,
			"failed":        r.Failed,
			"error":         deref(r.Error),
		})
	}
	failedRows := []map[string]any{}
	skippedReport := []map[string]any{}
	for _, r := range runs {
		if r.Failed {
			failedRows = append(failedRows, map[string]any{
				"file":          r.File,
				"analyzer_type": r.AnalyzerType,
				"error":         deref(r.Error),
			})
		}
		if r.SkippedLines > 0 {
			skippedReport = append(skippedReport, map[string]any{
				"file":          r.File,
				"analyzer_type": r.AnalyzerType,
				"skipped_lines": r.SkippedLines,
			})
		}
	}
	scenario := stringOr(manifest["scenario"], filepath.Base(filepath.Dir(manifestPath)))
	if skippedFiles == nil {
		skippedFiles = []map[string]any{}
	}
	if referenceFiles == nil {
		referenceFiles = []map[string]any{}
	}
	return map[string]any{
		"scenario":     scenario,
		"data_source":  dataSourceOf(manifest, manifestPath),
		"manifest":     manifestPath,
		"summary": map[string]any{
			"analyzer_outputs":   okCount,
			"failed_analyzers":   failedCount,
			"skipped_lines":      totalSkipped,
			"reference_files":    len(referenceFiles),
			"finding_count":      findingTotal,
			"comparison_reports": len(comparisonPaths),
		},
		"analysis_summaries": summaries,
		"analyzer_runs":      analyzerRuns,
		"skipped_files":      skippedFiles,
		"reference_files":    referenceFiles,
		"comparison_paths":   comparisonPaths,
		"failed_analyzers":   failedRows,
		"skipped_line_report": skippedReport,
	}
}

// deref returns the pointed string or nil for a *string.
func deref(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

// strPtr returns &s.
func strPtr(s string) *string { return &s }

// stringOr extracts a string from a generic value, falling back to
// `fallback` when the value is missing / non-string.
func stringOr(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}

// dictOf coerces a value to map[string]any (or empty for non-dicts).
func dictOf(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// mergeDicts merges `over` onto `under` (over wins). Pure-ish: returns
// a fresh map, leaves inputs alone.
func mergeDicts(under, over map[string]any) map[string]any {
	out := make(map[string]any, len(under)+len(over))
	for k, v := range under {
		out[k] = v
	}
	for k, v := range over {
		out[k] = v
	}
	return out
}

// scalarString stringifies a JSON-decoded scalar (int / float / bool /
// string) the way Python's str(value) does.
func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// If integer-valued float (JSON numbers always decode as
		// float64), emit without trailing zeros. Matches Python where
		// the field could be int or float.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case bool:
		if t {
			return "True"
		}
		return "False"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// truthyInt returns true if `v` is a non-zero integer-ish value
// (Python's `if file_entry.get("top_n"):`).
func truthyInt(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case float64:
		return t != 0
	case int:
		return t != 0
	case int64:
		return t != 0
	case string:
		return t != "" && t != "0"
	default:
		return true
	}
}

// effectiveMappingFormat picks the "format" used for mapping override
// lookup: explicit `format` field wins; otherwise infer "jennifer_csv"
// from a .csv suffix. Mirrors `_effective_mapping_format`.
func effectiveMappingFormat(fileEntry map[string]any, sourcePath string) string {
	if v, ok := fileEntry["format"].(string); ok && v != "" {
		return v
	}
	if strings.EqualFold(filepath.Ext(sourcePath), ".csv") {
		return "jennifer_csv"
	}
	return ""
}

// manifestFiles returns the list of `files[]` dict entries.
func manifestFiles(manifest map[string]any) []map[string]any {
	raw, ok := manifest["files"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// dataSourceOf prefers the explicit `data_source` field. Otherwise
// looks for "real" / "synthetic" segments in the manifest path.
// Mirrors `_data_source`.
func dataSourceOf(manifest map[string]any, manifestPath string) string {
	if v, ok := manifest["data_source"].(string); ok && (v == "real" || v == "synthetic") {
		return v
	}
	parts := strings.Split(manifestPath, string(os.PathSeparator))
	for _, p := range parts {
		if p == "real" {
			return "real"
		}
		if p == "synthetic" {
			return "synthetic"
		}
	}
	return "unknown"
}

