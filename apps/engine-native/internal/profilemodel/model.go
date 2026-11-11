// Package profilemodel is the exact, producer-independent profile core. Wire
// weights are decimal strings; no renderer or legacy integer DTO owns this data.
package profilemodel

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const Version = "1.0.0"

// Operational allocation ceilings; not diagnostic thresholds.
const MaxFrames = 100_000
const MaxSamples = 100_000
const MaxDepth = 1024
const MaxNodes = 200_000
const MaxTreeBytes = 64 << 20

type Frame struct {
	ID       string `json:"id"`
	Symbol   string `json:"symbol"`
	Display  string `json:"display"`
	Module   string `json:"module,omitempty"`
	BuildID  string `json:"build_id,omitempty"`
	File     string `json:"file,omitempty"`
	Line     *int64 `json:"line,omitempty"`
	Runtime  string `json:"runtime,omitempty"`
	Language string `json:"language,omitempty"`
	Async    bool   `json:"async,omitempty"`
	Inline   bool   `json:"inline,omitempty"`
	Native   bool   `json:"native,omitempty"`
	Kernel   bool   `json:"kernel,omitempty"`
}
type Stack struct {
	ID     string   `json:"id"`
	Frames []string `json:"frames"`
}
type Metric struct {
	BaseKind string `json:"base_kind,omitempty"`
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Unit     string `json:"unit"`
	// Scale maps a stored weight to the declared unit. It is explicit and exact.
	Scale  string `json:"scale"`
	Signed bool   `json:"signed"`
}
type Sample struct {
	TimestampNull bool                `json:"-"`
	IntervalNull  bool                `json:"-"`
	StackID       string              `json:"stack_id"`
	MetricID      string              `json:"metric_id"`
	Weight        string              `json:"weight"`
	Timestamp     *string             `json:"timestamp,omitempty"`
	Interval      *string             `json:"interval,omitempty"`
	Labels        map[string][]string `json:"labels,omitempty"`
	SourceRef     string              `json:"source_ref,omitempty"`
}
type Profile struct {
	TimeUnit  string   `json:"time_unit,omitempty"`
	TimeScale string   `json:"time_scale,omitempty"`
	ID        string   `json:"id"`
	Process   string   `json:"process,omitempty"`
	Thread    string   `json:"thread,omitempty"`
	Worker    string   `json:"worker,omitempty"`
	Clock     string   `json:"clock"`
	Samples   []Sample `json:"samples"`
}
type Quality struct {
	Lost             *string  `json:"lost"`
	Unknown          *string  `json:"unknown"`
	Truncated        *string  `json:"truncated"`
	SymbolResolution *string  `json:"symbol_resolution"`
	ClockConfidence  string   `json:"clock_confidence"`
	ConversionLoss   []string `json:"conversion_loss"`
}
type ProfileSet struct {
	Extensions      map[string]json.RawMessage `json:"extensions,omitempty"`
	SchemaVersion   string                     `json:"schema_version"`
	Producer        string                     `json:"producer"`
	ProducerVersion string                     `json:"producer_version,omitempty"`
	Format          string                     `json:"format"`
	SourceHashes    []string                   `json:"source_sha256"`
	CaptureOptions  json.RawMessage            `json:"capture_options"`
	Frames          []Frame                    `json:"frames"`
	Stacks          []Stack                    `json:"stacks"`
	Metrics         []Metric                   `json:"metrics"`
	Profiles        []Profile                  `json:"profiles"`
	Quality         Quality                    `json:"quality"`
}
type Selection struct {
	Filters    []PathFilter `json:"filters,omitempty"`
	ProfileIDs []string     `json:"profile_ids"`
	MetricID   string       `json:"metric_id"`
	// Ordered frame IDs, not display names; recursive occurrences stay distinct.
	Path  []string `json:"path,omitempty"`
	Match string   `json:"match,omitempty"` // prefix or contains; empty only without path
}

var decimal = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var digest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func Number(text string) (*big.Rat, error) {
	if len(text) > 1024 || !decimal.MatchString(text) {
		return nil, fmt.Errorf("invalid exact decimal")
	}
	if dot := strings.IndexByte(text, '.'); dot >= 0 && len(text)-dot-1 > 128 {
		return nil, budget("decimal scale budget exceeded")
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("invalid exact decimal")
	}
	return value, nil
}

