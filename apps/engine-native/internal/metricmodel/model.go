// Package metricmodel owns exact metric identities and interval arithmetic.
package metricmodel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const Version = "1.0.0"

// Normalizer is the original saved-result interpretation; never rewrite it on read.
const Normalizer = "1.0.0"
const CadenceNormalizer = "1.1.0"
const ObservationNormalizer = "1.2.0"

// EpochNormalizer discloses a known start reached from an unknown predecessor.
const EpochNormalizer = "1.3.0"

func supportedNormalizer(v string) bool {
	return v == Normalizer || v == CadenceNormalizer || v == ObservationNormalizer || v == EpochNormalizer
}

const MaxBytes = 32 << 20
const MaxPoints = 100_000
const MaxSeries = 10_000
const MaxBuckets = 4096
const MaxTotalBuckets = 500_000

type Source struct {
	ID              string          `json:"id"`
	SHA256          string          `json:"sha256"`
	Name            string          `json:"name"`
	Format          string          `json:"format"`
	Producer        string          `json:"producer"`
	ProducerVersion string          `json:"producer_version"`
	ExportID        string          `json:"export_id,omitempty"`
	Query           json.RawMessage `json:"query,omitempty"`
}
type Scope struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	SchemaURL  string          `json:"schema_url"`
	Attributes json.RawMessage `json:"attributes"`
	Dropped    string          `json:"dropped_attributes"`
}
type Series struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	Unit              string          `json:"unit"`
	Kind              string          `json:"kind"`
	Temporality       string          `json:"temporality"`
	Monotonic         bool            `json:"monotonic"`
	Origin            string          `json:"origin"`
	Resource          json.RawMessage `json:"resource"`
	ResourceSchemaURL string          `json:"resource_schema_url"`
	Scope             Scope           `json:"scope"`
	Attributes        json.RawMessage `json:"attributes"`
	ExpectedStep      *string         `json:"expected_step_ns,omitempty"`
}
type Ref struct {
	SourceID string `json:"source_id"`
	Position string `json:"position"`
}
type Buckets struct {
	Offset int32    `json:"offset"`
	Counts []string `json:"counts"`
}
type Histogram struct {
	Count         string   `json:"count"`
	Sum           *string  `json:"sum"`
	Min           *string  `json:"min"`
	Max           *string  `json:"max"`
	Bounds        []string `json:"bounds,omitempty"`
	Counts        []string `json:"counts,omitempty"`
	Scale         *int32   `json:"scale,omitempty"`
	ZeroCount     string   `json:"zero_count,omitempty"`
	ZeroThreshold *string  `json:"zero_threshold,omitempty"`
	Positive      *Buckets `json:"positive,omitempty"`
	Negative      *Buckets `json:"negative,omitempty"`
}
type Point struct {
	ID        string          `json:"id"`
	SeriesID  string          `json:"series_id"`
	Start     *string         `json:"start_ns"`
	End       *string         `json:"end_ns"`
	Value     *string         `json:"value"`
	Special   string          `json:"special,omitempty"`
	Flags     uint32          `json:"flags"`
	Histogram *Histogram      `json:"histogram,omitempty"`
	Exemplars json.RawMessage `json:"exemplars,omitempty"`
	Refs      []Ref           `json:"refs"`
}
type Set struct {
	Version    string   `json:"schema_version"`
	Normalizer string   `json:"normalizer"`
	Sources    []Source `json:"sources"`
	Series     []Series `json:"series"`
	Points     []Point  `json:"points"`
	Duplicates int      `json:"deduplicated_points"`
	Warnings   []string `json:"warnings"`
}

