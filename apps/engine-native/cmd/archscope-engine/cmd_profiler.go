// `profiler` group — stub. The async-profiler / Jennifer / flamegraph
// analyzers are still Python-only; they will live in apps/profiler-native
// once the desktop shell takes over (T-352 follow-up). The group is
// registered here only so it shows up in `--help` and so users get a
// clear pointer rather than "unknown command".
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `profiler` 명령 그룹 — 스텁(stub) 구현.
//
// 왜 스텁인가?
//   async-profiler collapsed/SVG/HTML 파싱과 drilldown/breakdown 는
//   현재 Python 측에만 구현되어 있고, Go 포트는 데스크톱 셸
//   (apps/profiler-native) 이 직접 흡수하는 방향(T-352 follow-up)으로
//   계획되어 있습니다. 따라서 engine-native CLI 에서는 의도적으로
//   "이 분석기는 여기 없다" 는 명시적 안내 메시지를 반환합니다.
//
// 왜 그래도 등록은 하는가?
//   사용자가 `archscope-engine --help` 또는 `archscope-engine profiler`
//   를 실행했을 때 "unknown command" 가 아닌, 어디로 가야 하는지를
//   알려주기 위함입니다(`python -m archscope_engine.cli profiler` 안내).
//   이는 parity gate 의 stderr 형식 비교에서도 "동일한 길잡이 메시지"
//   를 유지하는 데 도움이 됩니다.
//
// stub 헬퍼
//   stub("name", "short") 는 같은 RunE — 즉 "profilerStubMessage 를
//   에러로 반환" — 만 가지는 cobra.Command 를 반복 생성하는 팩토리
//   클로저입니다. 6개의 리프(analyze-collapsed/-svg/-html/-jennifer-csv/
//   drilldown/breakdown)를 한 줄씩 등록.
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const profilerStubMessage = "profiler analyzers live in apps/profiler-native (T-352 follow-up); engine-native does not ship them"

func init() {
	group := &cobra.Command{
		Use:   "profiler",
		Short: "Profiler analysis commands (not shipped by engine-native).",
		Long: `Profiler analysis is provided by the desktop shell at
apps/profiler-native (T-352 follow-up). engine-native intentionally
does not ship these analyzers — for command-line profiler analysis,
use ` + "`python -m archscope_engine.cli profiler`" + `.`,
	}

	stub := func(use, short string) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("%s", profilerStubMessage)
			},
		}
	}

	group.AddCommand(stub("analyze-collapsed", "Analyze a collapsed-stack profile (not in engine-native)."))
	group.AddCommand(stub("analyze-flamegraph-svg", "Analyze a FlameGraph SVG (not in engine-native)."))
	group.AddCommand(stub("analyze-flamegraph-html", "Analyze a FlameGraph HTML wrapper (not in engine-native)."))
	group.AddCommand(stub("analyze-jennifer-csv", "Analyze a Jennifer APM CSV (not in engine-native)."))
	group.AddCommand(stub("drilldown", "Apply drill-down filters (not in engine-native)."))
	group.AddCommand(stub("breakdown", "Compute execution breakdown (not in engine-native)."))

	rootCmd.AddCommand(group)
}
