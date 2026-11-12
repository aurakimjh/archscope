package metrichistory_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func denseSet(t *testing.T, count, seriesCount int) m.Set {
	t.Helper()
	set := m.Set{Version: m.Version, Normalizer: m.Normalizer, Sources: []m.Source{{ID: "s", SHA256: m.Hash("synthetic"), Name: "synthetic", Format: "snapshot-sequence-json", Producer: "synthetic"}}, Warnings: []string{}}
	for i := 0; i < seriesCount; i++ {
		series := m.Series{Name: fmt.Sprintf("metric-%d", i), Unit: "count", Kind: "sum", Temporality: "cumulative", Monotonic: true, Origin: "synthetic", Resource: json.RawMessage(`{}`), Scope: m.Scope{Attributes: json.RawMessage(`{}`), Dropped: "0"}, Attributes: json.RawMessage(`{}`)}
		series.ID = m.SeriesID(series)
		set.Series = append(set.Series, series)
	}
	for i := 0; i < count; i++ {
		point := m.Point{SeriesID: set.Series[i%seriesCount].ID, Start: m.Pointer("0"), End: m.Pointer(strconv.FormatInt(int64(i/seriesCount+1)*1000000000, 10)), Value: m.Pointer(strconv.Itoa(i / seriesCount)), Refs: []m.Ref{{SourceID: "s", Position: strconv.Itoa(i)}}}
		point.ID = m.PointID(point)
		set.Points = append(set.Points, point)
	}
	return set
}
func TestR1301CompactSelectedStorageAndLegacyReader(t *testing.T) {
	ctx := context.Background()
	set := denseSet(t, 40000, 4)
	ids := []string{}
	for _, series := range set.Series {
		ids = append(ids, series.ID)
	}
	result, e := a.Build(ctx, set, m.Selection{SeriesIDs: ids}, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	all, _ := json.Marshal(result)
	selected, e := a.Build(ctx, set, m.Selection{SeriesIDs: ids[:1]}, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(selected)
	core, e := a.ReadCore(selected)
	if e != nil {
		t.Fatal(e)
	}
	if result.Metadata.SchemaVersion != "0.2.0" || len(core.Model.Points) != 10000 || len(core.Model.Series) != 1 || len(body)*3 > len(all) {
		t.Fatalf("selection did not reduce storage: all=%d selected=%d points=%d", len(all), len(body), len(core.Model.Points))
	}
	if bytes.Contains(body, []byte(`"observed_histogram"`)) || bytes.Contains(body, []byte(`"rate_meaning"`)) {
		t.Fatal("rich query rows still duplicated in saved tables")
	}
	legacyBytes, e := os.ReadFile("testdata/payload-0.1.0.saved.json")
	if e != nil {
		t.Fatal(e)
	}
	legacy, e := resultcontract.Decode(legacyBytes)
	if e != nil {
		t.Fatal(e)
	}
	if legacy.Metadata.SchemaVersion != "0.1.0" {
		t.Fatal("legacy fixture version")
	}
	if e = a.Validate(legacy); e != nil {
		t.Fatal(e)
	}
	legacyCore, e := a.ReadCore(legacy)
	if e != nil || len(legacyCore.Model.Points) != 8 {
		t.Fatal(e)
	}
	if e = a.Validate(selected); e != nil {
		t.Fatal(e)
	}
	t.Logf("40000 points compact bytes=%d; one of four series bytes=%d", len(all), len(body))
}
func TestR1301AdmittedLargeModelFitsResultCeiling(t *testing.T) {
	set := denseSet(t, 100000, 10)
	modelBytes, _ := json.Marshal(set)
	if len(modelBytes) > m.MaxBytes {
		t.Fatalf("fixture exceeds independent model ceiling: %d", len(modelBytes))
	}
	ids := []string{}
	for _, series := range set.Series {
		ids = append(ids, series.ID)
	}
	result, e := a.Build(context.Background(), set, m.Selection{SeriesIDs: ids}, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(result)
	if len(body) >= 64<<20 {
		t.Fatal("admitted compact model cannot be saved", len(body))
	}
	if dir := os.Getenv("ARCHSCOPE_METRIC_STORAGE_EVIDENCE_DIR"); dir != "" {
		if e := os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		os.WriteFile(filepath.Join(dir, "model.json"), modelBytes, 0600)
		os.WriteFile(filepath.Join(dir, "result.json"), body, 0600)
		selection, _ := json.Marshal(m.Selection{SeriesIDs: ids})
		os.WriteFile(filepath.Join(dir, "selection.json"), selection, 0600)
	}
	t.Logf("100000 timed points model bytes=%d result bytes=%d", len(modelBytes), len(body))
}
