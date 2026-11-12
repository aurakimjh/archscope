//go:build ignore

package main

import (
	"context"
	"encoding/json"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/runtimeevidence"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"os"
	"path/filepath"
)

func main() {
	base := os.Args[1]
	ctx := context.Background()
	sets := []r.Set{}
	for _, tc := range []struct{ in, options, out string }{{"python-3.14.4.json", "python-options.json", "python-set.json"}, {"node-25.9.0.json", "node-options.json", "node-set.json"}, {"ps-l.csv", "cpu-options.json", "cpu-set.json"}, {"jstack.txt", "thread-options.json", "thread-set.json"}, {"postgres-estimated.json", "postgres-options.json", "plan-estimated-set.json"}, {"postgres-actual.json", "postgres-options.json", "plan-actual-set.json"}, {"rust-1.94.1-backtrace.txt", "rust-options.json", "rust-set.json"}} {
		raw, e := os.ReadFile(filepath.Join(base, tc.in))
		if e != nil {
			panic(e)
		}
		opts, e := os.ReadFile(filepath.Join(base, tc.options))
		if e != nil {
			panic(e)
		}
		o, e := p.DecodeOptions(opts)
		if e != nil {
			panic(e)
		}
		set, e := p.ParseBytes(ctx, raw, o)
		if e != nil {
			panic(e)
		}
		write(base, tc.out, set)
		sets = append(sets, set)
	}
	merged, e := r.Merge(ctx, sets)
	if e != nil {
		panic(e)
	}
	write(base, "runtime-evidence-set.json", merged)
}
func write(base, name string, v any) {
	raw, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(filepath.Join(base, name), append(raw, '\n'), 0600); e != nil {
		panic(e)
	}
}
