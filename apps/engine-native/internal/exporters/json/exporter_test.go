package json

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestWriteRoundTripsAnalysisResult(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "nested", "result.json")

	result := models.New("access_log", "nginx_combined_with_response_time")
	result.SourceFiles = []string{"sample.log"}
	result.Summary = map[string]any{"total_requests": 1, "avg_response_ms": 123.4}
	result.Series = map[string]any{
		"requests_per_minute": []map[string]any{
			{"time": "2026-04-29T10:00:00+0900", "value": 1},
		},
	}
	result.Tables = map[string]any{
		"sample_records": []map[string]any{{"uri": "/api/orders", "status": 200}},
	}

	if err := Write(out, result); err != nil {
		t.Fatalf("Write: %v", err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasSuffix(string(body), "\n") {
		t.Errorf("output should end with newline; got %q", body[len(body)-5:])
	}

	var loaded any
	if err := json.Unmarshal(body, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	loadedMap, ok := loaded.(map[string]any)
	if !ok {
		t.Fatalf("loaded payload is not a map: %T", loaded)
	}
	if loadedMap["type"] != "access_log" {
		t.Errorf("type = %v, want access_log", loadedMap["type"])
	}
	summary, _ := loadedMap["summary"].(map[string]any)
	if summary["total_requests"].(float64) != 1 {
		t.Errorf("summary.total_requests = %v", summary["total_requests"])
	}
}

func TestWriteCreatesParentDirectories(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "deeply", "nested", "missing", "dir", "result.json")
	if err := Write(out, map[string]string{"hello": "world"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output not created: %v", err)
	}
}

func TestWriteRoundTripsPlainDict(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "result.json")
	payload := map[string]any{
		"type": "custom",
		"metadata": map[string]any{
			"schema_version": "0.1.0",
		},
	}
	if err := Write(out, payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	body, _ := os.ReadFile(out)
	var loaded map[string]any
	if err := json.Unmarshal(body, &loaded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if loaded["type"] != "custom" {
		t.Errorf("type = %v", loaded["type"])
	}
}

func TestMarshalPreservesNonASCIIVerbatim(t *testing.T) {
	body, err := Marshal(map[string]string{"label": "한글"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), "한글") {
		t.Errorf("non-ASCII not preserved: %q", body)
	}
	// JSON should NOT contain escaped \uXXXX sequences for the Korean
	// characters — Python's ensure_ascii=False parity.
	if strings.Contains(string(body), `\u`) {
		t.Errorf("Marshal escaped non-ASCII: %q", body)
	}
}

func TestMarshalDoesNotEscapeHTMLChars(t *testing.T) {
	body, err := Marshal(map[string]string{"text": "<a href='x'>&Y</a>"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), "<a href='x'>&Y</a>") {
		t.Errorf("HTML chars escaped: %q", body)
	}
}

func TestMarshalIndentsTwoSpaces(t *testing.T) {
	body, err := Marshal(map[string]any{"a": map[string]any{"b": 1}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), "  \"a\": {") {
		t.Errorf("expected 2-space indent; got %q", body)
	}
}
