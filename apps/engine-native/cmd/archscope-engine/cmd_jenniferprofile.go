// `jennifer-profile` group — Jennifer Profile Export analyzer (MSA
// timeline). MVP1 surface: file → AnalysisResult JSON. MVP2 added
// MSA grouping + Network Gap; MVP3 added Timeline Signature stats;
// MVP4 added parallelism + HTML report.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `jennifer-profile` 명령 그룹.
//
// Jennifer APM 의 Profile Export 텍스트 파일을 분석합니다. 단일
// 트랜잭션의 호출 트리 + 메타데이터(헤더 + body) 를 한 파일로
// 떨궈주는 형식이며, MSA 환경에서 여러 서비스의 export 를 함께 묶어
// 분석하는 것이 주 사용 케이스입니다.
//
// MVP 단계
//   MVP1 : 단일 파일 → AnalysisResult JSON.
//   MVP2 : 다중 파일 그룹화(GUID 또는 TXID 상관) + Network Gap 계산.
//   MVP3 : 동일 호출 시그니처 통계(Timeline Signature stats).
//   MVP4 : 병렬도(parallelism) 분석 + 자기 충족적 HTML 보고서.
//
// 2개 리프 명령
//   analyze     : 결과를 AnalysisResult JSON 으로 출력.
//   report-html : 같은 파이프라인을 돌린 뒤 HTML 보고서 한 파일로 묶음.
//
// 핵심 옵션
//   --in (반복/콤마)              : 다중 입력. splitCommaSeparated 로
//                                   정규화한 뒤 AnalyzeFiles 에 전달.
//   --fallback-correlation-to-txid: GUID 가 없는 export(구버전 또는
//                                   특정 환경)에서 TXID 를 상관 키로 대체.
//   --header-body-tolerance-ms    : 헤더 사전 집계와 본문 합의 허용 오차.
//                                   APM 라인 타임스탬프가 수 ms drift
//                                   할 수 있어 0~수 ms 가 일반적.
//
// 흐름 (analyze)
//   splitCommaSeparated → AnalyzeFiles(paths, opts) → AnalysisResult →
//   writeJSONResult.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jenniferprofile"
)

func init() {
	group := &cobra.Command{
		Use:   "jennifer-profile",
		Short: "Jennifer Profile Export analysis commands (MSA timeline).",
		Long: "Parse Jennifer Profile Export files and emit an " +
			"AnalysisResult envelope. Multi-file batches are supported via " +
			"repeatable / comma-separated --in.",
	}

	var (
		ins            []string
		fallbackToTxid bool
		toleranceMs    int
		out            string
	)

	analyze := &cobra.Command{
		Use:     "analyze",
		Short:   "Analyze Jennifer Profile Export file(s).",
		Example: `  archscope-engine jennifer-profile analyze --in profile_001.txt --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := splitCommaSeparated(ins)
			if len(paths) == 0 {
				return fmt.Errorf("--in is required (at least one path)")
			}
			result, err := jenniferprofile.AnalyzeFiles(paths, jenniferprofile.Options{
				FallbackCorrelationToTxid: fallbackToTxid,
				HeaderBodyToleranceMs:     toleranceMs,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringSliceVar(&ins, "in", nil,
		"path to Jennifer profile export (repeatable; comma-separated also supported)")
	analyze.Flags().BoolVar(&fallbackToTxid, "fallback-correlation-to-txid", false,
		"when GUID is missing, use TXID as the correlation key (MVP2)")
	analyze.Flags().IntVar(&toleranceMs, "header-body-tolerance-ms", 0,
		"max ms drift between header pre-aggregates and body sums (default 1)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	report := &cobra.Command{
		Use:     "report-html",
		Short:   "Render a self-contained HTML report from Jennifer profile(s).",
		Example: `  archscope-engine jennifer-profile report-html --in profile_001.txt --out report.html`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := splitCommaSeparated(ins)
			if len(paths) == 0 {
				return fmt.Errorf("--in is required (at least one path)")
			}
			result, err := jenniferprofile.AnalyzeFiles(paths, jenniferprofile.Options{
				FallbackCorrelationToTxid: fallbackToTxid,
				HeaderBodyToleranceMs:     toleranceMs,
			})
			if err != nil {
				return err
			}
			html := jenniferprofile.RenderHTMLReport(result)
			if out == "" || out == "-" {
				_, err := os.Stdout.WriteString(html)
				return err
			}
			return os.WriteFile(out, []byte(html), 0o644)
		},
	}
	report.Flags().StringSliceVar(&ins, "in", nil,
		"path to Jennifer profile export (repeatable; comma-separated also supported)")
	report.Flags().BoolVar(&fallbackToTxid, "fallback-correlation-to-txid", false,
		"when GUID is missing, use TXID as the correlation key")
	report.Flags().IntVar(&toleranceMs, "header-body-tolerance-ms", 0,
		"max ms drift between header pre-aggregates and body sums (default 1)")
	report.Flags().StringVar(&out, "out", "-", "output HTML path; `-` for stdout")

	group.AddCommand(analyze)
	group.AddCommand(report)
	rootCmd.AddCommand(group)
}
