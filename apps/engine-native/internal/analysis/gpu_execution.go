package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	export "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/gputrace"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
)

type GPUExecutionInventory struct {
	ResultSHA  string     `json:"result_sha256,omitempty"`
	SourceSHA  string     `json:"source_sha256,omitempty"`
	Sources    []g.Source `json:"sources"`
	Tracks     []g.Track  `json:"tracks"`
	TrackCount int        `json:"track_count"`
	Events     int        `json:"event_count"`
	Summaries  int        `json:"summary_count"`
	Warnings   []string   `json:"warnings"`
}

func InspectGPUExecution(s g.Set) GPUExecutionInventory {
	return GPUExecutionInventory{Sources: s.Sources, Tracks: s.Tracks[:min(64, len(s.Tracks))], TrackCount: len(s.Tracks), Events: len(s.Events), Summaries: len(s.Summaries), Warnings: s.Warnings}
}

type gpuSemantic struct {
	set        g.Set
	projection a.Projection
}

func withGPU[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, g.Set, a.Projection, string) (T, error)) (T, error) {
	return withSemantic(s, parent, handle, digest, a.ResultType, "gpu_execution_core", func(ctx context.Context, raw []byte) (any, int, error) {
		set, e := g.Decode(ctx, raw)
		if e != nil {
			return nil, 0, e
		}
		p, e := a.Project(ctx, set)
		if e != nil {
			return nil, 0, e
		}
		bytes, _ := json.Marshal(p)
		return gpuSemantic{set, p}, len(bytes), nil
	}, func(ctx context.Context, value any, sha string) (T, error) {
		v := value.(gpuSemantic)
		return run(ctx, v.set, v.projection, sha)
	})
}
func (s *Service) GPUExecutionInventory(ctx context.Context, handle string) (GPUExecutionInventory, error) {
	v, e := withGPU(s, ctx, handle, "", func(ctx context.Context, set g.Set, _ a.Projection, digest string) (GPUExecutionInventory, error) {
		v := InspectGPUExecution(set)
		v.ResultSHA = digest
		return v, ctx.Err()
	})
	return gpuCopy(v), e
}
func (s *Service) QueryGPUExecution(ctx context.Context, handle, digest string, q a.Query) (a.Page, error) {
	if digest == "" {
		return a.Page{}, fmt.Errorf("GPU_TRACE_RESULT_STALE")
	}
	v, e := withGPU(s, ctx, handle, digest, func(ctx context.Context, set g.Set, p a.Projection, sha string) (a.Page, error) {
		return a.QuerySet(ctx, set, p, sha, q)
	})
	return gpuCopy(v), e
}
func (s *Service) ExportGPUExecutionSVG(ctx context.Context, handle, digest string, q a.Query) ([]byte, error) {
	if digest == "" {
		return nil, fmt.Errorf("GPU_TRACE_RESULT_STALE")
	}
	return withGPU(s, ctx, handle, digest, func(ctx context.Context, set g.Set, p a.Projection, sha string) ([]byte, error) {
		return export.SVG(ctx, set, p, sha, q)
	})
}
func gpuCopy[T any](v T) T {
	raw, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(raw, &out)
	return out
}
