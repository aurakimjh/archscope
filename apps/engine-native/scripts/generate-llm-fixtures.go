//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
)

func main() {
	base := os.Args[1]
	for _, tc := range []struct{ input, options, output string }{{"sdk-python-1.38.0.otlp.json", "neutral-options.json", "llm-trace-set.json"}, {"edge-cases.otlp.json", "neutral-options.json", "edge-set.json"}, {"genai-subset.otlp.json", "genai-options.json", "genai-set.json"}, {"openinference-subset.otlp.json", "openinference-options.json", "openinference-set.json"}} {
		raw, e := os.ReadFile(filepath.Join(base, tc.input))
		if e != nil {
			panic(e)
		}
		opts, e := os.ReadFile(filepath.Join(base, tc.options))
		if e != nil {
			panic(e)
		}
		o, e := p.DecodeLLMOptions(opts)
		if e != nil {
			panic(e)
		}
		set, e := p.ParseLLMBytes(context.Background(), raw, o)
		if e != nil {
			panic(e)
		}
		body, e := json.MarshalIndent(set, "", "  ")
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(base, tc.output), append(body, '\n'), 0600); e != nil {
			panic(e)
		}
	}
}
