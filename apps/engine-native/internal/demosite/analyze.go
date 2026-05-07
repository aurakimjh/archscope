// Package demosite — analyzer dispatch + metadata annotation. Mirrors
// `_analyze_file` and `_annotate_demo_metadata` in
// archscope_engine.demo_site_runner.
//
// Profiler caveat: the Python source dispatches to
// `analyze_collapsed_profile` / `analyze_jennifer_csv_profile`, but
// engine-native does NOT ship the profiler analyzers (they live in
// apps/profiler-native; see cmd/archscope-engine/cmd_profiler.go).
// The `profiler_collapsed` analyzer_type therefore returns a typed
// "not supported in engine-native" error from the Go runner — the
// runner records this as a failed run rather than crashing the
// scenario, matching the Python orchestrator's per-file try/except.
//
// Jennifer CSV is the one profiler shape we DO have via
// `internal/analyzers/jenniferprofile`; we route it through that
// analyzer so the demo runner can still process Jennifer fixtures.
package demosite

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/accesslog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/exception"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gclog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jenniferprofile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jfr"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/otel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtime"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddump"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// analyzeFile dispatches `fileEntry["analyzer_type"]` to the
// matching engine-native analyzer.
func analyzeFile(fileEntry map[string]any, sourcePath string) (models.AnalysisResult, error) {
	analyzerType := stringOr(fileEntry["analyzer_type"], "")
	topN := intOr(fileEntry["top_n"], 20)
	switch analyzerType {
	case "access_log":
		format := stringOr(fileEntry["format"], "nginx")
		return accesslog.Analyze(sourcePath, format, accesslog.Options{})
	case "profiler_collapsed":
		// Jennifer CSV — the only profiler shape engine-native ships.
		if stringOr(fileEntry["format"], "") == "jennifer_csv" || strings.EqualFold(filepath.Ext(sourcePath), ".csv") {
			return jenniferprofile.AnalyzeFile(sourcePath, jenniferprofile.Options{})
		}
		// Generic collapsed profiles live in apps/profiler-native;
		// flag this clearly so users see the right pointer.
		return models.AnalysisResult{}, fmt.Errorf("profiler_collapsed (wall) is provided by apps/profiler-native; not available in engine-native demo runner")
	case "jfr_recording":
		return jfr.Analyze(sourcePath, jfr.Options{TopN: topN})
	case "gc_log":
		return gclog.Analyze(sourcePath, gclog.Options{TopN: topN})
	case "thread_dump":
		return threaddump.Analyze(sourcePath, threaddump.Options{TopN: topN})
	case "exception", "exception_stack":
		return exception.Analyze(sourcePath, exception.Options{TopN: topN})
	case "nodejs_stack":
		return runtime.AnalyzeNodejsStack(sourcePath, runtime.Options{TopN: topN})
	case "python_traceback":
		return runtime.AnalyzePythonTraceback(sourcePath, runtime.Options{TopN: topN})
	case "go_panic":
		return runtime.AnalyzeGoPanic(sourcePath, runtime.Options{TopN: topN})
	case "dotnet_exception_iis":
		return runtime.AnalyzeDotnetExceptionIIS(sourcePath, runtime.Options{TopN: topN})
	case "otel_logs":
		return otel.Analyze(sourcePath, otel.Options{TopN: topN})
	default:
		return models.AnalysisResult{}, fmt.Errorf("unsupported analyzer_type: %s", analyzerType)
	}
}

// annotateDemoMetadata writes the `demo_site` and (for OTel)
// `manifest_validation` sub-keys onto the result's metadata.Extra
// map. Mirrors `_annotate_demo_metadata`.
func annotateDemoMetadata(
	result *models.AnalysisResult,
	manifest map[string]any,
	fileEntry map[string]any,
	manifestPath string,
) {
	dataSource := dataSourceOf(manifest, manifestPath)
	scenario := stringOr(manifest["scenario"], filepath.Base(filepath.Dir(manifestPath)))
	if result.Metadata.Extra == nil {
		result.Metadata.Extra = map[string]any{}
	}
	demoSite := map[string]any{
		"scenario":                 scenario,
		"data_source":              dataSource,
		"manifest":                 manifestPath,
		"manifest_analyzer_type":   fileEntry["analyzer_type"],
		"manifest_file":            fileEntry["file"],
		"expected_categories":      manifest["expected_categories"],
		"expected_key_signals":     manifest["expected_key_signals"],
	}
	result.Metadata.Extra["demo_site"] = demoSite

	if result.Type == "otel_logs" {
		annotateOtelManifestValidation(result, manifest)
	}

	if result.Type == "access_log" && dataSource == "real" && strings.Contains(scenario, "baseline") {
		errorRate := floatOr(result.Summary["error_rate"], 0)
		if errorRate >= 5 {
			result.AddFinding(
				"warning",
				"BASELINE_ANOMALY",
				"Real baseline data includes elevated 5xx/error-rate signals.",
				map[string]any{"error_rate": result.Summary["error_rate"]},
			)
		}
	}
}

