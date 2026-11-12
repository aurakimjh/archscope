package packs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
	"regexp"
	"sort"
	"strings"
)

// Mapping is a closed interpreter vocabulary, not a user expression language.
// Supported aggregation is preservation of each independent snapshot sample.
type Mapping struct {
	Version          string            `json:"schema_version"`
	ID               string            `json:"id"`
	Aliases          []string          `json:"aliases"`
	Producer         string            `json:"producer"`
	ProducerVersions []string          `json:"producer_versions"`
	ProducerPath     string            `json:"producer_path"`
	VersionPath      string            `json:"version_path"`
	Match            map[string]string `json:"match"`
	NamePaths        []string          `json:"name_paths"`
	ValuePaths       []string          `json:"value_paths"`
	Scale            string            `json:"scale"`
	Unit             string            `json:"unit"`
	Kind             string            `json:"measurement_kind"`
	Pack             Ref               `json:"-"`
}
type Record struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
	Unit   string `json:"unit"`
	Kind   string `json:"measurement_kind"`
}

var decimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]{1,128})?$`)

func pointer(p string) bool {
	if p == "" || len(p) > 256 || !strings.HasPrefix(p, "/") || strings.Count(p, "/") > 8 {
		return false
	}
	for i := 0; i < len(p); i++ {
		if p[i] == '~' {
			i++
			if i >= len(p) || (p[i] != '0' && p[i] != '1') {
				return false
			}
		}
	}
	return true
}
func (m Mapping) Validate() error {
	if m.Version != Interpreter || !ID.MatchString(m.ID) || len(m.Aliases) > 16 || m.Producer == "" || len(m.Producer) > 128 || len(m.ProducerVersions) == 0 || len(m.ProducerVersions) > 16 || len(m.Match) == 0 || len(m.Match) > 8 {
		return problem("PACK_MAPPING_INVALID", "mapping identity/producer/match required")
	}
	for _, a := range m.Aliases {
		if !ID.MatchString(a) || a == "auto" {
			return problem("PACK_MAPPING_INVALID", "invalid alias")
		}
	}
	if !pointer(m.ProducerPath) || !pointer(m.VersionPath) || m.Match[m.ProducerPath] != m.Producer {
		return problem("PACK_MAPPING_INVALID", "literal producer path must bind the declared producer")
	}
	versionAllowed := false
	for _, v := range m.ProducerVersions {
		versionAllowed = versionAllowed || v == m.Match[m.VersionPath]
	}
	if !versionAllowed {
		return problem("PACK_MAPPING_INVALID", "literal producer version must be declared")
	}
	for p, s := range m.Match {
		if !pointer(p) || s == "" || len(s) > 128 {
			return problem("PACK_MAPPING_INVALID", "bounded literal producer match required")
		}
	}
	for _, list := range [][]string{m.NamePaths, m.ValuePaths} {
		if len(list) < 1 || len(list) > 8 {
			return problem("PACK_MAPPING_INVALID", "1..8 alias field paths required")
		}
		seen := map[string]bool{}
		for _, p := range list {
			if !pointer(p) || seen[p] {
				return problem("PACK_MAPPING_INVALID", "invalid field path")
			}
			seen[p] = true
		}
	}
	units := map[string]bool{"seconds": true, "milliseconds": true, "microseconds": true, "nanoseconds": true, "bytes": true, "count": true, "ratio": true, "percent": true}
	if !units[m.Unit] || (m.Kind != "gauge" && m.Kind != "counter") || !decimal.MatchString(m.Scale) || len(m.Scale) > 256 {
		return problem("PACK_MAPPING_INVALID", "unsupported unit/kind/exact scale")
	}
	scale, _ := new(big.Rat).SetString(m.Scale)
	if scale == nil || scale.Sign() <= 0 || scale.Num().BitLen() > 512 || scale.Denom().BitLen() > 512 {
		return problem("PACK_MAPPING_INVALID", "scale must be bounded and positive")
	}
	return nil
}
func field(obj any, p string) (any, bool) {
	for _, part := range strings.Split(p[1:], "/") {
		key := strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		m, ok := obj.(map[string]any)
		if !ok {
			return nil, false
		}
		obj, ok = m[key]
		if !ok {
			return nil, false
		}
	}
	return obj, true
}
func aliases(obj any, paths []string) (any, error) {
	var found any
	present := false
	var prior []byte
	for _, p := range paths {
		if v, ok := field(obj, p); ok {
			b, _ := json.Marshal(v)
			if present && !bytes.Equal(prior, b) {
				return nil, problem("PACK_ALIAS_CONFLICT", "multiple field aliases disagree; choose a mapping explicitly")
			}
			found = v
			present = true
			prior = b
		}
	}
	if !present {
		return nil, problem("PACK_FIELD_MISSING", "required mapped field absent")
	}
	return found, nil
}
func (m Mapping) Evaluate(raw []byte) (Record, error) {
	if len(raw) > 64<<10 {
		return Record{}, problem("PACK_BUDGET_EXCEEDED", "mapping record exceeds 64 KiB")
	}
	var obj any
	if e := wirejson.Decode(raw, &obj); e != nil {
		return Record{}, problem("PACK_INPUT_INVALID", "input must be a bounded JSON object")
	}
	if _, ok := obj.(map[string]any); !ok {
		return Record{}, problem("PACK_INPUT_INVALID", "input must be an object")
	}
	for p, want := range m.Match {
		v, ok := field(obj, p)
		text, isText := v.(string)
		if !ok || !isText || text != want {
			return Record{}, problem("PACK_PRODUCER_MISMATCH", "producer/version literal conditions do not match")
		}
	}
	name, e := aliases(obj, m.NamePaths)
	if e != nil {
		return Record{}, e
	}
	metric, ok := name.(string)
	if !ok || metric == "" || len(metric) > 256 {
		return Record{}, problem("PACK_INPUT_INVALID", "metric name must be bounded text")
	}
	if resultcontract.ContainsDisplayControls(metric) {
		return Record{}, problem("PACK_INPUT_INVALID", "metric name contains control or bidirectional formatting characters")
	}
	value, e := aliases(obj, m.ValuePaths)
	if e != nil {
		return Record{}, e
	}
	var text string
	switch n := value.(type) {
	case json.Number:
		r, e := wirejson.RationalNumber(n)
		if e != nil {
			return Record{}, problem("PACK_INPUT_INVALID", "invalid numeric value")
		}
		text = r.RatString()
	case string:
		if !decimal.MatchString(n) || len(n) > 1024 {
			return Record{}, problem("PACK_INPUT_INVALID", "invalid exact decimal string")
		}
		text = n
	default:
		return Record{}, problem("PACK_INPUT_INVALID", "numeric value required; null/boolean are not zero")
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok || m.Kind == "counter" && number.Sign() < 0 {
		return Record{}, problem("PACK_INPUT_INVALID", "exact number required; counter cannot be negative")
	}
	scale, _ := new(big.Rat).SetString(m.Scale)
	number.Mul(number, scale)
	if number.Num().BitLen() > 4096 || number.Denom().BitLen() > 2048 {
		return Record{}, problem("PACK_BUDGET_EXCEEDED", "exact arithmetic budget")
	}
	den := new(big.Int).Set(number.Denom())
	twos, fives := 0, 0
	for den.Bit(0) == 0 {
		den.Rsh(den, 1)
		twos++
	}
	five := big.NewInt(5)
	rem := new(big.Int)
	for {
		q := new(big.Int)
		q.QuoRem(den, five, rem)
		if rem.Sign() != 0 {
			break
		}
		den = q
		fives++
	}
	places := max(twos, fives)
	if den.Cmp(big.NewInt(1)) != 0 || places > 128 {
		return Record{}, problem("PACK_SCALE_INEXACT", "scaled value has no bounded finite decimal representation")
	}
	scaled := number.FloatString(places)
	if strings.Contains(scaled, ".") {
		scaled = strings.TrimRight(strings.TrimRight(scaled, "0"), ".")
	}
	if scaled == "-0" {
		scaled = "0"
	}
	return Record{metric, scaled, m.Unit, m.Kind}, nil
}
func Conformance(r *Revision) error { return ConformanceContext(context.Background(), r) }
func ConformanceContext(ctx context.Context, r *Revision) error {
	operations := 0
	byID := map[string]Mapping{}
	for _, p := range r.Manifest.Mappings {
		var m Mapping
		if e := validateSchema("mapping", r.Files[p]); e != nil {
			return problem("PACK_MAPPING_INVALID", "mapping schema failed")
		}
		if e := Strict(r.Files[p], &m); e != nil {
			return problem("PACK_MAPPING_INVALID", e.Error())
		}
		if e := m.Validate(); e != nil {
			return e
		}
		if _, exists := byID[m.ID]; exists {
			return problem("PACK_MAPPING_INVALID", "duplicate canonical mapping ID")
		}
		m.Pack = r.Ref
		byID[m.ID] = m
		r.Mappings = append(r.Mappings, m)
	}
	counts := map[string]int{}
	acceptedCounts, rejectedCounts := map[string]int{}, map[string]int{}
	for _, f := range r.Manifest.Fixtures {
		m, ok := byID[f.Mapping]
		if !ok || r.Manifest.Files[f.Input].Role != "fixture" || r.Manifest.Files[f.Expected].Role != "fixture" {
			return problem("PACK_FIXTURE_INVALID", "fixture references must bind a declared mapping and data")
		}
		lines := bytes.Split(bytes.TrimSpace(r.Files[f.Input]), []byte{'\n'})
		if len(lines) == 0 || len(lines) > 1000 {
			return problem("PACK_FIXTURE_INVALID", "fixture row limit")
		}
		out := []Record{}
		accepted := true
		for _, line := range lines {
			operations++
			if operations > 10000 {
				return problem("PACK_BUDGET_EXCEEDED", "conformance operation budget")
			}
			if e := ctx.Err(); e != nil {
				return e
			}
			v, e := m.Evaluate(line)
			if e != nil {
				accepted = false
				break
			}
			out = append(out, v)
		}
		if accepted != f.Accept {
			return problem("PACK_CONFORMANCE_FAILED", "fixture acceptance disagrees")
		}
		if accepted {
			var expected []Record
			if e := Strict(r.Files[f.Expected], &expected); e != nil {
				return problem("PACK_FIXTURE_INVALID", "invalid expected rows")
			}
			a, _ := json.Marshal(out)
			b, _ := json.Marshal(expected)
			if !bytes.Equal(a, b) {
				return problem("PACK_CONFORMANCE_FAILED", "exact mapped output differs")
			}
		}
		counts[f.Mapping]++
		if f.Accept {
			acceptedCounts[f.Mapping]++
		} else {
			rejectedCounts[f.Mapping]++
		}
	}
	for id := range byID {
		if counts[id] == 0 || acceptedCounts[id] == 0 || rejectedCounts[id] == 0 {
			return problem("PACK_FIXTURE_MISSING", fmt.Sprintf("mapping %s has no conformance case", id))
		}
	}
	sort.Slice(r.Mappings, func(i, j int) bool { return r.Mappings[i].ID < r.Mappings[j].ID })
	return nil
}

func (m Mapping) Copy() Mapping {
	out := m
	out.Aliases = append([]string{}, m.Aliases...)
	out.ProducerVersions = append([]string{}, m.ProducerVersions...)
	out.NamePaths = append([]string{}, m.NamePaths...)
	out.ValuePaths = append([]string{}, m.ValuePaths...)
	out.Match = map[string]string{}
	for k, v := range m.Match {
		out.Match[k] = v
	}
	return out
}
