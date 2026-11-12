package metrichistory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// SourceError is a bounded, value-free source diagnostic. Paths address decoded
// JSON (including gzip inputs); unknown object keys are redacted. No input value,
// filename, producer label or raw Go error is retained in the diagnostic.
// Offset is a one-based decoded JSON byte offset when the decoder supplies one.
type SourceError struct {
	Code, Path, Rule string
	Offset           int64
}

func boundedDiagnostic(s string, limit int) string {
	if len(s) > limit {
		s = s[:limit] + "..."
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || r > 126 {
			return '?'
		}
		return r
	}, s)
}
func (e *SourceError) Error() string {
	return fmt.Sprintf("%s: path=%s rule=%s byte_offset=%d; entire source refused", boundedDiagnostic(e.Code, 80), boundedDiagnostic(e.Path, 256), boundedDiagnostic(e.Rule, 96), e.Offset)
}

var diagnosticCode = regexp.MustCompile(`^(METRIC|APPLE_CAPTURE|SNAPSHOT_CAPTURE)_[A-Z0-9_]+`)

func diagnostic(path, rule string, err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || strings.HasPrefix(err.Error(), "METRIC_BUDGET_EXCEEDED") {
		return err
	}
	var known *SourceError
	if errors.As(err, &known) {
		return err
	}
	if path == "" {
		path = "/"
	}
	code := diagnosticCode.FindString(err.Error())
	if code == "" {
		code = "METRIC_SOURCE_INVALID"
	}
	return &SourceError{Code: code, Path: path, Rule: rule}
}
func decimalAt(raw json.RawMessage, path string) (*string, string, error) {
	v, s, e := decimal(raw)
	return v, s, diagnostic(path, "finite_exact_decimal_or_declared_nonfinite; exponent_-128_to_128; scale_le128", e)
}
func integerAt(raw json.RawMessage, signed, zero bool, path string) (string, error) {
	v, e := integer(raw, signed, zero)
	rule := "uint64_integer"
	if signed {
		rule = "signed_int64_integer"
	}
	return v, diagnostic(path, rule, e)
}
func timeAt(raw json.RawMessage, path string) (*string, error) {
	v, e := timeField(raw)
	return v, diagnostic(path, "uint64_nanoseconds; zero_is_unavailable", e)
}
func finiteAt(raw json.RawMessage, path string) (*string, error) {
	v, e := finiteOptional(raw)
	return v, diagnostic(path, "finite_exact_decimal; exponent_-128_to_128; scale_le128", e)
}

// The source decoder retains the existing budget/strictness policy. A failure
// gets a path without serializing the offending value or Go destination type.
func decodeSource(ctx context.Context, body []byte, out any, strict bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	if strict {
		err = m.Strict(body, out)
	} else {
		err = wirejson.DecodeWithLimits(body, out, wirejson.ResultLimits(MaxInputBytes, 128))
	}
	if err == nil || strings.HasPrefix(err.Error(), "METRIC_BUDGET_EXCEEDED") {
		return err
	}
	var mismatch *json.UnmarshalTypeError
	var syntax *json.SyntaxError
	var limit *wirejson.LimitError
	issue := &SourceError{Code: "METRIC_JSON_SHAPE_INVALID", Path: "/", Rule: "declared_JSON_field_types"}
	switch {
	case errors.As(err, &mismatch):
		issue.Offset = mismatch.Offset
	case errors.As(err, &syntax):
		issue.Code = "METRIC_JSON_INVALID"
		issue.Rule = "valid_UTF8_JSON"
		issue.Offset = syntax.Offset
	case errors.As(err, &limit):
		issue.Code = "METRIC_JSON_BUDGET"
		issue.Rule = "bounded_JSON_" + limit.Resource
	default:
		if !strings.HasPrefix(err.Error(), "json: unknown field") {
			issue.Code = "METRIC_JSON_INVALID"
			issue.Rule = "valid_UTF8_single_JSON_without_duplicate_keys"
		}
		if code := diagnosticCode.FindString(err.Error()); code != "" {
			issue.Code = code
			issue.Rule = "bounded_declared_JSON"
		}
	}
	if issue.Offset > 0 {
		path, scanErr := sourceOffsetPath(ctx, body, issue.Offset)
		if scanErr != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		issue.Path = path
	}
	return issue
}

// Keys come only from declared structs, never user-controlled labels/attribute
// names. Traversal is read-only, streaming and used only after a decoder error.
var sourceKeys = func() map[string]bool {
	keys := map[string]bool{}
	seen := map[reflect.Type]bool{}
	var visit func(reflect.Type)
	visit = func(t reflect.Type) {
		for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}
		if seen[t] || t.Kind() != reflect.Struct {
			return
		}
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if tag := strings.Split(f.Tag.Get("json"), ",")[0]; tag != "" && tag != "-" {
				keys[tag] = true
			}
			visit(f.Type)
		}
	}
	for _, v := range []any{otlpFile{}, historyFile{}, appleCapture{}, pgCapture{}, ssCapture{}} {
		visit(reflect.TypeOf(v))
	}
	for _, key := range []string{"status", "data", "resultType", "result", "metric", "values", "histograms", "warnings", "infos", "resource_metrics"} {
		keys[key] = true
	}
	return keys
}()

