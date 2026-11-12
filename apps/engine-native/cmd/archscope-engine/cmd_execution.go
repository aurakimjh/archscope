package main

import (
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/spf13/cobra"
)

func init() {
	group := &cobra.Command{Use: "execution", Short: "Offline benchmark exports, condition-checked comparison and JVM crash text."}
	for _, op := range []string{"inspect-source", "normalize-source", "import-source", "query", "compare"} {
		var input, target, output, options, optionsFile, query string
		cmd := &cobra.Command{Use: op, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signalContext(cmd.Context())
			defer stop()
			publish := func(v any) error {
				raw, err := json.Marshal(v)
				if err != nil {
					return err
				}
				raw = append(raw, '\n')
				if output == "-" {
					_, err = cmd.OutOrStdout().Write(raw)
					return err
				}
				root, err := cases.DefaultRoot()
				if err != nil {
					return err
				}
				return localfiles.Publish(ctx, output, raw, root)
			}
			readResult := func(path string) (e.Set, string, error) {
				raw, err := readMetricDocument(path, 64<<20)
				if err != nil {
					return e.Set{}, "", err
				}
				result, err := resultcontract.Decode(raw)
				if err != nil {
					return e.Set{}, "", err
				}
				if err = a.Validate(result); err != nil {
					return e.Set{}, "", err
				}
				set, err := a.ReadCore(ctx, result)
				return set, p.Digest(raw), err
			}
			if op == "query" || op == "compare" {
				set, sha, err := readResult(input)
				if err != nil {
					return err
				}
				if op == "query" {
					var q a.Query
					if err = e.Strict([]byte(query), &q, 4096); err != nil {
						return err
					}
					page, err := a.QuerySet(ctx, set, sha, q)
					if err != nil {
						return err
					}
					return publish(page)
				}
				if target == "" {
					return fmt.Errorf("EXECUTION_TARGET_REQUIRED")
				}
				right, rightSHA, err := readResult(target)
				if err != nil {
					return err
				}
				var q a.CompareQuery
				if err = e.Strict([]byte(query), &q, 4096); err != nil {
					return err
				}
				comparison, err := a.CompareSets(ctx, set, right, sha, rightSHA, q)
				if err != nil {
					return err
				}
				return publish(comparison)
			}
			raw := []byte(options)
			var err error
			if optionsFile != "" {
				raw, err = readMetricDocument(optionsFile, 64<<10)
				if err != nil {
					return err
				}
			}
			o, err := p.DecodeOptions(raw)
			if err != nil {
				return err
			}
			set, sha, err := p.ParseFile(ctx, input, o)
			if err != nil {
				return err
			}
			switch op {
			case "inspect-source":
				inventory := analysis.InspectExecution(set)
				inventory.SourceSHA = sha
				return publish(inventory)
			case "normalize-source":
				return publish(set)
			default:
				result, err := a.Build(ctx, set, "sanitized offline execution evidence")
				if err != nil {
					return err
				}
				return publish(result)
			}
		}}
		cmd.Flags().StringVar(&input, "in", "", "raw source, canonical model or saved result")
		cmd.Flags().StringVar(&target, "target", "", "ordered comparison target saved result; baseline is --in")
		cmd.Flags().StringVar(&output, "out", "-", "new destination; no overwrite or managed store writes")
		cmd.Flags().StringVar(&options, "options", `{"format":"execution-evidence-json"}`, "strict source format and sanitized collection context")
		cmd.Flags().StringVar(&optionsFile, "options-file", "", "bounded source context JSON file")
		defaultQuery := `{"kind":"metrics","limit":25}`
		if op == "compare" {
			defaultQuery = `{"limit":25}`
		}
		cmd.Flags().StringVar(&query, "query", defaultQuery, "strict selector and digest-bound cursor")
		group.AddCommand(cmd)
	}
	rootCmd.AddCommand(group)
}
