package profilemodel_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
	pp "github.com/google/pprof/profile"
)

func fixture(t *testing.T) (m.ProfileSet, m.Selection) {
	t.Helper()
	body, e := catalog.Contracts.ReadFile("contracts/profile-set.json")
	if e != nil {
		t.Fatal(e)
	}
	set, e := m.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	return set, m.Selection{ProfileIDs: []string{"worker-a", "worker-b"}, MetricID: "duration"}
}
func copySet(t *testing.T, set m.ProfileSet) m.ProfileSet {
	t.Helper()
	body, e := json.Marshal(set)
	if e != nil {
		t.Fatal(e)
	}
	out, e := m.Decode(body)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func tree(t *testing.T, set m.ProfileSet, selection m.Selection) m.Tree {
	t.Helper()
	out, e := m.BuildTree(context.Background(), set, selection)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func checkDecomposition(t *testing.T, node m.TreeNode) {
	t.Helper()
	inclusive, _ := m.Number(node.Inclusive)
	total, _ := m.Number(node.Self)
	for _, child := range node.Children {
		value, _ := m.Number(child.Inclusive)
		total.Add(total, value)
		checkDecomposition(t, child)
	}
	if inclusive.Cmp(total) != 0 {
		t.Fatal("self + children != inclusive", node.ID)
	}
}
func TestExactMultiProfileMetricsIdentityZeroAndRecursion(t *testing.T) {
	set, selection := fixture(t)
	before, _ := json.Marshal(set)
	out := tree(t, set, selection)
	if out.Root.Inclusive != "0.35" || out.SampleRecords != 3 || out.Availability != "observed" {
		t.Fatal(out)
	}
	checkDecomposition(t, out.Root)
	if set.Frames[1].Display != set.Frames[2].Display || set.Frames[1].ID == set.Frames[2].ID {
		t.Fatal("module identity collapsed into display name")
	}
	display := set.Frames[0]
	display.Display = "renamed presentation"
	if m.FrameID(display) != set.Frames[0].ID {
		t.Fatal("display rename changed source identity")
	}
	selection.MetricID = "samples"
	if out := tree(t, set, selection); out.Root.Inclusive != "5" {
		t.Fatal(out)
	}
	selection.MetricID = "memory"
	if out := tree(t, set, selection); out.Root.Inclusive != "9007199254740993" {
		t.Fatal(out)
	}
	selection.MetricID = "duration"
	selection.ProfileIDs = []string{"worker-b"}
	set.Profiles[1].Samples[0].Weight = "0"
	zero := tree(t, set, selection)
	if zero.Root.Inclusive != "0" || zero.Root.Share != nil || zero.SampleRecords != 2 || zero.Availability != "observed" || len(zero.Root.Children) == 0 {
		t.Fatal("known zero lost", zero)
	}
	selection.ProfileIDs = []string{"worker-a"}
	selection.Path = []string{set.Frames[2].ID}
	selection.Match = "contains"
	empty := tree(t, set, selection)
	if empty.Availability != "unavailable" || empty.Reason != "no_selected_observations" {
		t.Fatal(empty)
	}
	original, _ := m.Decode(before)
	again, _ := json.Marshal(original)
	if string(again) != string(before) {
		t.Fatal("model roundtrip changed exact evidence")
	}
}
func TestLosslessProjectionScaleCollisionRangeAndSignedRefusal(t *testing.T) {
	set, selection := fixture(t)
	if _, e := m.LegacyStackProjection(context.Background(), set, selection, "1", true); e == nil {
		t.Fatal("fraction rounded into integer")
	}
	projection, e := m.LegacyStackProjection(context.Background(), set, selection, "0.01", true)
	if e != nil {
		t.Fatal(e)
	}
	total := 0
	for _, value := range projection.Stacks {
		total += value
	}
	if total != 35 || projection.TickScale != "0.01" || len(projection.AggregatedDimensions) == 0 {
		t.Fatal(projection)
	}
	if _, e = m.LegacyStackProjection(context.Background(), set, selection, "0.01", false); e == nil {
		t.Fatal("display collisions merged")
	}
	selection.MetricID = "memory"
	set.Profiles[0].Samples[2].Weight = "9223372036854775808"
	if _, e = m.LegacyStackProjection(context.Background(), set, selection, "1", true); e == nil {
		t.Fatal("native int overflow accepted")
	}
	set, selection = fixture(t)
	set.Metrics[0].Kind = "signed_difference"
	set.Metrics[0].BaseKind = "wall_time"
	set.Metrics[0].Signed = true
	set.Profiles[0].Samples[0].Weight = "-0.25"
	if out := tree(t, set, selection); out.Root.Inclusive != "-0.15" {
		t.Fatal(out)
	}
	if _, e = m.LegacyStackProjection(context.Background(), set, selection, "0.01", true); e == nil {
		t.Fatal("signed data projected to legacy counts")
	}
}
func TestMetricScaleUsesPhysicalUnitAcrossTreeProjectionAndPprof(t *testing.T) {
	set, selection := fixture(t)
	set.Metrics[0].Scale = "0.001"
	if out := tree(t, set, selection); out.Root.Inclusive != "0.00035" {
		t.Fatal("scale not applied", out)
	}
	projection, e := m.LegacyStackProjection(context.Background(), set, selection, "0.00001", true)
	if e != nil {
		t.Fatal(e)
	}
	sum := 0
	for _, value := range projection.Stacks {
		sum += value
	}
	if sum != 35 {
		t.Fatal(sum)
	}
	body, e := m.ExportPprof(context.Background(), set, selection, "0.00001")
	if e != nil {
		t.Fatal(e)
	}
	profile, e := pp.ParseData(body)
	if e != nil {
		t.Fatal(e)
	}
	total := int64(0)
	for _, sample := range profile.Sample {
		total += sample.Value[0]
	}
	if total != 35 || profile.SampleType[0].Unit != "ticks" || !strings.Contains(profile.Comments[0], `"tick_scale":"0.00001"`) {
		t.Fatal(profile)
	}
	if _, e = m.ExportPprof(context.Background(), set, selection, "1"); e == nil {
		t.Fatal("fractional pprof value truncated")
	}
}
func TestExactCompareConditionsAndImmutableDrilldown(t *testing.T) {
	set, selection := fixture(t)
	before, _ := json.Marshal(set)
	other := copySet(t, set)
	other.Profiles[0].Samples[0].Weight = "0.30"
	difference, e := m.Compare(context.Background(), set, selection, other, selection)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, row := range difference.Rows {
		if row.ID == "root" {
			found = true
			if row.Delta != "0.05" || row.Percent == nil || *row.Percent != "100/7" {
				t.Fatal(row)
			}
		}
	}
	if !found {
		t.Fatal("missing root difference")
	}
	for _, edit := range []func(*m.ProfileSet){func(s *m.ProfileSet) { s.CaptureOptions = json.RawMessage(`{"other":true}`) }, func(s *m.ProfileSet) { s.Metrics[0].Unit = "milliseconds" }, func(s *m.ProfileSet) { s.Metrics[0].Scale = "0.001" }, func(s *m.ProfileSet) { s.ProducerVersion = "other" }, func(s *m.ProfileSet) { s.Format = "different-normalization" }, func(s *m.ProfileSet) { s.Quality.ConversionLoss = []string{"different conversion"} }} {
		other = copySet(t, set)
		edit(&other)
		if _, e = m.Compare(context.Background(), set, selection, other, selection); e == nil {
			t.Fatal("incompatible populations/units compared")
		}
	}
	stages, e := m.Drilldown(context.Background(), set, selection, []m.PathFilter{{Path: []string{set.Frames[1].ID}, Match: "contains"}, {Path: []string{set.Frames[0].ID, set.Frames[0].ID}, Match: "prefix"}})
	if e != nil {
		t.Fatal(e)
	}
	if len(stages) != 3 || stages[0].Tree.Root.Inclusive != "0.35" || stages[1].Tree.Root.Inclusive != "0.25" || stages[2].Tree.Root.Inclusive != "0.25" {
		t.Fatal(stages)
	}
	after, _ := json.Marshal(set)
	if string(before) != string(after) {
		t.Fatal("query mutated canonical evidence")
	}
}
func TestNullZeroAbsentTimeUnknownExtensionsAndInvalidReferences(t *testing.T) {
	set, selection := fixture(t)
	set.Profiles[0].Clock = "relative"
	set.Profiles[0].TimeUnit = "seconds"
	set.Profiles[0].TimeScale = "0.001"
	zero := "0"
	set.Profiles[0].Samples[0].Timestamp = &zero
	set.Profiles[0].Samples[1].TimestampNull = true
	set.Extensions = map[string]json.RawMessage{"future": json.RawMessage(`{"exact":9007199254740993,"unknown":null}`)}
	clone := copySet(t, set)
	if clone.Profiles[0].Samples[0].Timestamp == nil || *clone.Profiles[0].Samples[0].Timestamp != "0" || !clone.Profiles[0].Samples[1].TimestampNull || clone.Profiles[0].Samples[2].Timestamp != nil || clone.Profiles[0].Samples[2].TimestampNull {
		t.Fatal("time presence collapsed")
	}
	encoded, _ := json.Marshal(clone)
	if !strings.Contains(string(encoded), "9007199254740993") {
		t.Fatal("unknown extension rounded")
	}
	if _, e := m.BuildTree(context.Background(), set, m.Selection{MetricID: selection.MetricID}); e == nil {
		t.Fatal("implicit profile selection")
	}
	for _, edit := range []func(*m.ProfileSet){func(s *m.ProfileSet) { s.Frames[0].Module = "changed-with-stale-id" }, func(s *m.ProfileSet) { s.Profiles[0].Samples[0].StackID = "absent" }, func(s *m.ProfileSet) { s.Profiles[0].Samples[0].Weight = "-1" }, func(s *m.ProfileSet) { s.Profiles[0].Samples[0].Weight = "NaN" }, func(s *m.ProfileSet) { s.Metrics[0].Unit = "count" }, func(s *m.ProfileSet) { s.SchemaVersion = "2.0.0" }, func(s *m.ProfileSet) { s.Profiles[0].TimeScale = "" }} {
		bad := copySet(t, set)
		edit(&bad)
		if e := bad.Validate(); e == nil {
			t.Fatal("invalid model accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := m.BuildTree(ctx, set, selection); e == nil {
		t.Fatal("cancelled query completed")
	}
	if _, e := m.DecodeSelection([]byte(`{"metric_id":"duration","profile_ids":["worker-a"],"ignored_required":true}`)); e == nil {
		t.Fatal("unknown selection field ignored")
	}
}
func TestLegacyGoldenCountsUseExactCoreWithoutInventedDuration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "examples", "profiler", "sample-wall.collapsed")
	body, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(body)
	stacks, _, e := profiler.ParseCollapsedFile(path)
	if e != nil {
		t.Fatal(e)
	}
	set, selection, e := m.FromLegacyCollapsed(stacks, hex.EncodeToString(sum[:]), m.Metric{ID: "counts", Kind: "sample_count", Unit: "count", Scale: "1"})
	if e != nil {
		t.Fatal(e)
	}
	if out := tree(t, set, selection); out.Root.Inclusive != "32629" {
		t.Fatal(out)
	}
	projection, e := m.LegacyStackProjection(context.Background(), set, selection, "1", false)
	if e != nil || !reflect.DeepEqual(projection.Stacks, stacks) {
		t.Fatal("legacy golden projection drift", e)
	}
	for _, p := range set.Profiles {
		if p.Clock != "aggregate" {
			t.Fatal("invented chronology")
		}
		for _, sample := range p.Samples {
			if sample.Timestamp != nil || sample.Interval != nil {
				t.Fatal("invented time")
			}
		}
	}
	value, _ := m.Number("0.123456789012345678901234567890")
	if value.Cmp(new(big.Rat).SetFrac(big.NewInt(1), big.NewInt(10))) == 0 {
		t.Fatal("decimal precision lost")
	}
}

func TestR2N01DifferentSourceProfileComparisonKeepsReferences(t *testing.T) {
	set, selection := fixture(t)
	other := copySet(t, set)
	other.SourceHashes = []string{strings.Repeat("f", 64)}
	other.Profiles[0].Samples[0].Weight = "0.30"
	diff, e := m.Compare(context.Background(), set, selection, other, selection)
	if e != nil {
		t.Fatal(e)
	}
	if diff.IdentityBasis != "declared" || diff.ComparisonMode != "baseline_target" || diff.Baseline.Population == diff.Target.Population || diff.Baseline.ModelSHA256 == diff.Target.ModelSHA256 || diff.Baseline.SourceHashes[0] == diff.Target.SourceHashes[0] {
		t.Fatal(diff)
	}
	found := false
	for _, row := range diff.Rows {
		if row.ID == "root" {
			found = true
			if row.Delta != "0.05" {
				t.Fatal(row)
			}
		}
	}
	if !found {
		t.Fatal("missing root")
	}
	other.Profiles[0].Worker = "other-worker"
	if _, e = m.Compare(context.Background(), set, selection, other, selection); e == nil {
		t.Fatal("different entity mapping accepted implicitly")
	}
}