// annotateOtelManifestValidation cross-checks the OTel result against
// `expected_key_signals`. Mirrors `_annotate_otel_manifest_validation`.
func annotateOtelManifestValidation(result *models.AnalysisResult, manifest map[string]any) {
	expected := dictOf(manifest["expected_key_signals"])
	if len(expected) == 0 {
		return
	}
	expectedServices, _ := expected["services"].([]any)
	actualServices := map[string]struct{}{}
	if rows, ok := result.Series["service_distribution"].([]any); ok {
		for _, row := range rows {
			m, ok := row.(map[string]any)
			if !ok {
				continue
			}
			svc, ok := m["service"].(string)
			if !ok || svc == "" || svc == "(unknown)" {
				continue
			}
			actualServices[svc] = struct{}{}
		}
	}
	validation := map[string]any{}

	if expectedServices != nil {
		expSet := map[string]struct{}{}
		for _, s := range expectedServices {
			if str, ok := s.(string); ok {
				expSet[str] = struct{}{}
			}
		}
		var missing []string
		for s := range expSet {
			if _, ok := actualServices[s]; !ok {
				missing = append(missing, s)
			}
		}
		sortedSlice(missing)
		validation["expected_services"] = sortedKeys(expSet)
		validation["actual_services"] = sortedKeys(actualServices)
		validation["missing_services"] = missing
		if len(missing) > 0 {
			result.AddFinding(
				"warning",
				"OTEL_EXPECTED_SERVICE_MISSING",
				"Manifest expected services that were not observed in OTel logs.",
				map[string]any{"missing_services": strings.Join(missing, ", ")},
			)
		}
	}
	validateExpectedCount(result, validation, expected, "traces", "unique_traces", "OTEL_EXPECTED_TRACE_COUNT_MISMATCH")
	validateExpectedCount(result, validation, expected, "failed_traces", "failed_traces", "OTEL_EXPECTED_FAILED_TRACE_COUNT_MISMATCH")
	if expectedSuccessful, ok := expected["successful_traces"]; ok {
		actualSuccessful := intOr(result.Summary["unique_traces"], 0) - intOr(result.Summary["failed_traces"], 0)
		validation["expected_successful_traces"] = expectedSuccessful
		validation["actual_successful_traces"] = actualSuccessful
		if expInt, ok := expectedSuccessful.(int); ok && expInt != actualSuccessful {
			result.AddFinding(
				"warning",
				"OTEL_EXPECTED_SUCCESSFUL_TRACE_COUNT_MISMATCH",
				"Manifest expected successful trace count differs from analysis.",
				map[string]any{"expected": expectedSuccessful, "actual": actualSuccessful},
			)
		} else if expFloat, ok := expectedSuccessful.(float64); ok && int(expFloat) != actualSuccessful {
			result.AddFinding(
				"warning",
				"OTEL_EXPECTED_SUCCESSFUL_TRACE_COUNT_MISMATCH",
				"Manifest expected successful trace count differs from analysis.",
				map[string]any{"expected": expectedSuccessful, "actual": actualSuccessful},
			)
		}
	}
	demoSite, _ := result.Metadata.Extra["demo_site"].(map[string]any)
	if demoSite != nil {
		demoSite["manifest_validation"] = validation
	}
}

// validateExpectedCount compares an expected count against an actual
// summary value and appends a warning finding on mismatch.
func validateExpectedCount(result *models.AnalysisResult, validation, expected map[string]any, expectedKey, actualKey, findingCode string) {
	expectedValue := expected[expectedKey]
	actualValue := result.Summary[actualKey]
	validation["expected_"+expectedKey] = expectedValue
	validation["actual_"+actualKey] = actualValue
	expInt, expIsInt := expectedValue.(int)
	if !expIsInt {
		if f, ok := expectedValue.(float64); ok && f == float64(int(f)) {
			expInt = int(f)
			expIsInt = true
		}
	}
	actualInt, actualIsInt := actualValue.(int)
	if !actualIsInt {
		if f, ok := actualValue.(float64); ok && f == float64(int(f)) {
			actualInt = int(f)
			actualIsInt = true
		}
	}
	if expIsInt && actualIsInt && expInt != actualInt {
		result.AddFinding(
			"warning",
			findingCode,
			"Manifest expected count differs from OTel analysis.",
			map[string]any{"expected": expectedValue, "actual": actualValue},
		)
	} else if expIsInt && !actualIsInt {
		result.AddFinding(
			"warning",
			findingCode,
			"Manifest expected count differs from OTel analysis.",
			map[string]any{"expected": expectedValue, "actual": actualValue},
		)
	}
}

// intOr coerces a JSON-decoded value to int, falling back to `fallback`.
func intOr(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case nil:
		return fallback
	}
	return fallback
}

// floatOr coerces a JSON-decoded value to float64, falling back.
func floatOr(v any, fallback float64) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case nil:
		return fallback
	}
	return fallback
}

// sortedKeys returns the keys of a string-set in lex order.
func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortedSlice(out)
	return out
}

func sortedSlice(s []string) {
	sort.Strings(s)
}
