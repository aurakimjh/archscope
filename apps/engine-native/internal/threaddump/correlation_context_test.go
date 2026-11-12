package threaddump_test

import (
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/multithread"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	td "github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/javajstack"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/threaddump/plugins/pythondump"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR18ActualPython314NamesAndUnknownHeaderRefusal(t *testing.T) {
	path := "../catalog/contracts/runtime-evidence-faulthandler-3.14.4.txt"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := pythondump.FaulthandlerPlugin{}
	if !p.CanParse(string(raw)) {
		t.Fatal("actual header unrecognized")
	}
	b, err := p.Parse(path)
	if err != nil || len(b.Snapshots) != 3 {
		t.Fatal(len(b.Snapshots), err)
	}
	names := map[string]bool{}
	for _, s := range b.Snapshots {
		names[s.ThreadName] = true
	}
	if !names["synthetic-w1"] || !names["synthetic-w2"] {
		t.Fatal(names)
	}
	path = filepath.Join(t.TempDir(), "bad.txt")
	os.WriteFile(path, []byte("Thread 0x1 [worker] unsupported header\n"), 0600)
	if _, err = p.Parse(path); err == nil || !strings.Contains(err.Error(), "RUNTIME_THREAD_FORMAT_UNRECOGNIZED") {
		t.Fatal(err)
	}
}
func TestR18ActualJstackContextPathAndDecimalNid(t *testing.T) {
	bundles := []models.ThreadDumpBundle{}
	for _, file := range []string{"jdk21-blocked-1.txt", "jdk21-blocked-2.txt"} {
		b, err := javajstack.New().Parse("../catalog/contracts/runtime-evidence-" + file)
		if err != nil {
			t.Fatal(err)
		}
		b.DumpIndex = len(bundles)
		bundles = append(bundles, b)
		for _, s := range b.Snapshots {
			if n, ok := s.Metadata["native_tid"].(string); ok && !strings.HasPrefix(n, "0x") && s.Metadata["native_tid_encoding"] != "os_tid_decimal" {
				t.Fatal(s.Metadata)
			}
		}
	}
	out, err := a.Analyze(bundles, a.Options{Threshold: 2})
	if err != nil {
		t.Fatal(err)
	}
	if out.Metadata.Diagnostics.WarningCount == 0 {
		t.Fatal("missing context silently suppressed persistence")
	}
	raw, err := os.ReadFile("../catalog/contracts/runtime-evidence-jdk21-correlation-context.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := td.DecodeCorrelationContext(raw)
	if err != nil {
		t.Fatal(err)
	}
	qualified, err := td.ApplyCorrelationContext(bundles, c)
	if err != nil {
		t.Fatal(err)
	}
	out, err = a.Analyze(qualified, a.Options{Threshold: 2})
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary["persistent_blocked_threads"] != 1 {
		t.Fatal(out.Summary)
	}
	if bundles[0].Metadata["correlation_context"] != nil {
		t.Fatal("caller bundle mutated")
	}
	c.Threads[0].RuntimeID = "not-observed"
	if _, err = td.ApplyCorrelationContext(bundles, c); err == nil {
		t.Fatal("unobserved id accepted")
	}
}
