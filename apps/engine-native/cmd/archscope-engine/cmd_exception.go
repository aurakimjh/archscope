// `exception` group — mirrors typer's exception_app surface.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `exception` 명령 그룹 — Java 예외 스택 분석기.
//
// 입력 형태
//   다중 예외가 포함된 평문 텍스트 파일. 일반적으로 애플리케이션
//   로그에서 grep 으로 추출한 stack trace 모음.
//
// 처리 흐름
//   1) --in 검증.
//   2) exception.Options 를 채움:
//        • TopN     : 결과 테이블의 상위 N 행. 0 이면 분석기 기본값.
//        • MaxLines : 매우 큰 파일에서의 조기 종료 가드.
//        • Strict   : 일반적으로 파서는 잘못된 라인을 skip 하고
//          metadata.diagnostics 에 카운트만 기록하지만, --strict 가
//          켜지면 첫 skip 이 fatal 에러로 즉시 보고됩니다.
//          (CI 의 회귀 차단 용도.)
//   3) exception.Analyze → AnalysisResult.
//   4) writeJSONResult 로 출력.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/exception"
)

func init() {
	group := &cobra.Command{
		Use:   "exception",
		Short: "Java exception stack analysis commands.",
		Long:  "Analyze Java exception stack traces. Mirrors the typer exception group.",
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
		Short: "Analyze Java exception stack traces.",
		Long:  "Parse a Java exception/stack file and emit an AnalysisResult JSON envelope.",
		Example: `  archscope-engine exception analyze \
    --in examples/exceptions/sample-java-exception.txt \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := exception.Analyze(in, exception.Options{
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
	analyze.Flags().StringVar(&in, "in", "", "path to exception/stack file (required)")
	analyze.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
	analyze.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	analyze.Flags().BoolVar(&strict, "strict", false, "surface parser skips as fatal errors")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
