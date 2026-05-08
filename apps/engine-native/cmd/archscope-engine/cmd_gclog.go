// `gc-log` group — mirrors typer's gc_log_app surface.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `gc-log` 명령 그룹 — HotSpot Unified GC 로그 분석기.
//
// 대상 포맷
//   JDK 9+ 의 -Xlog:gc 통합 포맷. G1 / Parallel / ZGC / Shenandoah
//   가 모두 같은 라인 형식(`[time][gc] ...`)을 따르므로 한 파서에서
//   처리합니다. 헤더 블록(JVM Info)도 같이 추출해 summary 에 포함.
//
// 처리 흐름
//   1) gclog.Options 구성(TopN / MaxLines / Strict — exception 그룹과
//      동일한 의미).
//   2) gclog.Analyze:
//        • 헤더 파서가 먼저 첫 ~수 KB 에서 JVM Info 추출
//        • 본문 파서가 라인별로 GC 이벤트(시간/유형/지연/heap before·
//          after/원인)를 records 로 누적
//        • 분석기가 percentile, 컬렉터별 비교, 시계열 통계 산출
//   3) AnalysisResult 의 summary/series/tables/charts 에 매핑되어
//      JSON 으로 출력.
//
// 큰 로그 처리
//   --max-lines 로 조기 종료 가능. percentile 은 T-049 의 bounded
//   sampling 정책에 따라 표본 추출(메모리 상한이 있으므로 정확도-
//   메모리 trade-off 가 documented in PARSER_DESIGN.md).
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gclog"
)

func init() {
	group := &cobra.Command{
		Use:   "gc-log",
		Short: "GC log analysis commands.",
		Long:  "Analyze HotSpot unified GC logs. Mirrors the typer gc-log group.",
	}

	var (
		in       string
		topN     int
		maxLines int
		strict   bool
		out      string
	)

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze a HotSpot unified GC log.",
		Long:  "Parse a HotSpot unified GC log and emit an AnalysisResult JSON envelope.",
		Example: `  archscope-engine gc-log analyze \
    --in examples/gc-logs/sample-hotspot-gc.log \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := gclog.Analyze(in, gclog.Options{
				TopN:     topN,
				MaxLines: maxLines,
				Strict:   strict,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to GC log (required)")
	analyze.Flags().IntVar(&topN, "top-n", 0, "top-N rows in tables.events (0 = analyzer default)")
	analyze.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	analyze.Flags().BoolVar(&strict, "strict", false, "surface parser skips as fatal errors")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
