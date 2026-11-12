//go:build ignore

package main

import (
	"context"
	"encoding/json"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/systemtrace"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"os"
	"path/filepath"
)

func main() {
	base := "internal/parsers/systemtrace/testdata"
	for _, tc := range []struct{ file, format, capture, output string }{{"perf-script.txt", "perf-script", "perf-capture.json", "perf-set.json"}, {"events.jsonl", "events-jsonl", "events-capture.json", "system-evidence-set.json"}, {"bpftrace-hist.jsonl", "bpftrace-jsonl", "bpftrace-capture.json", "histogram-set.json"}} {
		raw, e := os.ReadFile(filepath.Join(base, tc.capture))
		if e != nil {
			panic(e)
		}
		var capture m.Capture
		if e = m.Strict(raw, &capture, 128<<10); e != nil {
			panic(e)
		}
		s, e := p.ParseFile(context.Background(), filepath.Join(base, tc.file), p.Options{Format: tc.format, Capture: capture})
		if e != nil {
			panic(e)
		}
		body, _ := json.Marshal(s)
		if e = os.WriteFile(filepath.Join(base, tc.output), body, 0600); e != nil {
			panic(e)
		}
	}
}
