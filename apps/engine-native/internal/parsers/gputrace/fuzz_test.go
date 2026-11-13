package gputrace_test

import (
	"context"
	"encoding/json"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gputrace"
	"os"
	"testing"
	"time"
)

func FuzzGPUJSONSQLite(f *testing.F) {
	cases := []struct{ input, options string }{{"pytorch-rank-0.json", "pytorch-rank-0-options.json"}, {"nsight-sanitized.sqlite", "nsight-options.json"}}
	options := []p.Options{}
	for i, c := range cases {
		raw, err := os.ReadFile("../../catalog/contracts/gpu-execution-" + c.input)
		if err != nil {
			f.Fatal(err)
		}
		encoded, err := os.ReadFile("../../catalog/contracts/gpu-execution-" + c.options)
		if err != nil {
			f.Fatal(err)
		}
		option, err := p.DecodeOptions(encoded)
		if err != nil {
			f.Fatal(err)
		}
		options = append(options, option)
		f.Add(uint8(i), raw)
	}
	f.Add(uint8(0), []byte(`{"traceEvents":[`))
	f.Add(uint8(1), []byte("SQLite format 3\x00"))
	f.Fuzz(func(t *testing.T, kind uint8, raw []byte) {
		if len(raw) > 256<<10 {
			t.Skip()
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		set, err := p.ParseBytes(ctx, raw, options[int(kind)%len(options)])
		if err != nil {
			return
		}
		if err = set.Validate(context.Background()); err != nil {
			t.Fatal("parser published invalid model", err)
		}
		body, err := json.Marshal(set)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = g.Decode(context.Background(), body); err != nil {
			t.Fatal("canonical readback", err)
		}
	})
}
