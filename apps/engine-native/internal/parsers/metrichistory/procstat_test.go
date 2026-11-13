package metrichistory_test

import (
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	csv "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	html "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"os"
	"path/filepath"
	"testing"
)

func TestActualLinuxProcStatScopeAndCommonConsumers(t *testing.T) {
	raw, e := os.ReadFile("testdata/procstat/actual.json")
	if e != nil {
		t.Fatal(e)
	}
	set := parse(t, raw, p.Options{})
	if len(set.Series) != 10 || len(set.Points) != 30 || set.Sources[0].Producer != "linux-proc-stat" || set.Sources[0].ProducerVersion != "7.0.14-linuxkit" {
		t.Fatal(set.Sources)
	}
	var f struct {
		History struct {
			Snapshots []struct {
				Metrics []struct {
					Name  string
					Value string
				}
			}
		}
	}
	if e = json.Unmarshal(raw, &f); e != nil {
		t.Fatal(e)
	}
	expected := map[string][]string{}
	for _, s := range f.History.Snapshots {
		for _, m := range s.Metrics {
			expected[m.Name] = append(expected[m.Name], m.Value)
		}
	}
	for _, r := range rows(t, set) {
		if r.Rate != nil || r.Delta != nil || r.Value == nil || r.Status != "observed_gauge" {
			t.Fatal(r)
		}
		v := expected[r.Name]
		if len(v) == 0 || v[0] != *r.Value {
			t.Fatal(r)
		}
		expected[r.Name] = v[1:]
	}
	result, e := a.Build(ctx, set, selected(set), "actual Docker Linux")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(result); e != nil {
		t.Fatal(e)
	}
	if e = csv.Write(filepath.Join(t.TempDir(), "r.csv"), result); e != nil {
		t.Fatal(e)
	}
	if e = html.Write(filepath.Join(t.TempDir(), "r.html"), result); e != nil {
		t.Fatal(e)
	}
	for _, change := range []func(map[string]any){func(c map[string]any) { c["producer_version"] = "0.0.0" }, func(c map[string]any) {
		c["history"].(map[string]any)["snapshots"].([]any)[0].(map[string]any)["metrics"].([]any)[0].(map[string]any)["kind"] = "sum"
	}} {
		bad := changedCapture(t, raw, change)
		if _, e = p.ParseBytes(ctx, bad, "bad", p.Options{}); e == nil {
			t.Fatal("accepted incompatible semantics")
		}
	}
}
