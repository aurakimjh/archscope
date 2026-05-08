// `otel` group — mirrors typer's otel_app surface.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `otel` 명령 그룹 — OpenTelemetry JSONL 로그 분석기.
//
// 입력
//   라인 단위 JSON(JSONL) 형식의 OTel 로그 export.
//   각 라인이 하나의 LogRecord 또는 batch 의 element 입니다.
//
// 처리 흐름
//   1) --in 검증.
//   2) otel.Options.TopN 설정(기본 0 = 분석기 기본값).
//   3) otel.Analyze 호출 — 라인별 JSON 디코드 + 메트릭/타임시리즈
//      집계 + 상위 타입/리소스 통계 산출.
//   4) AnalysisResult JSON 출력.
//
// 라인 손상 정책
//   파싱 실패 라인은 skip + diagnostics 카운트(parser_error_handling
//   policy 와 동일).
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/otel"
)

func init() {
	group := &cobra.Command{
		Use:   "otel",
		Short: "OpenTelemetry input analysis commands.",
		Long:  "Analyze OpenTelemetry JSONL log exports. Mirrors the typer otel group.",
	}

	var (
		in   string
		topN int
		out  string
	)

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze line-delimited OpenTelemetry JSON logs.",
		Long:  "Parse line-delimited OpenTelemetry JSON logs and emit an AnalysisResult JSON envelope.",
		Example: `  archscope-engine otel analyze \
    --in examples/otel/sample-otel-logs.jsonl \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := otel.Analyze(in, otel.Options{TopN: topN})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to OTel JSONL logs (required)")
	analyze.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
