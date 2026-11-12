package systemtrace_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/systemtrace"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"os"
	"strings"
	"testing"
)

func load(t *testing.T, file, format, capture string) m.Set {
	t.Helper()
	raw, e := os.ReadFile("testdata/" + capture)
	if e != nil {
		t.Fatal(e)
	}
	var c m.Capture
	if e = m.Strict(raw, &c, 128<<10); e != nil {
		t.Fatal(e)
	}
	s, e := p.ParseFile(context.Background(), "testdata/"+file, p.Options{Format: format, Capture: c})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestActualPerf6190ExportPreservesNanosecondsAndPeriod(t *testing.T) {
	s := load(t, "perf-script.txt", "perf-script", "perf-capture.json")
	if len(s.Records) != 3 || *s.Records[0].Time != "1000000001" || *s.Records[1].Period != "2000000" || s.Records[0].Stack[0].Address != "400100" || s.Records[0].Fields["comm"] != "synthetic" {
		t.Fatal(s)
	}
	body, _ := os.ReadFile("testdata/perf-script.txt")
	if s.SourceSHA256 != m.Digest(body) {
		t.Fatal("source digest")
	}
	crlf := bytes.ReplaceAll(body, []byte("\n"), []byte("\r\n"))
	copy, e := p.ParseBytes(context.Background(), crlf, p.Options{Format: s.Format, Capture: s.Capture})
	if e != nil {
		t.Fatal(e)
	}
	if copy.Records[1].Byte != int64(bytes.Index(crlf, []byte("synthetic   123/124   [001]"))) {
		t.Fatal("CRLF byte coordinate", copy.Records[1].Byte)
	}
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	z.Write(body)
	z.Close()
	got, e := p.ParseBytes(context.Background(), compressed.Bytes(), p.Options{Format: s.Format, Capture: s.Capture})
	if e != nil || got.SourceSHA256 != m.Digest(compressed.Bytes()) {
		t.Fatal("compressed source identity", e)
	}
}
func TestBpftrace022InclusiveBucketsAndOpenOverflow(t *testing.T) {
	s := load(t, "bpftrace-hist.jsonl", "bpftrace-jsonl", "bpftrace-capture.json")
	if len(s.Records) != 1 || len(s.Records[0].Histogram.Buckets) != 10 || *s.Records[0].Histogram.Buckets[0].Upper != "3" || s.Records[0].Histogram.Buckets[1].Count != "0" {
		t.Fatal(s)
	}
	raw := []byte(`{"type":"hist","data":{"@latency":[{"max":-1,"count":1},{"min":0,"max":9,"count":9007199254740993},{"min":10,"count":1}]}}`)
	got, e := p.ParseBytes(context.Background(), raw, p.Options{Format: s.Format, Capture: s.Capture})
	if e != nil {
		t.Fatal(e)
	}
	b := got.Records[0].Histogram.Buckets
	if b[0].Lower != nil || b[2].Upper != nil || b[1].Count != "9007199254740993" {
		t.Fatal(b)
	}
}
func TestConstrainedSourcesRejectGuessesWithCoordinates(t *testing.T) {
	s := load(t, "perf-script.txt", "perf-script", "perf-capture.json")
	original, _ := os.ReadFile("testdata/perf-script.txt")
	for _, bad := range [][]byte{bytes.Replace(original, []byte("1.000000001"), []byte("1.000"), 1), []byte("PERFILE2"), bytes.Replace(original, []byte("cpu-clock:HG"), []byte("unannounced-event"), 1), bytes.Replace(original, []byte("123/124"), []byte("123/999"), 1)} {
		_, e := p.ParseBytes(context.Background(), bad, p.Options{Format: s.Format, Capture: s.Capture})
		if e == nil || !strings.Contains(e.Error(), "line ") {
			t.Fatal("unqualified perf shape accepted", e)
		}
	}
	bpf := load(t, "bpftrace-hist.jsonl", "bpftrace-jsonl", "bpftrace-capture.json")
	for _, bad := range []string{`{"type":"map","data":{"@latency":4}}`, `{"type":"hist","data":{"@unknown":[{"min":0,"max":1,"count":1}]}}`, `{"type":"hist","data":{"@latency":[{"min":0,"max":10,"count":1},{"min":10,"max":20,"count":1}]}}`, `{"type":"hist","data":{"@latency":[{"min":0,"max":1,"count":1.5}]}}`, `{"type":"hist","type":"hist","data":{}}`} {
		if _, e := p.ParseBytes(context.Background(), []byte(bad), p.Options{Format: bpf.Format, Capture: bpf.Capture}); e == nil {
			t.Fatal("invalid BPF accepted", bad)
		}
	}
	bpf.Capture.ScriptSHA256 = ""
	if _, e := p.ParseBytes(context.Background(), []byte(`{}`), p.Options{Format: bpf.Format, Capture: bpf.Capture}); e == nil {
		t.Fatal("script semantics guessed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := p.ParseBytes(ctx, original, p.Options{Format: s.Format, Capture: s.Capture}); e == nil {
		t.Fatal("cancellation ignored")
	}
	if _, e := p.DecodeOptions([]byte(`{"format":"perf-script","execute":"dangerous"}`)); e == nil {
		t.Fatal("executable option accepted")
	}
	var doc map[string]any
	raw, _ := json.Marshal(s)
	json.Unmarshal(raw, &doc)
	doc["unexpected"] = true
	raw, _ = json.Marshal(doc)
	if _, e := m.Decode(context.Background(), raw); e == nil {
		t.Fatal("unknown model field accepted")
	}
}

func TestConstrainedPerfSchedulerTextUsesTraceFieldContract(t *testing.T) {
	s := load(t, "events.jsonl", "events-jsonl", "events-capture.json")
	s.Capture.Tool = "perf"
	s.Capture.ToolVersion = "6.1.190"
	s.Capture.Fields = m.PerfSchedulerFields
	s.Capture.ScriptSHA256 = ""
	source := []byte("synthetic 123/124 [000] 1.000000001: 1 sched:sched_switch: prev_comm=synthetic prev_pid=124 prev_prio=120 prev_state=S ==> next_comm=other next_pid=126 next_prio=120\nother 125/126 [001] 1.000000101: 1 sched:sched_wakeup: comm=synthetic pid=124 prio=120 target_cpu=000\nother 125/126 [001] 1.000000201: 1 sched:sched_switch: prev_comm=other prev_pid=126 prev_prio=120 prev_state=R+ ==> next_comm=synthetic next_pid=124 next_prio=120\n")
	got, e := p.ParseBytes(context.Background(), source, p.Options{Format: "perf-script", Capture: s.Capture})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Records) != 5 || got.Records[0].State != "sleeping" || got.Records[3].State != "runnable" || got.Records[2].Entity != "worker-generation-a" {
		t.Fatal(got)
	}
	s.Capture.Fields = m.PerfFields
	if _, e = p.ParseBytes(context.Background(), source, p.Options{Format: "perf-script", Capture: s.Capture}); e == nil {
		t.Fatal("trace data accepted under sample field contract")
	}
}
