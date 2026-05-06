package threaddump

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// fakePlugin is a deterministic in-memory plugin used to drive
// registry behaviour without dragging in the real jstack / goroutine /
// pyspy ports.
type fakePlugin struct {
	formatID  string
	language  string
	matcher   string
	parseFunc func(path string) (models.ThreadDumpBundle, error)
}

func (f *fakePlugin) FormatID() string  { return f.formatID }
func (f *fakePlugin) Language() string  { return f.language }
func (f *fakePlugin) CanParse(head string) bool {
	return strings.Contains(head, f.matcher)
}
func (f *fakePlugin) Parse(path string) (models.ThreadDumpBundle, error) {
	if f.parseFunc != nil {
		return f.parseFunc(path)
	}
	return models.ThreadDumpBundle{
		SourceFile:   path,
		SourceFormat: f.formatID,
		Language:     f.language,
		Snapshots:    []models.ThreadSnapshot{},
		Metadata:     map[string]any{},
	}, nil
}

type fakeMultiPlugin struct {
	*fakePlugin
	bundlesPerFile int
}

func (f *fakeMultiPlugin) ParseAll(path string) ([]models.ThreadDumpBundle, error) {
	out := make([]models.ThreadDumpBundle, 0, f.bundlesPerFile)
	for i := 0; i < f.bundlesPerFile; i++ {
		out = append(out, models.ThreadDumpBundle{
			SourceFile:   path,
			SourceFormat: f.formatID,
			Language:     f.language,
			Snapshots:    []models.ThreadSnapshot{},
			Metadata:     map[string]any{},
		})
	}
	return out, nil
}

func writeTempFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestRegisterAndPlugins(t *testing.T) {
	r := NewRegistry()
	a := &fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"}
	b := &fakePlugin{formatID: "go_goroutine", language: "go", matcher: "goroutine 1 ["}
	r.Register(a)
	r.Register(b)
	plugins := r.Plugins()
	if len(plugins) != 2 || plugins[0].FormatID() != "java_jstack" {
		t.Fatalf("Plugins() = %+v", plugins)
	}
}

func TestGetReturnsKnownFormat(t *testing.T) {
	r := NewRegistry()
	a := &fakePlugin{formatID: "java_jstack", language: "java"}
	r.Register(a)
	got, err := r.Get("java_jstack")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FormatID() != "java_jstack" {
		t.Fatalf("Get returned %q", got.FormatID())
	}
}

func TestGetUnknownFormat(t *testing.T) {
	r := NewRegistry()
	_, err := r.Get("does_not_exist")
	var ufe *UnknownFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *UnknownFormatError, got %T: %v", err, err)
	}
	if ufe.Source != "does_not_exist" {
		t.Fatalf("Source = %q", ufe.Source)
	}
}

func TestDetectFormatPicksFirstMatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"})
	r.Register(&fakePlugin{formatID: "go_goroutine", language: "go", matcher: "goroutine"})
	plugin := r.DetectFormat("Full thread dump of \"main\" #1 daemon")
	if plugin == nil || plugin.FormatID() != "java_jstack" {
		t.Fatalf("DetectFormat = %v", plugin)
	}
}

func TestDetectFormatNoMatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"})
	if got := r.DetectFormat("nothing recognizable here"); got != nil {
		t.Fatalf("DetectFormat should be nil, got %v", got)
	}
}

func TestParseOneRespectsFormatOverride(t *testing.T) {
	r := NewRegistry()
	java := &fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"}
	r.Register(java)
	// Body deliberately doesn't match the matcher; FormatOverride
	// must bypass head sniffing entirely.
	path := writeTempFile(t, "dump.txt", "no header bytes here\n")
	bundle, err := r.ParseOne(path, ParseOptions{FormatOverride: "java_jstack"})
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	if bundle.SourceFormat != "java_jstack" {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
	if bundle.DumpLabel == nil || *bundle.DumpLabel != "dump.txt" {
		t.Fatalf("DumpLabel = %v", bundle.DumpLabel)
	}
}

func TestParseOneSniffsFromHead(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{formatID: "go_goroutine", language: "go", matcher: "goroutine 1 [running]"})
	path := writeTempFile(t, "go-dump.txt", "goroutine 1 [running]:\nmain.foo()\n")
	bundle, err := r.ParseOne(path, ParseOptions{})
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	if bundle.SourceFormat != "go_goroutine" {
		t.Fatalf("SourceFormat = %q", bundle.SourceFormat)
	}
}

func TestParseOneUnknownFormatIncludesPreview(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"})
	path := writeTempFile(t, "garbage.txt", "this is not any known thread dump format\n")
	_, err := r.ParseOne(path, ParseOptions{})
	var ufe *UnknownFormatError
	if !errors.As(err, &ufe) {
		t.Fatalf("expected *UnknownFormatError, got %T: %v", err, err)
	}
	if ufe.HeadPreview == "" {
		t.Fatalf("HeadPreview should be populated")
	}
	if ufe.Source != path {
		t.Fatalf("Source = %q, want %q", ufe.Source, path)
	}
}

