package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"github.com/spf13/cobra"
)

func metricHistoryCommands() []*cobra.Command {
	out := []*cobra.Command{}
	for _, operation := range []string{"history-inspect", "history-normalize", "history-import", "history-query", "history-merge"} {
		var input, output, format, producer, version, exportID, namespace, queryJSON, selectionJSON string
		var inputs []string
		command := &cobra.Command{Use: operation, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			var set m.Set
			var e error
			if operation == "history-query" {
				body, e := readMetricDocument(input, 64<<20)
				if e != nil {
					return e
				}
				result, e := resultcontract.DecodeWithLimits(body, wirejson.ResultLimits(64<<20, 128))
				if e != nil {
					return e
				}
				if e = analyzer.Validate(result); e != nil {
					return e
				}
				core, e := analyzer.ReadCore(result)
				if e != nil {
					return e
				}
				var query m.Query
				if e = m.Strict([]byte(queryJSON), &query); e != nil {
					return e
				}
				sum := sha256.Sum256(body)
				page, e := m.QuerySet(ctx, core.Model, hex.EncodeToString(sum[:]), query)
				if e != nil {
					return e
				}
				return writeJSONAny(page, output)
			}
			if operation == "history-merge" {
				sets := []m.Set{}
				for _, path := range inputs {
					body, e := readMetricDocument(path, m.MaxBytes)
					if e != nil {
						return e
					}
					model, e := m.Decode(body)
					if e != nil {
						return e
					}
					sets = append(sets, model)
				}
				set, e = m.Merge(ctx, sets)
			} else {
				if input == "" {
					return fmt.Errorf("--in required")
				}
				set, e = parser.ParseFile(ctx, input, parser.Options{Format: format, Producer: producer, ProducerVersion: version, ExportID: exportID, Namespace: namespace, Query: json.RawMessage(queryJSON)})
			}
			if e != nil {
				return e
			}
			if operation == "history-inspect" {
				return writeJSONAny(parser.Inspect(set), output)
			}
			if operation == "history-normalize" || operation == "history-merge" {
				body, e := json.Marshal(set)
				if e != nil {
					return e
				}
				if output == "-" {
					_, e = cmd.OutOrStdout().Write(append(body, '\n'))
					return e
				}
				root, e := cases.DefaultRoot()
				if e != nil {
					return e
				}
				return localfiles.Publish(ctx, output, append(body, '\n'), root)
			}
			var selection m.Selection
			if selectionJSON != "" {
				if e = m.Strict([]byte(selectionJSON), &selection); e != nil {
					return e
				}
			} else {
				for _, series := range set.Series {
					selection.SeriesIDs = append(selection.SeriesIDs, series.ID)
				}
			}
			result, e := analyzer.Build(ctx, set, selection, input)
			if e != nil {
				return e
			}
			body, e := json.Marshal(result)
			if e != nil {
				return e
			}
			if output == "-" {
				_, e = cmd.OutOrStdout().Write(append(body, '\n'))
				return e
			}
			root, e := cases.DefaultRoot()
			if e != nil {
				return e
			}
			return localfiles.Publish(ctx, output, append(body, '\n'), root)
		}}
		command.Flags().StringVar(&input, "in", "", "original source or saved result")
		command.Flags().StringVar(&output, "out", "-", "destination; normalization refuses existing output")
		command.Flags().StringVar(&format, "format", "auto", "otlp-json, snapshot-sequence-json, prometheus-range-json")
		command.Flags().StringVar(&producer, "producer", "", "declared producer")
		command.Flags().StringVar(&version, "producer-version", "", "declared producer version")
		command.Flags().StringVar(&exportID, "export-id", "", "declared export identity for exact position deduplication")
		command.Flags().StringVar(&namespace, "namespace", "", "explicit stream namespace; formats still remain separate")
		command.Flags().StringVar(&queryJSON, "query", "{}", "query provenance; history-query takes selection/cursor/limit here")
		command.Flags().StringVar(&selectionJSON, "selection", "", "explicit series_ids/from_ns/to_ns; import defaults to all retained series")
		command.Flags().StringSliceVar(&inputs, "inputs", nil, "canonical model files to merge without cross-format guessing")
		out = append(out, command)
	}
	return out
}
func readMetricDocument(path string, limit int64) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	body, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("METRIC_DOCUMENT_BUDGET")
	}
	return body, nil
}
