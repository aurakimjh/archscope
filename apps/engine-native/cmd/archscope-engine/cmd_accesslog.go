// `access-log` group — mirrors typer's access_log_app surface.
//
// Leaves
//   archscope-engine access-log analyze --in <path> [--format nginx]
//                                       [--max-lines N]
//                                       [--start-time RFC3339]
//                                       [--end-time RFC3339]
//                                       [--out <path>]
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `access-log` 명령 그룹.
//
// 책임 범위
//   • Python typer 의 access_log_app 표면을 그대로 모방한 Cobra group.
//   • 현재 리프 명령은 `analyze` 1개. 향후 `summary`, `top-urls` 등이
//     추가될 가능성을 고려해 group 형태로 유지.
//
// 처리 흐름 (analyze)
//   1) 사용자 플래그 검증 — `--in` 은 필수. 없으면 즉시 에러.
//   2) accesslog.Options 구성:
//        • MaxLines : 0 이면 무제한, 양수면 N 라인에서 조기 종료(대용량
//          로그 sampling 용. T-014 에서 도입).
//        • StartTime/EndTime : helpers.parseTimeFlag 가 빈 문자열을
//          nil 로 변환해 주므로 분석기는 단순 nil 체크만 수행.
//   3) accesslog.Analyze(in, format, opts) 호출 — 파싱과 통계 생성을
//      모두 끝낸 AnalysisResult 를 반환.
//   4) writeJSONResult 로 indent JSON 직렬화 후 stdout 또는 파일 출력.
//
// 포맷 처리
//   • format 의 기본값은 "nginx"(combined). IIS/Apache 등은 사용자가
//     명시적으로 지정. 분석기 내부에서 라인 정규식이 분기됩니다.
//
// 에러 정책
//   • 파일 자체가 없거나 권한 문제 = 치명적(엔진이 즉시 에러).
//   • 라인 단위 파싱 실패 = 건너뛰고 metadata.diagnostics 에 카운트.
//     Python parser_error_handling policy 와 동일.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/accesslog"
)

func init() {
	group := &cobra.Command{
		Use:   "access-log",
		Short: "Access log analysis commands.",
		Long:  "Analyze HTTP access logs (nginx, IIS, Apache). Mirrors the typer access-log group.",
	}

	var (
		in        string
		format    string
		maxLines  int
		startTime string
		endTime   string
		out       string
	)

	analyze := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze an access log and write an AnalysisResult JSON file.",
		Long: `Parse a HTTP access log (nginx-combined by default) and emit a
models.AnalysisResult envelope as JSON.`,
		Example: `  archscope-engine access-log analyze \
    --in examples/access-logs/sample-nginx-access.log \
    --format nginx \
    --out result.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			opts := accesslog.Options{MaxLines: maxLines}
			start, err := parseTimeFlag("start-time", startTime)
			if err != nil {
				return err
			}
			opts.StartTime = start
			end, err := parseTimeFlag("end-time", endTime)
			if err != nil {
				return err
			}
			opts.EndTime = end

			result, err := accesslog.Analyze(in, format, opts)
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	analyze.Flags().StringVar(&in, "in", "", "path to access log (required)")
	analyze.Flags().StringVar(&format, "format", "nginx", "log format label")
	analyze.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	analyze.Flags().StringVar(&startTime, "start-time", "", "RFC3339 lower bound (inclusive)")
	analyze.Flags().StringVar(&endTime, "end-time", "", "RFC3339 upper bound (inclusive)")
	analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(analyze)
	rootCmd.AddCommand(group)
}
