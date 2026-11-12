// Package llmmodel stores body-free LLM execution observations over trace spans.
package llmmodel

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

const Version = "1.0.0"
const PrivacyVersion = "llm-structure-only.1.0.0"
const MaxBytes = 32 << 20
const MaxSpans = 50000
const MaxRelations = 100000

type Source struct {
	SHA256          string `json:"sha256"`
	Format          string `json:"format"`
	Producer        string `json:"producer"`
	ProducerVersion string `json:"producer_version"`
	Profile         string `json:"semantic_profile"`
	Namespace       string `json:"trace_namespace"`
	Clock           string `json:"clock"`
	Privacy         string `json:"privacy_policy"`
	OmittedFields   string `json:"omitted_field_count"`
}

// Context is shared rather than repeated in each span. Only technical identity
// fields survive normalization; arbitrary attributes and raw names do not.
type Context struct {
	ID                string `json:"id"`
	Service           string `json:"service"`
	Instance          string `json:"instance"`
	Host              string `json:"host_id"`
	PID               string `json:"process_pid"`
	Container         string `json:"container_id"`
	Pod               string `json:"pod_uid"`
	Scope             string `json:"scope_name"`
	ScopeVersion      string `json:"scope_version"`
	SchemaURL         string `json:"scope_schema_url"`
	ResourceSchemaURL string `json:"resource_schema_url"`
	DroppedResource   string `json:"dropped_resource_attributes"`
	DroppedScope      string `json:"dropped_scope_attributes"`
}
type Span struct {
	ID                string  `json:"id"`
	Position          string  `json:"source_position"`
	TraceID           string  `json:"trace_id"`
	SpanID            string  `json:"span_id"`
	Parent            string  `json:"parent_span_id"`
	ContextID         string  `json:"context_id"`
	Kind              string  `json:"kind"`
	Role              string  `json:"role"`
	Boundary          string  `json:"boundary"`
	Population        string  `json:"population"`
	Operation         string  `json:"operation"`
	Provider          string  `json:"provider"`
	Model             string  `json:"model"`
	Revision          string  `json:"model_revision"`
	Tokenizer         string  `json:"tokenizer_revision"`
	TokenMethod       string  `json:"token_counting_method"`
	InputTokens       *string `json:"input_tokens"`
	OutputTokens      *string `json:"output_tokens"`
	Start             *string `json:"start_ns"`
	End               *string `json:"end_ns"`
	FirstToken        *string `json:"first_token_ns"`
	LastToken         *string `json:"last_token_ns"`
	Status            string  `json:"status"`
	Outcome           string  `json:"outcome"`
	ErrorType         string  `json:"error_type"`
	RequestNamespace  string  `json:"request_namespace"`
	LogicalRequest    string  `json:"logical_request_id"`
	Attempt           string  `json:"attempt_id"`
	PreviousAttempt   string  `json:"previous_attempt_id"`
	AttemptKind       string  `json:"attempt_kind"`
	Route             string  `json:"route_id"`
	Accounting        string  `json:"token_authority"` // explicit client/server, none
	Flags             string  `json:"flags"`
	DroppedAttributes string  `json:"dropped_attributes"`
	DroppedEvents     string  `json:"dropped_events"`
	DroppedLinks      string  `json:"dropped_links"`
}
type Link struct {
	Span              string `json:"span_record_id"`
	TraceID           string `json:"trace_id"`
	SpanID            string `json:"span_id"`
	Flags             string `json:"flags"`
	DroppedAttributes string `json:"dropped_attributes"`
	Position          string `json:"source_position"`
}
type Event struct {
	ID       string  `json:"id"`
	Span     string  `json:"span_record_id"`
	Kind     string  `json:"kind"`
	Time     *string `json:"time_ns"`
	Position string  `json:"source_position"`
}
type Set struct {
	Version  string    `json:"schema_version"`
	Source   Source    `json:"source"`
	Contexts []Context `json:"contexts"`
	Spans    []Span    `json:"spans"`
	Links    []Link    `json:"links"`
	Events   []Event   `json:"events"`
}

