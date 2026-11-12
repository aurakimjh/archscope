package analysis_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
)

func TestT638GenericImportQueryMaterializeCaseAndExports(t *testing.T) {
	s, _, _ := setup(t, nil)
	ctx := context.Background()
	body, e := catalog.Contracts.ReadFile("contracts/metric-history-set.json")
	if e != nil {
		t.Fatal(e)
	}
	model, e := m.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	selection := m.Selection{SeriesIDs: []string{model.Series[0].ID}}
	options, _ := json.Marshal(selection)
	path := filepath.Join(t.TempDir(), "model.json")
	os.WriteFile(path, body, 0600)
	job := wait(t, s, start(t, s, analysis.Request{Path: path, Importer: "metrics.history-json", Options: options}))
	if job.State != "completed" {
		t.Fatal(job)
	}
	inv, e := s.MetricInventory(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	page, e := s.QueryMetricHistory(ctx, job.ResultHandle, inv.ResultSHA256, m.Query{Selection: selection, Limit: 1})
	if e != nil || page.Total != 2 || page.Next == "" {
		t.Fatal(page, e)
	}
	if _, e = s.QueryMetricHistory(ctx, job.ResultHandle, strings.Repeat("a", 64), m.Query{Selection: selection, Limit: 1}); e == nil {
		t.Fatal("stale digest accepted")
	}
	selection.From = page.Rows[0].End
	view, lease, e := s.MaterializeMetricSelection(ctx, job.ResultHandle, inv.ResultSHA256, selection)
	if e != nil {
		t.Fatal(e)
	}
	defer s.ReleaseResultLease(lease.ID)
	if view.Handle == job.ResultHandle {
		t.Fatal("original replaced")
	}
	raw, _, e := s.Read(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		out, e := s.Export(job.ResultHandle, format)
		if e != nil || len(out) == 0 {
			t.Fatal(format, e)
		}
		if format == "json" && !bytes.Equal(out, raw) {
			t.Fatal("snapshot changed")
		}
	}
	caps, e := s.DescribeResult(ctx, job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, provider := range caps.Providers {
		if provider.ID == "metrics:timeline" {
			found = provider.Availability == "available"
		}
	}
	if !found {
		t.Fatal("timeline unavailable", caps)
	}
	store, e := cases.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	c, e := store.Create("metrics")
	if e != nil {
		t.Fatal(e)
	}
	c, e = store.SaveResult(cases.SaveResult{CaseID: c.ID, ExpectedRevision: c.Revision}, raw)
	if e != nil {
		t.Fatal(e)
	}
	restored, e := store.ExportSnapshot(c.ID, c.Results[0].ID)
	if e != nil || !bytes.Equal(restored, raw) {
		t.Fatal("case bytes changed", e)
	}
	loaded, e := s.LoadSnapshot(ctx, restored)
	if e != nil || !loaded.Compatibility.Allowed {
		t.Fatal(loaded, e)
	}
	invalidPath := filepath.Join(t.TempDir(), "invalid.json")
	os.WriteFile(invalidPath, []byte(`{"schema_version":"9"}`), 0600)
	bad := wait(t, s, start(t, s, analysis.Request{Path: invalidPath, Importer: "metrics.history-json", Options: options}))
	if bad.State != "failed" || bad.Problem == nil || bad.Problem.Code != "IMPORT_FORMAT_MISMATCH" {
		t.Fatal("invalid model reported as I/O failure", bad)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.QueryMetricHistory(cancelled, job.ResultHandle, inv.ResultSHA256, m.Query{Selection: selection, Limit: 1}); e == nil {
		t.Fatal("cancel ignored")
	}
}

func TestT638MetricTimelineScaleAndUntimedAvailability(t *testing.T) {
	for _, n := range []int{8000, 16000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, _, _ := setup(t, nil)
			ctx := context.Background()
			var source bytes.Buffer
			source.WriteString(`{"schema_version":"1.0.0","snapshots":[`)
			for i := 0; i < n; i++ {
				if i > 0 {
					source.WriteByte(',')
				}
				fmt.Fprintf(&source, `{"time_unix_nano":"%d","metrics":[{"name":"synthetic","unit":"count","kind":"gauge","value":"9007199254740993"}]}`, int64(i+1)*1000000000)
			}
			source.WriteString(`,{"metrics":[{"name":"synthetic","unit":"count","kind":"gauge","value":"1"}]}]}`)
			set, e := p.ParseBytes(ctx, source.Bytes(), "scale.json", p.Options{})
			if e != nil {
				t.Fatal(e)
			}
			result, e := a.Build(ctx, set, m.Selection{SeriesIDs: []string{set.Series[0].ID}}, "scale")
			if e != nil {
				t.Fatal(e)
			}
			body, _ := json.Marshal(result)
			view, e := s.LoadSnapshot(ctx, body)
			if e != nil {
				t.Fatal(e)
			}
			started := time.Now()
			page, e := s.QueryResult(ctx, view.Handle, "metrics:timeline", n-1, 1)
			if e != nil || page.Total != n || len(page.Rows) != 1 {
				t.Fatal(page, e)
			}
			warm := time.Now()
			for i := 0; i < 3; i++ {
				page, e = s.QueryResult(ctx, view.Handle, "metrics:timeline", 0, 25)
				if e != nil || page.Total != n {
					t.Fatal(e)
				}
			}
			t.Logf("points=%d untimed=1 bytes=%d cold=%s warm3=%s", n, len(body), warm.Sub(started), time.Since(warm))
			untimed, e := s.QueryResult(ctx, view.Handle, "metrics:points", 0, 1)
			if e != nil || untimed.Total != n+1 {
				t.Fatal("untimed point lost", e)
			}
		})
	}
}
