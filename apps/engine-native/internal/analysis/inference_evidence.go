package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/inference"
	history "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

func withInference[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, n.Set, string) (T, error)) (out T, err error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return out, e
	}
	defer finish()
	lease, e := s.AcquireResult(handle)
	if e != nil {
		return out, e
	}
	defer s.ReleaseResultLease(lease.ID)
	raw, result, e := s.Read(handle)
	if e != nil {
		return out, e
	}
	actual := bytesDigest(raw)
	if digest != "" && digest != actual {
		return out, fmt.Errorf("INFERENCE_RESULT_STALE")
	}
	if verdict := s.results.Check(result, "compute"); !verdict.Allowed {
		return out, fmt.Errorf("INFERENCE_RESULT_UNSUPPORTED: %s", verdict.Reason)
	}
	set, e := a.ReadCore(result)
	if e != nil {
		return out, e
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	return run(ctx, set, actual)
}
func (s *Service) InferenceInventory(ctx context.Context, handle string) (a.Inventory, error) {
	return withInference(s, ctx, handle, "", func(ctx context.Context, set n.Set, digest string) (a.Inventory, error) {
		v := a.Inspect(set)
		v.ResultSHA256 = digest
		return v, ctx.Err()
	})
}
func signalSelection(set n.Set, ids []string) (m.Selection, error) {
	out := m.Selection{SeriesIDs: []string{}}
	if len(ids) == 0 || len(ids) > n.MaxSignals {
		return out, fmt.Errorf("INFERENCE_SIGNAL_SELECTION_REQUIRED")
	}
	known := map[string]string{}
	for _, v := range set.Signals {
		known[v.ID] = v.SeriesID
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if known[id] == "" || seen[id] {
			return out, fmt.Errorf("INFERENCE_SIGNAL_SELECTION_INVALID")
		}
		seen[id] = true
		out.SeriesIDs = append(out.SeriesIDs, known[id])
	}
	return out, nil
}
func (s *Service) QueryInferenceMetric(ctx context.Context, handle, digest, signal, cursor string, limit int) (m.Page, error) {
	if digest == "" {
		return m.Page{}, fmt.Errorf("INFERENCE_RESULT_STALE")
	}
	return withInference(s, ctx, handle, digest, func(ctx context.Context, set n.Set, actual string) (m.Page, error) {
		selection, e := signalSelection(set, []string{signal})
		if e != nil {
			return m.Page{}, e
		}
		return m.QuerySet(ctx, set.Metrics, actual, m.Query{Selection: selection, Cursor: cursor, Limit: limit})
	})
}
func (s *Service) MaterializeInferenceMetrics(ctx context.Context, handle, digest string, signals []string) (ResultView, ResultLease, error) {
	if digest == "" {
		return ResultView{}, ResultLease{}, fmt.Errorf("INFERENCE_RESULT_STALE")
	}
	body, e := withInference(s, ctx, handle, digest, func(ctx context.Context, set n.Set, actual string) ([]byte, error) {
		selection, e := signalSelection(set, signals)
		if e != nil {
			return nil, e
		}
		result, e := history.Build(ctx, set.Metrics, selection, "explicit inference signal selection")
		if e != nil {
			return nil, e
		}
		result.Metadata.Extra["inference_projection"] = map[string]any{"parent_result_sha256": actual, "signals": signals, "identity_basis": "declared", "aggregation": "none"}
		return json.Marshal(result)
	})
	if e != nil {
		return ResultView{}, ResultLease{}, e
	}
	return s.LoadOwnedSnapshot(ctx, body)
}
