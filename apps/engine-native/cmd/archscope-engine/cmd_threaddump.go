// `thread-dump` group — mirrors typer's thread_dump_app surface:
// analyze (single jstack), analyze-multi (multi-dump correlator),
// analyze-locks (lock contention), to-collapsed (FlameGraph collapsed
// stack format).
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `thread-dump` 명령 그룹 — 단일/멀티 thread dump 분석기들의
// CLI 표면.
//
// 4개 리프 명령
//   analyze        : 단일 Java jstack 덤프 분석. JVM 전용(다른 런타임은
//                    호환되지 않음 — multi 측 명령 사용).
//   analyze-multi  : 멀티 덤프 상관 분석. 5개 런타임 자동 감지.
//                    LONG_RUNNING_THREAD, PERSISTENT_BLOCKED_THREAD,
//                    LATENCY_SECTION_DETECTED 등의 finding 산출.
//   analyze-locks  : 단일/멀티 덤프에서 owner/waiter 그래프 + DFS
//                    데드락 탐지.
//   to-collapsed   : FlameGraph 호환 collapsed 스택 변환기.
//                    Go 측은 비교 자동화를 위해 JSON({stack: count}) 로
//                    출력하고, Python 측은 텍스트 ("<stack> <count>") 로
//                    출력 — parity gate 가 양 형식 모두 비교할 수 있도록
//                    설계되어 있습니다.
//
// 형식 자동 감지 (analyze-multi / analyze-locks / to-collapsed)
//   td.DefaultRegistry.ParseMany 가 각 입력 파일의 첫 4 KB 헤더를
//   plugin.CanParse 로 sniff 해 알맞은 thread-dump 플러그인에 라우팅.
//   사용자가 --format 으로 명시하면 sniff 를 건너뛰고 강제 지정.
//
// 멀티 입력 표기
//   --in 은 반복 또는 콤마 구분 모두 허용 (helpers.splitCommaSeparated).
//   하나라도 다른 포맷이면 ParseMany 가 MixedFormatError 로 즉시 거부
//   (--format 으로 우회 가능).
//
// 옵션 의미
//   --top-n      : 결과 테이블의 상위 N 행. 0=분석기 기본값.
//   --threshold  : analyze-multi 의 "연속 N개 덤프에서 동일 상태"
//                  finding 임계치. 기본값은 분석기가 결정.
//   --no-thread-name (to-collapsed): 합성 root 프레임으로 thread name 을
//                  prepend 할지 여부. 기본은 prepend(스레드별 색상 구분에
//                  유리). 끄면 동일 스택이 더 잘 합쳐져 통계가 두꺼워짐.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/lockcontention"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/multithread"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddump"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/threaddumpcollapsed"
	td "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
)

