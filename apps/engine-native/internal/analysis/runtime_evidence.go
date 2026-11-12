package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	profile "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtimeevidence"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
)

type RuntimeInventory struct {
	ResultSHA    string      `json:"result_sha256,omitempty"`
	SourceSHA    string      `json:"source_sha256,omitempty"`
	Sources      []r.Source  `json:"sources"`
	Processes    []r.Process `json:"processes"`
	Entities     []r.Entity  `json:"entities"`
	EntityCount  int         `json:"entity_count"`
	Truncated    bool        `json:"truncated"`
	Observations int         `json:"observations"`
	Plans        int         `json:"plan_nodes"`
	Profiles     int         `json:"profile_refs"`
	Warnings     []string    `json:"warnings"`
}

func InspectRuntime(s r.Set) RuntimeInventory {
	value := RuntimeInventory{Sources: s.Sources[:min(16, len(s.Sources))], Processes: s.Processes[:min(20, len(s.Processes))], Entities: s.Entities[:min(20, len(s.Entities))], EntityCount: len(s.Entities), Truncated: len(s.Entities) > 20 || len(s.Processes) > 20 || len(s.Sources) > 16, Observations: len(s.Observations), Plans: len(s.Plans), Profiles: len(s.Profiles), Warnings: []string{"Task/runtime worker/native TID identities differ; snapshots and open handles do not prove continuous wait, CPU use or GIL contention.", "CPU context checks require explicit process/thread lifetime and aligned observation window. Qualified declarations are not authenticated causal attribution.", "Allocation snapshots, event-loop statistics, GC phases and plan estimates/actual per-loop values stay separate; no rate, percentile sum or parent-child duration sum is invented.", "Inventory is capped; query sources/processes/entities for full bounded pages. Sensitive environment/payload/SQL predicates are excluded; sanitize technical identifiers before import."}}
	raw, _ := json.Marshal(value)
	var copied RuntimeInventory
	json.Unmarshal(raw, &copied)
	return copied
}
func withRuntime[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, r.Set, string) (T, error)) (T, error) {
	return withSemantic(s, parent, handle, digest, a.ResultType, "runtime_core", func(ctx context.Context, raw []byte) (any, int, error) {
		set, e := r.Decode(ctx, raw)
		return set, 0, e
	}, func(ctx context.Context, value any, digest string) (T, error) { return run(ctx, value.(r.Set), digest) })
}
func (s *Service) RuntimeInventory(ctx context.Context, handle string) (RuntimeInventory, error) {
	return withRuntime(s, ctx, handle, "", func(ctx context.Context, set r.Set, digest string) (RuntimeInventory, error) {
		v := InspectRuntime(set)
		v.ResultSHA = digest
		return v, ctx.Err()
	})
}
func (s *Service) QueryRuntime(ctx context.Context, handle, digest string, q a.Query) (a.Page, error) {
	if digest == "" {
		return a.Page{}, fmt.Errorf("RUNTIME_RESULT_STALE")
	}
	return withRuntime(s, ctx, handle, digest, func(ctx context.Context, set r.Set, digest string) (a.Page, error) {
		return a.QuerySet(ctx, set, digest, q)
	})
}

func (s *Service) MaterializeRuntimeAllocation(ctx context.Context, handle, digest, entity, source, metric string) (ResultView, ResultLease, error) {
	if digest == "" {
		return ResultView{}, ResultLease{}, fmt.Errorf("RUNTIME_RESULT_STALE")
	}
	body, e := withRuntime(s, ctx, handle, digest, func(ctx context.Context, set r.Set, hash string) ([]byte, error) {
		model, selection, e := a.AllocationProfile(ctx, set, entity, source, metric)
		if e != nil {
			return nil, e
		}
		result, e := profile.BuildSet(ctx, model, selection, "provided live traced allocation snapshot")
		if e != nil {
			return nil, e
		}
		result.Metadata.Extra["runtime_allocation_projection"] = map[string]string{"parent_result_sha256": hash, "entity_id": entity, "source_id": source, "metric": metric, "semantics": "provided live traced snapshot; no cross-capture sum"}
		return json.Marshal(result)
	})
	if e != nil {
		return ResultView{}, ResultLease{}, e
	}
	return s.LoadOwnedSnapshot(ctx, body)
}
