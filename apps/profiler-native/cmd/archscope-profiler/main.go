// ─────────────────────────────────────────────────────────────────────
// [한글] archscope-profiler — profiler 분석 단일-바이너리 CLI 진입점.
//
// 책임/목적
//   profiler 4개 입력 형식 (collapsed / Jennifer CSV / flamegraph SVG /
//   flamegraph HTML) 중 정확히 하나를 받아 AnalysisResult JSON 으로 출력
//   하는 stand-alone CLI. archscope-profiler-app(데스크톱) 의 sidecar 로도,
//   CI/스크립트의 단발 분석에도 동일하게 사용된다.
//
// CLI 옵션
//   - --collapsed / --jennifer-csv / --flamegraph-svg / --flamegraph-html
//     : 입력 파일 (정확히 1개 필수, 동시 지정 불가)
//   - --out                  : 결과 JSON 경로. 비우면 stdout.
//   - --interval-ms          : sample interval (default 100ms)
//   - --elapsed-sec          : 측정 시간(초). <0 이면 unset.
//   - --top-n                : top N 표 크기 (default 20)
//   - --profile-kind         : wall|cpu|lock (default "wall")
//   - --timeline-base-method : timeline 분석의 base method
//   - --debug-log            : parse 실패 시 portable JSON 로그 기록
//   - --debug-log-dir        : 로그 출력 디렉토리 (default ./archscope-debug)
//
// 흐름
//   1) 플래그 파싱 / 상호배타 검증.
//   2) 입력 형식별 (analyzerType, parserName, sourceFile) 결정.
//   3) --debug-log 가 켜져 있으면 DebugLog 인스턴스 생성.
//   4) Options 조립 후 AnalyzeXxxFile 호출 (입력별로 분기).
//   5) JSON 직렬화 후 stdout 또는 파일에 출력. 에러는 exit code 1 + stderr.
// ─────────────────────────────────────────────────────────────────────

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler"
)

// [한글] main — 플래그 파싱 → 입력 형식별 분석기 호출 → JSON 출력.
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

// [한글] fail — stderr 에 prefix 와 함께 메시지 출력 후 exit(1).
func fail(message string) {
	fmt.Fprintln(os.Stderr, "archscope-profiler:", message)
	os.Exit(1)
}
