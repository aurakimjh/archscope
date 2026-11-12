package analysis

import (
	"context"
	"encoding/json"
	"fmt"

	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/traceimport"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
)

type LLMInventory struct {
	ResultSHA         string      `json:"result_sha256,omitempty"`
	SourceSHA         string      `json:"source_sha256"`
	Profile           string      `json:"semantic_profile"`
	Namespace         string      `json:"trace_namespace"`
	Privacy           string      `json:"privacy_policy"`
	Spans             int         `json:"span_count"`
	Links             int         `json:"link_count"`
	Events            int         `json:"event_count"`
	ContextCount      int         `json:"context_count"`
	ContextsTruncated bool        `json:"contexts_truncated"`
	Contexts          []l.Context `json:"contexts"`
	Warnings          []string    `json:"warnings"`
}

func InspectLLM(set l.Set) LLMInventory {
	return LLMInventory{SourceSHA: set.Source.SHA256, Profile: set.Source.Profile, Namespace: set.Source.Namespace, Privacy: set.Source.Privacy, Spans: len(set.Spans), Links: len(set.Links), Events: len(set.Events), Contexts: append([]l.Context(nil), set.Contexts[:min(20, len(set.Contexts))]...), ContextCount: len(set.Contexts), ContextsTruncated: len(set.Contexts) > 20, Warnings: []string{"Raw names, content, tool arguments, status messages and arbitrary attributes were excluded; technical identity fields still require sanitized input.", "Parents/links and explicit request/attempt IDs are source declarations, not causal proof or a complete trace.", "Client/server, retry/fallback and lifecycle populations remain separate; no cross-attempt token sum."}}
}

type llmSemantic struct {
	set        l.Set
	projection a.LLMProjection
}

func withLLM[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, l.Set, a.LLMProjection, string) (T, error)) (T, error) {
	return withSemantic(s, parent, handle, digest, a.ResultType, "llm_core", func(ctx context.Context, raw []byte) (any, int, error) {
		set, e := l.Decode(ctx, raw)
		if e != nil {
			return nil, 0, e
		}
		projection, e := a.ProjectLLM(ctx, set)
		if e != nil {
			return nil, 0, e
		}
		charge, _ := json.Marshal(projection)
		return llmSemantic{set, projection}, len(charge), nil
	}, func(ctx context.Context, value any, hash string) (T, error) {
		cached := value.(llmSemantic)
		return run(ctx, cached.set, cached.projection, hash)
	})
}
func (s *Service) LLMInventory(ctx context.Context, handle string) (LLMInventory, error) {
	return withLLM(s, ctx, handle, "", func(ctx context.Context, set l.Set, _ a.LLMProjection, digest string) (LLMInventory, error) {
		v := InspectLLM(set)
		v.ResultSHA = digest
		return v, ctx.Err()
	})
}
func (s *Service) QueryLLMTrace(ctx context.Context, handle, digest string, q a.LLMQuery) (a.LLMPage, error) {
	if digest == "" {
		return a.LLMPage{}, fmt.Errorf("LLMTRACE_RESULT_STALE")
	}
	return withLLM(s, ctx, handle, digest, func(ctx context.Context, set l.Set, projection a.LLMProjection, actual string) (a.LLMPage, error) {
		return a.QueryLLMProjected(ctx, set, projection, actual, q)
	})
}
