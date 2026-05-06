// `report` group — wires the exporter packages (html, pptx, csv,
// json, reportdiff) into Cobra subcommands. Mirrors typer's
// report_app surface, plus extras for csv and indented json that the
// Python CLI exposes via separate exporters.
//
// All exporters' Write helpers accept `any` (they normalise via
// internal toMap), so we load the AnalysisResult JSON as a generic
// map[string]any and pass it through.
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
