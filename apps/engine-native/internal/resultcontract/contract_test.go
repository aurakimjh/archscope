package resultcontract_test

import (
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"reflect"
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

func TestReviewR04PopulationIdentityNotLabelOrJobTimestamp(t *testing.T) {
	registry, a := fixture(t)
	descriptor := measurement()
	descriptor.Population = "all"
	a.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": descriptor}
	a.Metadata.Extra["population_ids"] = []string{"all"}
	identity := c.PopulationIdentity{SourceHashes: []string{strings.Repeat("a", 64)}, SelectionHash: strings.Repeat("b", 64), SemanticRevision: "fixture@1.0.0", Coverage: "all records"}
	a.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"all": identity}
	clone := func() models.AnalysisResult {
		raw, _ := json.Marshal(a)
		out, e := c.Decode(raw)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	b := clone()
	b.CreatedAt = "other import"
	b.Summary["value"] = 1
	if delta, e := registry.Compare(a, b, "/summary/value"); e != nil || delta.Value == nil || *delta.Value != "1" {
		t.Fatal(delta, e)
	}
	renamed := clone()
	renamed.Metadata.Extra["population_ids"] = []string{"local_other_label"}
	renamed.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"local_other_label": identity}
	local := descriptor
	local.Population = "local_other_label"
	renamed.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": local}
	if _, e := registry.Compare(a, renamed, "/summary/value"); e != nil {
		t.Fatal("local label overrode matching identity", e)
	}

	for _, edit := range []func(*c.PopulationIdentity){
		func(i *c.PopulationIdentity) { i.SourceHashes = []string{strings.Repeat("c", 64)} },
		func(i *c.PopulationIdentity) { i.SelectionHash = strings.Repeat("d", 64) },
		func(i *c.PopulationIdentity) { i.SemanticRevision = "fixture@2.0.0" },
		func(i *c.PopulationIdentity) { i.Coverage = "filtered" },
	} {
		different := identity
		edit(&different)
		b = clone()
		b.Metadata.Extra["populations"] = map[string]c.PopulationIdentity{"all": different}
		if _, e := registry.Compare(a, b, "/summary/value"); e == nil {
			t.Fatal("label overrode population identity")
		}
	}
	b = clone()
	delete(b.Metadata.Extra, "populations")
	if _, e := registry.Compare(a, b, "/summary/value"); e == nil {
		t.Fatal("label-only population compared")
	}
}

func TestReviewR06PatchVersionsOptionalFieldsAndMajorRejection(t *testing.T) {
	registry, r := fixture(t)
	r.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.1"}
	raw, _ := json.Marshal(measurement())
	var projected map[string]any
	json.Unmarshal(raw, &projected)
	projected["future_optional"] = map[string]any{"exact": json.Number("9007199254740993")}
	r.Metadata.Extra["measurement"] = map[string]any{"/summary/value": projected}
	if v := registry.Check(r, "compute"); !v.Allowed {
		t.Fatal(v)
	}
	saved, _ := json.Marshal(r)
	again, e := c.Decode(saved)
	if e != nil {
		t.Fatal(e)
	}
	saved, _ = json.Marshal(again)
	if !strings.Contains(string(saved), "9007199254740993") {
		t.Fatal("optional field lost")
	}
	for _, version := range []string{"1.1.0", "2.0.0"} {
		r.Metadata.Extra["extensions"] = map[string]string{"measurement": version}
		if registry.Check(r, "save").Allowed {
			t.Fatal("unsupported version accepted", version)
		}
	}
}

func TestReviewR12ExportCollisionAndCollectionAvailability(t *testing.T) {
	registry, r := fixture(t)
	r.Series["measurement_availability"] = []any{"original"}
	if _, e := c.ExportPresentation(r, true); e == nil {
		t.Fatal("export silently overwrote source series")
	}
	delete(r.Series, "measurement_availability")
	r.Tables["rows"] = []map[string]any{{"weight": nil, "availability": "unavailable", "reason": "not_measured"}, {"weight": 0, "availability": "observed", "reason": ""}}
	m := measurement()
	m.AvailabilityColumn = "availability"
	m.ReasonColumn = "reason"
	r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/tables/rows": {Columns: map[string]c.Measurement{"weight": m}}}
	if e := registry.Validate(r); e != nil {
		t.Fatal(e)
	}
	html, e := c.ExportPresentation(r, true)
	if e != nil {
		t.Fatal(e)
	}
	rows := html.Tables["rows"].([]any)
	if rows[0].(map[string]any)["weight"] != "—" || rows[1].(map[string]any)["weight"] != json.Number("0") {
		t.Fatal(rows)
	}
	appendix := html.Series["measurement_availability"].([]map[string]any)
	if len(appendix) != 2 || appendix[0]["reason"] != "not_measured" || appendix[1]["availability"] != "observed" {
		t.Fatal(appendix)
	}
	original := r.Tables["rows"].([]map[string]any)
	if original[0]["weight"] != nil {
		t.Fatal("presentation mutated source")
	}
	csv, e := c.ExportPresentation(r, false)
	if e != nil {
		t.Fatal(e)
	}
	if csv.Tables["rows"].([]any)[0].(map[string]any)["weight"] != nil {
		t.Fatal("CSV null became zero")
	}
}

