package metrics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseOpenMetricsSamples(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.prom")
	input := "# HELP http_requests_total requests\nhttp_requests_total{service=\"api\",code=\"500\"} 3 1778916000000\n# EOF\nbad line\n"
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}
	samples, diags, err := ParseFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(samples) != 1 || samples[0].Labels["service"] != "api" || samples[0].Value != 3 {
		t.Fatalf("samples=%+v", samples)
	}
	if diags.SkippedLines != 1 {
		t.Fatalf("diagnostics=%+v", diags)
	}
}