func TestParseManyRejectsMixedFormats(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"})
	r.Register(&fakePlugin{formatID: "go_goroutine", language: "go", matcher: "goroutine 1 ["})
	javaPath := writeTempFile(t, "java.txt", "Full thread dump\n")
	goPath := writeTempFile(t, "go.txt", "goroutine 1 [running]:\n")
	_, err := r.ParseMany([]string{javaPath, goPath}, ParseOptions{})
	var mfe *MixedFormatError
	if !errors.As(err, &mfe) {
		t.Fatalf("expected *MixedFormatError, got %T: %v", err, err)
	}
	if len(mfe.Formats) != 2 {
		t.Fatalf("Formats = %+v", mfe.Formats)
	}
	// Error message should list both formats.
	msg := mfe.Error()
	if !strings.Contains(msg, "java_jstack") || !strings.Contains(msg, "go_goroutine") {
		t.Fatalf("error message missing formats: %s", msg)
	}
}

func TestParseManyOverrideSkipsMixedCheck(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"})
	javaPath1 := writeTempFile(t, "a.txt", "Full thread dump #1\n")
	javaPath2 := writeTempFile(t, "b.txt", "anything\n")
	bundles, err := r.ParseMany([]string{javaPath1, javaPath2}, ParseOptions{FormatOverride: "java_jstack"})
	if err != nil {
		t.Fatalf("ParseMany: %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("bundles = %d, want 2", len(bundles))
	}
	if bundles[0].DumpIndex != 0 || bundles[1].DumpIndex != 1 {
		t.Fatalf("DumpIndex sequence wrong: %d, %d", bundles[0].DumpIndex, bundles[1].DumpIndex)
	}
}

func TestParseManyMultiBundlePlugin(t *testing.T) {
	r := NewRegistry()
	multi := &fakeMultiPlugin{
		fakePlugin: &fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"},
		bundlesPerFile: 3,
	}
	r.Register(multi)
	path := writeTempFile(t, "concat.txt", "Full thread dump\n")
	bundles, err := r.ParseMany([]string{path}, ParseOptions{})
	if err != nil {
		t.Fatalf("ParseMany: %v", err)
	}
	if len(bundles) != 3 {
		t.Fatalf("bundles = %d, want 3", len(bundles))
	}
	for i, b := range bundles {
		if b.DumpIndex != i {
			t.Errorf("bundles[%d].DumpIndex = %d, want %d", i, b.DumpIndex, i)
		}
		if b.DumpLabel == nil {
			t.Errorf("bundles[%d].DumpLabel nil", i)
			continue
		}
		want := fmt.Sprintf("concat.txt#%d", i+1)
		if *b.DumpLabel != want {
			t.Errorf("bundles[%d].DumpLabel = %q, want %q", i, *b.DumpLabel, want)
		}
	}
}

func TestParseManyDumpIndexAcrossFiles(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakePlugin{formatID: "java_jstack", language: "java", matcher: "Full thread dump"})
	a := writeTempFile(t, "a.txt", "Full thread dump\n")
	b := writeTempFile(t, "b.txt", "Full thread dump\n")
	bundles, err := r.ParseMany([]string{a, b}, ParseOptions{})
	if err != nil {
		t.Fatalf("ParseMany: %v", err)
	}
	if len(bundles) != 2 {
		t.Fatalf("bundles = %d", len(bundles))
	}
	if bundles[0].DumpIndex != 0 || bundles[1].DumpIndex != 1 {
		t.Errorf("DumpIndex = %d, %d", bundles[0].DumpIndex, bundles[1].DumpIndex)
	}
}

func TestParseOneSurfacesPluginError(t *testing.T) {
	r := NewRegistry()
	want := errors.New("plugin exploded")
	r.Register(&fakePlugin{
		formatID: "java_jstack", language: "java", matcher: "Full thread dump",
		parseFunc: func(path string) (models.ThreadDumpBundle, error) {
			return models.ThreadDumpBundle{}, want
		},
	})
	path := writeTempFile(t, "x.txt", "Full thread dump\n")
	_, err := r.ParseOne(path, ParseOptions{})
	if !errors.Is(err, want) {
		t.Fatalf("error mismatch: %v", err)
	}
}

func TestDefaultRegistryIsUsable(t *testing.T) {
	// Sanity: package-level singleton is constructed and accepts
	// registrations like a normal registry.
	if DefaultRegistry == nil {
		t.Fatal("DefaultRegistry is nil")
	}
	before := len(DefaultRegistry.Plugins())
	DefaultRegistry.Register(&fakePlugin{formatID: "test_only", language: "test"})
	t.Cleanup(func() {
		// Trim our test plugin so the singleton stays clean across tests.
		DefaultRegistry.plugins = DefaultRegistry.plugins[:before]
	})
	if len(DefaultRegistry.Plugins()) != before+1 {
		t.Fatalf("DefaultRegistry register did not append")
	}
}

func TestUnknownFormatErrorTruncatesPreview(t *testing.T) {
	preview := strings.Repeat("a", 500)
	err := &UnknownFormatError{Source: "x", HeadPreview: preview}
	msg := err.Error()
	// 200-char truncation in error message.
	if !strings.Contains(msg, strings.Repeat("a", 200)) {
		t.Fatalf("preview not present at full 200 chars: %s", msg[:100])
	}
	if strings.Contains(msg, strings.Repeat("a", 201)) {
		t.Fatalf("preview not truncated to 200 chars")
	}
}
