package threaddumpcollapsed

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// jstackBundle reproduces the Python test fixture (`_JSTACK`) at the
// model layer. The Python tests parse two `worker-1` snapshots with the
// same stack and one `proxy-handler` snapshot whose CGLIB suffix has
// already been normalized away by the parser plugin (T-194). Because
// this package operates on already-parsed bundles, we encode the
// post-normalization shape directly.
func jstackBundle() models.ThreadDumpBundle {
	bundle := models.NewThreadDumpBundle("td.txt", "java_jstack", "java")
	worker := workerSnapshot("worker-1::a")
	workerCopy := workerSnapshot("worker-1::b")
	proxy := proxySnapshot("proxy-handler::a")
	bundle.Snapshots = []models.ThreadSnapshot{worker, workerCopy, proxy}
	return bundle
}

func workerSnapshot(id string) models.ThreadSnapshot {
	snap := models.NewThreadSnapshot(id, "worker-1", models.ThreadStateRunnable)
	worker := "com.example.Worker"
	pool := "com.example.Pool"
	// Snapshots are stored top-of-stack first (matching jstack output);
	// the converter is responsible for reversing them so the root sits
	// on the left of the collapsed line.
	snap.StackFrames = []models.StackFrame{
		{Function: "run", Module: &worker},
		{Function: "exec", Module: &pool},
	}
	return snap
}

func proxySnapshot(id string) models.ThreadSnapshot {
	snap := models.NewThreadSnapshot(id, "proxy-handler", models.ThreadStateRunnable)
	payment := "com.example.PaymentService"
	snap.StackFrames = []models.StackFrame{
		{Function: "charge", Module: &payment},
	}
	return snap
}

func TestConvertAggregatesIdenticalStacks(t *testing.T) {
	counts := Convert([]models.ThreadDumpBundle{jstackBundle()}, DefaultOptions())
	workerKey := "worker-1;com.example.Pool.exec;com.example.Worker.run"
	if got := counts[workerKey]; got != 2 {
		t.Fatalf("expected worker stack count 2, got %d (counts=%v)", got, counts)
	}
	matched := false
	for key := range counts {
		if strings.HasPrefix(key, "proxy-handler;com.example.PaymentService.charge") {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("expected proxy-handler stack present, got %v", counts)
	}
}

func TestConvertOmitsThreadNameWhenRequested(t *testing.T) {
	counts := Convert([]models.ThreadDumpBundle{jstackBundle()}, Options{IncludeThreadName: false})
	key := "com.example.Pool.exec;com.example.Worker.run"
	if got := counts[key]; got != 2 {
		t.Fatalf("expected stack-only worker count 2, got %d (counts=%v)", got, counts)
	}
	for key := range counts {
		if strings.HasPrefix(key, "worker-1;") {
			t.Fatalf("expected no thread-name prefix, found %q", key)
		}
	}
}

func TestConvertAggregatesAcrossMultipleBundles(t *testing.T) {
	bundles := []models.ThreadDumpBundle{jstackBundle(), jstackBundle(), jstackBundle()}
	counts := Convert(bundles, DefaultOptions())
	key := "worker-1;com.example.Pool.exec;com.example.Worker.run"
	if got := counts[key]; got != 6 {
		t.Fatalf("expected aggregated worker count 6, got %d", got)
	}
}

func TestSortedLinesPlacesHighestCountFirst(t *testing.T) {
	counts := Convert([]models.ThreadDumpBundle{jstackBundle()}, DefaultOptions())
	lines := SortedLines(counts)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines, got 0")
	}
	head := lines[0]
	if !strings.HasSuffix(head, " 2") {
		t.Fatalf("expected head line to end with ' 2', got %q", head)
	}
	if !strings.HasPrefix(head, "worker-1;") {
		t.Fatalf("expected head line to start with 'worker-1;', got %q", head)
	}
}

