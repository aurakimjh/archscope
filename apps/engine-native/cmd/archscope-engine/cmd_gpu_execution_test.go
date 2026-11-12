package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestT644ActualCLIAndCommonConsumers(t *testing.T) {
	t.Setenv("ARCHSCOPE_STATE_ROOT", t.TempDir())
	dir := t.TempDir()
	engine := buildBinary(t)
	run := func(args ...string) []byte {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, engine, args...).CombinedOutput()
		if err != nil {
			t.Fatal(args, err, string(out))
		}
		return out
	}
	source := "../../internal/catalog/contracts/gpu-execution-nsight-sanitized.sqlite"
	options := "../../internal/catalog/contracts/gpu-execution-nsight-options.json"
	model := filepath.Join(dir, "model.json")
	run("gpu-trace", "normalize-source", "--in", source, "--options-file", options, "--out", model)
	result := filepath.Join(dir, "result.json")
	run("evidence", "import", "--in", model, "--importer", "trace.gpu-execution-json", "--options", "{}", "--out", result)
	var set struct {
		Sources []struct {
			ID string `json:"id"`
		} `json:"sources"`
	}
	raw, _ := os.ReadFile(model)
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}
	q, _ := json.Marshal(map[string]any{"kind": "events", "source_id": set.Sources[0].ID, "limit": 1})
	out := run("gpu-trace", "query", "--in", result, "--query", string(q))
	if !strings.Contains(string(out), `"total":19`) {
		t.Fatal(string(out))
	}
	svg := filepath.Join(dir, "selection.svg")
	run("gpu-trace", "export-svg", "--in", result, "--query", string(q), "--out", svg)
	for _, format := range []string{"json", "raw-json", "csv", "html", "report-pack"} {
		run("evidence", "export", "--in", result, "--export-format", format, "--out", filepath.Join(dir, "view."+format))
	}
	run("evidence", "query", "--in", result, "--provider", "gpu:gpu_busy")
	args := []string{"gpu-trace", "query", "--in", result, "--query", `{"kind":"events","from_ns":"1","to_ns":"2","limit":1}`}
	if _, err := exec.Command(engine, args...).CombinedOutput(); err == nil {
		t.Fatal("cross-clock query accepted")
	}
}
