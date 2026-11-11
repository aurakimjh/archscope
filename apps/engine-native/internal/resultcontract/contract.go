// Package resultcontract validates stored envelopes and measurement semantics.
// Imported capability strings never grant an operation.
package resultcontract

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type Range struct{ Min, Before string }

func Version(v string) ([3]uint64, bool) {
	var out [3]uint64
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		if len(p) > 1 && p[0] == '0' {
			return out, false
		}
		n, e := strconv.ParseUint(p, 10, 32)
		if e != nil || p == "" || strings.HasPrefix(p, "+") {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
func compare(a, b [3]uint64) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
func (r Range) Accepts(v string) bool {
	n, ok := Version(v)
	lo, a := Version(r.Min)
	hi, b := Version(r.Before)
	return ok && a && b && compare(n, lo) >= 0 && compare(n, hi) < 0
}

type ValueSchema struct {
	Nullable                bool
	Encodings, Kinds, Units []string
	Aggregations            []string
	RequirePopulation       bool
}
type Descriptor struct {
	ValidatePayload        func(models.AnalysisResult) error `json:"-"`
	Operations             []string
	LegacyFindingsOptional bool
	Type                   string
	Payload                Range
	Envelope               Range
	Extensions             map[string]Range
	Required               map[string][]string
	Scalars                map[string]ValueSchema
	Collections            map[string]map[string]ValueSchema
}
type Registry struct{ types map[string][]Descriptor }

func New(ds ...Descriptor) (*Registry, error) {
	r := &Registry{types: map[string][]Descriptor{}}
	for _, d := range ds {
		if d.Type == "" || !d.Payload.Accepts(d.Payload.Min) || !d.Envelope.Accepts(d.Envelope.Min) {
			return nil, fmt.Errorf("invalid result descriptor")
		}
		for _, prior := range r.types[d.Type] {
			if prior.Payload.Accepts(d.Payload.Min) || d.Payload.Accepts(prior.Payload.Min) {
				return nil, fmt.Errorf("overlapping result type payload ranges")
			}
		}
		body, _ := json.Marshal(d)
		var frozen Descriptor
		if e := json.Unmarshal(body, &frozen); e != nil {
			return nil, e
		}
		frozen.ValidatePayload = d.ValidatePayload
		r.types[d.Type] = append(r.types[d.Type], frozen)
	}
	return r, nil
}

func (r *Registry) HasType(kind string) bool { return len(r.types[kind]) > 0 }
func (r *Registry) descriptor(kind, version string) (Descriptor, bool) {
	for _, d := range r.types[kind] {
		if d.Payload.Accepts(version) {
			return d, true
		}
	}
	return Descriptor{}, false
}
func (r *Registry) SupportsPayload(kind, version string) bool {
	_, ok := r.descriptor(kind, version)
	return ok
}

type Verdict struct {
	Allowed  bool   `json:"allowed"`
	Reason   string `json:"reason,omitempty"`
	ReadOnly bool   `json:"readOnly"`
}

func (r *Registry) Check(result models.AnalysisResult, operation string) Verdict {
	deny := func(reason string) Verdict { return Verdict{false, reason, true} }
	d, ok := r.descriptor(result.Type, result.Metadata.SchemaVersion)
	if !ok {
		if r.HasType(result.Type) {
			return deny("payload_requires_original_reanalysis")
		}
		return deny("unknown_result_type")
	}
	if envelope, exists := result.Metadata.Extra["envelope_schema_version"]; exists {
		v, ok := envelope.(string)
		if !ok || !d.Envelope.Accepts(v) {
			return deny("unsupported_envelope_version")
		}
	}
	if !d.Payload.Accepts(result.Metadata.SchemaVersion) {
		return deny("payload_requires_original_reanalysis")
	}
	known := map[string]bool{"display": true, "save": true, "compute": true, "compare": true, "json": true, "csv": true, "html": true, "report-pack": true}
	if !known[operation] || (len(d.Operations) > 0 && !includes(d.Operations, operation)) {
		return deny("unknown_operation")
	}
	extensions, e := extensionVersions(result)
	if e != nil {
		return deny("invalid_extension_versions")
	}
	for _, key := range d.Required[operation] {
		v, ok := extensions[key]
		supported, exists := d.Extensions[key]
		if !ok || !exists || !supported.Accepts(v) {
			return deny("unsupported_required_extension")
		}
	}
	// Known blocks in an unsupported revision cannot be reinterpreted as v1.
	for key, v := range extensions {
		if support, known := d.Extensions[key]; known && !support.Accepts(v) {
			return deny("unsupported_known_extension")
		}
	}
	if e = r.Validate(result); e != nil {
		return deny("invalid_result: " + e.Error())
	}
	return Verdict{Allowed: true}
}
func extensionVersions(r models.AnalysisResult) (map[string]string, error) {
	versions := map[string]string{}
	raw, exists := r.Metadata.Extra["extensions"]
	if !exists {
		return versions, nil
	}
	b, e := json.Marshal(raw)
	if e != nil {
		return nil, e
	}
	if e = wirejson.Decode(b, &versions); e != nil || versions == nil {
		return nil, fmt.Errorf("extensions must map IDs to semver strings")
	}
	for _, v := range versions {
		if _, ok := Version(v); !ok {
			return nil, fmt.Errorf("invalid extension version")
		}
	}
	return versions, nil
}

type Measurement struct {
	Kind               string                 `json:"measurement_kind,omitempty"`
	Unit               string                 `json:"unit,omitempty"`
	Scope              string                 `json:"scope,omitempty"`
	Population         string                 `json:"population_ref,omitempty"`
	Aggregation        string                 `json:"aggregation,omitempty"`
	Encoding           string                 `json:"numeric_encoding,omitempty"`
	Availability       string                 `json:"availability,omitempty"`
	Reason             string                 `json:"reason,omitempty"`
	Evidence           string                 `json:"evidence_ref,omitempty"`
	Columns            map[string]Measurement `json:"columns,omitempty"`
	AvailabilityColumn string                 `json:"availability_column,omitempty"`
	ReasonColumn       string                 `json:"reason_column,omitempty"`
}

func Measurements(r models.AnalysisResult) (map[string]Measurement, error) {
	m := map[string]Measurement{}
	raw, ok := r.Metadata.Extra["measurement"]
	if !ok {
		return m, nil
	}
	b, e := json.Marshal(raw)
	if e != nil {
		return nil, e
	}
	// Unknown optional fields stay in Metadata.Extra's original block. The
	// typed projection interprets only known fields within the declared range.
	e = wirejson.Decode(b, &m)
	if m == nil && e == nil {
		e = fmt.Errorf("measurement must be an object")
	}
	return m, e
}
func Decode(b []byte) (models.AnalysisResult, error) {
	return DecodeWithLimits(b, wirejson.LegacyResultLimits())
}
func DecodeWithLimits(b []byte, limits wirejson.Limits) (models.AnalysisResult, error) {
	var r models.AnalysisResult
	if e := wirejson.DecodeWithLimits(b, &r, limits); e != nil {
		return r, e
	}
	if r.Type == "" || r.CreatedAt == "" || r.SourceFiles == nil || r.Summary == nil || r.Series == nil || r.Tables == nil || r.Charts == nil {
		return r, fmt.Errorf("incomplete result envelope")
	}
	return r, nil
}
func (r *Registry) Validate(result models.AnalysisResult) error {
	d, ok := r.descriptor(result.Type, result.Metadata.SchemaVersion)
	if !ok {
		return fmt.Errorf("unregistered result type")
	}
	if d.ValidatePayload != nil {
		if e := d.ValidatePayload(result); e != nil {
			return e
		}
	}
	if result.Metadata.Findings == nil && !d.LegacyFindingsOptional {
		return fmt.Errorf("findings must be an array")
	}
	m, e := Measurements(result)
	if e != nil {
		return e
	}
	versions, e := extensionVersions(result)
	if e != nil {
		return e
	}
	if support, known := d.Extensions["measurement"]; len(m) > 0 && (!known || !support.Accepts(versions["measurement"])) {
		return fmt.Errorf("measurement extension version required")
	}
	for path, measurement := range m {
		section, key, e := pointer(path)
		if e != nil {
			return e
		}
		if section == "summary" {
			schema, ok := d.Scalars[path]
			value, exists := result.Summary[key]
			if !ok || !exists || len(measurement.Columns) > 0 || measurement.AvailabilityColumn != "" || measurement.ReasonColumn != "" {
				return fmt.Errorf("unknown scalar target")
			}
			if e = validateValue(result, value, measurement, schema); e != nil {
				return e
			}
			continue
		}
		columns, ok := d.Collections[path]
		if !ok {
			return fmt.Errorf("unknown collection target")
		}
		var target any
		if section == "series" {
			target, ok = result.Series[key]
		} else {
			target, ok = result.Tables[key]
		}
		if !ok {
			return fmt.Errorf("missing collection target")
		}
		raw, e := json.Marshal(target)
		if e != nil {
			return e
		}
		var rows []map[string]any
		if e = wirejson.Decode(raw, &rows); e != nil || rows == nil {
			return fmt.Errorf("collection must be rows")
		}
		if len(measurement.Columns) == 0 {
			return fmt.Errorf("collection columns required")
		}
		for col, cm := range measurement.Columns {
			schema, ok := columns[col]
			if !ok {
				return fmt.Errorf("undefined measurement column")
			}
			if cm.AvailabilityColumn != "" {
				if _, ok = columns[cm.AvailabilityColumn]; !ok {
					return fmt.Errorf("undefined availability column")
				}
			}
			if cm.ReasonColumn != "" {
				if _, ok = columns[cm.ReasonColumn]; !ok {
					return fmt.Errorf("undefined reason column")
				}
			}
			for _, row := range rows {
				value, exists := row[col]
				if !exists {
					return fmt.Errorf("missing column value")
				}
				actual := cm
				if cm.AvailabilityColumn != "" {
					actual.Availability, _ = row[cm.AvailabilityColumn].(string)
				}
				if cm.ReasonColumn != "" {
					actual.Reason, _ = row[cm.ReasonColumn].(string)
				}
				if e = validateValue(result, value, actual, schema); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func pointer(path string) (string, string, error) {
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] != "" || (parts[1] != "summary" && parts[1] != "series" && parts[1] != "tables") {
		return "", "", fmt.Errorf("unsupported JSON Pointer")
	}
	s := parts[2]
	var key strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '~' {
			key.WriteByte(s[i])
			continue
		}
		i++
		if i >= len(s) || (s[i] != '0' && s[i] != '1') {
			return "", "", fmt.Errorf("invalid JSON Pointer escape")
		}
		if s[i] == '0' {
			key.WriteByte('~')
		} else {
			key.WriteByte('/')
		}
	}
	return parts[1], key.String(), nil
}
func includes(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
func referenced(result models.AnalysisResult, key, id string) bool {
	if id == "" {
		return false
	}
	b, _ := json.Marshal(result.Metadata.Extra[key])
	var ids []string
	wirejson.Decode(b, &ids)
	return includes(ids, id)
}
func validateValue(result models.AnalysisResult, value any, m Measurement, s ValueSchema) error {
	if m.Kind == "" || m.Unit == "" || m.Scope == "" || m.Aggregation == "" || !includes(s.Kinds, m.Kind) || !includes(s.Units, m.Unit) || !includes(s.Encodings, m.Encoding) || !includes(s.Aggregations, m.Aggregation) {
		return fmt.Errorf("measurement kind/unit/scope/encoding mismatch")
	}
	if m.Population != "" && !referenced(result, "population_ids", m.Population) {
		return fmt.Errorf("unknown population reference")
	}
	if s.RequirePopulation && m.Population == "" {
		return fmt.Errorf("population reference required")
	}
	if m.Evidence != "" && !referenced(result, "evidence_ids", m.Evidence) {
		return fmt.Errorf("unknown evidence reference")
	}
	switch m.Availability {
	case "unavailable":
		if value != nil || !s.Nullable || m.Reason == "" {
			return fmt.Errorf("unavailable requires permitted null and reason")
		}
		return nil
	case "derived", "estimated":
		if m.Evidence == "" {
			return fmt.Errorf("derived/estimated requires evidence reference")
		}
	case "observed":
	default:
		return fmt.Errorf("unknown availability")
	}
	if value == nil {
		return fmt.Errorf("observed value cannot be null")
	}
	_, e := numeric(value, m.Encoding)
	return e
}

var integerText = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
var decimalText = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func numeric(v any, encoding string) (*big.Rat, error) {
	var text string
	switch encoding {
	case "exact_integer_string", "decimal_string":
		s, ok := v.(string)
		if !ok || len(s) > 1024 {
			return nil, fmt.Errorf("expected bounded exact numeric string")
		}
		text = s
		if encoding == "exact_integer_string" && !integerText.MatchString(s) {
			return nil, fmt.Errorf("invalid exact integer")
		}
		if encoding == "decimal_string" && (!decimalText.MatchString(s) || len(strings.TrimPrefix(strings.TrimPrefix(s, "-"), strings.SplitN(strings.TrimPrefix(s, "-"), ".", 2)[0])) > 129) {
			return nil, fmt.Errorf("invalid decimal scale")
		}
	case "json_number":
		switch v.(type) {
		case string, bool:
			return nil, fmt.Errorf("numeric encoding requires JSON number")
		}
		b, e := json.Marshal(v)
		if e != nil {
			return nil, e
		}
		var n json.Number
		if e = wirejson.Decode(b, &n); e != nil {
			return nil, e
		}
		text = string(n)
		r, arithmeticErr := wirejson.RationalNumber(n)
		if arithmeticErr != nil {
			return nil, arithmeticErr
		}
		if r.IsInt() && new(big.Int).Abs(r.Num()).Cmp(big.NewInt(9007199254740991)) > 0 {
			return nil, fmt.Errorf("unsafe JS integer requires exact_integer_string")
		}
		f, e := strconv.ParseFloat(text, 64)
		if e != nil {
			return nil, e
		}
		projected := strconv.FormatFloat(f, 'g', -1, 64)
		round, ok := new(big.Rat).SetString(projected)
		if !ok || round.Cmp(r) != 0 {
			return nil, fmt.Errorf("high precision decimal requires decimal_string")
		}
	default:
		return nil, fmt.Errorf("unknown numeric encoding")
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("invalid exact number")
	}
	return r, nil
}

type ScalarView struct {
	Present      bool    `json:"present"`
	Availability string  `json:"availability"`
	Reason       string  `json:"reason,omitempty"`
	Value        *string `json:"value"`
	Unit         string  `json:"unit,omitempty"`
	Population   string  `json:"population,omitempty"`
}

func Scalar(result models.AnalysisResult, path string) (ScalarView, error) {
	section, key, e := pointer(path)
	if e != nil || section != "summary" {
		return ScalarView{}, fmt.Errorf("expected summary pointer")
	}
	v, exists := result.Summary[key]
	if !exists {
		return ScalarView{Availability: "absent", Reason: "field_absent"}, nil
	}
	m, e := Measurements(result)
	if e != nil {
		return ScalarView{}, e
	}
	descriptor, ok := m[path]
	if !ok {
		return ScalarView{}, fmt.Errorf("measurement descriptor required")
	}
	out := ScalarView{Present: true, Availability: descriptor.Availability, Reason: descriptor.Reason, Unit: descriptor.Unit, Population: descriptor.Population}
	if v != nil {
		if s, ok := v.(string); ok {
			out.Value = &s
		} else {
			b, e := json.Marshal(v)
			if e != nil {
				return out, e
			}
			s := string(b)
			out.Value = &s
		}
	}
	return out, nil
}

type Delta struct {
	Value      *string             `json:"value"`
	Percent    *string             `json:"percent"`
	Reason     string              `json:"reason,omitempty"`
	Comparison *ComparisonEvidence `json:"comparison,omitempty"`
}

func (r *Registry) Compare(a, b models.AnalysisResult, path string) (Delta, error) {
	for _, v := range []models.AnalysisResult{a, b} {
		if verdict := r.Check(v, "compare"); !verdict.Allowed {
			return Delta{}, fmt.Errorf("%s", verdict.Reason)
		}
	}
	if a.Type != b.Type || a.Metadata.SchemaVersion != b.Metadata.SchemaVersion {
		return Delta{}, fmt.Errorf("semantic revision mismatch; original reanalysis required")
	}
	am, e := Measurements(a)
	if e != nil {
		return Delta{}, e
	}
	bm, e := Measurements(b)
	if e != nil {
		return Delta{}, e
	}
	x, ok := am[path]
	y, ok2 := bm[path]
	if !ok || !ok2 {
		return Delta{Reason: "measurement_absent"}, nil
	}
	if x.Kind != y.Kind || x.Unit != y.Unit || (x.Population == "") != (y.Population == "") || x.Scope != y.Scope || x.Aggregation != y.Aggregation {
		return Delta{}, fmt.Errorf("measurement population/unit/kind mismatch")
	}
	comparison, e := comparePopulations(a, b, x.Population, y.Population)
	if e != nil {
		return Delta{}, e
	}
	av, e := Scalar(a, path)
	if e != nil {
		return Delta{}, e
	}
	bv, e := Scalar(b, path)
	if e != nil {
		return Delta{}, e
	}
	if av.Value == nil || bv.Value == nil {
		return Delta{Reason: "unavailable", Comparison: comparison}, nil
	}
	left, e := numeric(a.Summary[mustKey(path)], x.Encoding)
	if e != nil {
		return Delta{}, e
	}
	right, e := numeric(b.Summary[mustKey(path)], y.Encoding)
	if e != nil {
		return Delta{}, e
	}
	delta := new(big.Rat).Sub(right, left)
	text := delta.RatString()
	out := Delta{Value: &text, Comparison: comparison}
	if left.Sign() == 0 {
		out.Reason = "zero_baseline"
		return out, nil
	}
	percent := new(big.Rat).Mul(new(big.Rat).Quo(delta, left), big.NewRat(100, 1)).RatString()
	out.Percent = &percent
	return out, nil
}
func mustKey(path string) string { _, key, _ := pointer(path); return key }

// ExactNumber returns an owned exact rational without coercing strings or units.
func ExactNumber(v any, encoding string) (*big.Rat, error) { return numeric(v, encoding) }

// PopulationIdentity retains observation provenance. Cross-source comparison
// additionally requires a supported, source-independent comparison policy.
type PopulationIdentity struct {
	Comparison       *PopulationComparison `json:"comparison,omitempty"`
	SourceHashes     []string              `json:"source_sha256"`
	SelectionHash    string                `json:"selection_hash"`
	SemanticRevision string                `json:"semantic_revision"`
	Coverage         string                `json:"coverage"`
}

var sha256Text = regexp.MustCompile(`^[0-9a-f]{64}$`)

func populationIdentity(result models.AnalysisResult, id string) (PopulationIdentity, error) {
	body, e := json.Marshal(result.Metadata.Extra["populations"])
	if e != nil {
		return PopulationIdentity{}, e
	}
	var all map[string]PopulationIdentity
	if e = wirejson.Decode(body, &all); e != nil {
		return PopulationIdentity{}, fmt.Errorf("population identity required")
	}
	identity, exists := all[id]
	if !exists || len(identity.SourceHashes) == 0 || !sha256Text.MatchString(identity.SelectionHash) || identity.SemanticRevision == "" || identity.Coverage == "" {
		return PopulationIdentity{}, fmt.Errorf("population identity required")
	}
	sort.Strings(identity.SourceHashes)
	for i, hash := range identity.SourceHashes {
		if !sha256Text.MatchString(hash) || (i > 0 && identity.SourceHashes[i-1] == hash) {
			return PopulationIdentity{}, fmt.Errorf("invalid population source identity")
		}
	}
	return identity, nil
}
