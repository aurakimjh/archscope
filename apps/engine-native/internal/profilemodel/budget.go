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
			if len(v) > 32<<20-size {
				size += len(v)
				return false
			}
			if !utf8.ValidString(v) {
				return false
			}
			size += len(v)
		}
		return true
	}
	if !add(s.Producer, s.ProducerVersion, s.Format, string(s.CaptureOptions)) {
		return stringBudgetError(size, "model string budget or UTF-8 invalid")
	}
	if !add(s.SchemaVersion) || !add(s.SourceHashes...) || !add(s.Quality.ClockConfidence) || !add(s.Quality.ConversionLoss...) {
		return stringBudgetError(size, "source/quality string budget exceeded")
	}
	for _, count := range []*string{s.Quality.Lost, s.Quality.Unknown, s.Quality.Truncated, s.Quality.SymbolResolution} {
		if count != nil && !add(*count) {
			return stringBudgetError(size, "quality count budget exceeded")
		}
	}
	for _, metric := range s.Metrics {
		if !add(metric.ID, metric.Kind, metric.BaseKind, metric.Unit, metric.Scale) {
			return stringBudgetError(size, "metric string budget exceeded")
		}
	}

	for _, f := range s.Frames {
		if !add(f.ID, f.Symbol, f.Display, f.Module, f.BuildID, f.File, f.Runtime, f.Language) {
			return stringBudgetError(size, "frame string budget exceeded")
		}
	}
	for _, stack := range s.Stacks {
		if !add(stack.ID) || !add(stack.Frames...) {
			return stringBudgetError(size, "stack string budget exceeded")
		}
	}
	for _, p := range s.Profiles {
		if !add(p.ID, p.Process, p.Thread, p.Worker, p.Clock, p.TimeUnit, p.TimeScale) {
			return stringBudgetError(size, "profile string budget exceeded")
		}
		for _, sample := range p.Samples {
			if !add(sample.StackID, sample.MetricID, sample.Weight, sample.SourceRef) {
				return stringBudgetError(size, "sample string budget exceeded")
			}
			for key, values := range sample.Labels {
				if !add(key) || !add(values...) {
					return stringBudgetError(size, "label string budget exceeded")
				}
			}
		}
	}
	for key, value := range s.Extensions {
		if !add(key, string(value)) {
			return stringBudgetError(size, "model extension budget exceeded")
		}
	}
	return nil
}

func stringBudgetError(size int, fallback string) error {
	if size > 32<<20 {
		return limit("model_strings", int64(size), 32<<20)
	}
	return budget(fallback)
}
