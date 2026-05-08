// `report` group — wires the exporter packages (html, pptx, csv,
// json, reportdiff) into Cobra subcommands. Mirrors typer's
// report_app surface, plus extras for csv and indented json that the
// Python CLI exposes via separate exporters.
//
// All exporters' Write helpers accept `any` (they normalise via
// internal toMap), so we load the AnalysisResult JSON as a generic
// map[string]any and pass it through.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] `report` 명령 그룹 — exporter 패키지를 CLI 표면으로 노출.
//
// 책임
//   분석은 일절 하지 않습니다. 이미 디스크에 저장된 AnalysisResult JSON
//   파일 1개(또는 diff 의 경우 2개)를 입력으로 받아서, 다른 형식의
//   보고서 산출물로 렌더링하는 역할만 담당합니다.
//
// 5개 리프 명령
//   html  : 자기 충족적 단일 HTML 보고서 (외부 자산 0개).
//   pptx  : 슬라이드 1장당 한 섹션이 들어가는 PowerPoint 덱.
//   csv   : 섹션 헤더가 포함된 단일 CSV(엑셀에서 그대로 열리는 형태).
//   json  : 들여쓰기가 정규화된 JSON(diff 도구의 노이즈 줄이기용).
//   diff  : --before / --after 두 결과를 비교한 comparison_report 산출.
//
// 데이터 흐름
//   readJSONFile(in) → map[string]any → exporter.Write(out, map).
//   exporter 내부의 toMap helper 가 임의의 JSON 호환 입력을 정규화하므로
//   AnalysisResult 의 임의 확장 필드(metadata.* 등)도 그대로 전달됩니다.
//
// JSON 라운드트립의 효용
//   `report json --in result.json --out result.indented.json` 를 양쪽
//   엔진(Python/Go)에서 돌리면, parity gate 가 두 엔진의 결과를
//   "공백·키 순서 노이즈 없이" 비교할 수 있습니다 — 회귀 위치를 찾기
//   훨씬 쉽게 만들어 주는 파이프라인 설계입니다.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	enginejson "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/pptx"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/reportdiff"
)

func init() {
	group := &cobra.Command{
		Use:   "report",
		Short: "Report export commands.",
		Long: `Render an AnalysisResult JSON file as HTML, PPTX, CSV, or
indented JSON; or build a before/after comparison report.`,
	}

	// ── report html ────────────────────────────────────────────────
	{
		var (
			in  string
			out string
		)
		cmd := &cobra.Command{
			Use:   "html",
			Short: "Render an AnalysisResult JSON file as a portable HTML report.",
			Long:  "Load `--in` as an AnalysisResult and write a self-contained HTML report to `--out`.",
			Example: `  archscope-engine report html \
    --in result.json --out report.html`,
			RunE: func(c *cobra.Command, _ []string) error {
				if in == "" || out == "" {
					return fmt.Errorf("--in and --out are required")
				}
				payload, err := readJSONFile(in)
				if err != nil {
					return err
				}
				return html.Write(out, payload)
			},
		}
		cmd.Flags().StringVar(&in, "in", "", "AnalysisResult JSON input (required)")
		cmd.Flags().StringVar(&out, "out", "", "HTML report output path (required)")
		group.AddCommand(cmd)
	}

	// ── report pptx ────────────────────────────────────────────────
	{
		var (
			in  string
			out string
		)
		cmd := &cobra.Command{
			Use:   "pptx",
			Short: "Render an AnalysisResult JSON file as a PowerPoint deck.",
			Long:  "Load `--in` as an AnalysisResult and write a minimal PowerPoint deck to `--out`.",
			Example: `  archscope-engine report pptx \
    --in result.json --out report.pptx`,
			RunE: func(c *cobra.Command, _ []string) error {
				if in == "" || out == "" {
					return fmt.Errorf("--in and --out are required")
				}
				payload, err := readJSONFile(in)
				if err != nil {
					return err
				}
				return pptx.Write(out, payload)
			},
		}
		cmd.Flags().StringVar(&in, "in", "", "AnalysisResult JSON input (required)")
		cmd.Flags().StringVar(&out, "out", "", "PowerPoint .pptx output path (required)")
		group.AddCommand(cmd)
	}

	// ── report csv ─────────────────────────────────────────────────
	{
		var (
			in  string
			out string
		)
		cmd := &cobra.Command{
			Use:   "csv",
			Short: "Render an AnalysisResult JSON file as a single CSV with section headers.",
			Long:  "Load `--in` as an AnalysisResult and write a sectioned CSV to `--out`.",
			Example: `  archscope-engine report csv \
    --in result.json --out report.csv`,
			RunE: func(c *cobra.Command, _ []string) error {
				if in == "" || out == "" {
					return fmt.Errorf("--in and --out are required")
				}
				payload, err := readJSONFile(in)
				if err != nil {
					return err
				}
				return csv.Write(out, payload)
			},
		}
		cmd.Flags().StringVar(&in, "in", "", "AnalysisResult JSON input (required)")
		cmd.Flags().StringVar(&out, "out", "", "CSV output path (required)")
		group.AddCommand(cmd)
	}

	// ── report json ────────────────────────────────────────────────
	{
		var (
			in  string
			out string
		)
		cmd := &cobra.Command{
			Use:   "json",
			Short: "Round-trip an AnalysisResult JSON file (pretty-print with 2-space indent + trailing newline).",
			Long: `Load --in, normalise via the JSON exporter (matches the engine's
schema-stable shape), and write to --out. Useful for diffing two
analyzer runs without spurious whitespace noise.`,
			Example: `  archscope-engine report json \
    --in result.json --out result.indented.json`,
			RunE: func(c *cobra.Command, _ []string) error {
				if in == "" || out == "" {
					return fmt.Errorf("--in and --out are required")
				}
				payload, err := readJSONFile(in)
				if err != nil {
					return err
				}
				return enginejson.Write(out, payload)
			},
		}
		cmd.Flags().StringVar(&in, "in", "", "AnalysisResult JSON input (required)")
		cmd.Flags().StringVar(&out, "out", "", "JSON output path (required)")
		group.AddCommand(cmd)
	}

	// ── report diff ────────────────────────────────────────────────
	{
		var (
			before string
			after  string
			out    string
			label  string
		)
		cmd := &cobra.Command{
			Use:   "diff",
			Short: "Build a before/after comparison report from two AnalysisResult JSONs.",
			Long: `Load --before and --after as AnalysisResult JSONs and emit a
comparison_report at --out. The report includes per-metric deltas and
a side-by-side findings count.`,
			Example: `  archscope-engine report diff \
    --before before.json --after after.json --out diff.json`,
			RunE: func(c *cobra.Command, _ []string) error {
				if before == "" || after == "" || out == "" {
					return fmt.Errorf("--before, --after, and --out are required")
				}
				report, err := reportdiff.BuildComparisonReport(before, after, label)
				if err != nil {
					return err
				}
				return enginejson.Write(out, report)
			},
		}
		cmd.Flags().StringVar(&before, "before", "", "AnalysisResult JSON for the baseline (required)")
		cmd.Flags().StringVar(&after, "after", "", "AnalysisResult JSON for the new run (required)")
		cmd.Flags().StringVar(&out, "out", "", "comparison report output path (required)")
		cmd.Flags().StringVar(&label, "label", "", "optional human-readable label written into the report")
		group.AddCommand(cmd)
	}

	rootCmd.AddCommand(group)
}