func TestWriteCollapsedEmitsSortedLines(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "nested", "result.collapsed")
	unique, err := WriteCollapsed([]models.ThreadDumpBundle{jstackBundle()}, output, DefaultOptions())
	if err != nil {
		t.Fatalf("WriteCollapsed: %v", err)
	}
	if unique == 0 {
		t.Fatalf("expected at least one unique stack")
	}
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	lines := nonEmptyLines(string(raw))
	if len(lines) != unique {
		t.Fatalf("expected %d non-empty lines, got %d", unique, len(lines))
	}
	if !strings.HasSuffix(lines[0], " 2") {
		t.Fatalf("expected head line count 2, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[0], "worker-1;") {
		t.Fatalf("expected head line to start with 'worker-1;', got %q", lines[0])
	}
}

func TestSanitizeReplacesSemicolonsAndNewlines(t *testing.T) {
	bundle := models.NewThreadDumpBundle("py.txt", "py_spy", "python")
	snap := models.NewThreadSnapshot("py::1", "weird;name", models.ThreadStateRunnable)
	snap.StackFrames = []models.StackFrame{
		{Function: "handler\nwith;ws"},
	}
	bundle.Snapshots = []models.ThreadSnapshot{snap}
	counts := Convert([]models.ThreadDumpBundle{bundle}, DefaultOptions())

	for key := range counts {
		head := strings.SplitN(key, ";", 2)[0]
		if strings.ContainsAny(head, ";\n\r") {
			t.Fatalf("thread-name segment leaked separator chars: %q", head)
		}
	}
	matched := false
	for key := range counts {
		if strings.HasPrefix(key, "weird_name;") {
			matched = true
		}
	}
	if !matched {
		t.Fatalf("expected sanitized 'weird_name' prefix, got %v", counts)
	}
	// Newlines collapsed to spaces, semicolons replaced — no key must
	// contain a newline anywhere.
	for key := range counts {
		if strings.ContainsAny(key, "\n\r") {
			t.Fatalf("collapsed line contained newline: %q", key)
		}
	}
}

func TestEmptyStackSnapshotsAreSkipped(t *testing.T) {
	bundle := models.NewThreadDumpBundle("td.txt", "java_jstack", "java")
	bundle.Snapshots = []models.ThreadSnapshot{
		models.NewThreadSnapshot("empty::1", "ghost", models.ThreadStateRunnable),
	}
	counts := Convert([]models.ThreadDumpBundle{bundle}, DefaultOptions())
	if len(counts) != 0 {
		t.Fatalf("expected empty-stack snapshots to be skipped, got %v", counts)
	}
}

func TestRenderFrameWithoutModule(t *testing.T) {
	bundle := models.NewThreadDumpBundle("td.txt", "go_runtime", "go")
	snap := models.NewThreadSnapshot("go::1", "g0", models.ThreadStateRunnable)
	snap.StackFrames = []models.StackFrame{
		{Function: "runtime.gopark"},
	}
	bundle.Snapshots = []models.ThreadSnapshot{snap}
	counts := Convert([]models.ThreadDumpBundle{bundle}, Options{IncludeThreadName: false})
	if got := counts["runtime.gopark"]; got != 1 {
		t.Fatalf("expected bare function rendering, got %v", counts)
	}
}

func TestSortedLinesTieBreaksLexicographically(t *testing.T) {
	bundle := models.NewThreadDumpBundle("td.txt", "java_jstack", "java")
	for _, name := range []string{"alpha", "beta"} {
		snap := models.NewThreadSnapshot("t::"+name, name, models.ThreadStateRunnable)
		mod := "pkg"
		snap.StackFrames = []models.StackFrame{{Function: "fn", Module: &mod}}
		bundle.Snapshots = append(bundle.Snapshots, snap)
	}
	counts := Convert([]models.ThreadDumpBundle{bundle}, DefaultOptions())
	lines := SortedLines(counts)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d (%v)", len(lines), lines)
	}
	if !strings.HasPrefix(lines[0], "alpha;") {
		t.Fatalf("expected lex-first 'alpha' to lead on tie, got %q", lines[0])
	}
}

func nonEmptyLines(text string) []string {
	out := make([]string, 0)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