func init() {
	group := &cobra.Command{
		Use:   "thread-dump",
		Short: "Java thread dump analysis commands.",
		Long: `Analyze single or multi-snapshot thread dumps. The single-dump
` + "`analyze`" + ` leaf accepts only Java jstack input; the multi-dump
leaves (` + "`analyze-multi`, `analyze-locks`, `to-collapsed`" + `) auto-detect the
format via the registry's header sniffer (Java jstack, Java jcmd JSON,
Go goroutines, Node.js report, Python dump, .NET CLR).`,
	}

	// ── analyze ────────────────────────────────────────────────────
	{
		var (
			in   string
			topN int
			out  string
		)
		analyze := &cobra.Command{
			Use:   "analyze",
			Short: "Analyze a Java thread dump text file.",
			Long:  "Parse a single Java jstack dump and emit an AnalysisResult JSON envelope.",
			Example: `  archscope-engine thread-dump analyze \
    --in examples/thread-dumps/sample-java-thread-dump.txt \
    --out result.json`,
			RunE: func(cmd *cobra.Command, args []string) error {
				if in == "" {
					return fmt.Errorf("--in is required")
				}
				result, err := threaddump.Analyze(in, threaddump.Options{TopN: topN})
				if err != nil {
					return err
				}
				return writeJSONResult(result, out)
			},
		}
		analyze.Flags().StringVar(&in, "in", "", "path to a Java jstack thread dump (required)")
		analyze.Flags().IntVar(&topN, "top-n", 0, "top-N stack signatures (0 = analyzer default)")
		analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
		group.AddCommand(analyze)
	}

	// ── analyze-multi ──────────────────────────────────────────────
	{
		var (
			inputs    []string
			topN      int
			threshold int
			format    string
			out       string
		)
		multi := &cobra.Command{
			Use:   "analyze-multi",
			Short: "Correlate threads across multiple dumps.",
			Long: `Correlate threads across multiple dumps and emit a thread_dump_multi
result. --in may be repeated and/or comma-separated.`,
			Example: `  archscope-engine thread-dump analyze-multi \
    --in dump1.txt --in dump2.txt --in dump3.txt \
    --out result.json`,
			RunE: func(cmd *cobra.Command, args []string) error {
				paths := splitCommaSeparated(inputs)
				if len(paths) == 0 {
					return fmt.Errorf("--in is required (repeat or comma-separated)")
				}
				bundles, err := td.DefaultRegistry.ParseMany(paths, td.ParseOptions{FormatOverride: format})
				if err != nil {
					return err
				}
				result, err := multithread.Analyze(bundles, multithread.Options{
					TopN:      topN,
					Threshold: threshold,
				})
				if err != nil {
					return err
				}
				return writeJSONResult(result, out)
			},
		}
		multi.Flags().StringSliceVar(&inputs, "in", nil, "path to a thread-dump file; repeat or comma-separated")
		multi.Flags().IntVar(&topN, "top-n", 0, "top-N table rows (0 = analyzer default)")
		multi.Flags().IntVar(&threshold, "threshold", 0, "consecutive-dump threshold for persistence findings")
		multi.Flags().StringVar(&format, "format", "", "force a thread-dump plugin format-id (skips header sniff)")
		multi.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
		group.AddCommand(multi)
	}

	// ── analyze-locks ──────────────────────────────────────────────
	{
		var (
			inputs []string
			topN   int
			format string
			out    string
		)
		locks := &cobra.Command{
			Use:   "analyze-locks",
			Short: "Analyze lock owner/waiter relationships across thread dumps.",
			Long: `Analyze lock owner/waiter relationships across one or more thread
dumps. Emits a thread_dump_locks AnalysisResult with deadlock and
contended-lock findings.`,
			Example: `  archscope-engine thread-dump analyze-locks \
    --in dump1.txt --in dump2.txt \
    --out locks.json`,
			RunE: func(cmd *cobra.Command, args []string) error {
				paths := splitCommaSeparated(inputs)
				if len(paths) == 0 {
					return fmt.Errorf("--in is required (repeat or comma-separated)")
				}
				bundles, err := td.DefaultRegistry.ParseMany(paths, td.ParseOptions{FormatOverride: format})
				if err != nil {
					return err
				}
				result, err := lockcontention.Analyze(bundles, lockcontention.Options{TopN: topN})
				if err != nil {
					return err
				}
				return writeJSONResult(result, out)
			},
		}
		locks.Flags().StringSliceVar(&inputs, "in", nil, "path to a thread-dump file; repeat or comma-separated")
		locks.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
		locks.Flags().StringVar(&format, "format", "", "force a thread-dump plugin format-id (skips header sniff)")
		locks.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
		group.AddCommand(locks)
	}

	// ── to-collapsed ───────────────────────────────────────────────
	{
		var (
			inputs       []string
			noThreadName bool
			format       string
			out          string
		)
		collapsed := &cobra.Command{
			Use:   "to-collapsed",
			Short: "Convert thread dumps to FlameGraph collapsed stacks.",
			Long: `Convert one or more thread dumps into a FlameGraph-compatible
stack→count map. Emitted as JSON (` + "`{stack: count}`" + `) on the Go side
to keep the parity comparison machine-readable; the Python CLI
counterpart writes the FlameGraph "<stack> <count>" text format.`,
			Example: `  archscope-engine thread-dump to-collapsed \
    --in dump.txt --out collapsed.json`,
			RunE: func(cmd *cobra.Command, args []string) error {
				paths := splitCommaSeparated(inputs)
				if len(paths) == 0 {
					return fmt.Errorf("--in is required (repeat or comma-separated)")
				}
				bundles, err := td.DefaultRegistry.ParseMany(paths, td.ParseOptions{FormatOverride: format})
				if err != nil {
					return err
				}
				counts := threaddumpcollapsed.Convert(bundles, threaddumpcollapsed.Options{
					IncludeThreadName: !noThreadName,
				})
				return writeJSONAny(counts, out)
			},
		}
		collapsed.Flags().StringSliceVar(&inputs, "in", nil, "path to a thread-dump file; repeat or comma-separated")
		collapsed.Flags().BoolVar(&noThreadName, "no-thread-name", false, "do not prepend the thread name as the synthetic root frame")
		collapsed.Flags().StringVar(&format, "format", "", "force a thread-dump plugin format-id (skips header sniff)")
		collapsed.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
		group.AddCommand(collapsed)
	}

	rootCmd.AddCommand(group)
}
