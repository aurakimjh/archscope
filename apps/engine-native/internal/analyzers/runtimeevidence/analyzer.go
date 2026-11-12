package runtimeevidence

import (
	"context"
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"reflect"
	"sort"
	"strconv"
)

const ResultType = "runtime_evidence"
const PayloadVersion = "0.1.0"

type GCCoverage struct {
	Entity      string  `json:"entity_id"`
	Source      string  `json:"source_id"`
	Clock       string  `json:"clock_id"`
	Population  string  `json:"population"`
	Intervals   string  `json:"pause_intervals"`
	Union       *string `json:"pause_union_ns"`
	Concurrent  *string `json:"concurrent_observed_ns"`
	Unqualified string  `json:"unqualified_intervals"`
	Status      string  `json:"status"`
}

func GCCoverages(ctx context.Context, s r.Set) ([]GCCoverage, error) {
	type span struct{ lo, hi uint64 }
	type group struct {
		row             GCCoverage
		spans           []span
		bad, concurrent int
		concurrentSum   uint64
	}
	sources := map[string]r.Source{}
	for _, v := range s.Sources {
		sources[v.ID] = v
	}
	groups := map[string]*group{}
	for _, v := range s.Observations {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if v.Kind != "gc" {
			continue
		}
		source := sources[v.Ref.Source]
		key := r.Hash([]string{v.Entity, v.Ref.Source, source.Clock, v.Population})
		g := groups[key]
		if g == nil {
			g = &group{row: GCCoverage{Entity: v.Entity, Source: v.Ref.Source, Clock: source.Clock, Population: v.Population, Status: "observed_pause_union; concurrent intervals separate"}}
			groups[key] = g
		}
		a, ok := m.Time(v.From)
		b, yes := m.Time(v.To)
		if !ok || !yes || b < a || source.Clock == "unknown" {
			g.bad++
			continue
		}
		if v.Metric == "concurrent_ns" {
			if ^uint64(0)-g.concurrentSum < b-a {
				return nil, fmt.Errorf("RUNTIME_GC_SUM_OVERFLOW")
			}
			g.concurrentSum += b - a
			g.concurrent++
			continue
		}
		g.spans = append(g.spans, span{a, b})
	}
	keys := []string{}
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []GCCoverage{}
	for _, k := range keys {
		g := groups[k]
		g.row.Intervals = strconv.Itoa(len(g.spans))
		g.row.Unqualified = strconv.Itoa(g.bad)
		if g.row.Clock != "unknown" {
			sort.Slice(g.spans, func(i, j int) bool { return g.spans[i].lo < g.spans[j].lo })
			var sum, lo, hi uint64
			for i, v := range g.spans {
				if i == 0 {
					lo, hi = v.lo, v.hi
				} else if v.lo <= hi {
					hi = max(hi, v.hi)
				} else {
					sum += hi - lo
					lo, hi = v.lo, v.hi
				}
			}
			if len(g.spans) > 0 {
				sum += hi - lo
				g.row.Union = m.Pointer(strconv.FormatUint(sum, 10))
			}
			if g.concurrent > 0 {
				g.row.Concurrent = m.Pointer(strconv.FormatUint(g.concurrentSum, 10))
			}
		}
		if g.bad > 0 {
			g.row.Status = "incomplete_intervals; qualified subset only; no pause ratio"
		}
		out = append(out, g.row)
	}
	return out, ctx.Err()
}
func Build(ctx context.Context, s r.Set, source string) (models.AnalysisResult, error) {
	out := models.New(ResultType, "runtime-evidence.v1")
	if e := s.Validate(ctx); e != nil {
		return out, e
	}
	gc, e := GCCoverages(ctx, s)
	if e != nil {
		return out, e
	}
	out.Metadata.SchemaVersion = PayloadVersion
	out.SourceFiles = []string{source}
	counts := map[string]int{}
	for _, v := range s.Observations {
		counts[v.Kind]++
	}
	out.Summary = map[string]any{"source_count": strconv.Itoa(len(s.Sources)), "process_count": strconv.Itoa(len(s.Processes)), "entity_count": strconv.Itoa(len(s.Entities)), "observation_count": strconv.Itoa(len(s.Observations)), "profile_references": strconv.Itoa(len(s.Profiles)), "plan_nodes": strconv.Itoa(len(s.Plans))}
	for _, key := range []string{"task_snapshot", "thread_snapshot", "allocation", "event_loop", "cpu", "gc", "access", "database"} {
		out.Summary[key+"_observations"] = strconv.Itoa(counts[key])
	}
	rows := []map[string]any{}
	timeline := []map[string]any{}
	sources := map[string]r.Source{}
	for _, v := range s.Sources {
		sources[v.ID] = v
	}
	for _, v := range s.Observations {
		rows = append(rows, map[string]any{"observation_id": v.ID, "entity_id": v.Entity, "kind": v.Kind, "metric": v.Metric, "value": v.Value, "unit": v.Unit, "observed_state": v.State, "inferred_category": v.Inferred, "measurement_basis": v.Basis, "stack_id": v.Stack, "population": v.Population})
		point := v.Time
		if point == nil {
			point = v.To
		}
		if sources[v.Ref.Source].Clock == "unix" {
			if stamp := m.Timestamp(point); stamp != nil {
				timeline = append(timeline, map[string]any{"time": *stamp, "clock": "unix", "event_id": v.ID, "entity_id": v.Entity, "kind": v.Kind, "metric": v.Metric, "unit": v.Unit, "value": v.Value})
			}
		}
	}
	out.Tables = map[string]any{"gc_coverage": gc, "observations": rows, "sources": s.Sources, "processes": s.Processes, "entities": s.Entities, "relations": s.Relations, "profile_refs": s.Profiles, "plan_nodes": s.Plans, "timeline": timeline}
	out.Metadata.Extra["runtime_core"] = s
	out.Metadata.Extra["extensions"] = map[string]string{"runtime_core": r.Version, "measurement": "1.0.0"}
	out.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	out.Metadata.Extra["identity_basis"] = "declared"
	out.Metadata.Extra["clock_ids"] = []string{"unix"}
	out.Metadata.Extra["limitations"] = []string{"Tasks, runtime worker IDs and native OS TIDs are different identities; snapshots are not continuous CPU/wait intervals.", "CPU/TID context qualification requires host/boot/namespace/process and thread lifetimes, aligned clock and containing interval; numeric nid/TID equality alone is insufficient. Source declarations are not authenticated attribution.", "Allocation snapshots are live traced memory, not cumulative allocation rate or heap object graphs; event-loop distributions and worker scopes are not combined.", "GC pause union uses explicit qualified intervals; concurrent durations remain separate and no pause ratio is invented from incomplete coverage.", "PostgreSQL plan estimates remain distinct from actual per-loop rows/time. Parent/child time is not added; raw SQL/predicates/environment/arguments are excluded from this model; technical names still need sanitized input.", "Profile references bind an explicit profile/entity and source/model hashes; relative/unknown profile clocks do not establish request or native-thread attribution."}
	measurements := map[string]resultcontract.Measurement{}
	for key := range out.Summary {
		measurements["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "runtime_observations", Encoding: "exact_integer_string", Aggregation: "count", Availability: "observed"}
	}
	out.Metadata.Extra["measurement"] = measurements
	raw, e := json.Marshal(out)
	if e != nil {
		return out, e
	}
	if len(raw) > 64<<20 {
		return out, fmt.Errorf("RUNTIME_RESULT_BYTES_BUDGET: bytes=%d limit=67108864; split original sources", len(raw))
	}
	return out, ctx.Err()
}
func ReadCore(ctx context.Context, v models.AnalysisResult) (r.Set, error) {
	if v.Type != ResultType || v.Metadata.SchemaVersion != PayloadVersion {
		return r.Set{}, fmt.Errorf("RUNTIME_RESULT_VERSION_UNSUPPORTED")
	}
	raw, e := json.Marshal(v.Metadata.Extra["runtime_core"])
	if e != nil {
		return r.Set{}, e
	}
	return r.Decode(ctx, raw)
}
func Validate(v models.AnalysisResult) error {
	s, e := ReadCore(context.Background(), v)
	if e != nil {
		return e
	}
	expected, e := Build(context.Background(), s, "")
	if e != nil {
		return e
	}
	for _, pair := range [][2]any{{v.Summary, expected.Summary}, {v.Tables, expected.Tables}, {v.Series, expected.Series}, {v.Charts, expected.Charts}, {v.Metadata.Extra["limitations"], expected.Metadata.Extra["limitations"]}, {v.Metadata.Extra["measurement"], expected.Metadata.Extra["measurement"]}, {v.Metadata.Extra["identity_basis"], expected.Metadata.Extra["identity_basis"]}} {
		a, _ := json.Marshal(pair[0])
		b, _ := json.Marshal(pair[1])
		var x, y any
		if wirejson.Decode(a, &x) != nil || wirejson.Decode(b, &y) != nil || !reflect.DeepEqual(x, y) {
			return fmt.Errorf("RUNTIME_RESULT_CORE_MISMATCH")
		}
	}
	return nil
}
func Descriptor() resultcontract.Descriptor {
	scalars := map[string]resultcontract.ValueSchema{}
	for _, key := range []string{"source_count", "process_count", "entity_count", "observation_count", "profile_references", "plan_nodes", "task_snapshot_observations", "thread_snapshot_observations", "allocation_observations", "event_loop_observations", "cpu_observations", "gc_observations", "access_observations", "database_observations"} {
		scalars["/summary/"+key] = resultcontract.ValueSchema{Kinds: []string{"record_count"}, Units: []string{"count"}, Encodings: []string{"exact_integer_string"}, Aggregations: []string{"count"}}
	}
	providers := []resultcontract.Provider{}
	for _, key := range []string{"sources", "processes", "entities", "observations", "relations", "profile_refs", "plan_nodes"} {
		providers = append(providers, resultcontract.Provider{ID: "runtime:" + key, Version: "1.0.0", Kind: "detail", Path: "/tables/" + key, Schema: "object-rows-v1"})
	}
	providers = append(providers, resultcontract.Provider{ID: "runtime:gc_coverage", Version: "1.0.0", Kind: "detail", Path: "/tables/gc_coverage", Schema: "object-rows-v1"})
	providers = append(providers, resultcontract.Provider{ID: "runtime:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/timeline", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "event_id"})
	return resultcontract.Descriptor{Type: ResultType, Payload: resultcontract.Range{Min: PayloadVersion, Before: "0.1.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}, Extensions: map[string]resultcontract.Range{"runtime_core": {Min: r.Version, Before: "1.0.1"}, "measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"runtime_core", "measurement"}, "compute": {"runtime_core", "measurement"}}, Scalars: scalars, Providers: providers, ValidatePayload: Validate}
}