func Hash(value any) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
func Canonical(raw []byte) (json.RawMessage, error) {
	var value any
	if e := wirejson.DecodeWithLimits(raw, &value, wirejson.ResultLimits(MaxBytes, 128)); e != nil {
		return nil, e
	}
	body, e := json.Marshal(value)
	return body, e
}
func Number(s string) (*big.Rat, error) { return profilemodel.Number(s) }
func Text(n *big.Rat) string            { return profilemodel.Text(n) }
func Uint(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.Sign() < 0 || n.BitLen() > 64 || n.String() != s {
		return nil, fmt.Errorf("METRIC_INTEGER_INVALID: canonical uint64 required")
	}
	return n, nil
}
func Time(s *string) (uint64, bool) {
	if s == nil {
		return 0, false
	}
	n, e := strconv.ParseUint(*s, 10, 64)
	return n, e == nil && strconv.FormatUint(n, 10) == *s
}
func SeriesID(s Series) string { s.ID = ""; return "metric-" + Hash(s) }
func PointID(p Point) string   { p.ID = ""; return "point-" + Hash(p) }
func payloadID(p Point) string { p.ID = ""; p.Refs = nil; return Hash(p) }
func Decode(raw []byte) (Set, error) {
	var s Set
	if len(raw) > MaxBytes {
		return s, budget("model_bytes", len(raw), MaxBytes)
	}
	if e := Strict(raw, &s); e != nil {
		return s, e
	}
	return s, s.Validate(context.Background())
}
func Strict(raw []byte, out any) error {
	if len(raw) > MaxBytes {
		return budget("json_bytes", len(raw), MaxBytes)
	}
	if e := wirejson.DecodeWithLimits(raw, new(any), wirejson.ResultLimits(MaxBytes, 128)); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	d.UseNumber()
	return d.Decode(out)
}
func (s Set) Validate(ctx context.Context) error {
	if s.Version != Version || !supportedNormalizer(s.Normalizer) {
		return fmt.Errorf("METRIC_VERSION_UNSUPPORTED")
	}
	if len(s.Sources) == 0 || s.Duplicates < 0 {
		return fmt.Errorf("METRIC_MODEL_INVALID: source required and duplicate count nonnegative")
	}
	for _, bound := range []struct {
		resource      string
		actual, limit int
	}{{"sources", len(s.Sources), 128}, {"series", len(s.Series), MaxSeries}, {"points", len(s.Points), MaxPoints}, {"warnings", len(s.Warnings), 128}} {
		if bound.actual > bound.limit {
			return budget(bound.resource, bound.actual, bound.limit)
		}
	}
	sources := map[string]bool{}
	sourceByID := map[string]Source{}
	series := map[string]Series{}
	seen := map[string]bool{}
	refs := map[string]bool{}
	buckets := 0
	duplicateRefs := 0
	for _, source := range s.Sources {
		if source.ID == "" || sources[source.ID] || len(source.SHA256) != 64 || source.Format == "" || source.Producer == "" || len(source.Name) > 1024 || len(source.ExportID) > 1024 || len(source.ID) > 1024 || len(source.Format) > 256 || len(source.Producer) > 256 || len(source.ProducerVersion) > 256 {
			return fmt.Errorf("METRIC_SOURCE_INVALID")
		}
		if _, e := hex.DecodeString(source.SHA256); e != nil {
			return e
		}
		sources[source.ID] = true
		sourceByID[source.ID] = source
		if len(source.Query) != 0 {
			if _, err := Canonical(source.Query); err != nil {
				return err
			}
		}
		if len(source.Query) > 64<<10 {
			return fmt.Errorf("METRIC_BUDGET_EXCEEDED: source provenance")
		}
	}
	for _, v := range s.Series {
		if v.ID != SeriesID(v) || series[v.ID].ID != "" || v.Name == "" || len(v.Name) > 4096 || len(v.Unit) > 1024 || len(v.Description) > 16<<10 || v.Origin == "" || len(v.Origin) > 4096 || len(v.ResourceSchemaURL) > 4096 || len(v.Scope.Name) > 4096 || len(v.Scope.Version) > 1024 || len(v.Scope.SchemaURL) > 4096 {
			return fmt.Errorf("METRIC_SERIES_INVALID")
		}
		if _, err := Uint(v.Scope.Dropped); err != nil {
			return fmt.Errorf("METRIC_SCOPE_DROPPED_INVALID")
		}
		if v.Kind != "sum" && v.Monotonic {
			return fmt.Errorf("METRIC_MONOTONIC_KIND_INVALID")
		}
		switch v.Kind {
		case "gauge":
			if v.Temporality != "unspecified" || v.Monotonic {
				return fmt.Errorf("METRIC_GAUGE_INVALID")
			}
		case "sum", "histogram", "exponential_histogram":
			if v.Temporality != "delta" && v.Temporality != "cumulative" && v.Temporality != "unspecified" {
				return fmt.Errorf("METRIC_TEMPORALITY_INVALID")
			}
		default:
			return fmt.Errorf("METRIC_KIND_UNSUPPORTED")
		}
		for _, raw := range []json.RawMessage{v.Resource, v.Scope.Attributes, v.Attributes} {
			if len(raw) > 64<<10 {
				return fmt.Errorf("METRIC_BUDGET_EXCEEDED: identity attributes")
			}
			canonical, e := Canonical(raw)
			var compact bytes.Buffer
			compactErr := json.Compact(&compact, raw)
			if e != nil || compactErr != nil || !bytes.Equal(compact.Bytes(), canonical) || len(canonical) == 0 || canonical[0] != '{' {
				return fmt.Errorf("METRIC_IDENTITY_NONCANONICAL")
			}
		}
		if v.ExpectedStep != nil {
			if step, ok := Time(v.ExpectedStep); !ok || step == 0 {
				return fmt.Errorf("METRIC_STEP_INVALID")
			}
		}
		series[v.ID] = v
	}
	for _, p := range s.Points {
		if e := ctx.Err(); e != nil {
			return e
		}
		v, ok := series[p.SeriesID]
		if !ok || p.ID != PointID(p) || seen[p.ID] || len(p.Refs) == 0 || len(p.Refs) > 128 || len(p.Exemplars) > 64<<10 {
			return fmt.Errorf("METRIC_POINT_INVALID")
		}
		seen[p.ID] = true
		duplicateRefs += len(p.Refs) - 1
		qualified := ""
		for _, ref := range p.Refs {
			key := ref.SourceID + "/" + ref.Position
			if !sources[ref.SourceID] || ref.Position == "" || len(ref.Position) > 1024 || refs[key] {
				return fmt.Errorf("METRIC_REFERENCE_INVALID")
			}
			refs[key] = true
			source := sourceByID[ref.SourceID]
			proof := provenanceKey(source, ref.Position)
			if qualified != "" && qualified != proof {
				return fmt.Errorf("METRIC_DEDUP_PROVENANCE_MISMATCH")
			}
			qualified = proof
		}
		end, ek := Time(p.End)
		start, sk := Time(p.Start)
		if p.End != nil && !ek || p.Start != nil && !sk || sk && ek && start > end {
			return fmt.Errorf("METRIC_TIME_INVALID")
		}
		if p.Special != "" && p.Special != "nan" && p.Special != "+inf" && p.Special != "-inf" && p.Special != "no_recorded_value" {
			return fmt.Errorf("METRIC_SPECIAL_INVALID")
		}
		if p.Value != nil {
			if _, e := Number(*p.Value); e != nil {
				return e
			}
			if p.Special != "" {
				return fmt.Errorf("METRIC_VALUE_INVALID")
			}
		}
		if v.Kind == "gauge" || v.Kind == "sum" {
			if p.Histogram != nil || p.Value == nil && p.Special == "" && p.Flags&1 == 0 {
				return fmt.Errorf("METRIC_VALUE_REQUIRED")
			}
			if v.Kind == "sum" && v.Monotonic && p.Value != nil {
				n, _ := Number(*p.Value)
				if n.Sign() < 0 {
					return fmt.Errorf("METRIC_MONOTONIC_NEGATIVE")
				}
			}
		} else {
			if p.Value != nil || p.Histogram == nil && p.Flags&1 == 0 {
				return fmt.Errorf("METRIC_HISTOGRAM_REQUIRED")
			}
			if p.Histogram != nil {
				count, e := validateHistogram(v.Kind, p.Histogram)
				if e != nil {
					return e
				}
				buckets += count
				if buckets > MaxTotalBuckets {
					return budget("total_buckets", buckets, MaxTotalBuckets)
				}
			}
		}
	}
	if duplicateRefs != s.Duplicates {
		return fmt.Errorf("METRIC_DEDUP_COUNT_MISMATCH")
	}
	body, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(body) > MaxBytes {
		return budget("model_bytes", len(body), MaxBytes)
	}
	return nil
}
func validateHistogram(kind string, h *Histogram) (int, error) {
	count, e := Uint(h.Count)
	if e != nil {
		return 0, e
	}
	for _, v := range []*string{h.Sum, h.Min, h.Max, h.ZeroThreshold} {
		if v != nil {
			if _, e = Number(*v); e != nil {
				return 0, e
			}
		}
	}
	if h.Min != nil && h.Max != nil {
		a, _ := Number(*h.Min)
		b, _ := Number(*h.Max)
		if a.Cmp(b) > 0 {
			return 0, fmt.Errorf("METRIC_HISTOGRAM_MIN_MAX")
		}
	}
	total := new(big.Int)
	n := 0
	if kind == "histogram" {
		if h.Scale != nil || h.Positive != nil || h.Negative != nil || h.ZeroCount != "" || h.ZeroThreshold != nil {
			return 0, fmt.Errorf("METRIC_HISTOGRAM_SHAPE")
		}
		if len(h.Bounds) > MaxBuckets || len(h.Counts) > MaxBuckets+1 {
			return 0, fmt.Errorf("METRIC_BUDGET_EXCEEDED: histogram buckets")
		}
		if len(h.Counts) == 0 {
			if len(h.Bounds) != 0 {
				return 0, fmt.Errorf("METRIC_HISTOGRAM_BUCKETS_MISSING")
			}
			return 0, nil
		}
		if len(h.Counts) != len(h.Bounds)+1 {
			return 0, fmt.Errorf("METRIC_HISTOGRAM_BUCKET_ARITY")
		}
		var previous *big.Rat
		for _, bound := range h.Bounds {
			value, e := Number(bound)
			if e != nil || previous != nil && value.Cmp(previous) <= 0 {
				return 0, fmt.Errorf("METRIC_HISTOGRAM_BOUNDS")
			}
			previous = value
		}
		for _, value := range h.Counts {
			x, e := Uint(value)
			if e != nil {
				return 0, e
			}
			total.Add(total, x)
			n++
		}
	} else {
		if h.Scale == nil || *h.Scale < -10 || *h.Scale > 20 || len(h.Bounds) != 0 || len(h.Counts) != 0 {
			return 0, fmt.Errorf("METRIC_EXPONENTIAL_SHAPE")
		}
		if h.ZeroThreshold == nil {
			return 0, fmt.Errorf("METRIC_ZERO_THRESHOLD_REQUIRED")
		}
		z, _ := Number(*h.ZeroThreshold)
		if z.Sign() < 0 {
			return 0, fmt.Errorf("METRIC_ZERO_THRESHOLD_NEGATIVE")
		}
		zero, e := Uint(h.ZeroCount)
		if e != nil {
			return 0, e
		}
		total.Add(total, zero)
		for _, side := range []*Buckets{h.Positive, h.Negative} {
			if side == nil {
				continue
			}
			if len(side.Counts) > MaxBuckets {
				return 0, fmt.Errorf("METRIC_BUDGET_EXCEEDED: exponential buckets")
			}
			for _, value := range side.Counts {
				x, e := Uint(value)
				if e != nil {
					return 0, e
				}
				total.Add(total, x)
				n++
			}
		}
	}
	if total.Cmp(count) != 0 {
		return 0, fmt.Errorf("METRIC_HISTOGRAM_COUNT_MISMATCH")
	}
	return n, nil
}

