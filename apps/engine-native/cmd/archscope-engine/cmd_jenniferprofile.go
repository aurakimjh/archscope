// `jennifer-profile` group — Jennifer Profile Export analyzer (MSA
// timeline). MVP1 surface: file → AnalysisResult JSON. MVP2 added
// MSA grouping + Network Gap; MVP3 added Timeline Signature stats;
// MVP4 added parallelism + HTML report.
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
