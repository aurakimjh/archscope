package profile_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	pp "github.com/google/pprof/profile"
)

const scopeHeader = `{"$schema":"https://www.speedscope.app/file-format-schema.json","exporter":"py-spy@0.4.1","shared":{"frames":[{"name":"root"},{"name":"leaf","file":"/synthetic/a.py","line":2}]},"profiles":`

func parse(t *testing.T, body, format string) m.ProfileSet {
	t.Helper()
	s, e := parser.ParseSourceBytes(context.Background(), []byte(body), parser.SourceOptions{Format: format})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func tree(t *testing.T, s m.ProfileSet, ids []string, metric string) m.Tree {
	t.Helper()
	v, e := m.BuildTree(context.Background(), s, m.Selection{ProfileIDs: ids, MetricID: metric})
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func TestSourceSampledFractionsZerosProfilesAndSavedRevision(t *testing.T) {
	body := scopeHeader + `[{"type":"sampled","name":"one","unit":"seconds","startValue":0,"endValue":1,"samples":[[0,1],[0]],"weights":[0.25,0]},{"type":"sampled","name":"two","unit":"seconds","startValue":0,"endValue":1,"samples":[[0]],"weights":[2.5e-1]},{"type":"sampled","name":"empty","unit":"seconds","startValue":0,"endValue":0,"samples":[],"weights":[]}]}`
	s := parse(t, body, "auto")
	if len(s.Profiles) != 3 || len(s.Profiles[0].Samples) != 2 || s.Profiles[0].Samples[1].Weight != "0" {
		t.Fatal(s.Profiles)
	}
	selection := m.Selection{ProfileIDs: []string{"profile-0", "profile-1"}, MetricID: "sampled-seconds"}
	v := tree(t, s, selection.ProfileIDs, selection.MetricID)
	if v.Root.Inclusive != "0.5" || v.SampleRecords != 3 {
		t.Fatal(v)
	}
	if s.Metrics[0].Kind != "duration" || s.Profiles[0].Clock != "aggregate" || s.Profiles[0].Samples[0].Timestamp != nil {
		t.Fatal("invented CPU time or sampled clock")
	}
	inv := parser.Inventory(s)
	if len(inv.Profiles) != 3 || len(inv.Profiles[2].Metrics) != 0 {
		t.Fatal(inv)
	}
	if _, e := analyzer.BuildSet(context.Background(), s, m.Selection{}, "original"); e == nil {
		t.Fatal("implicit selection")
	}
	result, e := analyzer.BuildSet(context.Background(), s, selection, "original")
	if e != nil {
		t.Fatal(e)
	}
	if result.Metadata.SchemaVersion != "0.4.0" {
		t.Fatal(result.Metadata)
	}
	if e = analyzer.ValidateModelResult(result); e != nil {
		t.Fatal(e)
	}
	result.Metadata.Extra["capture_policy"] = map[string]any{"sha256": "forged"}
	if e = analyzer.ValidateModelResult(result); e == nil {
		t.Fatal("forged policy accepted")
	}
}
func TestSourceEventedExclusiveIntervalsAndGaps(t *testing.T) {
	s := parse(t, scopeHeader+`[{"type":"evented","name":"worker","unit":"milliseconds","startValue":0,"endValue":10,"events":[{"type":"O","at":1,"frame":0},{"type":"O","at":3,"frame":1},{"type":"C","at":6,"frame":1},{"type":"C","at":9,"frame":0}]}]}`, "auto")
	v := tree(t, s, []string{"profile-0"}, "evented-milliseconds")
	if v.Root.Inclusive != "10" {
		t.Fatal(v)
	}
	totals := map[string]string{}
	for _, n := range v.Root.Children {
		totals[n.FrameID] = n.Inclusive
	}
	found := false
	for _, f := range s.Frames {
		if f.Symbol == "root" {
			if totals[f.ID] != "8" {
				t.Fatal(totals)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("root missing")
	}
	for _, sample := range s.Profiles[0].Samples {
		if sample.Timestamp == nil || sample.Interval == nil {
			t.Fatal("missing exact intervals")
		}
	}
}
func TestSourceRejectsMalformedAndCanceled(t *testing.T) {
	cases := []string{
		`[{"type":"sampled","unit":"seconds","startValue":0,"endValue":1,"samples":[[0]]}]`,
		`[{"type":"sampled","unit":"seconds","startValue":0,"endValue":1,"samples":[[0]],"weights":[-1]}]`,
		`[{"type":"sampled","unit":"none","startValue":0,"endValue":1,"samples":[[9]],"weights":[0]}]`,
		`[{"type":"evented","unit":"seconds","startValue":0,"endValue":1,"events":[{"type":"O","at":0,"frame":0}]}]`,
		`[{"type":"evented","unit":"seconds","startValue":0,"endValue":1,"events":[{"type":"C","at":0,"frame":0}]}]`,
		`[{"type":"evented","unit":"seconds","startValue":0,"endValue":1,"events":[{"type":"O","at":0},{"type":"C","at":1,"frame":0}]}]`,
		`[{"type":"evented","unit":"seconds","startValue":0,"endValue":1,"events":[{"type":"O","at":1,"frame":0},{"type":"C","at":0,"frame":0}]}]`,
		`[{"type":"sampled","unit":"seconds","startValue":0,"endValue":1,"samples":[[0]],"weights":[1e999]}]`,
	}
	for i, c := range cases {
		if _, e := parser.ParseSourceBytes(context.Background(), []byte(scopeHeader+c+"}"), parser.SourceOptions{}); e == nil {
			t.Fatalf("accepted malformed %d", i)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := parser.ParseSourceBytes(ctx, []byte("root 1\n"), parser.SourceOptions{Format: "pyspy-raw"}); e == nil {
		t.Fatal("ignored cancel")
	}
	for _, opts := range []parser.SourceOptions{{Format: "pyspy-raw", CaptureOptions: json.RawMessage(`{"token":"synthetic"}`)}, {Format: "auto"}} {
		if _, e := parser.ParseSourceBytes(context.Background(), []byte("root 1"), opts); e == nil {
			t.Fatal("accepted unsafe or ambiguous input")
		}
	}
	for _, opts := range []string{`{"format":"pprof","unknown":true}`, `{} {}`, `{"capture_options":null}`} {
		o, e := parser.DecodeSourceOptions([]byte(opts))
		if e == nil {
			_, e = parser.ParseSourceBytes(context.Background(), []byte("root 1"), o)
		}
		if e == nil {
			t.Fatal("invalid options", opts)
		}
	}
}
func TestSourceCollapsedExactAndNoClockInference(t *testing.T) {
	s := parse(t, "root;leaf 0.125\nroot 0\nroot;leaf 1e-3\n", "pyspy-raw")
	v := tree(t, s, []string{"profile-0"}, "samples")
	if v.Root.Inclusive != "0.126" || v.SampleRecords != 3 {
		t.Fatal(v)
	}
	if s.Metrics[0].Kind != "sample_count" || s.Profiles[0].Clock != "aggregate" {
		t.Fatal(s)
	}
}
func pprofBody(t *testing.T, p *pp.Profile) []byte {
	t.Helper()
	var b bytes.Buffer
	if e := p.Write(&b); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestSourcePprofEveryValueUnitLabelInlineAndPeriod(t *testing.T) {
	f := &pp.Function{ID: 1, Name: "root", Filename: "/synthetic/a.go"}
	g := &pp.Function{ID: 2, Name: "leaf"}
	loc := &pp.Location{ID: 1, Address: 123, Line: []pp.Line{{Function: g, Line: 3}, {Function: f, Line: 2}}}
	p := &pp.Profile{SampleType: []*pp.ValueType{{Type: "samples", Unit: "count"}, {Type: "cpu", Unit: "nanoseconds"}, {Type: "inuse_space", Unit: "bytes"}}, Function: []*pp.Function{f, g}, Location: []*pp.Location{loc}, Period: 100, PeriodType: &pp.ValueType{Type: "cpu", Unit: "nanoseconds"}, Sample: []*pp.Sample{{Location: []*pp.Location{loc}, Value: []int64{0, 9007199254740993, -2}, Label: map[string][]string{"worker": {"a"}, "tag": {"x", "y"}}, NumLabel: map[string][]int64{"bytes": {42}}, NumUnit: map[string][]string{"bytes": {"bytes"}}}, {Location: []*pp.Location{loc}, Value: []int64{1, 10, 5}, Label: map[string][]string{"worker": {"b"}}}}}
	s, e := parser.ParseSourceBytes(context.Background(), pprofBody(t, p), parser.SourceOptions{Format: "pprof"})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Profiles) != 2 || len(s.Metrics) != 3 || s.Metrics[2].Kind != "signed_difference" {
		t.Fatal(s)
	}
	if s.Profiles[0].Samples[1].Weight != "9007199254740993" || s.Profiles[0].Samples[0].Weight != "0" || s.Profiles[0].Samples[0].Labels["pprof.numeric.bytes"][0] != "42" {
		t.Fatal("lost exact values/labels")
	}
	st := s.Stacks[0]
	if st.Frames[0] != s.Frames[0].ID || s.Frames[0].Symbol != "root" || !s.Frames[1].Inline {
		t.Fatal("wrong inline order")
	}
	c, _, _ := s.SourceContract()
	if !strings.Contains(string(c.Attributes["pprof"]), `"period":"100"`) || !strings.Contains(string(c.Attributes["pprof"]), `"Address":123`) {
		t.Fatal(string(c.Attributes["pprof"]))
	}
	v := tree(t, s, []string{s.Profiles[0].ID, s.Profiles[1].ID}, "pprof-1")
	if v.Root.Inclusive != "9007199254741003" {
		t.Fatal(v.Root.Inclusive)
	}
}
func TestActualProducerFixturesPinnedAndExact(t *testing.T) {
	var manifest struct {
		Files []struct {
			File   string `json:"file"`
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	body, e := os.ReadFile("testdata/source-producers/manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(body, &manifest); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"pyspy-speedscope.json", "pyspy.raw", "go-heap.json"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/source-producers/" + name)
			if e != nil {
				t.Fatal(e)
			}
			sum := sha256.Sum256(raw)
			want := ""
			for _, file := range manifest.Files {
				if file.File == name {
					want = file.SHA256
				}
			}
			if hex.EncodeToString(sum[:]) != want {
				t.Fatal("fixture drift")
			}
			options := parser.SourceOptions{Producer: "py-spy", ProducerVersion: "0.4.1", Format: "pyspy-raw"}
			if name == "pyspy-speedscope.json" {
				options.Format = "speedscope-json"
			}
			if name == "go-heap.json" {
				var wrapper struct {
					Data    string `json:"data"`
					Version string `json:"producer_version"`
				}
				if e = json.Unmarshal(raw, &wrapper); e != nil {
					t.Fatal(e)
				}
				raw, e = base64.StdEncoding.DecodeString(wrapper.Data)
				if e != nil {
					t.Fatal(e)
				}
				options = parser.SourceOptions{Producer: "go.runtime/pprof", ProducerVersion: wrapper.Version, Format: "pprof"}
			}
			s, e := parser.ParseSourceBytes(context.Background(), raw, options)
			if e != nil {
				t.Fatal(e)
			}
			if name == "pyspy-speedscope.json" && len(s.Profiles) != 3 {
				t.Fatal(len(s.Profiles))
			}
			if name == "go-heap.json" && len(s.Metrics) != 4 {
				t.Fatal(s.Metrics)
			}
			for _, metric := range s.Metrics {
				ids := []string{}
				for _, profile := range parser.Inventory(s).Profiles {
					for _, id := range profile.Metrics {
						if id == metric.ID {
							ids = append(ids, profile.ID)
						}
					}
				}
				if len(ids) == 0 {
					continue
				}
				v := tree(t, s, ids, metric.ID)

				if name == "go-heap.json" {
					original, err := pp.ParseData(raw)
					if err != nil {
						t.Fatal(err)
					}
					column := -1
					for j, m := range s.Metrics {
						if m.ID == metric.ID {
							column = j
						}
					}
					expected := new(big.Int)
					for _, row := range original.Sample {
						expected.Add(expected, big.NewInt(row.Value[column]))
					}
					if v.Root.Inclusive != expected.String() {
						t.Fatal("pprof producer total changed", metric.ID, v.Root.Inclusive, expected)
					}
				} else if name == "pyspy-speedscope.json" {
					var original struct {
						Profiles []struct {
							Weights []json.Number `json:"weights"`
						} `json:"profiles"`
					}
					if err := json.Unmarshal(raw, &original); err != nil {
						t.Fatal(err)
					}
					expected := new(big.Rat)
					for _, p := range original.Profiles {
						for _, w := range p.Weights {
							n, ok := new(big.Rat).SetString(w.String())
							if !ok {
								t.Fatal(w)
							}
							expected.Add(expected, n)
						}
					}
					if v.Root.Inclusive != m.Text(expected) {
						t.Fatal("Speedscope producer total changed", v.Root.Inclusive, expected)
					}
				}
				if v.SampleRecords == 0 {
					t.Fatal("empty producer capture")
				}
				if s.CapturePolicy(metric.ID) == nil {
					t.Fatal("pinned producer policy unavailable")
				}
				if _, e := analyzer.BuildSet(context.Background(), s, m.Selection{ProfileIDs: ids, MetricID: metric.ID}, name); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}

func TestSourceBudgetsRejectInsteadOfTruncating(t *testing.T) {
	// Both decoded and original ceilings reject; a highly compressed source must
	// not turn into an apparently successful prefix parse.
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	block := bytes.Repeat([]byte(" "), 1<<20)
	for i := 0; i < 65; i++ {
		if _, e := z.Write(block); e != nil {
			t.Fatal(e)
		}
	}
	if e := z.Close(); e != nil {
		t.Fatal(e)
	}
	if _, e := parser.ParseSourceBytes(context.Background(), compressed.Bytes(), parser.SourceOptions{Format: "pyspy-raw"}); e == nil || !strings.Contains(e.Error(), "IMPORT_BUDGET_EXCEEDED") {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "oversize.raw")
	f, e := os.Create(file)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(parser.SourceMaxBytes + 1); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if _, e = parser.ParseSourceFile(context.Background(), file, parser.SourceOptions{Format: "pyspy-raw"}); e == nil {
		t.Fatal("oversize original accepted")
	}
	if _, e = parser.ParseSourceBytes(context.Background(), []byte(strings.Repeat("root 0\n", m.MaxSamples+1)), parser.SourceOptions{Format: "pyspy-raw"}); e == nil || !strings.Contains(e.Error(), "IMPORT_BUDGET_EXCEEDED") {
		t.Fatal("observation budget did not refuse", e)
	}
}

func TestR1101NoPolicyOriginalTypeUnitCannotCompare(t *testing.T) {
	makeSet := func(kind string) m.ProfileSet {
		f := &pp.Function{ID: 1, Name: "root"}
		loc := &pp.Location{ID: 1, Line: []pp.Line{{Function: f}}}
		p := &pp.Profile{SampleType: []*pp.ValueType{{Type: kind, Unit: "bytes"}}, Function: []*pp.Function{f}, Location: []*pp.Location{loc}, Sample: []*pp.Sample{{Location: []*pp.Location{loc}, Value: []int64{42}}}}
		s, e := parser.ParseSourceBytes(context.Background(), pprofBody(t, p), parser.SourceOptions{Format: "pprof"})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a, b := makeSet("alloc_space"), makeSet("inuse_space")
	sel := m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "pprof-0"}
	if a.CapturePolicy(sel.MetricID) != nil || b.CapturePolicy(sel.MetricID) != nil {
		t.Fatal("unexpected producer policy")
	}
	if a.Metrics[0].SourceType != "alloc_space" || parser.Inventory(b).Metrics[0].SourceType != "inuse_space" {
		t.Fatal("hidden original sample type")
	}
	if a.StrictComparisonIdentity(sel) == b.StrictComparisonIdentity(sel) || a.ComparisonIdentity(sel) == b.ComparisonIdentity(sel) {
		t.Fatal("source type absent from comparison identity")
	}
	if _, e := m.Compare(context.Background(), a, sel, b, sel); e == nil {
		t.Fatal("alloc_space compared with inuse_space")
	}
	result, e := analyzer.BuildSet(context.Background(), a, sel, "original")
	if e != nil || result.Metadata.SchemaVersion != "0.4.0" || result.Summary["source_type"] != "alloc_space" || result.Summary["source_unit"] != "bytes" {
		t.Fatal(result.Summary, e)
	}
}
func TestR1104UnitlessWeightIsNotProvenSampleCount(t *testing.T) {
	s := parse(t, scopeHeader+`[{"type":"sampled","unit":"none","startValue":0,"endValue":1,"samples":[[0]],"weights":[0.25]}]}`, "auto")
	if s.Metrics[0].Kind != "count" || s.Metrics[0].SourceUnit != "none" {
		t.Fatal(s.Metrics)
	}
	result, e := analyzer.BuildSet(context.Background(), s, m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: s.Metrics[0].ID}, "none")
	if e != nil || result.Summary["total_samples"] != nil {
		t.Fatal(result.Summary, e)
	}
}
