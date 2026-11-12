package cases

import (
	"context"
	"encoding/json"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT635SearchCoverageCoordinatesCacheRebuildAndStaleness(t *testing.T) {
	root := t.TempDir()
	s, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := s.Create("search")
	if e != nil {
		t.Fatal(e)
	}
	original := filepath.Join(t.TempDir(), "source.log")
	if e = os.WriteFile(original, []byte("alpha\nAuthorization: Bearer synthetic-secret\nalpha third\n"), 0600); e != nil {
		t.Fatal(e)
	}
	m, e = s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "source", Path: original, Decode: DecodeOptions{Encoding: "utf-8"}})
	if e != nil {
		t.Fatal(e)
	}
	q := SearchQuery{CaseID: m.ID, Revision: m.Revision, Text: "alpha", Limit: 1}
	page, e := s.Search(context.Background(), q)
	if e != nil || page.Matched != 2 || page.Next == "" || page.Rows[0].Line.Number != "1" || len(page.Coverage) != 1 || page.Coverage[0].Total != 3 {
		t.Fatal(page, e)
	}
	next := q
	next.Cursor = page.Next
	page2, e := s.Search(context.Background(), next)
	if e != nil || page2.Rows[0].Line.Number != "3" {
		t.Fatal(page2, e)
	}
	cache := filepath.Join(root, "indexes", "search-"+m.ID+".json")
	data, e := os.ReadFile(cache)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(data), "synthetic-secret") {
		t.Fatal("persisted unredacted secret")
	}
	opened, e := s.OpenEvidence(context.Background(), m.ID, m.Revision, m.Sources[0].ID, m.Sources[0].SHA256, "1")
	if e != nil || len(opened.Lines) != 3 || strings.Contains(opened.Lines[1].Text, "synthetic-secret") {
		t.Fatal(opened, e)
	}
	if _, e = s.OpenEvidence(context.Background(), m.ID, m.Revision, m.Sources[0].ID, strings.Repeat("0", 64), "1"); e == nil {
		t.Fatal("stale source link")
	}
	if e = os.Remove(cache); e != nil {
		t.Fatal(e)
	}
	rebuilt, e := s.Search(context.Background(), q)
	if e != nil || rebuilt.IndexSHA256 != page.IndexSHA256 {
		t.Fatal(rebuilt, e)
	}
	os.WriteFile(cache, []byte("corrupt"), 0600)
	if _, e = s.Search(context.Background(), q); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(original, []byte("changed\n"), 0600)
	if _, e = s.Search(context.Background(), next); e == nil {
		t.Fatal("changed-original cursor accepted")
	}
	missing, e := s.Search(context.Background(), q)
	if e != nil || missing.Matched != 0 || missing.Coverage[0].Reason != "original_missing_or_changed" {
		t.Fatal(missing, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = s.Search(ctx, q); e == nil {
		t.Fatal("ignored cancel")
	}
	m, e = s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "details", Title: "edited"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Search(context.Background(), q); e == nil {
		t.Fatal("stale case revision")
	}
}
func TestT635SearchIndexBudgetDisclosesPrefix(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := s.Create("bounded")
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "many.log")
	os.WriteFile(p, []byte(strings.Repeat("entry\n", MaxSearchRows+1)), 0600)
	m, e = s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "source", Path: p})
	if e != nil {
		t.Fatal(e)
	}
	page, e := s.Search(context.Background(), SearchQuery{CaseID: m.ID, Revision: m.Revision, Text: "entry", Limit: 1})
	if e != nil || page.Coverage[0].Indexed >= page.Coverage[0].Total || page.Coverage[0].Total != MaxSearchRows+1 || page.Coverage[0].Reason != "indexed_prefix_only" {
		t.Fatal(page.Coverage, e)
	}
}

func TestT635OriginalCoordinateNeverEscapesMillionLinePrefix(t *testing.T) {
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := s.Create("prefix")
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "prefix.log")
	os.WriteFile(p, []byte(strings.Repeat("row\n", MaxIndexedLines+1)), 0600)
	m, e = s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "source", Path: p})
	if e != nil {
		t.Fatal(e)
	}
	page, e := s.OpenEvidence(context.Background(), m.ID, m.Revision, m.Sources[0].ID, m.Sources[0].SHA256, "1000001")
	if e == nil || !strings.Contains(e.Error(), "CASE_LINK_OUTSIDE_INDEX") || !page.IndexTruncated || page.IndexedLines != "1000000" {
		t.Fatal(page.IndexedLines, page.IndexTruncated, e)
	}
}

func TestT635CaseTypedTimeAndFrameIndexUsesCanonicalPopulation(t *testing.T) {
	ctx := context.Background()
	s, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	m, e := s.Create("typed")
	if e != nil {
		t.Fatal(e)
	}
	raw := []byte(`{"$schema":"https://www.speedscope.app/file-format-schema.json","shared":{"frames":[{"name":"root"},{"name":"leaf"}]},"profiles":[{"type":"evented","name":"worker","unit":"seconds","startValue":0,"endValue":2,"events":[{"type":"O","at":0,"frame":0},{"type":"O","at":1,"frame":1},{"type":"C","at":1.5,"frame":1},{"type":"C","at":2,"frame":0}]}]}`)
	file := filepath.Join(t.TempDir(), "scope.json")
	os.WriteFile(file, raw, 0600)
	m, e = s.Apply(ctx, Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "source", Path: file})
	if e != nil {
		t.Fatal(e)
	}
	set, e := parser.ParseSourceBytes(ctx, raw, parser.SourceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	selection := profilemodel.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "evented-seconds"}
	result, e := analyzer.BuildSet(ctx, set, selection, "scope")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(result)
	m, e = s.SaveResult(SaveResult{CaseID: m.ID, ExpectedRevision: m.Revision, SourceIDs: []string{m.Sources[0].ID}}, body)
	if e != nil {
		t.Fatal(e)
	}
	from, to := "1", "1.5"
	q := SearchQuery{CaseID: m.ID, Revision: m.Revision, ResultID: m.Results[0].ID, Field: "profile.observation", Text: "leaf", Limit: 25, TimeFrom: &from, TimeTo: &to, Clock: "relative", TimeUnit: "seconds"}
	page, e := s.Search(ctx, q)
	if e != nil || page.Matched != 1 || page.Rows[0].Timestamp == nil || *page.Rows[0].Timestamp != "1" || len(page.Rows[0].FrameIDs) != 2 {
		t.Fatal(page, e)
	}
	q.Text = set.Frames[1].ID
	page, e = s.Search(ctx, q)
	if e != nil || page.Matched != 1 {
		t.Fatal("frame ID index mismatch", page, e)
	}
	q.ResultID = ""
	if _, e = s.Search(ctx, q); e == nil || !strings.Contains(e.Error(), "CASE_TIME_SCOPE_REQUIRED") {
		t.Fatal("implicitly aligned captures", e)
	}
}
