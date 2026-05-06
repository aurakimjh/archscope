// `profiler` group — stub. The async-profiler / Jennifer / flamegraph
// analyzers are still Python-only; they will live in apps/profiler-native
// once the desktop shell takes over (T-352 follow-up). The group is
// registered here only so it shows up in `--help` and so users get a
// clear pointer rather than "unknown command".
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const profilerStubMessage = "profiler analyzers live in apps/profiler-native (T-352 follow-up); engine-native does not ship them"

func init() {
	group := &cobra.Command{
		Use:   "profiler",
		Short: "Profiler analysis commands (not shipped by engine-native).",
		Long: `Profiler analysis is provided by the desktop shell at
apps/profiler-native (T-352 follow-up). engine-native intentionally
does not ship these analyzers — for command-line profiler analysis,
use ` + "`python -m archscope_engine.cli profiler`" + `.`,
	}

	stub := func(use, short string) *cobra.Command {
		return &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(cmd *cobra.Command, args []string) error {
				return fmt.Errorf("%s", profilerStubMessage)
			},
		}
	}

	group.AddCommand(stub("analyze-collapsed", "Analyze a collapsed-stack profile (not in engine-native)."))
	group.AddCommand(stub("analyze-flamegraph-svg", "Analyze a FlameGraph SVG (not in engine-native)."))
	group.AddCommand(stub("analyze-flamegraph-html", "Analyze a FlameGraph HTML wrapper (not in engine-native)."))
	group.AddCommand(stub("analyze-jennifer-csv", "Analyze a Jennifer APM CSV (not in engine-native)."))
	group.AddCommand(stub("drilldown", "Apply drill-down filters (not in engine-native)."))
	group.AddCommand(stub("breakdown", "Compute execution breakdown (not in engine-native)."))

	rootCmd.AddCommand(group)
}
