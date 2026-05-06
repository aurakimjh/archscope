// `thread-dump` group — mirrors typer's thread_dump_app surface:
// analyze (single jstack), analyze-multi (multi-dump correlator),
// analyze-locks (lock contention), to-collapsed (FlameGraph collapsed
// stack format).
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