func provenanceKey(source Source, position string) string {
	if source.ExportID != "" {
		return Hash([]string{"declared_export", source.Format, source.Producer, source.ProducerVersion, source.ExportID, position})
	}
	return Hash([]string{"original_bytes", source.SHA256, position})
}

// Normalize removes only provenance-qualified duplicate points and retains every
// original reference. A descriptor's origin prevents OTLP/range copy merging.
func Normalize(ctx context.Context, s Set) (Set, error) {
	unique := []Point{}
	seen := map[string]int{}
	source := map[string]Source{}
	for _, v := range s.Sources {
		source[v.ID] = v
	}
	for _, p := range s.Points {
		if e := ctx.Err(); e != nil {
			return s, e
		}
		p.ID = PointID(p)
		if len(p.Refs) != 1 {
			return s, fmt.Errorf("METRIC_NORMALIZE_REFERENCE_INVALID")
		}
		ref := p.Refs[0]
		v := source[ref.SourceID]
		provenance := provenanceKey(v, ref.Position)
		key := Hash([]any{provenance, payloadID(p)})
		if index, ok := seen[key]; ok {
			unique[index].Refs = append(unique[index].Refs, ref)
			s.Duplicates++
		} else {
			seen[key] = len(unique)
			unique = append(unique, p)
		}
	}
	for i := range unique {
		unique[i].ID = PointID(unique[i])
	}
	s.Points = unique
	sort.Slice(s.Series, func(i, j int) bool { return s.Series[i].ID < s.Series[j].ID })
	if e := s.Validate(ctx); e != nil {
		return s, e
	}
	return s, nil
}
func (s Set) SeriesByID(id string) (Series, bool) {
	for _, v := range s.Series {
		if v.ID == id {
			return v, true
		}
	}
	return Series{}, false
}
func CopyRaw(raw json.RawMessage) json.RawMessage { return append(json.RawMessage(nil), raw...) }
func IntegerText(raw json.RawMessage, signed bool) (string, error) {
	text := strings.TrimSpace(string(raw))
	if len(text) > 1 && text[0] == '"' {
		if e := json.Unmarshal(raw, &text); e != nil {
			return "", e
		}
	}
	if text == "" {
		return "", fmt.Errorf("METRIC_INTEGER_MISSING")
	}
	if signed {
		v, e := strconv.ParseInt(text, 10, 64)
		if e != nil {
			return "", e
		}
		return strconv.FormatInt(v, 10), nil
	}
	v, e := strconv.ParseUint(text, 10, 64)
	if e != nil {
		return "", e
	}
	return strconv.FormatUint(v, 10), nil
}
