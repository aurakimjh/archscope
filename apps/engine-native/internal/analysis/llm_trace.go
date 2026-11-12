package analysis

import (
	"context"
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
	return LLMInventory{SourceSHA: set.Source.SHA256, Profile: set.Source.Profile, Namespace: set.Source.Namespace, Privacy: set.Source.Privacy, Spans: len(set.Spans), Links: len(set.Links), Events: len(set.Events), Contexts: set.Contexts[:min(20, len(set.Contexts))], ContextCount: len(set.Contexts), ContextsTruncated: len(set.Contexts) > 20, Warnings: []string{"Raw names, content, tool arguments, status messages and arbitrary attributes were excluded; technical identity fields still require sanitized input.", "Parents/links and explicit request/attempt IDs are source declarations, not causal proof or a complete trace.", "Client/server, retry/fallback and lifecycle populations remain separate; no cross-attempt token sum."}}
}
func withLLM[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, l.Set, string) (T, error)) (out T, err error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return out, e
	}
	defer finish()
	owner, e := s.AcquireResult(handle)
	if e != nil {
		return out, e
	}
	defer s.ReleaseResultLease(owner.ID)
	raw, result, e := s.Read(handle)
	if e != nil {
		return out, e
	}
	actual := bytesDigest(raw)
	if digest != "" && digest != actual {
		return out, fmt.Errorf("LLMTRACE_RESULT_STALE")
	}
	if verdict := s.results.Check(result, "compute"); !verdict.Allowed {
		return out, fmt.Errorf("LLMTRACE_RESULT_UNSUPPORTED: %s", verdict.Reason)
	}
	set, e := a.ReadLLMCore(result)
	if e != nil {
		return out, e
	}
	return run(ctx, set, actual)
}
func (s *Service) LLMInventory(ctx context.Context, handle string) (LLMInventory, error) {
	return withLLM(s, ctx, handle, "", func(ctx context.Context, set l.Set, digest string) (LLMInventory, error) {
		v := InspectLLM(set)
		v.ResultSHA = digest
		return v, ctx.Err()
	})
}
func (s *Service) QueryLLMTrace(ctx context.Context, handle, digest string, q a.LLMQuery) (a.LLMPage, error) {
	if digest == "" {
		return a.LLMPage{}, fmt.Errorf("LLMTRACE_RESULT_STALE")
	}
	return withLLM(s, ctx, handle, digest, func(ctx context.Context, set l.Set, actual string) (a.LLMPage, error) {
		return a.QueryLLM(ctx, set, actual, q)
	})
}
