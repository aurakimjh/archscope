package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT639ActualCLIImportsQueriesExportsAndRefusals(t *testing.T) {
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	engine := buildBinary(t)
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, e := exec.CommandContext(ctx, engine, args...).CombinedOutput()
		if e != nil {
			t.Fatal(args, e, string(out))
		}
		return out
	}
	base := "../../internal/parsers/inference/testdata/"
	dir := t.TempDir()
	for _, mode := range []string{"nvidia-smi-csv", "metric-bindings-json"} {
		source, sidecar, flag := "nvidia-query.csv", "nvidia-capture.json", "--capture-file"
		if mode == "metric-bindings-json" {
			source, sidecar, flag = "metric-model.json", "bindings.json", "--bindings-file"
		}
		inspected := run("inference", "inspect-source", "--in", base+source, "--format", mode, flag, base+sidecar)
		if !strings.Contains(string(inspected), `"source_sha256"`) {
			t.Fatal(string(inspected))
		}
		model, result := filepath.Join(dir, mode+"-model.json"), filepath.Join(dir, mode+"-result.json")
		run("inference", "normalize-source", "--in", base+source, "--format", mode, flag, base+sidecar, "--out", model)
		run("evidence", "import", "--in", model, "--importer", "inference.evidence-json", "--options", "{}", "--out", result)
		for _, format := range []string{"json", "csv", "html", "report-pack"} {
			run("evidence", "export", "--in", result, "--export-format", format, "--out", filepath.Join(dir, mode+"."+format))
		}
		var inv struct {
			Signals []struct {
				ID string `json:"id"`
			} `json:"signals"`
		}
		json.Unmarshal(inspected, &inv)
		id := inv.Signals[0].ID
		query := run("inference", "query-metric", "--in", result, "--signal-id", id)
		if !strings.Contains(string(query), `"point_id"`) {
			t.Fatal(string(query))
		}
		metric := filepath.Join(dir, mode+"-metric.json")
		run("inference", "open-metrics", "--in", result, "--signal-id", id, "--out", metric)
		run("metrics", "history-query", "--in", metric, "--query", `{"selection":{"series_ids":["`+firstSeries(t, query)+`"]},"limit":25}`)
		run("visual", "create-spec", "--in", result, "--query", `{"kind":"provider_table","provider":"inference:observations","offset":0,"limit":25,"columns":["signal_id","original_value","normalized_value","normalized_unit","status"]}`, "--out", filepath.Join(dir, mode+"-visual.json"))
		if out, e := exec.Command(engine, "inference", "open-metrics", "--in", result, "--signal-id", "unknown").CombinedOutput(); e == nil || !strings.Contains(string(out), "INFERENCE_SIGNAL_SELECTION_INVALID") {
			t.Fatal("unmapped signal projected", e)
		}
		if out, e := exec.Command(engine, "inference", "normalize-source", "--in", base+source, "--format", mode, flag, base+sidecar, "--out", model).CombinedOutput(); e == nil || !strings.Contains(string(out), "EXISTS") {
			t.Fatal("overwrite allowed", e)
		}
	}
}
func firstSeries(t *testing.T, body []byte) string {
	t.Helper()
	var page struct {
		Rows []struct {
			Series string `json:"series_id"`
		} `json:"rows"`
	}
	if e := json.Unmarshal(body, &page); e != nil || len(page.Rows) == 0 {
		t.Fatal(e)
	}
	return page.Rows[0].Series
}
