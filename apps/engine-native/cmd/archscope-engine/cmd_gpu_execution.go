package main

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	export "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/gputrace"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gputrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/spf13/cobra"
)

func init() {
	group := &cobra.Command{Use: "gpu-trace", Short: "Offline CPU/GPU/rank execution intervals and bounded detailed SVG."}
	for _, op := range []string{"inspect-source", "normalize-source", "import-source", "merge-models", "query", "export-svg"} {
		var input, output, options, optionsFile, query string
		var inputs []string
		cmd := &cobra.Command{Use: op, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signalContext(cmd.Context())
			defer stop()
			write := func(raw []byte) error {
				if output == "-" {
					_, e := cmd.OutOrStdout().Write(raw)
					return e
				}
				root, e := cases.DefaultRoot()
				if e != nil {
					return e
				}
				return localfiles.Publish(ctx, output, raw, root)
			}
			publish := func(v any) error {
				raw, e := json.Marshal(v)
				if e != nil {
					return e
				}
				return write(append(raw, '\n'))
			}
			if op == "query" || op == "export-svg" {
				raw, e := readMetricDocument(input, 64<<20)
				if e != nil {
					return e
				}
				r, e := resultcontract.Decode(raw)
				if e != nil {
					return e
				}
				if e = a.Validate(r); e != nil {
					return e
				}
				set, e := a.ReadCore(ctx, r)
				if e != nil {
					return e
				}
				projection, e := a.Project(ctx, set)
				if e != nil {
					return e
				}
				var q a.Query
				if e = g.Strict([]byte(query), &q, 16<<10); e != nil {
					return e
				}
				if op == "export-svg" {
					body, e := export.SVG(ctx, set, projection, p.Digest(raw), q)
					if e != nil {
						return e
					}
					return write(body)
				}
				page, e := a.QuerySet(ctx, set, projection, p.Digest(raw), q)
				if e != nil {
					return e
				}
				return publish(page)
			}
			if op == "merge-models" {
				if len(inputs) < 1 || len(inputs) > 16 {
					return fmt.Errorf("GPU_TRACE_MERGE_BUDGET: --inputs needs 1..16 files")
				}
				sets := []g.Set{}
				for _, path := range inputs {
					s, _, e := p.ParseFile(ctx, path, p.Options{Format: "gpu-execution-json"})
					if e != nil {
						return e
					}
					sets = append(sets, s)
				}
				s, e := g.Merge(ctx, sets)
				if e != nil {
					return e
				}
				return publish(s)
			}
			raw := []byte(options)
			if optionsFile != "" {
				var e error
				raw, e = readMetricDocument(optionsFile, 64<<10)
				if e != nil {
					return e
				}
			}
			o, e := p.DecodeOptions(raw)
			if e != nil {
				return e
			}
			set, hash, e := p.ParseFile(ctx, input, o)
			if e != nil {
				return e
			}
			switch op {
			case "inspect-source":
				v := analysis.InspectGPUExecution(set)
				v.SourceSHA = hash
				return publish(v)
			case "normalize-source":
				return publish(set)
			default:
				r, e := a.Build(ctx, set, "sanitized offline execution intervals")
				if e != nil {
					return e
				}
				return publish(r)
			}
		}}
		cmd.Flags().StringVar(&input, "in", "", "source or saved result")
		cmd.Flags().StringVar(&output, "out", "-", "new destination; existing files refused")
		cmd.Flags().StringVar(&options, "options", `{"format":"gpu-execution-json"}`, "explicit source scope/format JSON")
		cmd.Flags().StringVar(&optionsFile, "options-file", "", "source options JSON file")
		cmd.Flags().StringVar(&query, "query", `{"kind":"events","limit":25}`, "bounded query; SVG requires one source_id, no cursor, at most2000 events/64 tracks")
		cmd.Flags().StringSliceVar(&inputs, "inputs", nil, "normalized files to merge; source clocks stay isolated")
		group.AddCommand(cmd)
	}
	rootCmd.AddCommand(group)
}
