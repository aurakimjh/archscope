package metrichistory_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"strings"
	"testing"
)

func sourceIssue(t *testing.T, raw []byte, options p.Options, code, path string) *p.SourceError {
	t.Helper()
	set, err := p.ParseBytes(context.Background(), raw, "SECRET_FILENAME", options)
	if err == nil {
		t.Fatal("invalid source accepted")
	}
	if len(set.Sources) != 0 || len(set.Series) != 0 || len(set.Points) != 0 {
		t.Fatal("partial source returned")
	}
	var issue *p.SourceError
	if !errors.As(err, &issue) {
		t.Fatalf("raw/untyped error: %v", err)
	}
	if issue.Code != code || issue.Path != path {
		t.Fatalf("got %s %s; want %s %s: %s", issue.Code, issue.Path, code, path, err)
	}
	if len(err.Error()) > 512 || issue.Rule == "" || !strings.Contains(err.Error(), "entire source refused") || strings.Contains(err.Error(), "strconv.") || strings.Contains(err.Error(), "Go struct") || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("unsafe/incomplete diagnostic", err)
	}
	return issue
}
func gaugeSource(points string) []byte {
	return []byte(`{"resourceMetrics":[{"scopeMetrics":[{"metrics":[{"name":"x","gauge":{"dataPoints":[` + points + `]}}]}]}]}`)
}
func TestR1304ThousandPointNumericCoordinatesAndGzip(t *testing.T) {
	for _, tc := range []struct{ field, value, code string }{{"asDouble", `"1e-200"`, "METRIC_NUMBER_EXPONENT_BUDGET"}, {"asDouble", `1.7976931348623157e308`, "METRIC_NUMBER_EXPONENT_BUDGET"}, {"asInt", `"9223372036854775808"`, "METRIC_INTEGER_RANGE"}, {"asInt", `"-9223372036854775809"`, "METRIC_INTEGER_RANGE"}, {"asDouble", `true`, "METRIC_NUMBER_INVALID"}, {"asDouble", `"0.` + strings.Repeat("0", 128) + `1"`, "METRIC_NUMBER_EXACT_BUDGET"}} {
		t.Run(tc.field+tc.code, func(t *testing.T) {
			points := make([]string, 1000)
			for i := range points {
				points[i] = `{"asInt":"1","timeUnixNano":"1"}`
			}
			points[777] = fmt.Sprintf(`{"%s":%s,"timeUnixNano":"1"}`, tc.field, tc.value)
			raw := gaugeSource(strings.Join(points, ","))
			want := "/resourceMetrics/0/scopeMetrics/0/metrics/0/gauge/dataPoints/777/" + tc.field
			sourceIssue(t, raw, p.Options{}, tc.code, want)
			var zipped bytes.Buffer
			w := gzip.NewWriter(&zipped)
			w.Write(raw)
			w.Close()
			sourceIssue(t, zipped.Bytes(), p.Options{}, tc.code, want)
		})
	}
}
func TestR1304DecoderShapeCoordinatesAndSafeText(t *testing.T) {
	raw := []byte(`{"schema_version":"1.0.0","snapshots":[{"time_unix_nano":"1","metrics":[]},{"time_unix_nano":123,"metrics":[]}]}`)
	issue := sourceIssue(t, raw, p.Options{}, "METRIC_JSON_SHAPE_INVALID", "/snapshots/1/time_unix_nano")
	if issue.Offset <= 0 {
		t.Fatal("missing one-based decoded byte offset")
	}
	sourceIssue(t, gaugeSource(`{"asInt":"1","flags":"SECRET_VALUE"}`), p.Options{}, "METRIC_JSON_SHAPE_INVALID", "/resourceMetrics/0/scopeMetrics/0/metrics/0/gauge/dataPoints/0/flags")
	sourceIssue(t, []byte(`{"resourceMetrics":{}}`), p.Options{}, "METRIC_JSON_SHAPE_INVALID", "/resourceMetrics")
	sourceIssue(t, []byte(`{"resource_metrics":[]}`), p.Options{}, "METRIC_OTLP_ROOT_REQUIRED", "/resource_metrics")
	sourceIssue(t, []byte(`{"resource_metrics":[]}`), p.Options{Format: "otlp-json"}, "METRIC_OTLP_ROOT_REQUIRED", "/resourceMetrics")
	// Unknown field names and raw strconv/decoder input text never reach Error().
	secret := strings.Repeat("SECRET", 500)
	sourceIssue(t, []byte(`{"schema_version":"1.0.0","snapshots":[],"`+secret+`":1}`), p.Options{}, "METRIC_JSON_SHAPE_INVALID", "/")
	sourceIssue(t, gaugeSource(`{"asInt":"`+secret+`"}`), p.Options{}, "METRIC_NUMBER_INVALID", "/resourceMetrics/0/scopeMetrics/0/metrics/0/gauge/dataPoints/0/asInt")
	sourceIssue(t, []byte("{\"resourceMetrics\":[}"), p.Options{}, "METRIC_JSON_INVALID", "/resourceMetrics")
	sourceIssue(t, []byte("{\"snapshots\":[],\"snapshots\":[]}"), p.Options{}, "METRIC_JSON_INVALID", "/")
	// Explicitly constructed diagnostics also have a bounded printable message.
	untrusted := &p.SourceError{Code: secret, Path: secret, Rule: "\n" + secret}
	if len(untrusted.Error()) > 512 || strings.Contains(untrusted.Error(), "\n") {
		t.Fatal("unbounded public diagnostic")
	}
}
func TestR1304NestedFieldsRangeAndDeferredModelValidation(t *testing.T) {
	sourceIssue(t, gaugeSource(`{"asInt":"1","attributes":[{"key":"SECRET_LABEL","value":{"arrayValue":{"values":[{"intValue":"9223372036854775808"}]}}}]}`), p.Options{}, "METRIC_INTEGER_RANGE", "/resourceMetrics/0/scopeMetrics/0/metrics/0/gauge/dataPoints/0/attributes/0/value/arrayValue/values/0/intValue")
	sourceIssue(t, gaugeSource(`{"asInt":"1","startTimeUnixNano":"2","timeUnixNano":"1"}`), p.Options{}, "METRIC_TIME_INVALID", "/resourceMetrics/0/scopeMetrics/0/metrics/0/gauge/dataPoints/0/startTimeUnixNano")
	for _, tc := range []struct{ values, code, path string }{{`[[0,"1e-200"]]`, "METRIC_NUMBER_EXPONENT_BUDGET", "/data/result/0/values/0/1"}, {`[["SECRET","1"]]`, "METRIC_NUMBER_INVALID", "/data/result/0/values/0/0"}, {`[[0]]`, "METRIC_RANGE_PAIR_INVALID", "/data/result/0/values/0"}} {
		sourceIssue(t, []byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"__name__":"x"},"values":`+tc.values+`}]}}`), p.Options{Query: json.RawMessage(`{"query":"x"}`)}, tc.code, tc.path)
	}
	sourceIssue(t, []byte(`{"schema_version":"1.0.0","snapshots":[{"time_unix_nano":"1","metrics":[{"name":"ok","kind":"gauge","value":"1"}]},{"time_unix_nano":"2","metrics":[{"name":"x","kind":"sum","temporality":"cumulative","monotonic":true,"value":"-1"}]}]}`), p.Options{}, "METRIC_MONOTONIC_NEGATIVE", "/snapshots/1/metrics/0/value")
	sourceIssue(t, []byte(`{"schema_version":"1.0.0","snapshots":[{"time_unix_nano":"1","metrics":[{"name":"h","kind":"histogram","temporality":"delta","histogram":{"count":"1","sum":null,"min":null,"max":null,"bounds":["1"],"counts":["SECRET","1"]}}]}]}`), p.Options{}, "METRIC_INTEGER_INVALID", "/snapshots/0/metrics/0/histogram/counts/0")
	// Both sides fail; deterministic side order must always name positive first.
	exp := []byte(`{"resourceMetrics":[{"scopeMetrics":[{"metrics":[{"name":"h","exponentialHistogram":{"aggregationTemporality":2,"dataPoints":[{"count":"1","positive":{"offset":0,"bucketCounts":["SECRET"]},"negative":{"offset":0,"bucketCounts":["SECRET"]}}]}}]}]}]}`)
	for i := 0; i < 20; i++ {
		sourceIssue(t, exp, p.Options{}, "METRIC_NUMBER_INVALID", "/resourceMetrics/0/scopeMetrics/0/metrics/0/exponentialHistogram/dataPoints/0/positive/bucketCounts/0")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := p.ParseBytes(ctx, gaugeSource(`{"asInt":"1"}`), "x", p.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation became data error", err)
	}
}
