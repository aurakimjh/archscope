package profilemodel

import (
	"unicode/utf8"
)

// checkStrings also bounds direct Go callers which have not passed Decode's
// 64 MiB wire limit. Count referenced strings, even if they share allocations.
func (s ProfileSet) checkStrings() error {
	size := 0
	add := func(values ...string) bool {
		for _, v := range values {
			if len(v) > 32<<20-size || !utf8.ValidString(v) {
				return false
			}
			size += len(v)
		}
		return true
	}
	if !add(s.Producer, s.ProducerVersion, s.Format, string(s.CaptureOptions)) {
		return budget("model string budget or UTF-8 invalid")
	}
	if !add(s.SchemaVersion) || !add(s.SourceHashes...) || !add(s.Quality.ClockConfidence) || !add(s.Quality.ConversionLoss...) {
		return budget("source/quality string budget exceeded")
	}
	for _, count := range []*string{s.Quality.Lost, s.Quality.Unknown, s.Quality.Truncated, s.Quality.SymbolResolution} {
		if count != nil && !add(*count) {
			return budget("quality count budget exceeded")
		}
	}
	for _, metric := range s.Metrics {
		if !add(metric.ID, metric.Kind, metric.BaseKind, metric.Unit, metric.Scale) {
			return budget("metric string budget exceeded")
		}
	}

	for _, f := range s.Frames {
		if !add(f.ID, f.Symbol, f.Display, f.Module, f.BuildID, f.File, f.Runtime, f.Language) {
			return budget("frame string budget exceeded")
		}
	}
	for _, stack := range s.Stacks {
		if !add(stack.ID) || !add(stack.Frames...) {
			return budget("stack string budget exceeded")
		}
	}
	for _, p := range s.Profiles {
		if !add(p.ID, p.Process, p.Thread, p.Worker, p.Clock, p.TimeUnit, p.TimeScale) {
			return budget("profile string budget exceeded")
		}
		for _, sample := range p.Samples {
			if !add(sample.StackID, sample.MetricID, sample.Weight, sample.SourceRef) {
				return budget("sample string budget exceeded")
			}
			for key, values := range sample.Labels {
				if !add(key) || !add(values...) {
					return budget("label string budget exceeded")
				}
			}
		}
	}
	for key, value := range s.Extensions {
		if !add(key, string(value)) {
			return budget("model extension budget exceeded")
		}
	}
	return nil
}
