// Runtime stack/log analyzers — four top-level groups (`nodejs`,
// `python-traceback`, `go-panic`, `dotnet`), all wrapping
// internal/analyzers/runtime with a different variant. Mirrors typer's
// nodejs_app / python_traceback_app / go_panic_app / dotnet_app.
package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtime"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// runtimeAnalyzeFunc abstracts the analyzer entrypoint so we can wire
// the same flag-binding helper to each variant. Every runtime analyzer
// has the same signature (path, Options) → (Result, error).
type runtimeAnalyzeFunc func(path string, opts runtime.Options) (models.AnalysisResult, error)

// addRuntimeGroup creates a `<use> analyze` group backed by `analyze`.
// The four runtime analyzers share the exact same flag set, so this
// helper avoids four near-identical bodies.
func addRuntimeGroup(use, short, long, example, inHelp string, analyze runtimeAnalyzeFunc) {
	group := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
	}

	var (
		in       string
		topN     int
		maxLines int
		strict   bool
		out      string
	)

	leaf := &cobra.Command{
		Use:     "analyze",
		Short:   short,
		Long:    long,
		Example: example,
		RunE: func(cmd *cobra.Command, args []string) error {
			if in == "" {
				return fmt.Errorf("--in is required")
			}
			result, err := analyze(in, runtime.Options{
				TopN:     topN,
				MaxLines: maxLines,
				Strict:   strict,
			})
			if err != nil {
				return err
			}
			return writeJSONResult(result, out)
		},
	}
	leaf.Flags().StringVar(&in, "in", "", inHelp)
	leaf.Flags().IntVar(&topN, "top-n", 0, "top-N rows (0 = analyzer default)")
	leaf.Flags().IntVar(&maxLines, "max-lines", 0, "stop after N lines (0 = unlimited)")
	leaf.Flags().BoolVar(&strict, "strict", false, "surface parser skips as fatal errors")
	leaf.Flags().StringVar(&out, "out", "-", "output path; `-` for stdout")

	group.AddCommand(leaf)
	rootCmd.AddCommand(group)
}

func init() {
	addRuntimeGroup(
		"nodejs",
		"Node.js log and stack analysis commands.",
		"Analyze Node.js error stack traces. Mirrors the typer nodejs group.",
		`  archscope-engine nodejs analyze \
    --in examples/runtime/sample-nodejs-stack.txt \
    --out result.json`,
		"path to Node.js stack/log file (required)",
		runtime.AnalyzeNodejsStack,
	)

	addRuntimeGroup(
		"python-traceback",
		"Python traceback analysis commands.",
		"Analyze Python traceback blocks. Mirrors the typer python-traceback group.",
		`  archscope-engine python-traceback analyze \
    --in examples/runtime/sample-python-traceback.txt \
    --out result.json`,
		"path to Python traceback file (required)",
		runtime.AnalyzePythonTraceback,
	)

	addRuntimeGroup(
		"go-panic",
		"Go panic and goroutine analysis commands.",
		"Analyze Go panic and goroutine dumps. Mirrors the typer go-panic group.",
		`  archscope-engine go-panic analyze \
    --in examples/runtime/sample-go-panic.txt \
    --out result.json`,
		"path to Go panic/goroutine dump (required)",
		runtime.AnalyzeGoPanic,
	)

	addRuntimeGroup(
		"dotnet",
		".NET exception and IIS log analysis commands.",
		"Analyze .NET exception stack traces and IIS W3C access logs.",
		`  archscope-engine dotnet analyze \
    --in examples/runtime/sample-dotnet-iis.txt \
    --out result.json`,
		"path to .NET exception or IIS W3C log (required)",
		runtime.AnalyzeDotnetExceptionIIS,
	)
}
