package gputrace_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gputrace"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	prefix := "gpu-execution-"
	if strings.HasPrefix(name, "gpu-execution-set") {
		prefix = ""
	}
	b, e := os.ReadFile("../../catalog/contracts/" + prefix + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestActualProducerFixturesAndNoRawContent(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		stem, input       string
		events, summaries int
	}{{"pytorch-rank-0", "pytorch-rank-0.json", 22, 0}, {"pytorch-rank-1", "pytorch-rank-1.json", 22, 0}, {"nsight", "nsight-sanitized.sqlite", 19, 0}, {"nsight-summary", "nsight-summary.csv", 0, 400}} {
		t.Run(c.stem, func(t *testing.T) {
			o, e := p.DecodeOptions(fixture(t, c.stem+"-options.json"))
			if e != nil {
				t.Fatal(e)
			}
			raw := fixture(t, c.input)
			s, e := p.ParseBytes(ctx, raw, o)
			if e != nil {
				t.Fatal(e)
			}
			if len(s.Events) != c.events || len(s.Summaries) != c.summaries {
				t.Fatal(len(s.Events), len(s.Summaries))
			}
			b, _ := json.Marshal(s)
			want, e := g.Decode(ctx, fixture(t, c.stem+"-set.json"))
			if e != nil {
				t.Fatal(e)
			}
			expected, _ := json.Marshal(want)
			if !bytes.Equal(b, expected) {
				t.Fatal("normalization drift")
			}
			for _, term := range []string{"host_name", "traceName", "Input Dims", "Call stack", "CUDA_VISIBLE_DEVICES"} {
				if bytes.Contains(b, []byte(`"`+term+`":`)) {
					t.Fatal("arbitrary field retained", term)
				}
			}
			projection, e := a.Project(ctx, s)
			if e != nil {
				t.Fatal(e)
			}
			if c.events == 0 && len(projection.Busy) != 0 {
				t.Fatal("summary became timing")
			}
			if c.stem == "nsight" && projection.Busy[0].KernelCount != "7" {
				t.Fatal(projection.Busy)
			}
			var compressed bytes.Buffer
			z := gzip.NewWriter(&compressed)
			z.Write(raw)
			z.Close()
			gzipSet, e := p.ParseBytes(ctx, compressed.Bytes(), o)
			if e != nil || gzipSet.Sources[0].SHA != p.Digest(compressed.Bytes()) {
				t.Fatal("gzip original hash", e)
			}
		})
	}
}
func TestSourceRefusalsCancellationAndExactDecimal(t *testing.T) {
	ctx := context.Background()
	raw := fixture(t, "pytorch-rank-0.json")
	o, _ := p.DecodeOptions(fixture(t, "pytorch-rank-0-options.json"))
	bad := o
	bad.Host = ""
	if _, e := p.ParseBytes(ctx, raw, bad); e == nil {
		t.Fatal("identity absent")
	}
	bad = o
	bad.ProducerVersion = "2.14.0"
	if _, e := p.ParseBytes(ctx, raw, bad); e == nil {
		t.Fatal("false qualification")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := p.ParseBytes(cancelled, raw, o); e == nil {
		t.Fatal("cancel ignored")
	}
	if _, e := p.DecodeOptions([]byte(`{"format":"x","format":"y"}`)); e == nil {
		t.Fatal("duplicate key")
	}
	for _, sample := range []string{`{"schemaVersion":1,"traceEvents":[{"ph":"X","cat":"cpu_op","name":"x","pid":1,"tid":1,"ts":1,"dur":0.0001}]}`, `{"schemaVersion":1,"traceEvents":[{"ph":"B","cat":"cpu_op","name":"x","pid":1,"tid":1,"ts":1}]}`} {
		if _, e := p.ParseBytes(ctx, []byte(sample), o); e == nil {
			t.Fatal("subnanosecond/unsupported phase")
		}
	}
	exact := []byte(`{"schemaVersion":1,"traceEvents":[{"ph":"X","cat":"cpu_op","name":"x","pid":1,"tid":1,"ts":9007199254740.993,"dur":0.001,"args":{"prompt":"SECRET_EXCLUDED"}}]}`)
	s, e := p.ParseBytes(ctx, exact, o)
	if e != nil || s.Events[0].From != "9007199254740993" || s.Events[0].To != "9007199254740994" {
		t.Fatal(s, e)
	}
	b, _ := json.Marshal(s)
	if bytes.Contains(b, []byte("SECRET_EXCLUDED")) {
		t.Fatal("args retained")
	}
	bad = o
	v := "3"
	bad.From = &v
	if _, e = p.ParseBytes(ctx, raw, bad); e == nil {
		t.Fatal("half time range")
	}
}
func TestSQLiteRejectsViewsSchemaSpoofAndSelectsWholeIntervals(t *testing.T) {
	ctx := context.Background()
	raw := fixture(t, "nsight-sanitized.sqlite")
	o, _ := p.DecodeOptions(fixture(t, "nsight-options.json"))
	dir := t.TempDir()
	path := filepath.Join(dir, "file.sqlite")
	os.WriteFile(path, raw, 0600)
	db, e := sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("UPDATE META_DATA_EXPORT SET value='3.12' WHERE name='EXPORT_SCHEMA_VERSION'"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	changed, _ := os.ReadFile(path)
	if _, e = p.ParseBytes(ctx, changed, o); e == nil {
		t.Fatal("schema version spoof")
	}
	os.WriteFile(path, raw, 0600)
	db, e = sql.Open("sqlite", path)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("ALTER TABLE StringIds RENAME TO originalStrings; CREATE VIEW StringIds AS SELECT * FROM originalStrings"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	changed, _ = os.ReadFile(path)
	if _, e = p.ParseBytes(ctx, changed, o); e == nil {
		t.Fatal("view executed")
	}
	from, to := "80380000", "80381000"
	o.From = &from
	o.To = &to
	s, e := p.ParseBytes(ctx, raw, o)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, v := range s.Events {
		if v.Kind == "gpu_kernel" {
			found = true
			if g.Cmp(v.From, from) >= 0 || g.Cmp(v.To, to) <= 0 {
				t.Fatal("interval clipped", v)
			}
		}
	}
	if !found {
		t.Fatal("crossing interval omitted")
	}
	missing := o
	missing.DeviceMap = nil
	if _, e = p.ParseBytes(ctx, raw, missing); e == nil {
		t.Fatal("device ordinal joined without map")
	}
}

func TestCallerCaptureOwnership(t *testing.T) {
	raw := fixture(t, "nsight-sanitized.sqlite")
	o, _ := p.DecodeOptions(fixture(t, "nsight-options.json"))
	from, to := "80000000", "80500000"
	o.From = &from
	o.To = &to
	set, err := p.ParseBytes(context.Background(), raw, o)
	if err != nil {
		t.Fatal(err)
	}
	o.DeviceMap["0"] = "mutated"
	from = "0"
	if err = set.Validate(context.Background()); err != nil {
		t.Fatal("caller changed stored scope", err)
	}
}

func TestStructuralTorchCUDAFlowScopeWithoutFalseProcessJoin(t *testing.T) {
	o, _ := p.DecodeOptions(fixture(t, "pytorch-rank-0-options.json"))
	o.DeviceMap = map[string]string{"0": "declared-gpu"}
	raw := []byte(`{"schemaVersion":1,"traceEvents":[{"ph":"X","cat":"cuda_runtime","name":"cudaLaunchKernel","pid":123,"tid":1,"ts":1,"dur":2,"args":{"correlation":7}},{"ph":"X","cat":"kernel","name":"nccl_symbol_is_not_rank_proof","pid":0,"tid":7,"ts":2,"dur":3,"args":{"correlation":7,"device":0,"stream":7,"context":1}},{"ph":"s","cat":"ac2g","name":"flow","pid":123,"tid":1,"ts":1,"id2":{"global":7}},{"ph":"f","cat":"ac2g","name":"flow","pid":0,"tid":7,"ts":3,"id2":{"global":7}}]}`)
	set, err := p.ParseBytes(context.Background(), raw, o)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := a.Project(context.Background(), set)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]bool{}
	for _, r := range projection.Relations {
		statuses[r.Status] = true
	}
	if !statuses["unresolved_launch"] || !statuses["explicit_flow_endpoints"] {
		t.Fatal(projection.Relations)
	}
	for _, event := range set.Events {
		if event.Name == "nccl_symbol_is_not_rank_proof" && event.Kind != "gpu_kernel" {
			t.Fatal("name inferred collective rank")
		}
	}
}
