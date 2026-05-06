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
	jenniferCSV := flag.String("jennifer-csv", "", "Path to a Jennifer APM flamegraph CSV file.")
	flamegraphSVG := flag.String("flamegraph-svg", "", "Path to a FlameGraph.pl/async-profiler SVG flamegraph.")
	flamegraphHTML := flag.String("flamegraph-html", "", "Path to an async-profiler HTML / inline-SVG-wrapped HTML flamegraph.")
	out := flag.String("out", "", "Optional output JSON path. Defaults to stdout.")
	intervalMS := flag.Float64("interval-ms", 100, "Sample interval in milliseconds.")
	elapsedSec := flag.Float64("elapsed-sec", -1, "Optional elapsed seconds. Negative means unset.")
	topN := flag.Int("top-n", 20, "Number of top rows to emit.")
	profileKind := flag.String("profile-kind", "wall", "Profile capture mode: wall, cpu, or lock.")
	timelineBaseMethod := flag.String("timeline-base-method", "", "Optional base method for timeline analysis.")
	debugLog := flag.Bool("debug-log", false, "Write a portable debug log on parse errors.")
	debugLogDir := flag.String("debug-log-dir", "", "Directory for debug log output (default: ./archscope-debug/).")
	flag.Parse()

	inputs := 0
	if *collapsed != "" {
		inputs++
	}
	if *jenniferCSV != "" {
		inputs++
	}
	if *flamegraphSVG != "" {
		inputs++
	}
	if *flamegraphHTML != "" {
		inputs++
	}
	if inputs == 0 {
		fail("one of --collapsed / --jennifer-csv / --flamegraph-svg / --flamegraph-html is required")
	}
	if inputs > 1 {
		fail("--collapsed, --jennifer-csv, --flamegraph-svg, --flamegraph-html are mutually exclusive")
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

	// Determine analyzer type and source file for debug log.
	var analyzerType, sourceFile, parserName string
	switch {
	case *jenniferCSV != "":
		analyzerType, sourceFile, parserName = "profiler_jennifer", *jenniferCSV, "jennifer_flamegraph_csv"
	case *flamegraphSVG != "":
		analyzerType, sourceFile, parserName = "profiler_collapsed", *flamegraphSVG, "flamegraph_svg"
	case *flamegraphHTML != "":
		analyzerType, sourceFile, parserName = "profiler_collapsed", *flamegraphHTML, "flamegraph_html"
	default:
		analyzerType, sourceFile, parserName = "profiler_collapsed", *collapsed, "async_profiler_collapsed"
	}

	var dl *profiler.DebugLog
	if *debugLog {
		dl = profiler.NewDebugLog(analyzerType, parserName, sourceFile)
	}

	options := profiler.Options{
		IntervalMS:         *intervalMS,
		ElapsedSec:         elapsed,
		TopN:               *topN,
		ProfileKind:        *profileKind,
		TimelineBaseMethod: *timelineBaseMethod,
		DebugLog:           dl,
		DebugLogDir:        *debugLogDir,
	}
	var (
		result profiler.AnalysisResult
		err    error
	)
	switch {
	case *jenniferCSV != "":
		result, err = profiler.AnalyzeJenniferFile(*jenniferCSV, options)
	case *flamegraphSVG != "":
		result, err = profiler.AnalyzeFlamegraphSVGFile(*flamegraphSVG, options)
	case *flamegraphHTML != "":
		result, err = profiler.AnalyzeFlamegraphHTMLFile(*flamegraphHTML, options)
	default:
		result, err = profiler.AnalyzeCollapsedFile(*collapsed, options)
	}
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
