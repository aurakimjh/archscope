package profilemodel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"sort"
)

const SourceExtension = "org.archscope.profile_source.v1"
const SourceNormalizer = "1.0.0"

type SourceMetric struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Unit string `json:"unit"`
}
type SourceProfile struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Start string `json:"start"`
	End   string `json:"end"`
	Unit  string `json:"unit"`
}
type SourceContract struct {
	Revision      string                     `json:"revision"`
	ProducerBasis string                     `json:"producer_basis"`
	FormatVersion string                     `json:"format_version"`
	Metrics       []SourceMetric             `json:"metrics"`
	Profiles      []SourceProfile            `json:"profiles"`
	Attributes    map[string]json.RawMessage `json:"attributes"`
}

func (s ProfileSet) SourceContract() (SourceContract, bool, error) {
	raw, ok := s.Extensions[SourceExtension]
	if !ok {
		return SourceContract{}, false, nil
	}
	var c SourceContract
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(&c); e != nil {
		return c, true, e
	}
	if c.Revision != SourceNormalizer || len(c.Metrics) != len(s.Metrics) || len(c.Profiles) != len(s.Profiles) || len(c.Attributes) > 32 {
		return c, true, fmt.Errorf("unsupported source normalizer contract")
	}
	expected := map[string]string{"pprof": "profile.proto.v1", "speedscope-json": "speedscope.schema.v1", "pyspy-raw": "collapsed.v1"}[s.Format]
	if expected == "" || c.FormatVersion != expected {
		return c, true, fmt.Errorf("unsupported source format revision")
	}
	if c.ProducerBasis != "declared" && c.ProducerBasis != "file_exporter" && c.ProducerBasis != "unknown" {
		return c, true, fmt.Errorf("invalid producer basis")
	}
	for i, m := range c.Metrics {
		if m.ID != s.Metrics[i].ID || m.Type == "" || m.Unit == "" {
			return c, true, fmt.Errorf("source metric binding mismatch")
		}
	}
	for i, p := range c.Profiles {
		if p.ID != s.Profiles[i].ID {
			return c, true, fmt.Errorf("source profile binding mismatch")
		}
	}
	return c, true, nil
}

type CapturePolicy struct {
	ID              string                     `json:"id"`
	Revision        string                     `json:"revision"`
	SHA256          string                     `json:"sha256"`
	Producer        string                     `json:"producer"`
	ProducerVersion string                     `json:"producer_version"`
	Format          string                     `json:"format"`
	Normalizer      string                     `json:"normalizer"`
	Metric          Metric                     `json:"metric"`
	Original        json.RawMessage            `json:"original_options"`
	Semantic        map[string]json.RawMessage `json:"semantic"`
	Incidental      map[string]json.RawMessage `json:"incidental"`
	StrictUnknown   map[string]json.RawMessage `json:"strict_unknown"`
}