func Hash(v any) string                   { return m.Hash(v) }
func ContextID(c Context) string          { c.ID = ""; return "context-" + Hash(c) }
func SpanID(source string, s Span) string { s.ID = ""; return "record-" + Hash([]any{source, s}) }
func EventID(e Event) string              { e.ID = ""; return "event-" + Hash(e) }
func Strict(raw []byte, out any, limits ...int) error {
	limit := MaxBytes
	if len(limits) > 0 && limits[0] < limit {
		limit = limits[0]
	}
	if len(raw) > limit {
		return fmt.Errorf("LLMTRACE_BUDGET: bytes=%d limit=%d", len(raw), limit)
	}
	return m.Strict(raw, out)
}
func Decode(ctx context.Context, raw []byte) (Set, error) {
	var s Set
	if e := Strict(raw, &s); e != nil {
		return s, e
	}
	return s, s.Validate(ctx)
}
func Text(s string) bool {
	return len(s) <= 1024 && utf8.ValidString(s) && !strings.ContainsAny(s, "\x00\r\n")
}

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)
var hex16 = regexp.MustCompile(`^[0-9a-f]{16}$`)
var digest = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TraceID(s string) bool    { return hex32.MatchString(s) && s != strings.Repeat("0", 32) }
func WireSpanID(s string) bool { return hex16.MatchString(s) && s != strings.Repeat("0", 16) }
func In(s string, values ...string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}
func (s Set) Validate(ctx context.Context) error {
	if s.Version != Version || s.Source.Privacy != PrivacyVersion {
		return fmt.Errorf("LLMTRACE_VERSION_UNSUPPORTED")
	}
	if !digest.MatchString(s.Source.SHA256) || s.Source.Format != "otlp-json" || s.Source.Namespace == "" || !In(s.Source.Clock, "unix", "unknown") || s.Source.Profile == "" {
		return fmt.Errorf("LLMTRACE_SOURCE_INVALID")
	}
	for _, v := range []string{s.Source.Producer, s.Source.ProducerVersion, s.Source.Profile, s.Source.Namespace} {
		if !Text(v) {
			return fmt.Errorf("LLMTRACE_SOURCE_INVALID")
		}
	}
	if _, e := m.Uint(s.Source.OmittedFields); e != nil {
		return e
	}
	if len(s.Contexts) > 4096 || len(s.Spans) > MaxSpans || len(s.Links) > MaxRelations || len(s.Events) > MaxRelations {
		return fmt.Errorf("LLMTRACE_BUDGET: context/span/link/event limits; split original export")
	}
	contexts := map[string]bool{}
	for _, v := range s.Contexts {
		if v.ID != ContextID(v) || contexts[v.ID] {
			return fmt.Errorf("LLMTRACE_CONTEXT_INVALID")
		}
		contexts[v.ID] = true
		for _, x := range []string{v.Service, v.Instance, v.Host, v.PID, v.Container, v.Pod, v.Scope, v.ScopeVersion, v.SchemaURL, v.ResourceSchemaURL} {
			if !Text(x) {
				return fmt.Errorf("LLMTRACE_CONTEXT_INVALID")
			}
		}
		for _, x := range []string{v.DroppedResource, v.DroppedScope} {
			if _, e := m.Uint(x); e != nil {
				return e
			}
		}
	}
	ids := map[string]bool{}
	positions := map[string]bool{}
	for _, v := range s.Spans {
		if e := ctx.Err(); e != nil {
			return e
		}
		if v.ID != SpanID(s.Source.SHA256, v) || ids[v.ID] || positions[v.Position] || !TraceID(v.TraceID) || !WireSpanID(v.SpanID) || v.Parent != "" && !WireSpanID(v.Parent) || !contexts[v.ContextID] {
			return fmt.Errorf("LLMTRACE_SPAN_INVALID: identity/position/context")
		}
		ids[v.ID] = true
		positions[v.Position] = true
		for _, x := range []string{v.Position, v.Operation, v.Provider, v.Model, v.Revision, v.Tokenizer, v.ErrorType, v.RequestNamespace, v.LogicalRequest, v.Attempt, v.PreviousAttempt, v.Route} {
			if !Text(x) {
				return fmt.Errorf("LLMTRACE_TEXT_INVALID")
			}
		}
		if !In(v.Kind, "UNSPECIFIED", "INTERNAL", "SERVER", "CLIENT", "PRODUCER", "CONSUMER") || !In(v.Role, "llm", "embedding", "retriever", "reranker", "tool", "chain", "agent", "gateway", "unclassified") || !In(v.Boundary, "client", "server", "gateway", "internal", "unknown") || !In(v.Population, "cold", "warm", "warmup", "recovery", "unknown") || !In(v.Status, "UNSET", "OK", "ERROR") || !In(v.Outcome, "success", "failed", "cancelled", "incomplete", "unknown") || !In(v.TokenMethod, "source_declared", "tokenizer", "estimated", "unknown") || !In(v.Accounting, "none", "client", "server") {
			return fmt.Errorf("LLMTRACE_SEMANTIC_INVALID")
		}
		for _, x := range []*string{v.InputTokens, v.OutputTokens, v.Start, v.End, v.FirstToken, v.LastToken} {
			if x != nil {
				if _, e := m.Uint(*x); e != nil {
					return e
				}
			}
		}
		for _, x := range []string{v.Flags, v.DroppedAttributes, v.DroppedEvents, v.DroppedLinks} {
			if _, e := m.Uint(x); e != nil {
				return e
			}
		}
		if (v.LogicalRequest != "" || v.Attempt != "" || v.PreviousAttempt != "") && v.RequestNamespace == "" {
			return fmt.Errorf("LLMTRACE_REQUEST_NAMESPACE_REQUIRED")
		}
		if v.Attempt != "" && (v.LogicalRequest == "" || !In(v.AttemptKind, "initial", "retry", "fallback", "unknown")) {
			return fmt.Errorf("LLMTRACE_ATTEMPT_INVALID")
		}
		if v.Attempt == "" && (v.AttemptKind != "" || v.PreviousAttempt != "" || v.Accounting != "none") {
			return fmt.Errorf("LLMTRACE_ATTEMPT_INVALID")
		}
	}
	for _, v := range s.Links {
		if !ids[v.Span] || !TraceID(v.TraceID) || !WireSpanID(v.SpanID) || !Text(v.Position) {
			return fmt.Errorf("LLMTRACE_LINK_INVALID")
		}
		for _, x := range []string{v.Flags, v.DroppedAttributes} {
			if _, e := m.Uint(x); e != nil {
				return e
			}
		}
	}
	events := map[string]bool{}
	for _, v := range s.Events {
		if v.ID != EventID(v) || events[v.ID] || !ids[v.Span] || !Text(v.Position) || !In(v.Kind, "first_token", "last_token", "startup", "compile_start", "compile_end", "cache_init", "model_load", "warmup_start", "warmup_end", "model_replace", "memory_reclaim", "scale_from_zero", "restart", "recovery_start", "recovery_end", "rate_limit", "cancel") {
			return fmt.Errorf("LLMTRACE_EVENT_INVALID")
		}
		events[v.ID] = true
		if v.Time != nil {
			if _, e := m.Uint(*v.Time); e != nil {
				return e
			}
		}
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(raw) > MaxBytes {
		return fmt.Errorf("LLMTRACE_MODEL_BUDGET: bytes=%d limit=%d spans=%d; split the original export by trace before normalization", len(raw), MaxBytes, len(s.Spans))
	}
	return ctx.Err()
}
