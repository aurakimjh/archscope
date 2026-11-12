package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	history "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/spf13/cobra"
)

func init() {
	group := &cobra.Command{Use: "inference", Short: "Offline device/serving evidence; explicit semantics, no live collection or aliases."}
	for _, op := range []string{"inspect-source", "normalize-source", "import-source", "query-metric", "open-metrics"} {
		var input, output, format, captureFile, bindingsFile, signal, cursor string
		var limit int
		var seriesIDs []string
		command := &cobra.Command{Use: op, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signalContext(cmd.Context())
			defer stop()
			publish := func(v any) error {
				body, e := json.Marshal(v)
				if e != nil {
					return e
				}
				body = append(body, '\n')
				if output == "-" {
					_, e = cmd.OutOrStdout().Write(body)
					return e
				}
				root, e := cases.DefaultRoot()
				if e != nil {
					return e
				}
				return localfiles.Publish(ctx, output, body, root)
			}
			if op == "query-metric" || op == "open-metrics" {
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
				set, e := a.ReadCore(result)
				if e != nil {
					return e
				}
				var selected n.Signal
				for _, v := range set.Signals {
					if v.ID == signal {
						selected = v
					}
				}
				if selected.ID == "" {
					return fmt.Errorf("INFERENCE_SIGNAL_SELECTION_INVALID")
				}
				selection := m.Selection{SeriesIDs: []string{selected.SeriesID}}
				if op == "query-metric" {
					page, e := m.QuerySet(ctx, set.Metrics, p.Digest(raw), m.Query{Selection: selection, Cursor: cursor, Limit: limit})
					if e != nil {
						return e
					}
					return publish(page)
				}
				effective, mapping, e := n.EffectiveMetrics(ctx, set)
				if e != nil {
					return e
				}
				selection.SeriesIDs = []string{mapping[selected.SeriesID]}
				metric, e := history.Build(ctx, effective, selection, "explicit inference signal selection")
				if e != nil {
					return e
				}
				metric.Metadata.Extra["inference_projection"] = map[string]any{"parent_result_sha256": p.Digest(raw), "signals": []string{signal}, "identity_basis": "declared", "aggregation": "none", "original_selection": m.Selection{SeriesIDs: []string{selected.SeriesID}}, "declared_metadata": n.SelectedDeclarations(set, []string{signal}), "metadata_basis": "original or explicitly declared range interpretation; original core remains unchanged"}
				return publish(metric)
			}
			options := p.Options{Format: format, SeriesIDs: seriesIDs}
			if captureFile != "" {
				body, e := readMetricDocument(captureFile, 128<<10)
				if e != nil {
					return e
				}
				var capture p.Capture
				if e = n.Strict(body, &capture); e != nil {
					return e
				}
				options.Capture = &capture
			}
			if bindingsFile != "" {
				body, e := readMetricDocument(bindingsFile, 128<<10)
				if e != nil {
					return e
				}
				options.Bindings = body
			}
			set, digest, e := p.ParseFile(ctx, input, options)
			if e != nil {
				return e
			}
			switch op {
			case "inspect-source":
				v := a.Inspect(set)
				v.SourceSHA256 = digest
				return publish(v)
			case "normalize-source":
				return publish(set)
			default:
				result, e := a.Build(ctx, set, filepath.Base(input))
				if e != nil {
					return e
				}
				return publish(result)
			}
		}}
		command.Flags().StringSliceVar(&seriesIDs, "retained-series-id", nil, "retain all points of selected original series during normalization")
		command.Flags().StringVar(&input, "in", "", "original file or saved inference result")
		command.Flags().StringVar(&output, "out", "-", "new destination or stdout")
		command.Flags().StringVar(&format, "format", "inference-evidence-json", "nvidia-smi-csv, metric-bindings-json or inference-evidence-json")
		command.Flags().StringVar(&captureFile, "capture-file", "", "explicit NVIDIA capture contract, at most 128 KiB")
		command.Flags().StringVar(&bindingsFile, "bindings-file", "", "explicit manifest without metrics, at most 128 KiB")
		command.Flags().StringVar(&signal, "signal-id", "", "one signal from inventory; no cross-scope aggregation")
		command.Flags().StringVar(&cursor, "cursor", "", "digest/query-bound original metric page cursor")
		command.Flags().IntVar(&limit, "limit", 25, "original metric page size, 1–200")
		group.AddCommand(command)
	}
	rootCmd.AddCommand(group)
}
func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
}