// CapturePolicy is resolved from compiled keys and pinned producer contracts.
// Imported ignore/whitelist/policy declarations have no authority. Unknown keys
// remain strict; old models keep opaque v2 equality and hashes.
func (s ProfileSet) CapturePolicy(metricID string) *CapturePolicy {
	contract, present, e := s.SourceContract()
	if e != nil || !present {
		return nil
	}
	metric, ok := s.Metric(metricID)
	if !ok || metric.Signed || metric.Scale != "1" {
		return nil
	}
	if (s.Format == "pprof" && contract.FormatVersion != "profile.proto.v1") || (s.Format == "speedscope-json" && contract.FormatVersion != "speedscope.schema.v1") || (s.Format == "pyspy-raw" && contract.FormatVersion != "collapsed.v1") {
		return nil
	}
	source := SourceMetric{}
	for _, m := range contract.Metrics {
		if m.ID == metricID {
			source = m
		}
	}
	semantic := []string{}
	incidental := []string{"pid", "output_path"}
	id := ""
	if s.Producer == "py-spy" && s.ProducerVersion == "0.4.1" && (s.Format == "pyspy-raw" || s.Format == "speedscope-json") && ((s.Format == "pyspy-raw" && source.Type == "sampled" && source.Unit == "count" && metric.Kind == "sample_count" && metric.Unit == "count") || (s.Format == "speedscope-json" && source.Type == "sampled" && source.Unit == "seconds" && metric.Kind == "duration" && metric.Unit == "seconds")) {
		id = "capture.py-spy.0.4.1"
		semantic = []string{"rate", "gil", "native", "idle", "subprocesses", "duration", "sampling_mode"}
	} else if s.Producer == "go.runtime/pprof" && s.ProducerVersion == "go1.26.10" && s.Format == "pprof" && ((source.Type == "inuse_space" || source.Type == "alloc_space") && source.Unit == "bytes" && metric.Kind == "bytes" && metric.Unit == "bytes" || (source.Type == "inuse_objects" || source.Type == "alloc_objects") && source.Unit == "count" && metric.Kind == "sample_count" && metric.Unit == "count") {
		id = "capture.go.pprof.go1.26.10"
		semantic = []string{"sample_rate", "period", "period_type", "profile_type", "source_period", "source_period_type"}
		if source.Type == "inuse_space" && metric.Kind == "bytes" && metric.Unit == "bytes" || source.Type == "inuse_objects" && metric.Kind == "sample_count" && metric.Unit == "count" {
			incidental = append(incidental, "duration", "source_duration")
		} else {
			semantic = append(semantic, "duration", "source_duration")
		}
	} else {
		return nil
	}
	sort.Strings(semantic)
	sort.Strings(incidental)
	binding := []any{id, "1.0.0", s.Producer, s.ProducerVersion, s.Format, contract.Revision, contract.FormatVersion, metric, source, semantic, incidental}
	policy := &CapturePolicy{ID: id, Revision: "1.0.0", SHA256: hash("", binding), Producer: s.Producer, ProducerVersion: s.ProducerVersion, Format: s.Format, Normalizer: contract.Revision, Metric: metric, Original: append(json.RawMessage(nil), s.CaptureOptions...), Semantic: map[string]json.RawMessage{}, Incidental: map[string]json.RawMessage{}, StrictUnknown: map[string]json.RawMessage{}}
	var options map[string]json.RawMessage
	if json.Unmarshal(s.CaptureOptions, &options) != nil {
		return nil
	}
	for key, value := range options {
		switch {
		case containsKey(semantic, key):
			policy.Semantic[key] = value
		case containsKey(incidental, key):
			policy.Incidental[key] = value
		default:
			policy.StrictUnknown[key] = value
		}
	}
	return policy
}
func containsKey(keys []string, key string) bool {
	for _, v := range keys {
		if v == key {
			return true
		}
	}
	return false
}
func (s ProfileSet) StrictComparisonIdentity(selection Selection) string {
	return hash("population-", []any{"profile-conditions-v2", s.comparisonConditions(selection)})
}
func (s ProfileSet) comparisonPolicyConditions(selection Selection) any {
	c := s.comparisonConditions(selection)
	contract, present, _ := s.SourceContract()
	if !present {
		return c
	}
	policy := s.CapturePolicy(selection.MetricID)
	if policy == nil {
		return []any{contract.Revision, c, nil}
	}
	var reduced any
	body, _ := json.Marshal([]any{policy.Semantic, policy.StrictUnknown})
	wirejson.Decode(body, &reduced)
	c.Capture = reduced
	return []any{contract.Revision, c, policy.SHA256}
}
func incidentalDifferences(a, b *CapturePolicy) []string {
	out := []string{}
	if a == nil || b == nil {
		return out
	}
	keys := map[string]bool{}
	for k := range a.Incidental {
		keys[k] = true
	}
	for k := range b.Incidental {
		keys[k] = true
	}
	for k := range keys {
		av, ap := a.Incidental[k]
		bv, bp := b.Incidental[k]
		if ap != bp || !sameConditions(av, bv) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
