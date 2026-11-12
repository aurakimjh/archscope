package analysis

import (
	"context"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/execution"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
)

type ExecutionInventory struct {
	ResultSHA         string     `json:"result_sha256,omitempty"`
	SourceSHA         string     `json:"source_sha256,omitempty"`
	Kind              string     `json:"kind"`
	Source            e.Source   `json:"source"`
	Context           *e.Context `json:"context"`
	MetricCount       int        `json:"metric_count"`
	SampleCount       int        `json:"sample_count"`
	CrashFrames       int        `json:"crash_frames"`
	ComparisonReasons []string   `json:"comparison_unavailable_reasons"`
	Warnings          []string   `json:"warnings"`
}

func InspectExecution(set e.Set) ExecutionInventory {
	_, reasons := e.ContextIdentity(set)
	frames := 0
	if set.Crash != nil {
		frames = len(set.Crash.Frames)
	}
	return e.JSONCopy(ExecutionInventory{Kind: set.Kind, Source: set.Source, Context: set.Context, MetricCount: len(set.Metrics), SampleCount: len(set.Samples), CrashFrames: frames, ComparisonReasons: reasons, Warnings: set.Limitations})
}
func withExecution[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, e.Set, string) (T, error)) (T, error) {
	return withSemantic(s, parent, handle, digest, a.ResultType, "execution_core", func(ctx context.Context, raw []byte) (any, int, error) {
		set, err := e.Decode(ctx, raw)
		return set, 0, err
	}, func(ctx context.Context, v any, sha string) (T, error) { return run(ctx, v.(e.Set), sha) })
}
func (s *Service) ExecutionInventory(ctx context.Context, handle string) (ExecutionInventory, error) {
	return withExecution(s, ctx, handle, "", func(ctx context.Context, set e.Set, sha string) (ExecutionInventory, error) {
		out := InspectExecution(set)
		out.ResultSHA = sha
		return out, ctx.Err()
	})
}
func (s *Service) QueryExecution(ctx context.Context, handle, digest string, q a.Query) (a.Page, error) {
	if digest == "" {
		return a.Page{}, fmt.Errorf("EXECUTION_RESULT_STALE")
	}
	return withExecution(s, ctx, handle, digest, func(ctx context.Context, set e.Set, sha string) (a.Page, error) { return a.QuerySet(ctx, set, sha, q) })
}
func (s *Service) CompareExecution(ctx context.Context, left, right, leftSHA, rightSHA string, q a.CompareQuery) (e.Comparison, error) {
	if leftSHA == "" || rightSHA == "" {
		return e.Comparison{}, fmt.Errorf("EXECUTION_RESULT_STALE")
	}
	owners := []ResultLease{}
	defer func() {
		for _, owner := range owners {
			s.ReleaseResultLease(owner.ID)
		}
	}()
	for _, handle := range []string{left, right} {
		lease, err := s.AcquireResult(handle)
		if err != nil {
			return e.Comparison{}, err
		}
		owners = append(owners, lease)
	}
	// Acquire load slots sequentially: concurrent comparisons must not each hold
	// one slot while waiting for another. Extra owners retain both revisions.
	base, err := withExecution(s, ctx, left, leftSHA, func(ctx context.Context, set e.Set, _ string) (e.Set, error) { return set, ctx.Err() })
	if err != nil {
		return e.Comparison{}, err
	}
	return withExecution(s, ctx, right, rightSHA, func(ctx context.Context, target e.Set, _ string) (e.Comparison, error) {
		out, err := a.CompareSets(ctx, base, target, leftSHA, rightSHA, q)
		return e.JSONCopy(out), err
	})
}