func TestReviewR01TreeBudgetIndependentFromMetadataBudget(t *testing.T) {
	r := models.New("profile_evidence", "test")
	tree := any(0)
	for i := 0; i < 256; i++ {
		tree = map[string]any{"children": []any{tree}}
	}
	r.Charts["flamegraph"] = tree
	raw, _ := json.Marshal(r)
	if _, e := c.Decode(raw); e != nil {
		t.Fatal("realistic stack rejected", e)
	}
	limits := wirejson.ResultLimits(int64(len(raw)), 128)
	if _, e := c.DecodeWithLimits(raw, limits); e == nil {
		t.Fatal("result tree budget ignored")
	}
	limits = wirejson.ResultLimits(int64(len(raw))-1, 1024)
	if _, e := c.DecodeWithLimits(raw, limits); e == nil {
		t.Fatal("result byte budget ignored")
	}
	r.Metadata.Extra["opaque"] = tree
	raw, _ = json.Marshal(r)
	if _, e := c.Decode(raw); e == nil {
		t.Fatal("metadata depth guard lost")
	}
}

func TestReviewR11BundledSchemaScalarColumnCollectionRuntimeAgreement(t *testing.T) {
	raw, e := catalog.Contracts.ReadFile("contracts/measurement-descriptor.schema.json")
	if e != nil {
		t.Fatal(e)
	}
	validate, e := ingestion.CompileSchema(raw)
	if e != nil {
		t.Fatal(e)
	}
	var schema struct {
		Defs map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"$defs"`
	}
	if e = json.Unmarshal(raw, &schema); e != nil {
		t.Fatal(e)
	}
	// Every public Go descriptor field belongs to at least one declared variant.
	typ := reflect.TypeOf(c.Measurement{})
	for i := 0; i < typ.NumField(); i++ {
		tag := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
		found := false
		for _, variant := range schema.Defs {
			_, ok := variant.Properties[tag]
			found = found || ok
		}
		if !found {
			t.Fatal("schema omitted Go field", tag)
		}
	}
	for _, collection := range []bool{false, true} {
		registry, result := fixture(t)
		m := measurement()
		var value c.Measurement = m
		if collection {
			m.AvailabilityColumn = "availability"
			m.ReasonColumn = "reason"
			value = c.Measurement{Columns: map[string]c.Measurement{"weight": m}}
			result.Tables["rows"] = []map[string]any{{"weight": 0, "availability": "observed", "reason": ""}}
			result.Metadata.Extra["measurement"] = map[string]c.Measurement{"/tables/rows": value}
		}
		encoded, _ := json.Marshal(value)
		if e = validate(encoded); e != nil {
			t.Fatal(collection, e)
		}
		if e = registry.Validate(result); e != nil {
			t.Fatal(e)
		}
	}
	for _, bad := range []string{`{}`, `{"columns":{}}`, `{"columns":{"weight":{"unit":"count"}}}`, `{"measurement_kind":"count","unit":"count","scope":"all","aggregation":"sum","numeric_encoding":"json_number","availability":"unknown"}`} {
		if e = validate([]byte(bad)); e == nil {
			t.Fatal("schema accepted malformed descriptor", bad)
		}
	}
}

func TestReviewR03OpaqueStorageDoesNotRunSchemaArithmetic(t *testing.T) {
	registry, r := fixture(t)
	r.Metadata.Extra["future_opaque"] = json.Number("1e1000000000")
	body, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	again, e := c.Decode(body)
	if e != nil {
		t.Fatal("opaque numeric exponent rejected", e)
	}
	if v := registry.Check(again, "save"); !v.Allowed {
		t.Fatal(v)
	}
	body, e = json.Marshal(again)
	if e != nil || !strings.Contains(string(body), "1e1000000000") {
		t.Fatal("opaque numeric text lost", e)
	}
}
