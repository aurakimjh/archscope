package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR17WarmLLMCacheIntegrityAndReleasedOwner(t *testing.T) {
	ctx := context.Background()
	c, e := ingestion.NewCatalog(nil, nil, nil)
	registry, e := resultcontract.New(a.LLMDescriptor())
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(t.TempDir(), c, registry)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	raw, _ := os.ReadFile("../catalog/contracts/llm-trace-set.json")
	set, e := l.Decode(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	base := set.Spans[0]
	set.Spans = nil
	set.Events = nil
	set.Links = nil
	for i := 1; i <= 20000; i++ {
		v := base
		v.Position = fmt.Sprint(i)
		v.SpanID = fmt.Sprintf("%016x", i)
		v.TraceID = fmt.Sprintf("%032x", i)
		v.Parent = ""
		v.LogicalRequest = ""
		v.Attempt = ""
		v.PreviousAttempt = ""
		v.AttemptKind = ""
		v.ID = l.SpanID(set.Source.SHA256, v)
		set.Spans = append(set.Spans, v)
	}
	result, e := a.BuildLLM(ctx, set, "20k synthetic")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(result)
	view, owner, e := s.LoadOwnedSnapshot(ctx, body)
	if e != nil {
		t.Fatal(e)
	}
	inventory, e := s.LLMInventory(ctx, view.Handle)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.semanticCache) != 1 {
		t.Fatal("semantic cache absent")
	}
	cached := s.semanticCache[0].value.(llmSemantic)
	before := &cached.set.Spans[0]
	begin := time.Now()
	page, e := s.QueryLLMTrace(ctx, view.Handle, inventory.ResultSHA, a.LLMQuery{Kind: "spans", Limit: 25})
	if e != nil || len(page.Rows) != 25 || page.Total != 20000 {
		t.Fatal(page.Total, e)
	}
	elapsed := time.Since(begin)
	cached = s.semanticCache[0].value.(llmSemantic)
	if &cached.set.Spans[0] != before {
		t.Fatal("warm query rebuilt entire graph")
	}
	t.Logf("20k span warm query: %s, page=25; no latency threshold or RSS claim", elapsed)
	path := filepath.Join(s.root, "results", view.Handle+".json")
	info, _ := os.Stat(path)
	data, _ := os.ReadFile(path)
	changed := strings.Replace(string(data), "20k synthetic", "bad synthetic", 1)
	if changed == string(data) {
		t.Fatal("tamper marker missing")
	}
	os.WriteFile(path, []byte(changed), 0600)
	os.Chtimes(path, info.ModTime(), info.ModTime())
	if _, e = s.QueryLLMTrace(ctx, view.Handle, inventory.ResultSHA, a.LLMQuery{Kind: "spans", Limit: 25}); e == nil {
		t.Fatal("warm cache bypassed integrity")
	}
	if e = s.ReleaseResultLease(owner.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.QueryLLMTrace(ctx, view.Handle, inventory.ResultSHA, a.LLMQuery{Kind: "spans", Limit: 25}); e == nil {
		t.Fatal("released owner reused cache")
	}
}
