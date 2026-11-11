package cases

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestR601ExportCaseAliasesAndManagedHardlinks(t *testing.T) {
	s, root := mustStore(t)
	m := mustCreate(t, s)
	head := filepath.Join(root, "cases", m.ID, "HEAD.json")
	before, _ := os.ReadFile(head)
	alias := filepath.Join(filepath.Dir(root), strings.ToUpper(filepath.Base(root)), "CASES", strings.ToUpper(m.ID), "HEAD.JSON")
	if _, e := os.Stat(alias); e == nil {
		if e = s.CheckExportDestination(m.ID, alias); e == nil {
			t.Fatal("case-insensitive alias allowed")
		}
	} else {
		t.Log("case-alias probe not available on this filesystem")
	}
	link := filepath.Join(t.TempDir(), "ordinary-export.json")
	if e := os.Link(head, link); e != nil {
		t.Fatal(e)
	}
	if e := s.CheckExportDestination(m.ID, link); e == nil {
		t.Fatal("managed hardlink allowed")
	}
	future := filepath.Join(filepath.Dir(root), strings.ToUpper(filepath.Base(root)), "CASES", strings.ToUpper(m.ID), "new.json")
	if _, e := os.Stat(filepath.Dir(future)); e == nil {
		if e = s.CheckExportDestination(m.ID, future); e == nil {
			t.Fatal("case-differing parent allowed")
		}
	}
	if e := s.CheckExportDestination(m.ID, filepath.Join(t.TempDir(), "safe.json")); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(head)
	if !bytes.Equal(before, after) {
		t.Fatal("guard changed authority")
	}
}
func TestR603SaveIsIdempotentAcrossPriorReferenceOrder(t *testing.T) {
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	body := resultBody(t)
	req := SaveResult{CaseID: m.ID, ExpectedRevision: m.Revision, Selection: json.RawMessage(`{"choice":1}`), SourceIDs: []string{}}
	m, e := s.SaveResult(req, body)
	if e != nil {
		t.Fatal(e)
	}
	first := m.Results[0]
	other := bytes.Replace(body, []byte(`"zero":0`), []byte(`"zero":1`), 1)
	if bytes.Equal(other, body) {
		t.Fatal("fixture unchanged")
	}
	req.ExpectedRevision = m.Revision
	m, e = s.SaveResult(req, other)
	if e != nil {
		t.Fatal(e)
	}
	revision := m.Revision
	req.ExpectedRevision = m.Revision
	req.Selection = json.RawMessage(`{ "choice" : 1 }`)
	for i := 0; i < 3; i++ {
		same, e := s.SaveResult(req, body)
		if e != nil || same.Revision != revision || len(same.Results) != 2 || same.Results[0].ID != first.ID {
			t.Fatal(same, e)
		}
	}
	req.OptionsHash = strings.Repeat("a", 64)
	different, e := s.SaveResult(req, body)
	if e != nil || len(different.Results) != 3 || different.Revision == revision {
		t.Fatal(different, e)
	}
}
func TestR604CorruptCaseDeletionIsExplicitAndTreeBound(t *testing.T) {
	s, root := mustStore(t)
	m := mustCreate(t, s)
	other := mustCreate(t, s)
	if _, e := s.CorruptDeletionPlan(context.Background(), m.ID); e == nil {
		t.Fatal("readable case allowed corrupt deletion")
	}
	external := file(t, "original.txt", []byte("synthetic original"))
	m = apply(t, s, m, Edit{Operation: "source", Path: external})
	head := filepath.Join(root, "cases", m.ID, "HEAD.json")
	os.WriteFile(head, []byte(`{"revision":"2","sha256":"`+strings.Repeat("0", 64)+`"}`), 0600)
	plan, e := s.CorruptDeletionPlan(context.Background(), m.ID)
	if e != nil || plan.Files == 0 || plan.Directory != "cases/"+m.ID {
		t.Fatal(plan, e)
	}
	if e = s.PurgeCorrupt(context.Background(), m.ID, ""); e == nil {
		t.Fatal("missing token accepted")
	}
	os.WriteFile(filepath.Join(root, "cases", m.ID, "changed.txt"), []byte("changed"), 0600)
	if e = s.PurgeCorrupt(context.Background(), m.ID, plan.Token); e == nil {
		t.Fatal("stale tree token accepted")
	}
	updated, e := s.CorruptDeletionPlan(context.Background(), m.ID)
	if e != nil || updated.Token == plan.Token {
		t.Fatal(updated, e)
	}
	if e = s.PurgeCorrupt(context.Background(), m.ID, updated.Token); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(root, "cases", m.ID)); !os.IsNotExist(e) {
		t.Fatal(e)
	}
	if _, e = s.Get(other.ID); e != nil {
		t.Fatal("other case removed", e)
	}
	if b, e := os.ReadFile(external); e != nil || string(b) != "synthetic original" {
		t.Fatal("external original changed", e)
	}
}
func TestR607LocalDisplayNamesAreSeparateFromArchivePaths(t *testing.T) {
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	name := "jstack_10:30:01.txt"
	filename := "source.txt"
	if runtime.GOOS != "windows" {
		filename = name
	}
	p := file(t, filename, []byte("synthetic\n"))
	m = apply(t, s, m, Edit{Operation: "source", Path: p, Name: name, Copy: true, SharingReviewed: true})
	if m.Sources[0].Name != name {
		t.Fatal(m.Sources)
	}
	if _, e := s.Portable(context.Background(), m.ID, ExportOptions{SourceIDs: []string{m.Sources[0].ID}, ManifestReviewed: true}); e != nil {
		t.Fatal("display label became archive path", e)
	}
	if safeName(name) {
		t.Fatal("archive rule weakened")
	}
	if _, e := prepareSource(Edit{Path: p, Name: "bad\nname"}); e == nil {
		t.Fatal("control character name allowed")
	}
}
func TestR606SourcePagesMatchEncodingFixturesAndReuseIndex(t *testing.T) {
	raw, _ := Contracts.ReadFile("contracts/evidence-text-v1.json")
	var fixtures struct {
		Cases []struct {
			ID, Encoding, Hex string
			AllowReplacement  bool `json:"allow_replacement"`
		}
	}
	if e := json.Unmarshal(raw, &fixtures); e != nil {
		t.Fatal(e)
	}
	for _, f := range fixtures.Cases {
		t.Run(f.ID, func(t *testing.T) {
			s, _ := mustStore(t)
			m := mustCreate(t, s)
			b, _ := hex.DecodeString(f.Hex)
			p := file(t, "source.txt", b)
			opts := DecodeOptions{Encoding: f.Encoding, AllowReplacement: f.AllowReplacement}
			d, e := DecodeText(b, opts)
			if e != nil {
				t.Fatal(e)
			}
			m = apply(t, s, m, Edit{Operation: "source", Path: p})
			page, e := s.SourcePage(context.Background(), m.ID, m.Sources[0].ID, opts, 0, 200)
			if e != nil {
				t.Fatal(e)
			}
			expected, _ := SourcePage(d, 0, 200)
			a, _ := json.Marshal(page)
			c, _ := json.Marshal(expected)
			if string(a) != string(c) {
				t.Fatal(string(a), string(c))
			}
			prior := s.page
			if _, e = s.SourcePage(context.Background(), m.ID, m.Sources[0].ID, opts, 0, 200); e != nil || s.page != prior {
				t.Fatal("index not reused", e)
			}
		})
	}
}
func TestR606IndexPrefixAndChangedSourceFailClosed(t *testing.T) {
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	raw := bytes.Repeat([]byte("x\n"), MaxIndexedLines+7)
	p := file(t, "short-lines.log", raw)
	m = apply(t, s, m, Edit{Operation: "source", Path: p})
	page, e := s.SourcePage(context.Background(), m.ID, m.Sources[0].ID, DecodeOptions{}, 0, 2)
	if e != nil || !page.IndexTruncated || page.TotalLines != strconv.Itoa(MaxIndexedLines+7) || page.IndexedLines != strconv.Itoa(MaxIndexedLines) || len(page.Lines) != 2 {
		t.Fatal(page, e)
	}
	pointer := s.page
	if _, e = s.SourcePage(context.Background(), m.ID, m.Sources[0].ID, DecodeOptions{}, 25, 2); e != nil || s.page != pointer {
		t.Fatal("cache miss", e)
	}
	info, _ := os.Stat(p)
	raw[0] = 'y'
	os.WriteFile(p, raw, 0600)
	os.Chtimes(p, info.ModTime(), info.ModTime())
	if _, e = s.SourcePage(context.Background(), m.ID, m.Sources[0].ID, DecodeOptions{}, 0, 2); e == nil || s.page != nil {
		t.Fatal("mtime spoof returned stale source", e)
	}
}
func TestR606LargePageMeasurement(t *testing.T) {
	if os.Getenv("ARCHSCOPE_T630_LARGE") != "1" {
		t.Skip("explicit synthetic 60 MiB allocation measurement")
	}
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	line := []byte(strings.Repeat("x", 77) + "\n")
	raw := bytes.Repeat(line, (60<<20)/len(line))
	p := file(t, "large.log", raw)
	m = apply(t, s, m, Edit{Operation: "source", Path: p})
	start := time.Now()
	first, e := s.SourcePage(context.Background(), m.ID, m.Sources[0].ID, DecodeOptions{}, 0, 200)
	if e != nil {
		t.Fatal(e)
	}
	initial := time.Since(start)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start = time.Now()
	for i := 0; i < 3; i++ {
		if _, e = s.SourcePage(context.Background(), m.ID, m.Sources[0].ID, DecodeOptions{}, 200*(i+1), 200); e != nil {
			t.Fatal(e)
		}
	}
	elapsed := time.Since(start)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 4<<20 {
		t.Fatal("page hits allocate full source/index", allocated)
	}
	t.Logf("synthetic_bytes=%d total_lines=%s first_ms=%.3f cached_3_ms=%.3f cached_3_alloc_bytes=%d compact_index_bytes=%d", len(raw), first.TotalLines, float64(initial.Microseconds())/1000, float64(elapsed.Microseconds())/1000, allocated, len(s.page.positions)*16)
}
