package profilemodel_test

import (
	"context"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"strings"
	"testing"
)

func TestT635CanonicalQueryDenominatorCursorAndEvidence(t *testing.T) {
	s, sel := fixture(t)
	q := m.Query{Selection: sel, View: "evidence", Limit: 1}
	page, e := m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil || page.Denominator != "0.35" || page.Matched != 3 || len(page.Rows) != 1 || page.Next == "" {
		t.Fatal(page, e)
	}
	q.Cursor = page.Next
	next, e := m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil || next.Rows[0].ID == page.Rows[0].ID || next.Population != page.Population || next.Denominator != page.Denominator {
		t.Fatal(next, e)
	}
	q.Text = "root"
	if _, e = m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q); e == nil {
		t.Fatal("stale query cursor accepted")
	}
	q.Text = ""
	if _, e = m.QuerySet(context.Background(), s, strings.Repeat("b", 64), q); e == nil {
		t.Fatal("stale result cursor accepted")
	}
	q.Cursor = ""
	q.View = "top"
	q.Limit = 20
	top, e := m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil || top.Denominator != "0.35" {
		t.Fatal(top, e)
	}
	q.View = "callers"
	q.FrameID = s.Frames[1].ID
	callers, e := m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil || callers.Denominator != "0.35" {
		t.Fatal(callers, e)
	}
	q.View = "callees"
	q.FrameID = s.Frames[0].ID
	if _, e = m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q); e != nil {
		t.Fatal(e)
	}
	q.View = "evidence"
	q.FrameID = ""
	q.Selection.MetricID = "memory"
	page, e = m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil || page.Denominator != "9007199254740993" {
		t.Fatal(page, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = m.QuerySet(ctx, s, strings.Repeat("a", 64), q); e == nil {
		t.Fatal("ignored cancel")
	}
}
func TestT635ExplicitEntityMappingPreservesOriginalsAndStrictGates(t *testing.T) {
	a := sourcePolicySet(t, `{"pid":1,"rate":100}`)
	b := sourcePolicySet(t, `{"pid":2,"rate":100}`)
	sel := m.Selection{ProfileIDs: []string{"profile-0"}, MetricID: "samples"}
	entity := func(s m.ProfileSet) m.ProfileEntity {
		p := s.Profiles[0]
		return m.ProfileEntity{ID: p.ID, Process: p.Process, Thread: p.Thread, Worker: p.Worker}
	}
	mapping := []m.EntityMapping{{Baseline: entity(a), Target: entity(b), Reason: "explicit synthetic worker mapping"}}
	if _, e := m.Compare(context.Background(), a, sel, b, sel); e == nil {
		t.Fatal("implicit PID mapping")
	}
	d, e := m.CompareMapped(context.Background(), a, sel, b, sel, mapping)
	if e != nil || d.IdentityBasis != "declared" || len(d.IncidentalDifferences) != 1 || d.Target.CapturePolicy == nil || !strings.Contains(string(d.Target.CapturePolicy.Original), `"pid":2`) {
		t.Fatal(d, e)
	}
	if b.Profiles[0].Process != "declared-pid:2" {
		t.Fatal("mutated original target")
	}
	mapping[0].Target.Process = "stale"
	if _, e = m.CompareMapped(context.Background(), a, sel, b, sel, mapping); e == nil {
		t.Fatal("stale entity tuple accepted")
	}
	mapping[0].Target = entity(b)
	b = sourcePolicySet(t, `{"pid":2,"rate":50}`)
	if _, e = m.CompareMapped(context.Background(), a, sel, b, sel, mapping); e == nil || !strings.Contains(e.Error(), "capture_options_mismatch") {
		t.Fatal("semantic capture bypass", e)
	}
	a.ProducerVersion = "future"
	b.ProducerVersion = "future"
	if _, e = m.CompareMapped(context.Background(), a, sel, b, sel, mapping); e == nil {
		t.Fatal("unknown policy relaxed")
	}
}

func TestT635EvidenceWindowScaleAndUnavailableClocks(t *testing.T) {
	s, sel := fixture(t)
	for i := range s.Profiles {
		s.Profiles[i].Clock = "relative"
		s.Profiles[i].TimeUnit = "seconds"
		s.Profiles[i].TimeScale = "0.1"
		for j := range s.Profiles[i].Samples {
			at := "10"
			s.Profiles[i].Samples[j].Timestamp = &at
		}
	}
	from, to := "1", "1.1"
	q := m.Query{Selection: sel, View: "evidence", Limit: 20, TimeFrom: &from, TimeTo: &to}
	page, e := m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil || page.Matched != 3 || page.Denominator != "0.35" {
		t.Fatal(page, e)
	}
	to = "1"
	page, e = m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil || page.Matched != 0 || page.Denominator != "0.35" {
		t.Fatal(page, e)
	}
	s.Profiles[1].TimeScale = "1"
	if _, e = m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q); e == nil {
		t.Fatal("mixed clocks accepted")
	}
}

func TestT635PagedFlamegraphDoesNotRenormalizeHiddenNodes(t *testing.T) {
	set, selection := fixture(t)
	q := m.Query{Selection: selection, View: "tree", Limit: 1}
	page, e := m.QuerySet(context.Background(), set, strings.Repeat("a", 64), q)
	if e != nil || len(page.Rows) != 1 || page.Next == "" {
		t.Fatal(page, e)
	}
	row := page.Rows[0]
	if row.LayoutWidth == nil || *row.LayoutWidth <= 0 || *row.LayoutWidth > 1 || len(row.Path) == 0 {
		t.Fatal(row)
	}
	q.Limit = 100
	all, e := m.QuerySet(context.Background(), set, strings.Repeat("a", 64), q)
	if e != nil {
		t.Fatal(e)
	}
	if *all.Rows[0].LayoutWidth != *row.LayoutWidth || all.Denominator != page.Denominator {
		t.Fatal("display budget changed canonical area")
	}
	for _, n := range all.Rows {
		if n.LayoutX != nil && (*n.LayoutX < 0 || *n.LayoutWidth < 0 || *n.LayoutX+*n.LayoutWidth > 1.000000001) {
			t.Fatal(n)
		}
	}
}

func TestR1202SelfSortAndCursorBinding(t *testing.T) {
	s, sel := fixture(t)
	q := m.Query{Selection: sel, View: "top", Limit: 1}
	legacy, e := m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil {
		t.Fatal(e)
	}
	q.Sort = "self"
	self, e := m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q)
	if e != nil {
		t.Fatal(e)
	}
	if legacy.Rows[0].ID == self.Rows[0].ID || legacy.QuerySHA256 == self.QuerySHA256 || self.Rows[0].Self == "0" {
		t.Fatal("self sort ineffective", legacy, self)
	}
	q.Cursor = legacy.Next
	if _, e = m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q); e == nil {
		t.Fatal("sort change reused cursor")
	}
	q.Cursor = ""
	q.Sort = "typo"
	if _, e = m.QuerySet(context.Background(), s, strings.Repeat("a", 64), q); e == nil {
		t.Fatal("unknown sort accepted")
	}
}