// Text renders only finite decimal results. Ratios use RatString separately.
func Text(value *big.Rat) string {
	denominator := new(big.Int).Set(value.Denom())
	two, five := big.NewInt(2), big.NewInt(5)
	a, b := 0, 0
	for new(big.Int).Mod(denominator, two).Sign() == 0 {
		denominator.Quo(denominator, two)
		a++
	}
	for new(big.Int).Mod(denominator, five).Sign() == 0 {
		denominator.Quo(denominator, five)
		b++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		panic("nondecimal profile weight")
	}
	text := value.FloatString(max(a, b))
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "-0" {
		text = "0"
	}
	return text
}
func hash(prefix string, value any) string {
	body, _ := json.Marshal(value)
	sum := sha256.Sum256(body)
	return prefix + hex.EncodeToString(sum[:])
}
func FrameID(f Frame) string         { f.ID = ""; f.Display = ""; return hash("frame-", f) }
func StackID(frames []string) string { return hash("stack-", frames) }
func (s ProfileSet) Validate() error {
	for _, bound := range []struct {
		resource        string
		actual, ceiling int
	}{
		{"frames", len(s.Frames), MaxFrames}, {"stacks", len(s.Stacks), MaxSamples}, {"profiles", len(s.Profiles), 1024}, {"source_hashes", len(s.SourceHashes), 1024}, {"metrics", len(s.Metrics), 64}, {"extensions", len(s.Extensions), 128}, {"quality_categories", len(s.Quality.ConversionLoss), 1024},
	} {
		if bound.actual > bound.ceiling {
			return limit(bound.resource, int64(bound.actual), int64(bound.ceiling))
		}
	}

	if s.SchemaVersion != Version || s.Producer == "" || s.Format == "" || len(s.SourceHashes) == 0 || len(s.SourceHashes) > 1024 || len(s.Frames) > MaxFrames || len(s.Stacks) > MaxSamples || len(s.Profiles) == 0 || len(s.Profiles) > 1024 {
		return budget("invalid ProfileSet header or budget")
	}
	references := 0
	for _, stack := range s.Stacks {
		references += len(stack.Frames)
		if len(stack.Frames) > MaxDepth {
			return limit("stack_depth", int64(len(stack.Frames)), MaxDepth)
		}
		if references > 1_000_000 {
			return limit("frame_references", int64(references), 1_000_000)
		}
	}
	records := 0
	for _, p := range s.Profiles {
		records += len(p.Samples)
		if records > MaxSamples {
			return limit("samples", int64(records), MaxSamples)
		}
	}
	if len(s.Extensions) > 128 || len(s.Quality.ConversionLoss) > 1024 {
		return budget("extension/quality budget exceeded")
	}
	if e := s.checkStrings(); e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, h := range s.SourceHashes {
		if !digest.MatchString(h) || seen[h] {
			return fmt.Errorf("invalid source hash set")
		}
		seen[h] = true
	}
	var options map[string]any
	if len(s.CaptureOptions) > 64<<10 || wirejson.Decode(s.CaptureOptions, &options) != nil || options == nil {
		return fmt.Errorf("capture_options must be a bounded object")
	}
	frames := map[string]bool{}
	for _, f := range s.Frames {
		if f.ID != FrameID(f) || frames[f.ID] || f.Symbol == "" || f.Display == "" || (f.Line != nil && *f.Line < 0) {
			return fmt.Errorf("invalid frame identity")
		}
		frames[f.ID] = true
	}
	stacks := map[string]bool{}
	for _, stack := range s.Stacks {
		if stack.ID != StackID(stack.Frames) || stacks[stack.ID] || len(stack.Frames) == 0 || len(stack.Frames) > MaxDepth {
			return fmt.Errorf("invalid stack identity/depth")
		}
		for _, id := range stack.Frames {
			if !frames[id] {
				return fmt.Errorf("unknown frame")
			}
		}
		stacks[stack.ID] = true
	}
	metrics := map[string]Metric{}
	for _, m := range s.Metrics {
		scale, e := Number(m.Scale)
		if m.ID == "" || !validMetric(m) || e != nil || scale.Sign() <= 0 {
			return fmt.Errorf("invalid metric")
		}
		if _, ok := metrics[m.ID]; ok {
			return fmt.Errorf("duplicate metric")
		}
		metrics[m.ID] = m
	}
	if len(metrics) == 0 || len(metrics) > 64 {
		return budget("metric budget exceeded")
	}
	profiles := map[string]bool{}
	count := 0
	for _, p := range s.Profiles {
		if p.ID == "" || p.Samples == nil || profiles[p.ID] || (p.Clock != "aggregate" && p.Clock != "relative" && p.Clock != "absolute") {
			return fmt.Errorf("invalid profile identity/clock")
		}
		profiles[p.ID] = true
		for _, sample := range p.Samples {
			if (sample.TimestampNull && sample.Timestamp != nil) || (sample.IntervalNull && sample.Interval != nil) {
				return fmt.Errorf("contradictory sample time presence")
			}
			count++
			if count > MaxSamples {
				return limit("samples", int64(count), MaxSamples)
			}
			metric, ok := metrics[sample.MetricID]
			weight, e := Number(sample.Weight)
			if !ok || !stacks[sample.StackID] || e != nil || (!metric.Signed && weight.Sign() < 0) {
				return fmt.Errorf("invalid sample weight/reference")
			}
			if p.Clock == "aggregate" && (sample.Timestamp != nil || sample.Interval != nil) {
				return fmt.Errorf("aggregate profile has no event chronology")
			}
			if sample.Timestamp != nil || sample.Interval != nil {
				timeMetric := Metric{Kind: "duration", Unit: p.TimeUnit, Scale: p.TimeScale}
				scale, e := Number(p.TimeScale)
				if !validMetric(timeMetric) || e != nil || scale.Sign() <= 0 {
					return fmt.Errorf("explicit sample clock unit/scale required")
				}
			}
			for _, point := range []*string{sample.Timestamp, sample.Interval} {
				if point != nil {
					v, e := Number(*point)
					if e != nil || (v.Sign() < 0 && (point == sample.Interval || p.Clock != "absolute")) {
						return fmt.Errorf("invalid sample time")
					}
				}
			}
		}
	}
	if s.Quality.ClockConfidence == "" {
		return fmt.Errorf("explicit clock quality required")
	}
	for _, count := range []*string{s.Quality.Lost, s.Quality.Unknown, s.Quality.Truncated} {
		if count != nil {
			n, e := Number(*count)
			if e != nil || n.Sign() < 0 || !n.IsInt() {
				return fmt.Errorf("invalid quality count")
			}
		}
	}
	if s.Quality.SymbolResolution != nil {
		n, e := Number(*s.Quality.SymbolResolution)
		if e != nil || n.Sign() < 0 || n.Cmp(big.NewRat(1, 1)) > 0 {
			return fmt.Errorf("invalid symbol resolution")
		}
	}
	return nil
}
func Decode(body []byte) (ProfileSet, error) {
	var s ProfileSet

	var fields map[string]json.RawMessage
	if e := decodeModelJSON(body, &fields); e != nil {
		return s, e
	}
	for _, key := range []string{"schema_version", "producer", "format", "source_sha256", "capture_options", "frames", "stacks", "metrics", "profiles", "quality"} {
		raw, exists := fields[key]
		if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return s, fmt.Errorf("missing required ProfileSet field: %s", key)
		}
	}
	var quality map[string]json.RawMessage
	if e := decodeModelJSON(fields["quality"], &quality); e != nil {
		return s, e
	}
	for _, key := range []string{"lost", "unknown", "truncated", "symbol_resolution", "clock_confidence", "conversion_loss"} {
		if _, exists := quality[key]; !exists {
			return s, fmt.Errorf("missing required quality field: %s", key)
		}
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&s); e != nil {
		return s, e
	}
	return s, s.Validate()
}

