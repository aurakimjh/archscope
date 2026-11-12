package main

import (
	"context"
	"encoding/json"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT638ActualCLIMetricHistoryRoundtripAndLegacy(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "engine")
	if output, e := exec.Command("go", "build", "-o", executable, ".").CombinedOutput(); e != nil {
		t.Fatal(e, string(output))
	}
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		out, e := exec.CommandContext(ctx, executable, args...).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(out))
		}
		return out
	}
	root := t.TempDir()
	input := "../../internal/parsers/metrichistory/testdata/snapshots.json"
	modelPath := filepath.Join(root, "model.json")
	run("metrics", "history-normalize", "--in", input, "--out", modelPath)
	body, e := os.ReadFile(modelPath)
	if e != nil {
		t.Fatal(e)
	}
	set, e := m.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	selection := m.Selection{SeriesIDs: []string{set.Series[0].ID}}
	options, _ := json.Marshal(selection)
	saved := filepath.Join(root, "result.json")
	run("evidence", "import", "--in", modelPath, "--importer", "metrics.history-json", "--options", string(options), "--out", saved)
	q, _ := json.Marshal(m.Query{Selection: selection, Limit: 2})
	out := run("metrics", "history-query", "--in", saved, "--query", string(q))
	var page m.Page
	if e = json.Unmarshal(out, &page); e != nil || page.Total != 4 || page.Next == "" || *page.Rows[1].Rate != "7" {
		t.Fatal(string(out), e)
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		run("evidence", "export", "--in", saved, "--export-format", format, "--out", filepath.Join(root, "export."+format))
	}
	merged := filepath.Join(root, "merged.json")
	run("metrics", "history-merge", "--inputs", modelPath+","+modelPath, "--out", merged)
	body, e = os.ReadFile(merged)
	if e != nil {
		t.Fatal(e)
	}
	dedup, e := m.Decode(body)
	if e != nil || len(dedup.Points) != 4 || dedup.Duplicates != 4 {
		t.Fatal("merge", e)
	}
	if out, e := exec.Command(executable, "metrics", "history-normalize", "--in", input, "--out", modelPath).CombinedOutput(); e == nil || !strings.Contains(string(out), "EXISTS") {
		t.Fatal("overwrite allowed", e, string(out))
	}
	legacy := filepath.Join(root, "snapshot.prom")
	os.WriteFile(legacy, []byte("# TYPE synthetic gauge\nsynthetic 3\n"), 0600)
	out = run("metrics", "import", "--in", legacy)
	if !strings.Contains(string(out), `"type": "metrics_snapshot"`) || strings.Contains(string(out), "metrics_core") {
		t.Fatal("legacy semantics changed", string(out))
	}
}
