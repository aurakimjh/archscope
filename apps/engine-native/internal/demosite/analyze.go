// Package demosite — analyzer dispatch + metadata annotation. Mirrors
// `_analyze_file` and `_annotate_demo_metadata` in
// archscope_engine.demo_site_runner.
//
// Profiler inputs are now in the same Go module under internal/profiler,
// so profiler_collapsed can dispatch collapsed, Jennifer CSV, SVG, and
// HTML flamegraph inputs without crossing module boundaries.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] demosite/analyze.go — 매니페스트의 analyzer_type → 실제 분석기
// 함수 dispatch + 메타데이터 주석.
//
// dispatch 표 (대표적 매핑)
//
//	access_log              → analyzers/accesslog.Analyze
//	gc_log                  → analyzers/gclog.Analyze
//	thread_dump             → analyzers/threaddump.Analyze (단일)
//	thread_dump_multi       → analyzers/multithread.Analyze
//	thread_dump_locks       → analyzers/lockcontention.Analyze
//	exception_stack         → analyzers/exception.Analyze
//	jfr_recording           → analyzers/jfr.Analyze
//	native_memory           → analyzers/jfr.AnalyzeNativeMemory
//	nodejs_stack/python_traceback/go_panic/dotnet → analyzers/runtime.*
//	otel_logs               → analyzers/otel.Analyze
//	jennifer_profile        → analyzers/jenniferprofile.AnalyzeFiles
//
// profiler 입력 처리
//
//	profiler_collapsed / flamegraph_svg / flamegraph_html / jennifer_csv 는
//	같은 Go 모듈의 internal/profiler 로 dispatch.
//
// 메타데이터 주석
//
//	_annotate_demo_metadata 가 결과에 다음 주석 추가:
//	  metadata.demo.source       : 매니페스트 entry source.
//	  metadata.demo.scenario     : 시나리오 이름.
//	  metadata.demo.data_source  : 데이터 출처.
//	  metadata.demo.label        : 사람이 보는 라벨.
//	index.html 가 이 메타로 카드/표를 그릴 때 사용.
package demosite

import (
	"encoding/json"
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
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
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
		return analyzeProfilerDemoFile(sourcePath, fileEntry, topN)
	case "jennifer_profile":
		return jenniferprofile.AnalyzeFile(sourcePath, jenniferprofile.Options{})
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

func analyzeProfilerDemoFile(sourcePath string, fileEntry map[string]any, topN int) (models.AnalysisResult, error) {
	opts := profiler.Options{
		IntervalMS:  100,
		TopN:        topN,
		ProfileKind: "wall",
	}
	if elapsed := floatOr(fileEntry["elapsed_sec"], -1); elapsed >= 0 {
		opts.ElapsedSec = &elapsed
	}
	var (
		result profiler.AnalysisResult
		err    error
	)
	switch strings.ToLower(stringOr(fileEntry["format"], "")) {
	case "jennifer_csv":
		result, err = profiler.AnalyzeJenniferFile(sourcePath, opts)
	case "flamegraph_svg":
		result, err = profiler.AnalyzeFlamegraphSVGFile(sourcePath, opts)
	case "flamegraph_html":
		result, err = profiler.AnalyzeFlamegraphHTMLFile(sourcePath, opts)
	default:
		switch strings.ToLower(filepath.Ext(sourcePath)) {
		case ".csv":
			result, err = profiler.AnalyzeJenniferFile(sourcePath, opts)
		case ".svg":
			result, err = profiler.AnalyzeFlamegraphSVGFile(sourcePath, opts)
		case ".html", ".htm":
			result, err = profiler.AnalyzeFlamegraphHTMLFile(sourcePath, opts)
		default:
			result, err = profiler.AnalyzeCollapsedFile(sourcePath, opts)
		}
	}
	if err != nil {
		return models.AnalysisResult{}, err
	}
	return profilerToModelResult(result)
}

func profilerToModelResult(result profiler.AnalysisResult) (models.AnalysisResult, error) {
	out := models.New(result.Type, result.Metadata.Parser)
	out.SourceFiles = result.SourceFiles
	out.CreatedAt = result.CreatedAt
	out.Summary = jsonMap(result.Summary)
	out.Series = jsonMap(result.Series)
	out.Tables = jsonMap(result.Tables)
	out.Charts = jsonMap(result.Charts)
	out.Metadata.SchemaVersion = result.Metadata.SchemaVersion
	out.Metadata.Extra["diagnostics"] = jsonAny(result.Metadata.Diagnostics)
	out.Metadata.Extra["timeline_scope"] = jsonAny(result.Metadata.TimelineScope)
	if result.Metadata.ProgressLogPath != "" {
		out.Metadata.Extra["progress_log_path"] = result.Metadata.ProgressLogPath
	}
	if result.Metadata.DiffSummary != nil {
		out.Metadata.Extra["diff_summary"] = result.Metadata.DiffSummary
	}
	if result.Metadata.DiffTables != nil {
		out.Metadata.Extra["diff_tables"] = result.Metadata.DiffTables
	}
	return out, nil
}

func jsonMap(value any) map[string]any {
	converted := jsonAny(value)
	if converted == nil {
		return map[string]any{}
	}
	if m, ok := converted.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func jsonAny(value any) any {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil
	}
	return out
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
		"scenario":               scenario,
		"data_source":            dataSource,
		"manifest":               manifestPath,
		"manifest_analyzer_type": fileEntry["analyzer_type"],
		"manifest_file":          fileEntry["file"],
		"expected_categories":    manifest["expected_categories"],
		"expected_key_signals":   manifest["expected_key_signals"],
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
