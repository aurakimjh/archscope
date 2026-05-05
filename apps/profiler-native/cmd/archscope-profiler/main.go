package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler"
)

func main() {
	collapsed := flag.String("collapsed", "", "Path to an async-profiler collapsed stack file.")
	out := flag.String("out", "", "Optional output JSON path. Defaults to stdout.")
	intervalMS := flag.Float64("interval-ms", 100, "Sample interval in milliseconds.")
	elapsedSec := flag.Float64("elapsed-sec", -1, "Optional elapsed seconds. Negative means unset.")
	topN := flag.Int("top-n", 20, "Number of top rows to emit.")
	profileKind := flag.String("profile-kind", "wall", "Profile capture mode: wall, cpu, or lock.")
	timelineBaseMethod := flag.String("timeline-base-method", "", "Optional base method for timeline analysis.")
	flag.Parse()

	if *collapsed == "" {
		fail("--collapsed is required")
	}
	if *intervalMS <= 0 {
		fail("--interval-ms must be positive")
	}
	if *topN <= 0 {
		fail("--top-n must be positive")
	}
	if *profileKind != "wall" && *profileKind != "cpu" && *profileKind != "lock" {
		fail("--profile-kind must be one of: wall, cpu, lock")
	}

	var elapsed *float64
	if *elapsedSec >= 0 {
		elapsed = elapsedSec
	}
	result, err := profiler.AnalyzeCollapsedFile(*collapsed, profiler.Options{
		IntervalMS:         *intervalMS,
		ElapsedSec:         elapsed,
		TopN:               *topN,
		ProfileKind:        *profileKind,
		TimelineBaseMethod: *timelineBaseMethod,
	})
	if err != nil {
		fail(err.Error())
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	payload = append(payload, '\n')
	if *out == "" {
		_, _ = os.Stdout.Write(payload)
		return
	}
	if err := os.WriteFile(*out, payload, 0o644); err != nil {
		fail(err.Error())
	}
	fmt.Fprintf(os.Stderr, "Wrote profiler result to %s\n", *out)
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "archscope-profiler:", message)
	os.Exit(1)
}
