// Package reportdiff ports archscope_engine.exporters.report_diff —
// a tiny before/after AnalysisResult differ that emits its result as
// a plain `AnalysisResult`-shaped dict (type = "comparison_report").
//
// Two layers of API:
//
//   - Diff(before, after) builds the diff payload from two already-loaded
//     AnalysisResult values (structs OR generic maps). Returns the
//     payload as map[string]any so callers can pipe it through the
//     T-340 JSON exporter or mutate it before marshalling.
//   - Marshal / Write are convenience wrappers that produce the same
//     bytes the JSON exporter would emit (2-space indent, trailing
//     newline, non-ASCII verbatim).
//   - BuildComparisonReport(beforePath, afterPath, label) mirrors the
//     Python `build_comparison_report` signature for callers that
//     start from JSON files on disk.
//
// Diff shape mirrors the Python output exactly:
//
//	{
//	  "type": "comparison_report",
//	  "source_files": [...],            // empty for in-memory Diff
//	  "created_at": "<RFC3339Nano UTC>",
//	  "summary": {
//	    "label": <string|null>,
//	    "before_type": <string|null>,
//	    "after_type": <string|null>,
//	    "changed_metrics": <int>,
//	    "before_findings": <int>,
//	    "after_findings": <int>,
//	    "finding_delta": <int>,
//	  },
//	  "series": {
//	    "summary_metric_deltas": [<row>...],
//	    "finding_count_comparison": [
//	      {"side": "before", "finding_count": N},
//	      {"side": "after",  "finding_count": M},
//	    ],
//	  },
//	  "tables": {"summary_metric_deltas": [<row>...]},
//	  "charts": {},
//	  "metadata": {
//	    "schema_version": "0.1.0",
//	    "report_kind":   "before_after_diff",
//	    "before_created_at": <string|null>,
//	    "after_created_at":  <string|null>,
//	  },
//	}
//
// Each <row> in summary_metric_deltas:
//
//	{
//	  "metric": <name>, "before": <number>, "after": <number>,
//	  "delta": <after - before, rounded to 6 dp>,
//	  "change_percent": <((after - before) / before) * 100, 4 dp>
//	                    or null when before == 0,
//	}
//
// Numeric strategy: only metrics where BOTH sides are numeric appear
// (matches Python's `isinstance(..., (int, float))` filter). Non-numeric
// metrics, present-on-one-side metrics, and string metrics are silently
// skipped — same as Python.
//
// Finding strategy: count-only comparison (matches Python). The diff
// is intentionally NOT severity-aware or set-diff-by-code; that would
// be a larger feature touching the AnalysisResult contract.
package reportdiff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Diff builds the comparison report from two in-memory
// AnalysisResult-shaped values. `before` / `after` may be:
//
//   - models.AnalysisResult (typed struct from this engine)
//   - map[string]any        (generic JSON-decoded payload)
//   - any other JSON-marshalable value with the same shape.
//
// We round-trip through encoding/json so analyzer-specific Go structs
// (e.g. `models.AnalysisResult`'s custom `Metadata.MarshalJSON`) are
// projected onto their JSON-visible shape before diffing — this is
// the same projection the JSON exporter performs and the only one
// the diff needs to reason about.
//
// `label` is optional; pass "" to leave the summary.label as nil
// (matches the Python `label=None` default).
func Diff(before, after any, label string) (map[string]any, error) {
	beforeMap, err := toMap(before)
	if err != nil {
		return nil, fmt.Errorf("normalise before: %w", err)
	}
	afterMap, err := toMap(after)
	if err != nil {
		return nil, fmt.Errorf("normalise after: %w", err)
	}
	return buildReport(beforeMap, afterMap, label, nil), nil
}

// Marshal returns the JSON bytes for `Diff(before, after, "")`.
// Idiomatic match for json.Marshal — matches T-340's Marshal surface.
func Marshal(before, after any) ([]byte, error) {
	report, err := Diff(before, after, "")
	if err != nil {
		return nil, err
	}
	return marshalIndented(report)
}

