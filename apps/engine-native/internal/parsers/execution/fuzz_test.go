package execution_test

import (
	"context"
	"encoding/json"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/execution"
	"os"
	"testing"
	"time"
)

func FuzzExecutionTextCSVJSON(f *testing.F) {
	cases := []struct{ input, options string }{{"hs-err-21.0.10.txt", "crash-options.json"}, {"jmeter-5.6.3.csv", "jmeter-options.json"}, {"k6-1.3.0.json", "k6-options.json"}}
	options := []p.Options{}
	for i, c := range cases {
		raw, err := os.ReadFile("../../catalog/contracts/execution-evidence-" + c.input)
		if err != nil {
			f.Fatal(err)
		}
		encoded, err := os.ReadFile("../../catalog/contracts/execution-evidence-" + c.options)
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
	f.Add(uint8(0), []byte("\xff\x00\n"))
	f.Add(uint8(1), []byte("timeStamp,elapsed\n\"unterminated"))
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
		if _, err = e.Decode(context.Background(), body); err != nil {
			t.Fatal("canonical readback", err)
		}
	})
}
