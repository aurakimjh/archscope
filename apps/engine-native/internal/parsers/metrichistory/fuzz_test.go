package metrichistory_test

import (
	"context"
	"encoding/json"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"os"
	"testing"
	"time"
)

func FuzzSocketSummaryText(f *testing.F) {
	baseline, err := os.ReadFile("../../catalog/contracts/system-db-linux-ss-capture.json")
	if err != nil {
		f.Fatal(err)
	}
	var source map[string]any
	if err = json.Unmarshal(baseline, &source); err != nil {
		f.Fatal(err)
	}
	text := source["snapshots"].([]any)[0].(map[string]any)["summary"].(string)
	f.Add(text)
	f.Add("")
	f.Add("TCP: inuse 18446744073709551616\n")
	f.Add("\xff\x00\r\n")
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 64<<10 {
			t.Skip()
		}
		var input map[string]any
		if err := json.Unmarshal(baseline, &input); err != nil {
			t.Fatal(err)
		}
		for _, snapshot := range input["snapshots"].([]any) {
			snapshot.(map[string]any)["summary"] = text
		}
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		set, err := p.ParseBytes(ctx, raw, "synthetic-socket-capture", p.Options{})
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
		if _, err = m.Decode(body); err != nil {
			t.Fatal("canonical readback", err)
		}
	})
}
