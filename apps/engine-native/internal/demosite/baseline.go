// Package demosite — baseline comparison. Mirrors
// `_write_baseline_comparison`, `_first_result_json`,
// `_result_json_by_analyzer`, `_analyzer_type_from_result_json`.
//
// When a non-baseline scenario references a baseline manifest, we
// look for an existing `<output_root>/<data_source>/normal-baseline/`
// directory; if missing, we run the baseline manifest first to
// populate it. Then for each analyzer_type we produce a
// `before/after` diff JSON + HTML pair via the reportdiff exporter.
package demosite

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	enginejson "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/reportdiff"
)

// writeBaselineComparison generates per-analyzer diff reports between
// the baseline scenario and the current one. Returns the list of
// generated paths (alternating json + html for each analyzer).
func writeBaselineComparison(
	scenario, scenarioOutputDir, baselineManifestPath, outputRoot string,
	afterJSONPaths []string,
) ([]string, error) {
	if scenario == "normal-baseline" || baselineManifestPath == "" {
		return nil, nil
	}
	baselineManifest, err := readManifest(baselineManifestPath)
	if err != nil {
		return nil, err
	}
	baselineDataSource := dataSourceOf(baselineManifest, baselineManifestPath)
	beforeDir := filepath.Join(outputRoot, baselineDataSource, "normal-baseline")
	beforeByAnalyzer := resultJSONByAnalyzer(beforeDir)
	if len(beforeByAnalyzer) == 0 {
		// Run the baseline manifest first to populate the directory.
		// Important: pass WritePPTX=false + no nested baseline so we
		// don't recurse forever. Mirrors the Python branch.
		baselineRun, err := RunDemoSiteManifest(RunOptions{
			ManifestPath: baselineManifestPath,
			OutputRoot:   outputRoot,
			WritePPTX:    false,
		})
		if err != nil {
			return nil, fmt.Errorf("run baseline %s: %w", baselineManifestPath, err)
		}
		beforeByAnalyzer = resultJSONByAnalyzer(baselineRun.OutputDir)
	}

	// Map after-JSON paths by their embedded analyzer_type so we pair
	// before/after on the same analyzer.
	afterByAnalyzer := map[string]string{}
	for _, p := range afterJSONPaths {
		if t := analyzerTypeFromResultJSON(p); t != "" {
			afterByAnalyzer[t] = p
		}
	}

	analyzers := make([]string, 0, len(afterByAnalyzer))
	for k := range afterByAnalyzer {
		analyzers = append(analyzers, k)
	}
	sort.Strings(analyzers)

	var out []string
	for _, analyzerType := range analyzers {
		afterJSON := afterByAnalyzer[analyzerType]
		beforeJSON, ok := beforeByAnalyzer[analyzerType]
		if !ok {
			continue
		}
		diffJSON := filepath.Join(scenarioOutputDir, fmt.Sprintf("normal-baseline-vs-%s.json", analyzerType))
		diffHTML := filepath.Join(scenarioOutputDir, fmt.Sprintf("normal-baseline-vs-%s.html", analyzerType))
		report, err := reportdiff.BuildComparisonReport(beforeJSON, afterJSON, fmt.Sprintf("normal-baseline vs %s (%s)", scenario, analyzerType))
		if err != nil {
			return nil, fmt.Errorf("build comparison %s: %w", analyzerType, err)
		}
		if err := enginejson.Write(diffJSON, report); err != nil {
			return nil, fmt.Errorf("write %s: %w", diffJSON, err)
		}
		if err := html.WriteWithOptions(diffHTML, report, html.Options{SourcePath: diffJSON}); err != nil {
			return nil, fmt.Errorf("write %s: %w", diffHTML, err)
		}
		out = append(out, diffJSON, diffHTML)
	}
	return out, nil
}

// resultJSONByAnalyzer indexes a directory of JSON outputs by the
// analyzer_type each one carries in `metadata.demo_site`. Mirrors
// `_result_json_by_analyzer`.
func resultJSONByAnalyzer(dir string) map[string]string {
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	sort.Strings(paths)
	for _, p := range paths {
		if t := analyzerTypeFromResultJSON(p); t != "" {
			out[t] = p
		}
	}
	return out
}

// analyzerTypeFromResultJSON pulls
// metadata.demo_site.manifest_analyzer_type from a result JSON file,
// skipping files that look like baseline diffs or run summaries.
func analyzerTypeFromResultJSON(path string) string {
	name := filepath.Base(path)
	if name == "run-summary.json" {
		return ""
	}
	if len(name) >= len("normal-baseline-vs-") && name[:len("normal-baseline-vs-")] == "normal-baseline-vs-" {
		return ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	metadata := dictOf(payload["metadata"])
	demoSite := dictOf(metadata["demo_site"])
	if t, ok := demoSite["manifest_analyzer_type"].(string); ok {
		return t
	}
	return ""
}
