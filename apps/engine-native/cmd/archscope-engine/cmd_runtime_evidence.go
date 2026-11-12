package main

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	profile "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtimeevidence"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/runtimeevidence"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"github.com/spf13/cobra"
)

func init() {
	group := &cobra.Command{Use: "runtime", Short: "Offline scoped runtime, CPU/thread and plan observations."}
	for _, op := range []string{"inspect-source", "normalize-source", "import-source", "merge-models", "query", "allocation-profile"} {
		var input, output, options, optionsFile, query, entity, sourceID, metric string
		var inputs []string
		cmd := &cobra.Command{Use: op, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signalContext(cmd.Context())
			defer stop()
			publish := func(v any) error {
				raw, e := json.Marshal(v)
				if e != nil {
					return e
				}
				raw = append(raw, '\n')
				if output == "-" {
					_, e = cmd.OutOrStdout().Write(raw)
					return e
				}
				root, e := cases.DefaultRoot()
				if e != nil {
					return e
				}
				return localfiles.Publish(ctx, output, raw, root)
			}
			if op == "query" || op == "allocation-profile" {
				raw, e := readMetricDocument(input, 64<<20)
				if e != nil {
					return e
				}
				result, e := resultcontract.Decode(raw)
				if e != nil {
					return e
				}
				if e = a.Validate(result); e != nil {
					return e
				}
				set, e := a.ReadCore(ctx, result)
				if e != nil {
					return e
				}
				if op == "allocation-profile" {
					model, selection, e := a.AllocationProfile(ctx, set, entity, sourceID, metric)
					if e != nil {
						return e
					}
					result, e := profile.BuildSet(ctx, model, selection, "provided live allocation snapshot")
					if e != nil {
						return e
					}
					return publish(result)
				}
				var q a.Query
				if e = r.Strict([]byte(query), &q, 16<<10); e != nil {
					return e
				}
				page, e := a.QuerySet(ctx, set, p.Digest(raw), q)
				if e != nil {
					return e
				}
				return publish(page)
			}
			if op == "merge-models" {
				if len(inputs) < 1 || len(inputs) > 16 {
					return fmt.Errorf("RUNTIME_MERGE_BUDGET: --inputs needs 1..16 normalized files")
				}
				sets := []r.Set{}
				for _, path := range inputs {
					set, _, e := p.ParseFile(ctx, path, p.Options{Format: "runtime-evidence-json"})
					if e != nil {
						return e
					}
					sets = append(sets, set)
				}
				merged, e := r.Merge(ctx, sets)
				if e != nil {
					return e
				}
				return publish(merged)
			}
			raw := []byte(options)
			if optionsFile != "" {
				var e error
				raw, e = readMetricDocument(optionsFile, 128<<10)
				if e != nil {
					return e
				}
			}
			opts, e := p.DecodeOptions(raw)
			if e != nil {
				return e
			}
			set, hash, e := p.ParseFile(ctx, input, opts)
			if e != nil {
				return e
			}
			switch op {
			case "inspect-source":
				v := analysis.InspectRuntime(set)
				v.SourceSHA = hash
				return publish(v)
			case "normalize-source":
				return publish(set)
			default:
				result, e := a.Build(ctx, set, "sanitized runtime evidence")
				if e != nil {
					return e
				}
				return publish(result)
			}
		}}
		cmd.Flags().StringVar(&entity, "entity-id", "", "one explicit allocation entity")
		cmd.Flags().StringVar(&sourceID, "source-id", "", "one source snapshot")
		cmd.Flags().StringVar(&metric, "metric", "live_bytes", "live_bytes or live_blocks")
		cmd.Flags().StringVar(&input, "in", "", "source/model/result file")
		cmd.Flags().StringVar(&output, "out", "-", "new destination; refuses overwrite and managed stores")
		cmd.Flags().StringVar(&options, "options", `{"format":"runtime-evidence-json"}`, "explicit format and sanitized collection context")
		cmd.Flags().StringVar(&optionsFile, "options-file", "", "bounded source context JSON")
		cmd.Flags().StringVar(&query, "query", `{"kind":"observations","limit":25}`, "exact bounded selector and digest-bound cursor")
		cmd.Flags().StringSliceVar(&inputs, "inputs", nil, "normalized runtime models to merge without identity guessing")
		group.AddCommand(cmd)
	}
	rootCmd.AddCommand(group)
}