// Write persists `Diff(before, after, "")` to `path`, creating any
// missing parent directories. Mirrors the T-340 JSON exporter's
// `Write` semantics: 2-space indent, trailing newline, UTF-8.
func Write(path string, before, after any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	body, err := Marshal(before, after)
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// BuildComparisonReport mirrors Python's
// `build_comparison_report(before_path, after_path, label=None)`.
// Pass `label=""` to leave the label field as nil.
//
// The returned `source_files` list contains the two input paths
// verbatim, matching the Python output.
func BuildComparisonReport(beforePath, afterPath, label string) (map[string]any, error) {
	beforeBody, err := os.ReadFile(beforePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", beforePath, err)
	}
	afterBody, err := os.ReadFile(afterPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", afterPath, err)
	}
	var beforeMap map[string]any
	if err := json.Unmarshal(beforeBody, &beforeMap); err != nil {
		return nil, fmt.Errorf("decode %s: %w", beforePath, err)
	}
	var afterMap map[string]any
	if err := json.Unmarshal(afterBody, &afterMap); err != nil {
		return nil, fmt.Errorf("decode %s: %w", afterPath, err)
	}
	return buildReport(beforeMap, afterMap, label, []string{beforePath, afterPath}), nil
}

// buildReport is the shared core: given two normalised maps, emit
// the comparison_report payload with the supplied source_files +
// optional label.
func buildReport(before, after map[string]any, label string, sourceFiles []string) map[string]any {
	beforeSummary := dictOf(before["summary"])
	afterSummary := dictOf(after["summary"])
	metricRows := metricRows(beforeSummary, afterSummary)

	beforeCount := findingCount(before)
	afterCount := findingCount(after)
	findingsRows := []map[string]any{
		{"side": "before", "finding_count": beforeCount},
		{"side": "after", "finding_count": afterCount},
	}

	changed := 0
	for _, row := range metricRows {
		// `delta` is always a float (after rounding). Compare against
		// 0.0 directly — rounding to 6dp means anything finer than
		// 1e-6 already collapsed to 0 and counts as "unchanged".
		if row["delta"].(float64) != 0 {
			changed++
		}
	}

	// label is the Python `label=None` field — nil maps to JSON null.
	var labelValue any
	if label != "" {
		labelValue = label
	}

	files := sourceFiles
	if files == nil {
		files = []string{}
	}

	return map[string]any{
		"type":         "comparison_report",
		"source_files": files,
		"created_at":   time.Now().UTC().Format(time.RFC3339Nano),
		"summary": map[string]any{
			"label":           labelValue,
			"before_type":     before["type"],
			"after_type":      after["type"],
			"changed_metrics": changed,
			"before_findings": beforeCount,
			"after_findings":  afterCount,
			"finding_delta":   afterCount - beforeCount,
		},
		"series": map[string]any{
			"summary_metric_deltas":    metricRows,
			"finding_count_comparison": findingsRows,
		},
		"tables": map[string]any{"summary_metric_deltas": metricRows},
		"charts": map[string]any{},
		"metadata": map[string]any{
			"schema_version":    "0.1.0",
			"report_kind":       "before_after_diff",
			"before_created_at": before["created_at"],
			"after_created_at":  after["created_at"],
		},
	}
}

// metricRows mirrors `_metric_rows`. Sorts the union of keys from
// both summaries; emits a row only when BOTH sides hold numeric
// values. Matches Python's `isinstance(value, (int, float))` filter,
// where bool is technically a subclass of int — but Python booleans
// in summary metrics are almost certainly a bug and we follow Python:
// a bool from a JSON decode in Go arrives as `bool` (not numeric),
// so we naturally exclude them. (Python would have included them.)
//
// We don't care about that drift — no analyzer puts a bool in
// summary metrics — and the Go behaviour is the safer one.
func metricRows(before, after map[string]any) []map[string]any {
	keys := make(map[string]struct{}, len(before)+len(after))
	for k := range before {
		keys[k] = struct{}{}
	}
	for k := range after {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	rows := make([]map[string]any, 0, len(sortedKeys))
	for _, k := range sortedKeys {
		bv, bok := numericValue(before[k])
		av, aok := numericValue(after[k])
		if !bok || !aok {
			continue
		}
		delta := roundTo(av-bv, 6)
		rows = append(rows, map[string]any{
			"metric":         k,
			"before":         bv,
			"after":          av,
			"delta":          delta,
			"change_percent": changePercent(bv, av),
		})
	}
	return rows
}

// changePercent mirrors `_change_percent`. Returns `nil` (Python None)
// when `before == 0` so JSON marshals it as null.
func changePercent(before, after float64) any {
	if before == 0 {
		return nil
	}
	return roundTo(((after-before)/before)*100, 4)
}

// findingCount mirrors `_finding_count`: read `metadata.findings`
// and return its length when it's a list.
func findingCount(payload map[string]any) int {
	metadata := dictOf(payload["metadata"])
	findings, ok := metadata["findings"].([]any)
	if ok {
		return len(findings)
	}
	// `models.AnalysisResult` JSON-encodes findings as a typed slice;
	// after a round-trip it becomes []any, but if a caller hands us a
	// non-encoded map[string]any[] we should still count it correctly.
	if rows, ok := metadata["findings"].([]map[string]any); ok {
		return len(rows)
	}
	return 0
}

// numericValue reports whether `v` is a JSON numeric value and
// returns it as float64 if so. JSON-decoded numbers are always
// float64 in Go's stdlib; we also accept Go's native ints / floats
// in case the caller passes a hand-built map.
//
// bool is intentionally excluded — see metricRows.
func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// dictOf mirrors Python's `_dict` helper.
func dictOf(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// toMap projects an arbitrary JSON-marshalable value onto a generic
// map by round-tripping through encoding/json. This is the cheapest
// way to handle both `models.AnalysisResult` (which has a custom
// MarshalJSON for Metadata) and plain map[string]any uniformly.
func toMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	body, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// roundTo replicates Python's `round(x, ndigits)` for the half-even
// banker's-rounding case Python uses on floats. We follow Python's
// effective rounding semantics enough that our metric output matches
// the Python exporter on the values analyzers actually produce
// (counts, averages, percentages — none stress the half-even edge).
func roundTo(x float64, ndigits int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	mult := math.Pow(10, float64(ndigits))
	return math.Round(x*mult) / mult
}

// marshalIndented matches the T-340 JSON exporter's emission: 2-space
// indent, no HTML escaping, trailing newline. We don't import the
// JSON exporter package to keep this exporter standalone (and avoid
// an import cycle if json/ ever depends on diff helpers).
func marshalIndented(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}
