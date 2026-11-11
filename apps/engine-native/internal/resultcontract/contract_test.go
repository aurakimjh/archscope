package resultcontract_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
	c "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

func fixture(t *testing.T) (*c.Registry, models.AnalysisResult) {
	t.Helper()
	value := c.ValueSchema{Nullable: true, Encodings: []string{"json_number", "exact_integer_string", "decimal_string"}, Kinds: []string{"count"}, Units: []string{"count"}, Aggregations: []string{"sum"}}
	r, e := c.New(c.Descriptor{Type: "wire_fixture", Payload: c.Range{Min: "1.0.0", Before: "1.1.0"}, Envelope: c.Range{Min: "1.0.0", Before: "2.0.0"}, Extensions: map[string]c.Range{"measurement": {Min: "1.0.0", Before: "1.1.0"}}, Scalars: map[string]c.ValueSchema{"/summary/value": value, "/summary/a~1b~0c": value}, Collections: map[string]map[string]c.ValueSchema{"/tables/rows": {"weight": value, "availability": {}, "reason": {}}}})
	if e != nil {
		t.Fatal(e)
	}
	result := models.New("wire_fixture", "test")
	result.Metadata.SchemaVersion = "1.0.0"
	result.Summary["value"] = 0
	result.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0"}
	result.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": measurement()}
	return r, result
}
func measurement() c.Measurement {
	return c.Measurement{Kind: "count", Unit: "count", Scope: "fixture", Aggregation: "sum", Encoding: "json_number", Availability: "observed"}
}
func TestW01W03W04CommonAndLegacyMetadataPrecisionAtomicity(t *testing.T) {
	raw, e := catalog.Contracts.ReadFile("contracts/wire-cases.json")
	if e != nil {
		t.Fatal(e)
	}
	input := `{"parser":"p","schema_version":"0.1.0","findings":[],"future":` + string(raw) + `}`
	var m models.Metadata
	if e = json.Unmarshal([]byte(input), &m); e != nil {
		t.Fatal(e)
	}
	m.Extra["parser"] = "wrong"
	m.Extra["diagnostics"] = map[string]any{"revived": true}
	body, e := json.Marshal(m)
	if e != nil || !strings.Contains(string(body), "9007199254740993") || strings.Contains(string(body), `"diagnostics":`) || strings.Contains(string(body), "wrong") {
		t.Fatal(string(body), e)
	}
	for _, next := range []string{`{"parser":"next","future":null}`, `{"parser":"last"}`} {
		if e = json.Unmarshal([]byte(next), &m); e != nil {
			t.Fatal(e)
		}
	}
	if len(m.Extra) != 0 {
		t.Fatal("stale common extension")
	}
	if e = json.Unmarshal([]byte(`{"parser":1,"future":null}`), &m); e == nil || m.Parser != "last" {
		t.Fatal("failed common decode changed receiver")
	}
	var legacy profiler.Metadata
	if e = json.Unmarshal([]byte(input), &legacy); e != nil {
		t.Fatal(e)
	}
	legacy.Extra["progress_log_path"] = json.RawMessage(`"revived"`)
	body, e = json.Marshal(legacy)
	if e != nil || !strings.Contains(string(body), "9007199254740993") || strings.Contains(string(body), "revived") {
		t.Fatal(string(body), e)
	}
	if e = json.Unmarshal([]byte(`{"parser":"last"}`), &legacy); e != nil || len(legacy.Extra) != 0 {
		t.Fatal(e)
	}
	if e = json.Unmarshal([]byte(`{"parser":false}`), &legacy); e == nil || legacy.Parser != "last" {
		t.Fatal("failed profiler decode changed receiver")
	}
}
func TestW02W07ExactDecimalsZeroNullAbsentAndDiff(t *testing.T) {
	registry, r := fixture(t)
	r.Summary["value"] = "0.123456789012345678901234567890"
	m := measurement()
	m.Encoding = "decimal_string"
	r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": m}
	if e := registry.Validate(r); e != nil {
		t.Fatal(e)
	}
	view, e := c.Scalar(r, "/summary/value")
	if e != nil || view.Value == nil || *view.Value != r.Summary["value"] {
		t.Fatal(view, e)
	}
	r.Summary["value"] = 0
	m.Encoding = "json_number"
	r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": m}
	delta, e := registry.Compare(r, r, "/summary/value")
	if e != nil || delta.Value == nil || *delta.Value != "0" || delta.Percent != nil || delta.Reason != "zero_baseline" {
		t.Fatal(delta, e)
	}
	r.Summary["value"] = nil
	m.Availability = "unavailable"
	m.Reason = "not_measured"
	r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": m}
	delta, e = registry.Compare(r, r, "/summary/value")
	if e != nil || delta.Value != nil || delta.Reason != "unavailable" {
		t.Fatal(delta, e)
	}
	view, e = c.Scalar(r, "/summary/value")
	if e != nil || view.Value != nil || !view.Present {
		t.Fatal(view, e)
	}
	delete(r.Summary, "value")
	view, e = c.Scalar(r, "/summary/value")
	if e != nil || view.Present || view.Availability != "absent" {
		t.Fatal(view, e)
	}
}
func TestW05BoundedDecoderRejectsAmbiguousAndInvalidJSONTransactionally(t *testing.T) {
	for _, raw := range []string{`{"a":1,"a":2}`, `{"a":NaN}`, `{"a":Infinity}`, strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65)} {
		out := map[string]any{"kept": true}
		if e := wirejson.Decode([]byte(raw), &out); e == nil || out["kept"] != true {
			t.Fatal("invalid JSON changed destination", raw[:min(len(raw), 90)])
		}
	}
	original := struct {
		A int `json:"a"`
		B int `json:"b"`
	}{9, 9}
	if e := wirejson.Decode([]byte(`{"a":1,"b":"bad"}`), &original); e == nil || original.A != 9 {
		t.Fatal("typed error mutated receiver")
	}
	if _, e := wirejson.Float64Exact(json.Number("9007199254740993")); e == nil {
		t.Fatal("lossy integer conversion")
	}
	if _, e := wirejson.Float64Exact(json.Number("1e-1000000000")); e == nil {
		t.Fatal("unbounded arithmetic exponent accepted")
	}
	limits := wirejson.DefaultLimits
	limits.ArrayItems = 1
	var out any
	if e := wirejson.DecodeWithLimits([]byte(`[1,2]`), &out, limits); e == nil {
		t.Fatal("array budget ignored")
	}
}
func TestW06PointersColumnsEncodingReferencesAndUnknownUnits(t *testing.T) {
	for _, edit := range []func(*models.AnalysisResult){
		func(r *models.AnalysisResult) {
			r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/~2bad": measurement()}
		},
		func(r *models.AnalysisResult) {
			r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/missing": measurement()}
		},
		func(r *models.AnalysisResult) { r.Summary["value"] = "123" },
		func(r *models.AnalysisResult) { r.Summary["value"] = json.Number("9007199254740993") },
		func(r *models.AnalysisResult) {
			m := measurement()
			m.Unit = "seconds"
			r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": m}
		},
		func(r *models.AnalysisResult) {
			m := measurement()
			m.Population = "absent"
			r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": m}
		},
		func(r *models.AnalysisResult) {
			r.Tables["rows"] = []map[string]any{{"weight": 1}}
			r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/tables/rows": {Columns: map[string]c.Measurement{"undefined": measurement()}}}
		},
	} {
		registry, r := fixture(t)
		edit(&r)
		if e := registry.Validate(r); e == nil {
			t.Fatal("invalid measurement accepted")
		}
	}
	registry, r := fixture(t)
	r.Summary["a/b~c"] = 0
	r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/a~1b~0c": measurement()}
	if e := registry.Validate(r); e != nil {
		t.Fatal("valid pointer escape rejected", e)
	}
}
func TestW08W09OperationSpecificVersionsAndNoRelabelMigration(t *testing.T) {
	registry, r := fixture(t)
	r.Metadata.Extra["extensions"] = map[string]string{"future": "9.0.0", "measurement": "1.0.0"}
	r.Metadata.Extra["future"] = map[string]any{"uninterpreted": true}
	if v := registry.Check(r, "save"); !v.Allowed {
		t.Fatal(v)
	}
	r.Metadata.SchemaVersion = "2.0.0"
	if v := registry.Check(r, "compute"); v.Allowed || !v.ReadOnly {
		t.Fatal(v)
	}
	old := models.New("profile_evidence", "old")
	old.Metadata.SchemaVersion = "0.2.0"
	old.Summary["total_samples"] = 123
	production, e := catalog.ResultRegistry()
	if e != nil {
		t.Fatal(e)
	}
	if v := production.Check(old, "compute"); v.Allowed {
		t.Fatal("old counts reinterpreted as duration")
	}
	required, e := c.New(c.Descriptor{Type: "wire_fixture", Payload: c.Range{"1.0.0", "1.1.0"}, Envelope: c.Range{"1.0.0", "2.0.0"}, Required: map[string][]string{"compare": {"future"}}})
	if e != nil {
		t.Fatal(e)
	}
	r = models.New("wire_fixture", "fixture")
	r.Metadata.SchemaVersion = "1.0.0"
	r.Metadata.Extra["extensions"] = map[string]string{"future": "9.0.0"}
	if v := required.Check(r, "display"); !v.Allowed {
		t.Fatal(v)
	}
	if v := required.Check(r, "compare"); v.Allowed {
		t.Fatal("unknown required operation extension activated")
	}
}
