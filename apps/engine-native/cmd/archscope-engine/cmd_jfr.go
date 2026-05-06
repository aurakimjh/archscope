// `jfr` group — mirrors typer's jfr_app surface plus the
// native-memory variant (T-340 added that on the Go side; the typer
// counterpart is `jfr analyze-native-memory`).
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/jfr"
)

func init() {
	group := &cobra.Command{
		Use:   "jfr",
		Short: "JFR recording analysis commands.",
		Long:  "Analyze JDK Flight Recorder (.jfr) recordings via `jfr print --json` output.",
	}

	// ── analyze-json ───────────────────────────────────────────────
	{
		var (
			in     string
			topN   int
			mode   string
			from   string
			to     string
			state  string
			minDur float64
			out    string
		)
		analyze := &cobra.Command{
			Use:   "analyze-json",
			Short: "Analyze JSON emitted by `jfr print --json`.",
			Long: `Filter and summarise JFR events from a ` + "`jfr print --json`" + ` dump.
The --mode flag controls which event family is emphasised in the
summary; --from / --to / --state / --min-duration-ms apply per-event
filters.`,
			Example: `  archscope-engine jfr analyze-json \
    --in examples/jfr/sample-jfr-print.json \
    --top-n 20 --out result.json`,
			RunE: func(cmd *cobra.Command, args []string) error {
				if in == "" {
					return fmt.Errorf("--in is required")
				}
				opts := jfr.Options{
					TopN:     topN,
					Mode:     mode,
					FromTime: from,
					ToTime:   to,
					State:    state,
				}
				if minDur > 0 {
					v := minDur
					opts.MinDurationMS = &v
				}
				result, err := jfr.Analyze(in, opts)
				if err != nil {
					return err
				}
				return writeJSONResult(result, out)
			},
		}
		analyze.Flags().StringVar(&in, "in", "", "path to JFR recording or `jfr print --json` output (required)")
		analyze.Flags().IntVar(&topN, "top-n", 0, "top-N notable events (0 = analyzer default)")
		analyze.Flags().StringVar(&mode, "mode", "all", "filter mode (all|cpu|wall|alloc|lock|gc|exception|io|nativemem)")
		analyze.Flags().StringVar(&from, "from", "", "lower-bound time filter (ISO 8601, HH:MM[:SS], or relative like +30s)")
		analyze.Flags().StringVar(&to, "to", "", "upper-bound time filter (same syntax as --from)")
		analyze.Flags().StringVar(&state, "state", "", "filter to events whose .state matches (case-insensitive)")
		analyze.Flags().Float64Var(&minDur, "min-duration-ms", 0, "drop events whose duration_ms < this")
		analyze.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
		group.AddCommand(analyze)
	}

	// ── analyze-native-memory ─────────────────────────────────────
	{
		var (
			in  string
			out string
		)
		nativeMem := &cobra.Command{
			Use:   "analyze-native-memory",
			Short: "Run the native-memory leak heuristic over a JFR recording.",
			Long: `Apply the leak-only / tail-ratio heuristic to a JFR recording's
native-memory events. Defaults match Python's analyze_native_memory:
leak_only=true, tail_ratio=0.10.`,
			Example: `  archscope-engine jfr analyze-native-memory \
    --in examples/jfr/sample-jfr-print.json \
    --out native-memory.json`,
			RunE: func(cmd *cobra.Command, args []string) error {
				if in == "" {
					return fmt.Errorf("--in is required")
				}
				result, err := jfr.AnalyzeNativeMemory(in, jfr.NewNativeMemoryOptions())
				if err != nil {
					return err
				}
				return writeJSONResult(result, out)
			},
		}
		nativeMem.Flags().StringVar(&in, "in", "", "path to JFR recording or `jfr print --json` output (required)")
		nativeMem.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")
		group.AddCommand(nativeMem)
	}

	rootCmd.AddCommand(group)
}
