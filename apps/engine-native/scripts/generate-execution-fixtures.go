//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
	"os"
	"path/filepath"
)

func main() {
	dir := os.Args[1]
	for _, c := range []struct{ name, input string }{{"k6", "k6-1.3.0.json"}, {"llmperf", "llmperf-summary.json"}, {"jmeter", "jmeter-5.6.3.csv"}, {"crash", "hs-err-21.0.10.txt"}} {
		options, err := os.ReadFile(filepath.Join(dir, c.name+"-options.json"))
		must(err)
		o, err := p.DecodeOptions(options)
		must(err)
		raw, err := os.ReadFile(filepath.Join(dir, c.input))
		must(err)
		set, err := p.ParseBytes(context.Background(), raw, o)
		must(err)
		body, err := json.MarshalIndent(set, "", "  ")
		must(err)
		must(os.WriteFile(filepath.Join(dir, c.name+"-set.json"), append(body, '\n'), 0644))
		if c.name == "k6" {
			must(os.WriteFile(filepath.Join(dir, "execution-evidence-set.json"), append(body, '\n'), 0644))
		}
		fmt.Println(c.name, "normalized")
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
