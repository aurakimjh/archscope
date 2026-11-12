package inference_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func fixture(t *testing.T) n.Set {
	t.Helper()
	raw, e := os.ReadFile("../../parsers/inference/testdata/inference-evidence-set.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := n.Decode(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestDeviceServingPrecisionResetAndIndependentScopes(t *testing.T) {
	s := fixture(t)
	rows, e := a.Observations(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	signals := map[string]n.Signal{}
	for _, v := range s.Signals {
		signals[v.ID] = v
	}
	tokens := []a.Observation{}
	statuses := map[string]int{}
	converted := false
	hist := false
	for _, r := range rows {
		v := signals[r.SignalID]
		statuses[strings.Split(r.Status, ":")[0]]++
		if v.Semantic == "output_tokens" {
			tokens = append(tokens, r)
		}
		if v.Semantic == "device_memory_used" && r.Raw != nil && *r.Raw == "8192" {
			if r.Value == nil || *r.Value != "8589934592" || r.Unit != "bytes" {
				t.Fatal(r)
			}
			converted = true
		}
		if v.Semantic == "ttft" && r.HistogramCount != nil && *r.HistogramCount == "2" {
			if r.Delta == nil || *r.Delta != "1" || r.Rate == nil || *r.Rate != "1" || r.RateUnit != "observations/second" || r.DeltaUnit != "observations" {
				t.Fatal(r)
			}
			hist = true
		}
	}
	if !converted || !hist || len(tokens) != 3 || *tokens[0].Raw != "9007199254740993" || tokens[0].Rate != nil || *tokens[1].Rate != "10" || tokens[1].RateUnit != "tokens/second" || tokens[2].Status != "reset" || !tokens[2].Gap || *tokens[2].Rate != "2" {
		t.Fatal(tokens, converted, hist)
	}
	for _, status := range []string{"not_available", "not_supported", "permission_denied"} {
		if statuses[status] != 1 {
			t.Fatal(status, statuses)
		}
	}
	result, e := a.Build(context.Background(), s, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(result); e != nil {
		t.Fatal(e)
	}
	if result.Summary["unmapped_series_count"] != "1" {
		t.Fatal(result.Summary)
	}
	requests := result.Tables["requests"].([]a.Request)
	if len(requests) != 3 || *requests[0].TTFT != "200000000" || *requests[0].TPOT != "300000000" || requests[1].TPOT != nil || requests[1].TPOTStatus != "at_least_two_output_tokens_required" || requests[2].Outcome != "cancelled" || requests[2].TTFT != nil {
		t.Fatal(requests)
	}
	// Saved projections cannot be edited independently of their one retained model.
	result.Tables["observations"] = []a.Observation{}
	if e = a.Validate(result); e == nil {
		t.Fatal("forged projection accepted")
	}
}
func TestExplicitIdentityUnitsFormsAndBodyRefusals(t *testing.T) {
	cases := map[string]func(*n.Set){
		"device attributed to worker": func(s *n.Set) { s.Signals[0].EntityID = "synthetic-worker-0" },
		"name based unit guess":       func(s *n.Set) { s.Signals[0].Conversion = "identity" },
		"pod replication":             func(s *n.Set) { v := s.Signals[0]; v.ID = "another-pod"; s.Signals = append(s.Signals, v) },
		"cross host link":             func(s *n.Set) { s.Entities[0].Host = "other-host" },
		"MIG missing parent":          func(s *n.Set) { s.Links = s.Links[1:] },
		"process reused lifetime": func(s *n.Set) {
			for i := range s.Entities {
				if s.Entities[i].Kind == "process" {
					s.Entities[i].ProcessStart = nil
				}
			}
		},
		"local index only": func(s *n.Set) { s.Entities[0].DeviceUUID = ""; s.Entities[0].LocalIndex = "0" },
		"counter as queue gauge": func(s *n.Set) {
			for i := range s.Signals {
				if s.Signals[i].Semantic == "output_tokens" {
					s.Signals[i].Semantic = "queue_requests"
				}
			}
		},
		"invalid warmup": func(s *n.Set) { s.Contexts[0].Warmup = "assume-ready" },
	}
	for name, edit := range cases {
		t.Run(name, func(t *testing.T) {
			s := fixture(t)
			edit(&s)
			if e := s.Validate(context.Background()); e == nil {
				t.Fatal("invalid contract accepted")
			}
		})
	}
	raw, _ := json.Marshal(fixture(t))
	raw = []byte(strings.Replace(string(raw), `"schema_version":"1.0.0"`, `"prompt":"private","schema_version":"1.0.0"`, 1))
	if _, e := n.Decode(context.Background(), raw); e == nil {
		t.Fatal("unknown body field accepted")
	}
	s := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := a.Build(ctx, s, ""); e == nil {
		t.Fatal("canceled build")
	}
}
func TestMissingAndInvalidObservationsDoNotBecomeZeros(t *testing.T) {
	s := fixture(t)
	target := s.Signals[0].SeriesID
	for i := range s.Metrics.Points {
		if s.Metrics.Points[i].SeriesID == target {
			s.Metrics.Points[i].Value = m.Pointer("101")
			s.Metrics.Points[i].ID = m.PointID(s.Metrics.Points[i])
		}
	}
	rows, e := a.Observations(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	invalid := 0
	for _, v := range rows {
		if v.SignalID == s.Signals[0].ID {
			if v.Value != nil || v.Status != "physical_value_out_of_range" || v.Raw == nil || *v.Raw != "101" {
				t.Fatal(v)
			}
			invalid++
		}
	}
	if invalid != 2 {
		t.Fatal(invalid)
	}
}
func TestT63910k20kSavedMetricReferenceScale(t *testing.T) {
	for _, count := range []int{10000, 20000} {
		s := fixture(t)
		var signal n.Signal
		for _, v := range s.Signals {
			if v.Semantic == "queue_requests" {
				signal = v
			}
		}
		var series m.Series
		for _, v := range s.Metrics.Series {
			if v.ID == signal.SeriesID {
				series = v
			}
		}
		s.Signals = []n.Signal{signal}
		s.Metrics.Series = []m.Series{series}
		s.Metrics.Points = []m.Point{}
		s.Quality = nil
		s.Events = nil
		for i := 0; i < count; i++ {
			p := m.Point{SeriesID: series.ID, End: m.Pointer(strconv.FormatInt(int64(i)*1000000, 10)), Value: m.Pointer("2"), Refs: []m.Ref{{SourceID: s.Metrics.Sources[0].ID, Position: "synthetic-scale/" + strconv.Itoa(i)}}}
			p.ID = m.PointID(p)
			s.Metrics.Points = append(s.Metrics.Points, p)
		}
		result, e := a.Build(context.Background(), s, "synthetic scale")
		if e != nil {
			t.Fatal(e)
		}
		body, _ := json.Marshal(result)
		if len(body) > 32<<20 {
			t.Fatal("oversized scale", len(body))
		}
		saved, e := resultcontract.Decode(body)
		if e != nil {
			t.Fatal(e)
		}
		if e = a.Validate(saved); e != nil {
			t.Fatal(e)
		}
		core, e := a.ReadCore(saved)
		if e != nil || len(core.Metrics.Points) != count {
			t.Fatal(e)
		}
		if dir := os.Getenv("ARCHSCOPE_INFERENCE_SCALE_EVIDENCE_DIR"); dir != "" {
			os.MkdirAll(dir, 0700)
			os.WriteFile(filepath.Join(dir, strconv.Itoa(count)+"-result.json"), body, 0600)
		}
		t.Logf("points=%d one-metric-model result_bytes=%d", count, len(body))
	}
}

func TestR17RationalRateAndLegacyProjection(t *testing.T) {
	raw, e := os.ReadFile("testdata/legacy-rational-0.2.0.json")
	if e != nil {
		t.Fatal(e)
	}
	var old models.AnalysisResult
	if e = json.Unmarshal(raw, &old); e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(old); e != nil {
		t.Fatal("legacy reader changed", e)
	}
	set, e := a.ReadCore(old)
	if e != nil {
		t.Fatal(e)
	}
	rows, e := a.Observations(context.Background(), set)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, row := range rows {
		if row.Rate != nil && *row.Rate == "20/3" {
			found = true
		}
	}
	if !found {
		t.Fatal("exact 100/15 rate lost", rows)
	}
	next, e := a.Build(context.Background(), set, "new explicit analysis")
	if e != nil || next.Metadata.SchemaVersion != "0.3.0" {
		t.Fatal(next.Metadata, e)
	}
	if e = a.Validate(next); e != nil {
		t.Fatal(e)
	}
	set.Signals[0].Semantic = "pcie_bytes"
	set.Signals[0].Conversion = "mib_to_bytes"
	set.Signals[0].Boundary = "device"
	set.Signals[0].EntityID = set.Entities[0].ID
	set.Signals[0].Declaration.Unit = "MiB"
	rows, e = a.Observations(context.Background(), set)
	if e != nil {
		t.Fatal(e)
	}
	found = false
	for _, row := range rows {
		if row.Rate != nil && *row.Rate == "20971520/3" && row.RateUnit == "bytes/second" {
			found = true
		}
	}
	if !found {
		t.Fatal("scaled rational rate lost", rows)
	}

}
