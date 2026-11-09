package profiler

import (
	"fmt"
	"math"
)

func ValidateValueUnit(unit string) error {
	switch unit {
	case "", "samples", "nanoseconds", "microseconds", "milliseconds", "seconds", "bytes", "unknown":
		return nil
	default:
		return fmt.Errorf("unsupported profile value unit %q", unit)
	}
}

// A lock sampling threshold is not a periodic sampling interval. A count of
// contended locks cannot be turned into wait duration by multiplying IntervalMS.
func valueContract(o Options) (unit, aggregation, reason string, secondsPerWeight *float64) {
	unit = o.ValueUnit
	if unit == "" {
		if o.ProfileKind == "lock" {
			unit = "unknown"
		} else {
			unit = "samples"
		}
	}
	factor := 0.0
	aggregation = "total_time"
	switch unit {
	case "samples":
		aggregation = "sample_count"
		if o.ProfileKind == "lock" {
			return unit, aggregation, "lock_sample_count_has_no_duration", nil
		}
		factor = o.IntervalMS / 1000
	case "nanoseconds":
		factor = 1e-9
	case "microseconds":
		factor = 1e-6
	case "milliseconds":
		factor = 1e-3
	case "seconds":
		factor = 1
	case "bytes":
		return unit, "total_value", "non_time_weight", nil
	default:
		return "unknown", "unknown", "unknown_weight_unit", nil
	}
	if factor <= 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		return unit, aggregation, "invalid_sampling_interval", nil
	}
	return unit, aggregation, "", &factor
}

func weightDuration(weight int, options Options) *float64 {
	_, _, _, factor := valueContract(options)
	if factor == nil {
		return nil
	}
	value := round(float64(weight)**factor, 9)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return &value
}

func ExportToPprofWithOptions(stacks map[string]int, out string, options Options) error {
	normalizeOptions(&options)
	if err := ValidateValueUnit(options.ValueUnit); err != nil {
		return err
	}
	unit, _, _, factor := valueContract(options)
	if unit == "samples" {
		interval := 0.0
		if factor != nil {
			interval = options.IntervalMS
		}
		return ExportToPprof(stacks, out, "samples", "count", interval)
	}
	// Preserve weighted values and their original units. Adding an interval
	// duration column here would multiply an already accumulated duration.
	return ExportToPprof(stacks, out, options.ProfileKind, unit, 0)
}