func (s ProfileSet) Metric(id string) (Metric, bool) {
	for _, m := range s.Metrics {
		if m.ID == id {
			return m, true
		}
	}
	return Metric{}, false
}
func (s ProfileSet) ValidateSelection(selection Selection) error {
	if e := s.Validate(); e != nil {
		return e
	}
	if _, ok := s.Metric(selection.MetricID); !ok || len(selection.ProfileIDs) == 0 {
		return fmt.Errorf("explicit profile and metric selection required")
	}
	ids := map[string]bool{}
	for _, p := range s.Profiles {
		ids[p.ID] = true
	}
	seen := map[string]bool{}
	for _, id := range selection.ProfileIDs {
		if !ids[id] || seen[id] {
			return fmt.Errorf("unknown or duplicate selected profile")
		}
		seen[id] = true
	}
	frames := map[string]bool{}
	for _, f := range s.Frames {
		frames[f.ID] = true
	}
	if len(selection.Path) > MaxDepth {
		return budget("selection path budget exceeded")
	}
	for _, id := range selection.Path {
		if !frames[id] {
			return fmt.Errorf("unknown selected frame")
		}
	}
	if selection.Match != "" && selection.Match != "prefix" && selection.Match != "contains" {
		return fmt.Errorf("unknown path match mode")
	}
	if len(selection.Path) > 0 && selection.Match != "prefix" && selection.Match != "contains" {
		return fmt.Errorf("explicit path match mode required")
	}
	if len(selection.Filters) > 64 {
		return budget("filter budget exceeded")
	}
	for _, filter := range selection.Filters {
		if len(filter.Path) == 0 || len(filter.Path) > MaxDepth || (filter.Match != "prefix" && filter.Match != "contains") {
			return fmt.Errorf("invalid path filter")
		}
		for _, id := range filter.Path {
			if !frames[id] {
				return fmt.Errorf("unknown filter frame")
			}
		}
	}

	return nil
}
func (s ProfileSet) SelectionIdentity(selection Selection) string {
	return s.selectionIdentity(selection, true)
}