func sourceOffsetPath(ctx context.Context, body []byte, target int64) (string, error) {
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	var walk func(string, int) (string, bool, error)
	walk = func(path string, depth int) (string, bool, error) {
		if err := ctx.Err(); err != nil {
			return path, false, err
		}
		if depth > 128 || len(path) > 256 {
			return path, true, nil
		}
		t, err := d.Token()
		if err != nil {
			return path, true, err
		}
		if target <= d.InputOffset() {
			return path, true, nil
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return path, false, nil
		}
		for i := 0; d.More(); i++ {
			child := strconv.Itoa(i)
			if delim == '{' {
				key, e := d.Token()
				if e != nil {
					return path, true, e
				}
				name, _ := key.(string)
				child = "?"
				if sourceKeys[name] {
					child = name
				}
			}
			next := strings.TrimSuffix(path, "/") + "/" + child
			if found, yes, e := walk(next, depth+1); yes || e != nil {
				return found, yes, e
			}
		}
		_, err = d.Token()
		return path, target <= d.InputOffset(), err
	}
	path, _, err := walk("/", 0)
	return path, err
}

// Validation failures are localized only on failure, using one-series/one-point
// candidates. Successful normalization/IDs/bytes and warm-query costs are unchanged.
func (b *builder) modelDiagnostic(set m.Set, err error) error {
	if err == nil || b.ctx.Err() != nil || strings.HasPrefix(err.Error(), "METRIC_BUDGET_EXCEEDED") {
		return err
	}
	candidate := m.Set{Version: set.Version, Normalizer: set.Normalizer, Sources: set.Sources, Warnings: []string{}}
	byID := map[string]m.Series{}
	for _, series := range set.Series {
		byID[series.ID] = series
		candidate.Series = []m.Series{series}
		candidate.Points = nil
		if failure := candidate.Validate(b.ctx); failure != nil {
			return diagnostic("/", "bounded_declared_metric_identity", failure)
		}
	}
	for _, point := range set.Points {
		if e := b.ctx.Err(); e != nil {
			return e
		}
		series := byID[point.SeriesID]
		candidate.Series = []m.Series{series}
		candidate.Points = []m.Point{point}
		candidate.Duplicates = len(point.Refs) - 1
		if failure := candidate.Validate(b.ctx); failure != nil {
			path := "/" + point.Refs[0].Position
			if b.set.Sources[0].Format == "otlp-json" {
				kind := series.Kind
				if kind == "exponential_histogram" {
					kind = "exponentialHistogram"
				}
				path = strings.Replace(path, "/dataPoints/", "/"+kind+"/dataPoints/", 1)
			}
			field := ""
			rule := "valid_metric_observation"
			if point.Histogram != nil {
				if b.set.Sources[0].Format == "snapshot-sequence-json" {
					if issue := snapshotHistogramDiagnostic(point.Histogram, path+"/histogram"); issue != nil {
						return issue
					}
				}
				field = "/histogram"
				rule = "valid_histogram_shape_counts_and_bounds"
				if b.set.Sources[0].Format == "otlp-json" {
					field = ""
				}
			}
			if point.Value != nil {
				field = "/value"
				rule = "valid_exact_metric_value"
			}
			return diagnostic(path+field, rule, failure)
		}
	}
	return diagnostic("/", "bounded_valid_metric_model", err)
}

// Numeric fields of a failing neutral histogram are checked only on the error
// path. Successful imports still use the original single model validator.
func snapshotHistogramDiagnostic(h *m.Histogram, path string) error {
	for _, f := range []struct {
		name  string
		value string
	}{{"count", h.Count}} {
		if _, e := m.Uint(f.value); e != nil {
			return diagnostic(path+"/"+f.name, "canonical_uint64", e)
		}
	}
	for _, f := range []struct {
		name  string
		value *string
	}{{"sum", h.Sum}, {"min", h.Min}, {"max", h.Max}, {"zero_threshold", h.ZeroThreshold}} {
		if f.value != nil {
			if _, e := m.Number(*f.value); e != nil {
				return diagnostic(path+"/"+f.name, "finite_exact_decimal; scale_le128", fmt.Errorf("METRIC_NUMBER_EXACT_BUDGET"))
			}
		}
	}
	for i, v := range h.Bounds {
		if _, e := m.Number(v); e != nil {
			return diagnostic(fmt.Sprintf("%s/bounds/%d", path, i), "finite_exact_decimal_bound", fmt.Errorf("METRIC_HISTOGRAM_BOUND_INVALID"))
		}
	}
	for i, v := range h.Counts {
		if _, e := m.Uint(v); e != nil {
			return diagnostic(fmt.Sprintf("%s/counts/%d", path, i), "canonical_uint64", e)
		}
	}
	for _, side := range []struct {
		name  string
		value *m.Buckets
	}{{"positive", h.Positive}, {"negative", h.Negative}} {
		if side.value != nil {
			for i, v := range side.value.Counts {
				if _, e := m.Uint(v); e != nil {
					return diagnostic(fmt.Sprintf("%s/%s/counts/%d", path, side.name, i), "canonical_uint64", e)
				}
			}
		}
	}
	return nil
}
