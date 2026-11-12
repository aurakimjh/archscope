package cases

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestR1203PersistedIndexMasksKeysOnBuildAndCacheHit(t *testing.T) {
	root := t.TempDir()
	store, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	m, e := store.Create("mask")
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "synthetic.log")
	raw := []byte("password=synthetic-secret\nAuthorization: Bearer synthetic-secret\n")
	if e = os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	m, e = store.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "source", Path: p})
	if e != nil {
		t.Fatal(e)
	}
	q := SearchQuery{CaseID: m.ID, Revision: m.Revision, Limit: 25}
	for i := 0; i < 2; i++ {
		page, e := store.Search(context.Background(), q)
		if e != nil {
			t.Fatal(e)
		}
		for _, row := range page.Rows {
			if strings.Contains(row.Text, "synthetic-secret") || strings.Contains(row.Text, "len=6> len=") {
				t.Fatal("unsafe snippet", row)
			}
		}
	}
	body, e := os.ReadFile(filepath.Join(root, "indexes", "search-"+m.ID+".json"))
	if e != nil || strings.Contains(string(body), "synthetic-secret") {
		t.Fatal("unsafe persistent index", e)
	}
	unchanged, e := os.ReadFile(p)
	if e != nil || string(unchanged) != string(raw) {
		t.Fatal("original changed", e)
	}
}
func TestR1204ResultLimitActionable(t *testing.T) {
	e := ResultSizeError(94 << 20).Error()
	for _, want := range []string{"98566144", "67108864", "sample count unavailable", "fewer profiles", "cannot be saved or pinned"} {
		if !strings.Contains(e, want) {
			t.Fatal(e)
		}
	}
}
