package gclog

import (
	"math"
	"strconv"
	"time"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gclog"
)

type observationClock struct {
	axis, reason string
	rateClock    string
	span         *float64
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Keep one coordinate system for the whole chart. Never fall back from a
// reset JVM uptime to calendar time and accidentally join two JVM sessions.
func observeClock(events []parser.Event) observationClock {
	c := observationClock{axis: "event_index", reason: "missing_time"}
	allUptime, allDates := len(events) > 0, len(events) > 0
	var lastUptime *float64
	reset := false
	for _, e := range events {
		allDates = allDates && e.Timestamp != nil
		valid := e.UptimeSec != nil && finite(*e.UptimeSec) && *e.UptimeSec >= 0
		allUptime = allUptime && valid
		if valid {
			if lastUptime != nil && *e.UptimeSec < *lastUptime {
				reset = true
			}
			lastUptime = e.UptimeSec
		}
	}
	if allDates {
		c.axis = "absolute"
	} else if allUptime {
		c.axis = "uptime_seconds"
	}
	if allUptime {
		c.rateClock = "uptime_seconds"
	} else if allDates {
		c.rateClock = "absolute"
	}
	if reset {
		c.reason = "clock_reset"
		return c
	}
	if len(events) < 2 {
		c.reason = "insufficient_events"
		return c
	}
	if !allUptime && !allDates {
		return c
	}
	span := 0.0
	for i := 1; i < len(events); i++ {
		dt := 0.0
		if allUptime {
			dt = *events[i].UptimeSec - *events[i-1].UptimeSec
		} else {
			dt = events[i].Timestamp.Sub(*events[i-1].Timestamp).Seconds()
		}
		if !finite(dt) || dt < 0 {
			c.reason = "clock_reset"
			return c
		}
		span += dt
	}
	if span <= 0 || !finite(span) {
		c.reason = "zero_time_span"
		return c
	}
	c.reason, c.span = "", &span
	return c
}

func (c observationClock) label(e parser.Event, index int) string {
	switch c.axis {
	case "absolute":
		return e.Timestamp.Format(time.RFC3339Nano)
	case "uptime_seconds":
		return formatFloat(*e.UptimeSec)
	default:
		return strconv.Itoa(index)
	}
}

func eventDelta(prev, curr *parser.Event) float64 {
	if prev.UptimeSec != nil && curr.UptimeSec != nil {
		dt := *curr.UptimeSec - *prev.UptimeSec
		if finite(dt) {
			return dt
		}
		return 0
	}
	if prev.Timestamp != nil && curr.Timestamp != nil {
		return curr.Timestamp.Sub(*prev.Timestamp).Seconds()
	}
	return 0
}

// Pause summaries include every parsed pause. Throughput uses the observed
// interval (first pause end, last pause end], so its numerator excludes the
// first pause, whose beginning is outside the measured window.
func windowThroughput(events []parser.Event, c observationClock) (any, any, string) {
	if c.span == nil {
		return nil, nil, c.reason
	}
	pause := 0.0
	for _, e := range events[1:] {
		if e.PauseMS == nil || !finite(*e.PauseMS) || *e.PauseMS < 0 {
			return nil, nil, "missing_pause"
		}
		pause += *e.PauseMS
	}
	if pause > *c.span*1000 {
		return nil, nil, "pause_exceeds_window"
	}
	return roundHalfEven((1-pause/(*c.span*1000))*100, 3), roundHalfEven(pause, 3), ""
}

func optionalAverage(values []float64) any {
	if len(values) == 0 {
		return nil
	}
	return avgFloat(values)
}

func rateUnavailable(values []float64, clockReason string) string {
	if clockReason != "" {
		return clockReason
	}
	if len(values) == 0 {
		return "insufficient_heap_measurements"
	}
	return ""
}

// Do not mix per-pair uptime deltas with a date-based observation window when
// only some records have uptime. Every rate in a result uses one common clock.
func (c observationClock) rateEvent(e parser.Event) parser.Event {
	if c.rateClock == "absolute" {
		e.UptimeSec = nil
	}
	return e
}
