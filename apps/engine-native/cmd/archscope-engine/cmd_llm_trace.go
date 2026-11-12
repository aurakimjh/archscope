package main

import (
	"encoding/json"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/localfiles"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/spf13/cobra"
)

func llmTraceCommands() []*cobra.Command {
	out := []*cobra.Command{}
	for _, op := range []string{"llm-inspect", "llm-normalize", "llm-import", "llm-query"} {
		var input, output, options, optionsFile, query string
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
			if op == "llm-query" {
				raw, e := readMetricDocument(input, 64<<20)
				if e != nil {
					return e
				}
				r, e := resultcontract.Decode(raw)
				if e != nil {
					return e
				}
				if e = a.ValidateLLM(r); e != nil {
					return e
				}
				set, e := a.ReadLLMCore(r)
				if e != nil {
					return e
				}
				var q a.LLMQuery
				if e = l.Strict([]byte(query), &q, 16<<10); e != nil {
					return e
				}
				page, e := a.QueryLLM(ctx, set, p.LLMDigest(raw), q)
				if e != nil {
					return e
				}
				return publish(page)
			}
			raw := []byte(options)
			if optionsFile != "" {
				var e error
				raw, e = readMetricDocument(optionsFile, 64<<10)
				if e != nil {
					return e
				}
			}
			opts, e := p.DecodeLLMOptions(raw)
			if e != nil {
				return e
			}
			set, e := p.ParseLLMFile(ctx, input, opts)
			if e != nil {
				return e
			}
			switch op {
			case "llm-inspect":
				return publish(analysis.InspectLLM(set))
			case "llm-normalize":
				return publish(set)
			case "llm-import":
				result, e := a.BuildLLM(ctx, set, "sanitized LLM structural source")
				if e != nil {
					return e
				}
				return publish(result)
			}
			return fmt.Errorf("LLMTRACE_OPERATION_UNSUPPORTED")
		}}
		cmd.Flags().StringVar(&input, "in", "", "local OTLP JSON/JSONL or saved body-free result")
		cmd.Flags().StringVar(&output, "out", "-", "new destination or stdout; no overwrites")
		cmd.Flags().StringVar(&options, "options", "{}", "explicit semantic_profile/trace_namespace/producer/producer_version/clock")
		cmd.Flags().StringVar(&optionsFile, "options-file", "", "options JSON, at most 64 KiB")
		cmd.Flags().StringVar(&query, "query", `{"kind":"spans","limit":25}`, "bounded query selector/cursor JSON")
		out = append(out, cmd)
	}
	return out
}
