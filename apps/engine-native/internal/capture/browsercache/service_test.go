package browsercache

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type memoryBackend struct {
	entries []Entry
	deleted []Entry
}

func (*memoryBackend) Available() bool             { return true }
func (b *memoryBackend) Entries() ([]Entry, error) { return append([]Entry(nil), b.entries...), nil }
func (b *memoryBackend) Delete(entry Entry) error  { b.deleted = append(b.deleted, entry); return nil }
func TestRecordedCleanupDoesNotWidenURLDomainOrPath(t *testing.T) {
	b := &memoryBackend{entries: []Entry{{Key: "https://example.test/app?token=secret", Kind: "cache"}, {Key: "https://example.test/other", Kind: "cache"}, {Key: "Cookie:private-user@example.test/app", Kind: "cookies"}, {Key: "Cookie:user@notexample.test/", Kind: "cookies"}, {Key: "Cookie:user@example.test/apple", Kind: "cookies"}}}
	s := New(b)
	p, err := s.Plan("cache", "recorded", []string{"https://example.test/app?token=secret"})
	if err != nil || p.Count != 1 {
		t.Fatal(p, err)
	}
	raw, _ := json.Marshal(p)
	if strings.Contains(string(raw), "secret") || strings.Contains(string(raw), "private-user") {
		t.Fatal("preview disclosed query or cookie user")
	}
	c, err := s.Plan("cookies", "recorded", []string{"https://sub.example.test/app/item"})
	if err != nil || c.Count != 1 {
		t.Fatal(c, err)
	}
	if len(b.deleted) != 0 {
		t.Fatal("preview mutated cache")
	}
	if r, err := s.Execute(c.ID); err != nil || r.Deleted != 1 {
		t.Fatal(r, err)
	}
	if r, err := s.Execute(c.ID); err != nil || r.Deleted != 1 || len(b.deleted) != 1 {
		t.Fatal("retry repeated deletion")
	}
}
func TestPreviewExpiresAndChangedEntriesAreSkipped(t *testing.T) {
	b := &memoryBackend{entries: []Entry{{Key: "https://example.test/", Kind: "cache", Modified: 1}}}
	s := New(b)
	p, err := s.Plan("cache", "all", nil)
	if err != nil {
		t.Fatal(err)
	}
	b.entries[0].Modified = 2
	if r, err := s.Execute(p.ID); err != nil || r.Changed != 1 || r.Deleted != 0 {
		t.Fatal(r, err)
	}
	p, err = s.Plan("cache", "all", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return p.ExpiresAt }
	if _, err = s.Execute(p.ID); err == nil {
		t.Fatal("expired preview executed")
	}
	if len(b.deleted) != 0 {
		t.Fatal("unexpected deletion")
	}
}
func TestCookiePathBoundaryAndUnknownTypes(t *testing.T) {
	e := Entry{Key: "Cookie:user@example.test/app", Kind: "cookies"}
	for _, raw := range []string{"https://example.test/apple", "https://example.test/app%2Fhidden", "https://notexample.test/app", "file:///app", "https://example.test/"} {
		if matches(e, []string{raw}) {
			t.Fatal(raw)
		}
	}
	if !matches(e, []string{"https://example.test/app"}) {
		t.Fatal("exact path missing")
	}
	s := New(&memoryBackend{})
	for _, kind := range []string{"history", "profile", ""} {
		if _, err := s.Plan(kind, "all", nil); err == nil {
			t.Fatal("unsupported cleanup")
		}
	}
	if _, err := s.Execute("unknown"); err == nil {
		t.Fatal("unknown plan accepted")
	}
}
