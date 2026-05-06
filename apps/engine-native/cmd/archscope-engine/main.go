// archscope-engine is the Go counterpart of `python -m
// archscope_engine.cli`. Each command group below mirrors the typer
// surface byte-for-byte (`gc-log` not `gclog`; `thread-dump
// analyze-multi` not `multithread`) so users, the parity gate, and
// the future demo runner (T-380) can swap engines without rewriting
// invocations.
//
// Build layout:
//   - main.go            (this file): root *cobra.Command + Execute.
//   - cmd_*.go:           one file per top-level group. Each file owns
//                         its leaves, flag bindings, and RunE handlers.
//   - helpers.go:         writeJSONResult, parseTimeFlag, readJSONFile.
//
// T-360 only refactors the CLI surface; analyzers/exporters under
// internal/ are untouched.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// Side-effect imports register thread-dump plugins on the default
	// registry. javajstack + gogoroutine self-register; the rest expose
	// New() factories we wire into the registry below.
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/dotnetclrstack"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/gogoroutine"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajcmdjson"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/nodejsreport"
	_ "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/pythondump"
)

// rootCmd is the parent of every command group. Each cmd_*.go file
// attaches its top-level group to it via a `func init()` so adding a
// new analyzer is a single-file change.
var rootCmd = &cobra.Command{
	Use:   "archscope-engine",
	Short: "ArchScope analysis engine CLI (Go)",
	Long: `ArchScope analysis engine CLI — the Go counterpart of
` + "`python -m archscope_engine.cli`" + `.

Every analyzer emits the same models.AnalysisResult JSON envelope as
the Python CLI; the parity gate at .github/workflows/profiler-native.yml
runs them side-by-side on every PR. Subcommand names mirror the typer
surface verbatim — see ` + "`archscope-engine <group> --help`" + ` for the
flags each leaf accepts.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "archscope-engine: %v\n", err)
		os.Exit(1)
	}
}
