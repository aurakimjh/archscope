package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func metricCacheService(t *testing.T) *Service {
	t.Helper()
	catalog, e := ingestion.NewCatalog(nil, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	registry, e := resultcontract.New(a.Descriptor(), a.LegacyDescriptor())
	if e != nil {
		t.Fatal(e)
	}
	service, e := New(t.TempDir(), catalog, registry)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(service.Close)
	return service
}
func metricCacheBody(t *testing.T, name string, legacy bool) ([]byte, m.Set) {
	t.Helper()
	raw, e := os.ReadFile("../catalog/contracts/metric-history-set.json")
	if e != nil {
		t.Fatal(e)
	}
	model, e := m.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	model.Sources[0].Name = name
	selection := m.Selection{SeriesIDs: []string{model.Series[0].ID}}
	var resultBody []byte
	if legacy {
		result, e := a.BuildLegacy(context.Background(), model, selection, name)
		if e != nil {
			t.Fatal(e)
		}
		resultBody, e = json.Marshal(result)
		if e != nil {
			t.Fatal(e)
		}
	} else {
		result, e := a.Build(context.Background(), model, selection, name)
		if e != nil {
			t.Fatal(e)
		}
		resultBody, e = json.Marshal(result)
		if e != nil {
			t.Fatal(e)
		}
	}
	return resultBody, model
}
func TestR1302MetricWarmIsolationEpochsAndCachePolicy(t *testing.T) {
	s := metricCacheService(t)
	ctx := context.Background()
	raw, set := metricCacheBody(t, "cache-fixture", false)
	view, owner, e := s.LoadOwnedSnapshot(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	inv, e := s.MetricInventory(ctx, view.Handle)
	if e != nil {
		t.Fatal(e)
	}
	if len(s.metricCache) != 1 {
		t.Fatal("cache absent")
	}
	first := s.metricCache[0].value
	q := m.Query{Selection: inv.Selection, Limit: 1}
	page, e := s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q)
	if e != nil || page.Next == "" {
		t.Fatal(page, e)
	}
	expected, e := m.QuerySet(ctx, set, inv.ResultSHA256, q)
	if e != nil || !reflect.DeepEqual(expected, page) {
		t.Fatal("stateless mismatch", e)
	}
	frozen, _ := json.Marshal(expected)
	expected = m.Page{}
	json.Unmarshal(frozen, &expected)
	inv.Series[0].Name = "mutated"
	inv.Selection.SeriesIDs[0] = "mutated"
	inv.Sources[0].Query = json.RawMessage(`{"mutated":true}`)
	page.Selection.SeriesIDs[0] = "mutated"
	page.Rows[0].Refs[0].Position = "mutated"
	if page.Rows[0].Value != nil {
		*page.Rows[0].Value = "999"
	}
	q.Selection = expected.Selection
	again, e := s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q)
	if e != nil || !reflect.DeepEqual(expected, again) {
		t.Fatal("caller mutated cached state", e)
	}
	if s.metricCache[0].value != first {
		t.Fatal("warm page rebuilt index")
	}
	stale := q
	stale.Cursor = expected.Next
	stale.Limit = 25
	if _, e = s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, stale); e == nil || !strings.Contains(e.Error(), "METRIC_CURSOR_STALE") {
		t.Fatal("stale cursor accepted", e)
	}
	if _, e = s.QueryMetricHistory(ctx, view.Handle, strings.Repeat("f", 64), q); e == nil {
		t.Fatal("wrong digest accepted")
	}
	// Expiration is lazy and fixed from admission, not extended on each hit.
	s.metricCache[0].expires = time.Now().Add(-time.Second)
	if _, e = s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q); e != nil {
		t.Fatal(e)
	}
	if s.metricCache[0].value == first {
		t.Fatal("expired entry reused")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q)
			if e != nil || !reflect.DeepEqual(expected, r) {
				t.Error(e)
			}
			r.Rows[0].Refs[0].Position = "thread-private"
		}()
	}
	wg.Wait()
	// Warm requests still hash bytes even with identical length and restored mtime.
	path := filepath.Join(s.root, "results", view.Handle+".json")
	stat, e := os.Stat(path)
	if e != nil {
		t.Fatal(e)
	}
	changed := bytes.Replace(raw, []byte("cache-fixture"), []byte("cache-tamper!"), 1)
	if len(changed) != len(raw) || bytes.Equal(changed, raw) {
		t.Fatal("bad tamper fixture")
	}
	if e = os.WriteFile(path, changed, 0600); e != nil {
		t.Fatal(e)
	}
	os.Chtimes(path, stat.ModTime(), stat.ModTime())
	if _, e = s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q); e == nil {
		t.Fatal("warm cache bypassed bytes")
	}
	os.WriteFile(path, raw, 0600)
	if e = s.ReleaseResultLease(owner.ID); e != nil {
		t.Fatal(e)
	}
	// Ordinary revisions remain retained after lease release until explicit
	// cleanup; the warm cache must not resurrect bytes after that cleanup.
	if _, e = s.CleanupEvidenceCache(); e != nil {
		t.Fatal(e)
	}
	if _, e = s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q); e == nil {
		t.Fatal("removed revision resurrected by cache")
	}
}
func TestR1302MetricCacheAdmissionCancellationAndLegacy(t *testing.T) {
	s := metricCacheService(t)
	ctx := context.Background()
	var last ResultView
	for i := 0; i < 3; i++ {
		raw, _ := metricCacheBody(t, fmt.Sprintf("entry-%d", i), false)
		view, _, e := s.LoadOwnedSnapshot(ctx, raw)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.MetricInventory(ctx, view.Handle); e != nil {
			t.Fatal(e)
		}
		last = view
	}
	if len(s.metricCache) != 2 || s.metricCharge > metricCacheBytes {
		t.Fatal("count/charge unbounded", s.metricCharge)
	}
	// Exercise byte eviction independently of count without allocating a huge model.
	s.metricCache = s.metricCache[:1]
	s.metricCache[0].charge = metricCacheBytes
	s.metricCharge = metricCacheBytes
	if _, e := s.MetricInventory(ctx, last.Handle); e != nil {
		t.Fatal(e)
	}
	if len(s.metricCache) != 1 || s.metricCharge >= metricCacheBytes {
		t.Fatal("byte eviction failed")
	}
	s.metricGate <- struct{}{}
	blocked, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { _, e := s.MetricInventory(blocked, last.Handle); done <- e }()
	cancel()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancelled admission succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel blocked by cache gate")
	}
	<-s.metricGate
	legacy, _ := metricCacheBody(t, "legacy-fixture", true)
	view, _, e := s.LoadOwnedSnapshot(ctx, legacy)
	if e != nil {
		t.Fatal(e)
	}
	inv, e := s.MetricInventory(ctx, view.Handle)
	if e != nil {
		t.Fatal(e)
	}
	q := m.Query{Selection: inv.Selection, Limit: 1}
	if _, e = s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q); e != nil {
		t.Fatal(e)
	}
	saved, _, e := s.Read(view.Handle)
	if e != nil || !bytes.Equal(saved, legacy) {
		t.Fatal("legacy saved bytes changed", e)
	}
	if s.metricCache[len(s.metricCache)-1].version != a.LegacyPayloadVersion {
		t.Fatal("legacy version lost")
	}
	s.Close()
	if len(s.metricCache) != 0 || s.metricCharge != 0 {
		t.Fatal("cache retained after close")
	}
}
func TestR1302Metric100kWarmPages(t *testing.T) {
	if testing.Short() {
		t.Skip("100k measured cache regression")
	}
	ctx := context.Background()
	s := metricCacheService(t)
	var source bytes.Buffer
	source.WriteString(`{"schema_version":"1.0.0","snapshots":[`)
	for i := 0; i < 100000; i++ {
		if i > 0 {
			source.WriteByte(',')
		}
		fmt.Fprintf(&source, `{"time_unix_nano":"%d","metrics":[{"name":"counter","unit":"count","kind":"sum","temporality":"cumulative","monotonic":true,"start_time_unix_nano":"0","value":"%d"}]}`, int64(i+1)*1000000000, i)
	}
	source.WriteString(`]}`)
	set, e := p.ParseBytes(ctx, source.Bytes(), "100k-synthetic.json", p.Options{})
	if e != nil {
		t.Fatal(e)
	}
	selection := m.Selection{SeriesIDs: []string{set.Series[0].ID}}
	result, e := a.Build(ctx, set, selection, "100k synthetic cache")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(result)
	view, _, e := s.LoadOwnedSnapshot(ctx, raw)
	if e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	inv, e := s.MetricInventory(ctx, view.Handle)
	if e != nil {
		t.Fatal(e)
	}
	cold := time.Since(start)
	if len(s.metricCache) != 1 {
		t.Fatal("100k not admitted", s.metricCharge)
	}
	index := s.metricCache[0].value.index
	q := m.Query{Selection: selection, Limit: 25}
	warm := time.Now()
	for i := 0; i < 3; i++ {
		page, e := s.QueryMetricHistory(ctx, view.Handle, inv.ResultSHA256, q)
		if e != nil || page.Total != 100000 || len(page.Rows) != 25 {
			t.Fatal(page.Total, e)
		}
		if s.metricCache[0].value.index != index {
			t.Fatal("warm index rebuilt")
		}
		q.Cursor = page.Next
	}
	elapsed := time.Since(warm)
	t.Logf("points=100000 result_bytes=%d cache_charge=%d cold=%s warm3=%s; every request rehashes bytes; structural charge is not RSS; no latency threshold", len(raw), s.metricCharge, cold, elapsed)
}
