package profilemodel_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	pp "github.com/google/pprof/profile"
	"os"
	"strings"
	"testing"
)

func sourcePolicySet(t *testing.T, options string) m.ProfileSet {
	t.Helper()
	s, e := parser.ParseSourceBytes(context.Background(), []byte("root;leaf 0.25\nroot 0\n"), parser.SourceOptions{Format: "pyspy-raw", Producer: "py-spy", ProducerVersion: "0.4.1", CaptureOptions: json.RawMessage(options)})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestCompiledCapturePolicyStrictUnknownPresenceAndDisclosures(t *testing.T) {
	sel := m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "samples"}
	a := sourcePolicySet(t, `{"rate":100,"output_path":"/synthetic/a"}`)
	b := sourcePolicySet(t, `{"rate":100,"output_path":"/synthetic/b"}`)
	d, e := m.Compare(context.Background(), a, sel, b, sel)
	if e != nil {
		t.Fatal(e)
	}
	if len(d.IncidentalDifferences) != 1 || d.IncidentalDifferences[0] != "output_path" || len(d.Warnings) != 1 || d.IdentityBasis != "declared" || d.Baseline.CapturePolicy == nil || d.Target.CapturePolicy == nil {
		t.Fatal(d)
	}
	if string(d.Baseline.CapturePolicy.Original) != string(a.CaptureOptions) || string(d.Target.CapturePolicy.Original) != string(b.CaptureOptions) {
		t.Fatal("original options lost")
	}
	if a.ComparisonIdentity(sel) != b.ComparisonIdentity(sel) || a.StrictComparisonIdentity(sel) == b.StrictComparisonIdentity(sel) {
		t.Fatal("policy projection/strict identity mismatch")
	}
	for _, pair := range [][2]string{
		{`{"rate":100}`, `{"rate":101}`}, {`{}`, `{"rate":null}`}, {`{"rate":null}`, `{"rate":0}`}, {`{}`, `{"unknown":null}`}, {`{"unknown":0}`, `{"unknown":1}`}, {`{"ignore_keys":["rate"],"rate":100}`, `{"ignore_keys":["rate"],"rate":200}`}, {`{"duration":1}`, `{"duration":2}`},
	} {
		left, right := sourcePolicySet(t, pair[0]), sourcePolicySet(t, pair[1])
		if _, e = m.Compare(context.Background(), left, sel, right, sel); e == nil || !strings.Contains(e.Error(), "capture_options_mismatch") {
			t.Fatal(pair, e)
		}
	}
	left, right := sourcePolicySet(t, `{}`), sourcePolicySet(t, `{"output_path":null}`)
	d, e = m.Compare(context.Background(), left, sel, right, sel)
	if e != nil || len(d.IncidentalDifferences) != 1 {
		t.Fatal("missing/null incidental disclosure", d, e)
	}
	left, right = sourcePolicySet(t, `{"pid":1}`), sourcePolicySet(t, `{"pid":2}`)
	if _, e = m.Compare(context.Background(), left, sel, right, sel); e == nil || !strings.Contains(e.Error(), "entity_mapping_required") {
		t.Fatal("policy bypassed entity gate", e)
	}
	for _, change := range []string{"version", "unknown-producer", "metric", "normalizer"} {
		right = copySet(t, a)
		switch change {
		case "version":
			right.ProducerVersion = "0.4.2"
		case "unknown-producer":
			right.Producer = "unknown"
		case "metric":
			right.Metrics[0].Scale = "2"
		case "normalizer":
			delete(right.Extensions, m.SourceExtension)
		}
		if right.CapturePolicy(sel.MetricID) != nil {
			t.Fatal("unqualified policy", change)
		}
		if _, e = m.Compare(context.Background(), a, sel, right, sel); e == nil {
			t.Fatal("mismatched policy accepted", change)
		}
	}
	// Unknown versions and old models retain opaque strict capture comparisons.
	left, right = sourcePolicySet(t, `{"output_path":"a"}`), sourcePolicySet(t, `{"output_path":"b"}`)
	left.ProducerVersion = "future"
	right.ProducerVersion = "future"
	if _, e = m.Compare(context.Background(), left, sel, right, sel); e == nil {
		t.Fatal("unknown producer relaxed")
	}
	old, oldSelection := fixture(t)
	if old.ComparisonIdentity(oldSelection) != old.StrictComparisonIdentity(oldSelection) {
		t.Fatal("old v2 identity relabeled")
	}
}
func TestSourceExactExportTicksAndRefusal(t *testing.T) {
	s := sourcePolicySet(t, `{}`)
	sel := m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "samples"}
	if _, e := m.ExportPprof(context.Background(), s, sel, "1"); e == nil {
		t.Fatal("rounded fraction")
	}
	body, e := m.ExportPprof(context.Background(), s, sel, "0.01")
	if e != nil {
		t.Fatal(e)
	}
	p, e := pp.ParseData(body)
	if e != nil {
		t.Fatal(e)
	}
	sum := int64(0)
	for _, sample := range p.Sample {
		sum += sample.Value[0]
	}
	if sum != 25 || p.SampleType[0].Unit != "ticks" || !strings.Contains(p.Comments[0], `"tick_scale":"0.01"`) {
		t.Fatal(p)
	}
}

func TestQualifiedHeapDurationAndExactCaptureIdentity(t *testing.T) {
	body, e := os.ReadFile("../parsers/profile/testdata/source-producers/go-heap.json")
	if e != nil {
		t.Fatal(e)
	}
	var wrapper struct {
		Data string `json:"data"`
	}
	if e = json.Unmarshal(body, &wrapper); e != nil {
		t.Fatal(e)
	}
	raw, e := base64.StdEncoding.DecodeString(wrapper.Data)
	if e != nil {
		t.Fatal(e)
	}
	parse := func(options string) m.ProfileSet {
		s, e := parser.ParseSourceBytes(context.Background(), raw, parser.SourceOptions{Format: "pprof", Producer: "go.runtime/pprof", ProducerVersion: "go1.26.10", CaptureOptions: json.RawMessage(options)})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a, b := parse(`{"duration":1}`), parse(`{"duration":2}`)
	for _, metric := range []string{"pprof-0", "pprof-1", "pprof-2", "pprof-3"} {
		sel := m.Selection{ProfileIDs: []string{a.Profiles[0].ID}, MetricID: metric}
		d, e := m.Compare(context.Background(), a, sel, b, sel)
		if metric == "pprof-0" || metric == "pprof-1" {
			if e == nil {
				t.Fatal("alloc duration relaxed")
			}
		} else if e != nil || len(d.IncidentalDifferences) != 1 || d.IncidentalDifferences[0] != "duration" {
			t.Fatal("inuse duration disclosure", d, e)
		}
	}
	// Hash exact unknown values without converting them to JavaScript/float64.
	left, right := sourcePolicySet(t, `{"unknown":9007199254740992}`), sourcePolicySet(t, `{"unknown":9007199254740993}`)
	sel := m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "samples"}
	if left.ComparisonIdentity(sel) == right.ComparisonIdentity(sel) {
		t.Fatal("capture integer identity rounded")
	}
}
