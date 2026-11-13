package traceimport

import (
	"encoding/json"
	csv "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	html "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	models "github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
	"os"
	"path/filepath"
	"testing"
)

func TestActualSentryUsesExistingTraceConsumers(t *testing.T) {
	r, e := Analyze("../../parsers/traceimport/testdata/sentry/actual.json", Options{Format: p.FormatSentryTransaction})
	if e != nil {
		t.Fatal(e)
	}
	if r.Type != "trace_import" || r.Metadata.SchemaVersion != "0.2.0" {
		t.Fatal(r.Type, r.Metadata)
	}
	raw, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	var old models.AnalysisResult
	if e = json.Unmarshal(raw, &old); e != nil {
		t.Fatal(e)
	}
	if old.Summary["total_spans"] != float64(3) {
		t.Fatal(old.Summary)
	}
	for _, exp := range []struct {
		name  string
		write func(string, any) error
	}{{"r.csv", csv.Write}, {"r.html", html.Write}} {
		path := filepath.Join(t.TempDir(), exp.name)
		if e = exp.write(path, r); e != nil {
			t.Fatal(e)
		}
		b, _ := os.ReadFile(path)
		if len(b) == 0 {
			t.Fatal("empty export")
		}
	}
}