// ComparisonIdentity v2 compares conditions, not measured quality counters.
// Original selection/source identity remains unchanged for provenance.
func (s ProfileSet) ComparisonIdentity(selection Selection) string {
	return hash("population-", []any{"profile-conditions-v2", s.comparisonConditions(selection)})
}

// LegacyComparisonIdentity validates pre-v2 saved declarations without rewriting
// their hashes or granting the new scalar policy to an old snapshot.
func (s ProfileSet) LegacyComparisonIdentity(selection Selection) string {
	return s.selectionIdentity(selection, false)
}

func (s ProfileSet) selectionIdentity(selection Selection, withSources bool) string {
	selection = CopySelection(selection)
	if len(selection.Path) == 0 {
		selection.Path = nil
		selection.Match = ""
	}
	if len(selection.Filters) == 0 {
		selection.Filters = nil
	}
	sources := append([]string(nil), s.SourceHashes...)
	sort.Strings(sources)
	if !withSources {
		sources = nil
	}
	ids := append([]string(nil), selection.ProfileIDs...)
	sort.Strings(ids)
	profiles := []Profile{}
	for _, id := range ids {
		for _, p := range s.Profiles {
			if p.ID == id {
				p.Samples = nil
				profiles = append(profiles, p)
			}
		}
	}
	var opts any
	_ = wirejson.Decode(s.CaptureOptions, &opts)
	return hash("population-", []any{s.SchemaVersion, s.Producer, s.ProducerVersion, s.Format, s.Quality, sources, profiles, opts, selection.MetricID, selection.Path, selection.Match, selection.Filters})
}

func validMetric(m Metric) bool {
	kind := m.Kind
	if kind == "signed_difference" {
		if !m.Signed || m.BaseKind == "" {
			return false
		}
		kind = m.BaseKind
	} else if m.Signed || m.BaseKind != "" {
		return false
	}
	switch kind {
	case "sample_count", "count", "allocation_count":
		return m.Unit == "count" || m.Unit == "samples" || m.Unit == "objects"
	case "cpu_time", "wall_time", "duration":
		return m.Unit == "seconds" || m.Unit == "milliseconds" || m.Unit == "microseconds" || m.Unit == "nanoseconds" || m.Unit == "ticks"
	case "bytes":
		return m.Unit == "bytes"
	case "cycles":
		return m.Unit == "cycles"
	}
	return false
}

func DecodeSelection(body []byte) (Selection, error) {
	var selection Selection
	if len(body) > 64<<10 {
		return selection, budget("selection byte budget exceeded")
	}
	if e := wirejson.Decode(body, &selection); e != nil {
		return selection, e
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	return selection, decoder.Decode(&selection)
}

func DecodeFilters(body []byte) ([]PathFilter, error) {
	var filters []PathFilter
	if len(body) > 64<<10 {
		return nil, budget("filter byte budget exceeded")
	}
	if e := wirejson.Decode(body, &filters); e != nil {
		return nil, e
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&filters); e != nil {
		return nil, e
	}
	if len(filters) > 64 {
		return nil, budget("filter count budget exceeded")
	}
	return filters, nil
}

func CopySelection(value Selection) Selection {
	value.ProfileIDs = append([]string(nil), value.ProfileIDs...)
	value.Path = append([]string(nil), value.Path...)
	filters := make([]PathFilter, len(value.Filters))
	for i, filter := range value.Filters {
		filters[i] = filter
		filters[i].Path = append([]string(nil), filter.Path...)
	}
	if value.Filters != nil {
		value.Filters = filters
	}
	return value
}
