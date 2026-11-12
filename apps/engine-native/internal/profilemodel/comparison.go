package profilemodel

import (
	"encoding/json"
	"sort"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type ComparisonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ComparisonError) Error() string { return e.Code + ": " + e.Message }
func comparisonRefusal(code, message string) error {
	return &ComparisonError{Code: code, Message: message}
}

type comparisonQualityClass struct {
	ClockConfidence string
	Symbols         string
	ConversionLoss  []string
}
type comparisonEntity struct{ ID, Process, Thread, Worker string }
type comparisonClock struct{ ID, Clock, TimeUnit, TimeScale string }
type comparisonConditions struct {
	Revision  [4]string
	Selection Selection
	Entities  []comparisonEntity
	Clocks    []comparisonClock
	Capture   any
	Quality   comparisonQualityClass
}

func (s ProfileSet) comparisonConditions(selection Selection) comparisonConditions {
	selection = CopySelection(selection)
	ids := append([]string(nil), selection.ProfileIDs...)
	sort.Strings(ids)
	selection.ProfileIDs = nil
	if len(selection.Path) == 0 {
		selection.Path = nil
		selection.Match = ""
	}
	if len(selection.Filters) == 0 {
		selection.Filters = nil
	}
	symbols := "unknown"
	if s.Quality.SymbolResolution != nil {
		symbols = "resolved"
		if n, e := Number(*s.Quality.SymbolResolution); e == nil && n.Sign() == 0 {
			symbols = "unresolved"
		}
	}
	losses := append([]string(nil), s.Quality.ConversionLoss...)
	sort.Strings(losses)
	c := comparisonConditions{Revision: [4]string{s.SchemaVersion, s.Producer, s.ProducerVersion, s.Format}, Selection: selection, Quality: comparisonQualityClass{ClockConfidence: s.Quality.ClockConfidence, Symbols: symbols, ConversionLoss: losses}}
	_ = wirejson.Decode(s.CaptureOptions, &c.Capture)
	for _, id := range ids {
		for _, p := range s.Profiles {
			if p.ID == id {
				c.Entities = append(c.Entities, comparisonEntity{p.ID, p.Process, p.Thread, p.Worker})
				c.Clocks = append(c.Clocks, comparisonClock{p.ID, p.Clock, p.TimeUnit, p.TimeScale})
			}
		}
	}
	return c
}

func sameConditions(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}
func comparable(a ProfileSet, as Selection, b ProfileSet, bs Selection) error {
	x, y := a.comparisonConditions(as), b.comparisonConditions(bs)
	if x.Revision != y.Revision {
		return comparisonRefusal("semantic_revision_mismatch", "producer, normalizer or model revision differs")
	}
	if !sameConditions(x.Selection, y.Selection) {
		return comparisonRefusal("selection_mismatch", "metric or ordered frame selection rules differ")
	}
	if !sameConditions(x.Entities, y.Entities) {
		return comparisonRefusal("entity_mapping_required", "profile/process/thread/worker identities differ; explicit mapping is required")
	}
	if !sameConditions(x.Clocks, y.Clocks) {
		return comparisonRefusal("clock_class_mismatch", "profile clock kind/unit/scale differs")
	}
	if !sameConditions(x.Quality, y.Quality) {
		return comparisonRefusal("quality_class_mismatch", "clock confidence, symbol availability or conversion-loss categories differ")
	}
	ac, ap, _ := a.SourceContract()
	bc, bp, _ := b.SourceContract()
	if ap != bp || ap && ac.Revision != bc.Revision {
		return comparisonRefusal("semantic_revision_mismatch", "source normalizer revision differs")
	}
	if !sameConditions(a.sourceMeasurement(as.MetricID), b.sourceMeasurement(bs.MetricID)) {
		return comparisonRefusal("source_measurement_mismatch", "original source sample type/unit or format revision differs; reimport compatible originals")
	}
	pa, pb := a.CapturePolicy(as.MetricID), b.CapturePolicy(bs.MetricID)
	if (pa == nil) != (pb == nil) || pa != nil && pa.SHA256 != pb.SHA256 {
		return comparisonRefusal("capture_policy_mismatch", "compiled capture policies differ")
	}
	if pa != nil {
		x.Capture = []any{pa.Semantic, pa.StrictUnknown}
		y.Capture = []any{pb.Semantic, pb.StrictUnknown}
	}
	if !sameConditions(x.Capture, y.Capture) {
		return comparisonRefusal("capture_options_mismatch", "declared capture options differ")
	}
	return nil
}
func copyQuality(q Quality) Quality {
	clone := func(v *string) *string {
		if v == nil {
			return nil
		}
		s := *v
		return &s
	}
	q.Lost, q.Unknown, q.Truncated, q.SymbolResolution = clone(q.Lost), clone(q.Unknown), clone(q.Truncated), clone(q.SymbolResolution)
	if q.ConversionLoss != nil {
		q.ConversionLoss = append([]string{}, q.ConversionLoss...)
	}
	return q
}
func qualityDifferences(a, b Quality) []string {
	out := []string{}
	for _, p := range []struct {
		name        string
		left, right *string
	}{{"lost", a.Lost, b.Lost}, {"unknown", a.Unknown, b.Unknown}, {"truncated", a.Truncated, b.Truncated}, {"symbol_resolution", a.SymbolResolution, b.SymbolResolution}} {
		if !sameQualityMeasurement(p.left, p.right) {
			out = append(out, p.name)
		}
	}
	return out
}

// Preserve the original text in references, but compare measured quantities
// exactly so formatting-only changes (0.5 vs 0.50) do not create a warning.
func sameQualityMeasurement(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	left, le := Number(*a)
	right, re := Number(*b)
	return le == nil && re == nil && left.Cmp(right) == 0
}
