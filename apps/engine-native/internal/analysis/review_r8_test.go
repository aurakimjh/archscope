package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func r8Service(t testing.TB) *Service {
	t.Helper()
	c, e := ingestion.NewCatalog(nil, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	registry, e := resultcontract.New(resultcontract.Descriptor{Type: "review.r8", Payload: resultcontract.Range{Min: "0.1.0", Before: "0.2.0"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Extensions: map[string]resultcontract.Range{"measurement": {Min: "1.0.0", Before: "2.0.0"}}, Scalars: map[string]resultcontract.ValueSchema{"/summary/count": {Kinds: []string{"record_count"}, Units: []string{"count"}, Aggregations: []string{"count"}, Encodings: []string{"json_number"}}}})
	if e != nil {
		t.Fatal(e)
	}
	s, e := New(t.TempDir(), c, registry)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s
}
func r8Body(t testing.TB, n, width int) []byte {
	t.Helper()
	r := models.New("review.r8", "synthetic")
	r.Summary["count"] = n
	r.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0"}
	r.Metadata.Extra["measurement"] = map[string]resultcontract.Measurement{"/summary/count": {Kind: "record_count", Unit: "count", Scope: "records", Aggregation: "count", Encoding: "json_number", Availability: "observed"}}
	rows := make([]map[string]any, n)
	for i := range rows {
		row := map[string]any{"number": json.Number(fmt.Sprint(i))}
		for j := 0; j < width; j++ {
			row[fmt.Sprintf("c%d", j)] = fmt.Sprintf("v%08d%d", i, j)
		}
		rows[i] = row
	}
	r.Tables["rows"] = rows
	body, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	return body
}
func r8Load(t testing.TB, s *Service, body []byte) string {
	t.Helper()
	view, e := s.LoadSnapshot(context.Background(), body)
	if e != nil {
		t.Fatal(e)
	}
	return view.Handle
}
func TestR802LargeTableCacheReuseCopiesAndLeaseLifetime(t *testing.T) {
	s := r8Service(t)
	body := r8Body(t, 100000, 2)
	handle := r8Load(t, s, body)
	one, _ := s.AcquireResult(handle)
	two, _ := s.AcquireResult(handle)
	started := time.Now()
	caps, e := s.DescribeResult(context.Background(), handle)
	if e != nil {
		t.Fatal(e)
	}
	first, e := s.QueryResult(context.Background(), handle, "generic:/tables/rows", 0, 25)
	if e != nil {
		t.Fatal(e)
	}
	cold := time.Since(started)
	entry := s.projections[handle]
	if entry == nil {
		t.Fatal("representative large table not admitted")
	}
	first.Columns[0] = "mutated"
	first.Rows[0][0].Text = "mutated"
	caps.Providers[0].ID = "mutated"
	started = time.Now()
	for i := 0; i < 20; i++ {
		out, e := s.QueryResult(context.Background(), handle, "generic:/tables/rows", 99975, 25)
		if e != nil || out.Total != 100000 || out.Next != -1 || out.Columns[0] == "mutated" || out.Rows[0][0].Text == "mutated" {
			t.Fatal(out, e)
		}
		if s.projections[handle] != entry {
			t.Fatal("cache rebuilt on warm query")
		}
		if _, e = s.DescribeResult(context.Background(), handle); e != nil {
			t.Fatal(e)
		}
	}
	warm := time.Since(started)
	t.Logf("rows=100000 bytes=%d charge=%d cold describe+page=%s warm 20 describe+last-page=%s", len(body), entry.projection.Weight(), cold, warm)
	// Parallel pages all share the same immutable validation and column index.
	var wait sync.WaitGroup
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if _, e := s.QueryResult(context.Background(), handle, "generic:/tables/rows", 50000, 25); e != nil {
				t.Error(e)
			}
		}()
	}
	wait.Wait()
	if e = s.ReleaseResultLease(one.ID); e != nil || s.projections[handle] != entry {
		t.Fatal("other owner lost cache", e)
	}
	if e = s.ReleaseResultLease(two.ID); e != nil || s.projections[handle] != nil {
		t.Fatal("final owner cache retained", e)
	}
	if _, e = s.DescribeResult(context.Background(), handle); e != nil {
		t.Fatal(e)
	}
	if e = s.Release(handle); e != nil || s.projections[handle] != nil || s.projectionCharge != 0 {
		t.Fatal("release retained projection", e)
	}
	if _, e = s.QueryResult(context.Background(), handle, "generic:summary", 0, 25); e == nil {
		t.Fatal("released handle revived")
	}
}
func TestR802TamperRestoredMtimeRemovalAndLRUBounds(t *testing.T) {
	s := r8Service(t)
	handle := r8Load(t, s, r8Body(t, 1, 1))
	if _, e := s.DescribeResult(context.Background(), handle); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(s.root, "results", handle+".json")
	info, _ := os.Stat(path)
	body, _ := os.ReadFile(path)
	body = []byte(strings.Replace(string(body), "v000000000", "tampered!!", 1))
	if e := os.WriteFile(path, body, 0600); e != nil {
		t.Fatal(e)
	}
	os.Chtimes(path, info.ModTime(), info.ModTime())
	if _, e := s.QueryResult(context.Background(), handle, "generic:summary", 0, 25); e == nil || !strings.Contains(e.Error(), "IMPORT_RESULT_CORRUPT") {
		t.Fatal("tamper hidden by cache", e)
	}
	if s.projections[handle] != nil {
		t.Fatal("corrupt entry retained")
	}
	for i := 2; i < 8; i++ {
		h := r8Load(t, s, r8Body(t, i, 1))
		if _, e := s.DescribeResult(context.Background(), h); e != nil {
			t.Fatal(e)
		}
		if len(s.projections) > maxProjectionEntries || s.projectionCharge > maxProjectionCharge {
			t.Fatal("cache budget exceeded")
		}
	}
	for h := range s.projections {
		if e := os.Remove(filepath.Join(s.root, "results", h+".json")); e != nil {
			t.Fatal(e)
		}
		if _, e := s.DescribeResult(context.Background(), h); e == nil {
			t.Fatal("deleted file served from cache")
		}
		if s.projections[h] != nil {
			t.Fatal("deleted entry retained")
		}
		break
	}
	if _, e := s.CleanupEvidenceCache(); e != nil {
		t.Fatal(e)
	}
	if len(s.projections) != 0 || s.projectionCharge != 0 {
		t.Fatal("cleanup retained projections")
	}
}
func TestR804ComparisonTargetRefusalAndWarmOriginalDigests(t *testing.T) {
	s := r8Service(t)
	baseBody := r8Body(t, 2, 1)
	base := r8Load(t, s, baseBody)
	unknown := models.New("unknown", "synthetic")
	unknownBody, _ := json.Marshal(unknown)
	target := r8Load(t, s, unknownBody)
	out, e := s.CompareResult(context.Background(), base, target, "compare:/summary/count")
	if e != nil || out.Allowed || out.Code != resultcontract.ComparisonResultUnsupported || out.Reason != "unknown_result_type" || out.Delta != nil {
		t.Fatal(out, e)
	}
	original := s.projections[base]
	out, e = s.CompareResult(context.Background(), base, base, "compare:/summary/count")
	if e != nil || !out.Allowed || out.Delta == nil || out.Delta.Value == nil || *out.Delta.Value != "0" || out.Delta.Comparison.Baseline.Handle != base || out.Delta.Comparison.Baseline.ResultSHA256 != original.projection.Digest() {
		t.Fatal(out, e)
	}
	if s.projections[base] != original {
		t.Fatal("compare rebuilt projection")
	}
	exported, e := s.Export(base, "raw-json")
	if e != nil || string(exported) != string(baseBody) {
		t.Fatal("original export changed", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = s.CompareResult(ctx, base, base, "compare:/summary/count"); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	s.Close()
	if len(s.projections) != 0 || s.projectionCharge != 0 {
		t.Fatal("close retained cache")
	}
}
func TestR803SourceAndTableEscapingDoesNotChangeExport(t *testing.T) {
	s := r8Service(t)
	var result models.AnalysisResult
	json.Unmarshal(r8Body(t, 1, 1), &result)
	result.SourceFiles = []string{"source\u202E\x07"}
	result.Tables["rows"].([]any)[0].(map[string]any)["c0"] = "metric\u2066\x01"
	body, _ := json.Marshal(result)
	handle := r8Load(t, s, body)
	caps, e := s.DescribeResult(context.Background(), handle)
	if e != nil || !caps.Sources[0].Escaped {
		t.Fatal(caps, e)
	}
	page, e := s.QueryResult(context.Background(), handle, "generic:/tables/rows", 0, 25)
	if e != nil || !page.Escaped || !page.Rows[0][0].Escaped {
		t.Fatal(page, e)
	}
	exported, e := s.Export(handle, "raw-json")
	if e != nil || string(exported) != string(body) {
		t.Fatal("display escaping changed export", e)
	}
}
func BenchmarkR802WarmDescribeAndPage100k(b *testing.B) {
	s := r8Service(b)
	body := r8Body(b, 100000, 10)
	handle := r8Load(b, s, body)
	if _, e := s.QueryResult(context.Background(), handle, "generic:/tables/rows", 0, 25); e != nil {
		b.Fatal(e)
	}
	if s.projections[handle] == nil {
		b.Fatal("100k wide table not cached")
	}
	b.Logf("bytes=%d projection_charge=%d", len(body), s.projectionCharge)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, e := s.DescribeResult(context.Background(), handle); e != nil {
			b.Fatal(e)
		}
		if _, e := s.QueryResult(context.Background(), handle, "generic:/tables/rows", 99975, 25); e != nil {
			b.Fatal(e)
		}
	}
}
