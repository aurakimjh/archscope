package traceimport

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

//go:embed contracts/llm-semantic-profiles.json
var llmContracts embed.FS

type LLMOptions struct {
	Profile         string `json:"semantic_profile"`
	Namespace       string `json:"trace_namespace"`
	Producer        string `json:"producer"`
	ProducerVersion string `json:"producer_version"`
	Clock           string `json:"clock"`
}
type semanticProfile struct {
	ID         string            `json:"id"`
	Role       string            `json:"role_attribute"`
	Roles      map[string]string `json:"roles"`
	Attributes map[string]string `json:"attributes"`
}
type semanticProfiles struct {
	Version  string            `json:"version"`
	Privacy  string            `json:"privacy_policy"`
	Profiles []semanticProfile `json:"profiles"`
	Events   map[string]string `json:"events"`
}

func LLMProfiles() ([]string, error) {
	p, e := readProfiles()
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for _, v := range p.Profiles {
		ids = append(ids, v.ID)
	}
	return ids, nil
}
func readProfiles() (semanticProfiles, error) {
	var p semanticProfiles
	raw, e := llmContracts.ReadFile("contracts/llm-semantic-profiles.json")
	if e != nil {
		return p, e
	}
	e = l.Strict(raw, &p, 64<<10)
	if e == nil && (p.Version != l.Version || p.Privacy != l.PrivacyVersion) {
		e = fmt.Errorf("LLMTRACE_PROFILE_VERSION")
	}
	return p, e
}
func DecodeLLMOptions(raw []byte) (LLMOptions, error) {
	var o LLMOptions
	return o, l.Strict(raw, &o, 64<<10)
}
func LLMDigest(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }

var llmSlots = make(chan struct{}, 2)

func ParseLLMFile(parent context.Context, path string, o LLMOptions) (l.Set, error) {
	select {
	case llmSlots <- struct{}{}:
		defer func() { <-llmSlots }()
	default:
		return l.Set{}, fmt.Errorf("LLMTRACE_IMPORT_BUSY")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	dir, e := os.MkdirTemp("", "llm-trace-source-")
	if e != nil {
		return l.Set{}, e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "original", ingestion.DefaultBudgets())
	if e != nil {
		return l.Set{}, e
	}
	defer lease.Close()
	f, e := lease.Open(ctx)
	if e != nil {
		return l.Set{}, e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if e != nil {
		return l.Set{}, e
	}
	s, e := ParseLLMBytes(ctx, raw, o)
	if e == nil {
		e = lease.Verify(ctx)
	}
	return s, e
}
func llmExpanded(raw []byte) ([]byte, error) {
	if len(raw) > 64<<20 {
		return nil, fmt.Errorf("LLMTRACE_SOURCE_BUDGET")
	}
	if bytes.HasPrefix(raw, []byte{31, 139}) {
		z, e := gzip.NewReader(bytes.NewReader(raw))
		if e != nil {
			return nil, e
		}
		defer z.Close()
		b, e := io.ReadAll(io.LimitReader(z, 64<<20+1))
		if e != nil {
			return nil, e
		}
		if len(b) > 64<<20 {
			return nil, fmt.Errorf("LLMTRACE_EXPANDED_BUDGET")
		}
		return b, nil
	}
	return raw, nil
}
func llmUint(raw json.Number) (string, error) {
	s := string(raw)
	if s == "" {
		return "0", nil
	}
	if _, e := m.Uint(s); e != nil {
		return "", fmt.Errorf("LLMTRACE_INTEGER_INVALID")
	}
	return s, nil
}
func llmTime(raw json.Number) (*string, error) {
	if string(raw) == "" {
		return nil, nil
	}
	s, e := llmUint(raw)
	if e != nil {
		return nil, e
	}
	return m.Pointer(s), nil
}
func llmAttrs(kvs []otlpKeyValue) (map[string]otlpValue, error) {
	out := map[string]otlpValue{}
	if len(kvs) > 4096 {
		return nil, fmt.Errorf("LLMTRACE_ATTRIBUTE_BUDGET")
	}
	for _, v := range kvs {
		if v.Key == "" {
			continue
		}
		if _, ok := out[v.Key]; ok {
			return nil, fmt.Errorf("LLMTRACE_DUPLICATE_ATTRIBUTE")
		}
		out[v.Key] = v.Value
	}
	return out, nil
}
func ParseLLMBytes(ctx context.Context, original []byte, o LLMOptions) (l.Set, error) {
	s := l.Set{Version: l.Version, Contexts: []l.Context{}, Spans: []l.Span{}, Links: []l.Link{}, Events: []l.Event{}}
	if o.Namespace == "" || !l.Text(o.Namespace) || !l.Text(o.Producer) || !l.Text(o.ProducerVersion) || !l.In(o.Clock, "unix", "unknown") {
		return s, fmt.Errorf("LLMTRACE_OPTIONS_INVALID: explicit namespace, clock and semantic profile required")
	}
	if o.Profile == LangfuseProfile {
		return parseLangfuse(ctx, original, o)
	}
	profiles, e := readProfiles()
	if e != nil {
		return s, e
	}
	var profile semanticProfile
	for _, p := range profiles.Profiles {
		if p.ID == o.Profile {
			profile = p
		}
	}
	if profile.ID == "" {
		return s, fmt.Errorf("LLMTRACE_PROFILE_UNSUPPORTED")
	}
	s.Source = l.Source{TokenTimingPolicy: l.TokenTimingPolicy, SHA256: LLMDigest(original), Format: "otlp-json", Producer: o.Producer, ProducerVersion: o.ProducerVersion, Profile: profile.ID, Namespace: o.Namespace, Clock: o.Clock, Privacy: l.PrivacyVersion, OmittedFields: "0"}
	body, e := llmExpanded(original)
	if e != nil {
		return s, e
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	contexts := map[string]bool{}
	omitted := 0
	envelopes := 0
	for {
		if e = ctx.Err(); e != nil {
			return s, e
		}
		var raw json.RawMessage
		e = decoder.Decode(&raw)
		if e == io.EOF {
			break
		}
		if e != nil {
			return s, fmt.Errorf("LLMTRACE_JSON_INVALID: %w", e)
		}
		if envelopes >= 128 {
			return s, fmt.Errorf("LLMTRACE_ENVELOPE_BUDGET")
		}
		var env otlpEnvelope
		if e = wirejson.DecodeWithLimits(raw, &env, wirejson.ResultLimits(64<<20, 128)); e != nil {
			return s, e
		}
		if len(env.ResourceSpans) == 0 {
			return s, fmt.Errorf("LLMTRACE_OTLP_ENVELOPE_REQUIRED")
		}
		for ri, resource := range env.ResourceSpans {
			ra, e := llmAttrs(resource.Resource.Attributes)
			if e != nil {
				return s, e
			}
			groups := append(append([]otlpScopeSpans{}, resource.ScopeSpans...), resource.InstrumentationLibrarySpans...)
			for gi, group := range groups {
				context := l.Context{Service: ra["service.name"].StringValue, Instance: ra["service.instance.id"].StringValue, Host: ra["host.id"].StringValue, PID: ra["process.pid"].String(), Container: ra["container.id"].StringValue, Pod: ra["k8s.pod.uid"].StringValue, Scope: group.Scope.Name, ScopeVersion: group.Scope.Version, SchemaURL: group.SchemaURL, ResourceSchemaURL: resource.SchemaURL}
				context.DroppedResource, e = llmUint(resource.Resource.Dropped)
				if e != nil {
					return s, e
				}
				context.DroppedScope, e = llmUint(group.Scope.Dropped)
				if e != nil {
					return s, e
				}
				context.ID = l.ContextID(context)
				if !contexts[context.ID] {
					if len(s.Contexts) >= 4096 {
						return s, fmt.Errorf("LLMTRACE_CONTEXT_BUDGET")
					}
					s.Contexts = append(s.Contexts, context)
					contexts[context.ID] = true
				}
				for key := range ra {
					if !l.In(key, "service.name", "service.instance.id", "host.id", "process.pid", "container.id", "k8s.pod.uid") {
						omitted++
					}
				}
				omitted += len(group.Scope.Attributes)
				for si, span := range group.Spans {
					if span.Kind < 0 || span.Kind > 5 || span.Status.Code < 0 || span.Status.Code > 2 {
						return s, fmt.Errorf("LLMTRACE_ENUM_INVALID")
					}
					if e = ctx.Err(); e != nil {
						return s, e
					}
					if len(s.Spans) >= l.MaxSpans {
						return s, fmt.Errorf("LLMTRACE_SPAN_BUDGET: split original export")
					}
					attrs, e := llmAttrs(span.Attributes)
					if e != nil {
						return s, e
					}
					selected := map[string]bool{profile.Role: true}
					for _, key := range profile.Attributes {
						selected[key] = true
					}
					for key := range attrs {
						if !selected[key] {
							omitted++
						}
					}
					if span.Name != "" {
						omitted++
					}
					if span.Status.Message != "" {
						omitted++
					}
					get := func(key string) string { return attrs[profile.Attributes[key]].StringValue }
					count := func(key string) (*string, error) {
						v, ok := attrs[profile.Attributes[key]]
						if !ok {
							return nil, nil
						}
						value := string(v.IntValue)
						if value == "" {
							return nil, fmt.Errorf("LLMTRACE_TOKEN_INTEGER_REQUIRED: %s", key)
						}
						if _, e := m.Uint(value); e != nil {
							return nil, e
						}
						return m.Pointer(value), nil
					}
					position := fmt.Sprintf("envelopes/%d/resourceSpans/%d/scopeSpans/%d/spans/%d", envelopes, ri, gi, si)
					v := l.Span{Position: position, TraceID: strings.ToLower(span.TraceID), SpanID: strings.ToLower(span.SpanID), Parent: strings.ToLower(span.ParentSpanID), ContextID: context.ID, Kind: otlpKindName(span.Kind), Role: profile.Roles[attrs[profile.Role].StringValue], Provider: get("provider"), Model: get("model"), Revision: get("revision"), Tokenizer: get("tokenizer"), TokenMethod: get("token_method"), Boundary: get("boundary"), Population: get("population"), Outcome: get("outcome"), ErrorType: get("error_type"), RequestNamespace: get("request_namespace"), LogicalRequest: get("logical_request"), Attempt: get("attempt"), PreviousAttempt: get("previous_attempt"), AttemptKind: get("attempt_kind"), Route: get("route"), Accounting: get("accounting"), Status: otlpStatusName(span.Status.Code)}
					if v.Parent == strings.Repeat("0", 16) {
						v.Parent = ""
					}
					if v.Role == "" {
						v.Role = "unclassified"
					}
					operation := get("operation")
					if _, ok := profile.Roles[operation]; ok || l.In(operation, "chat", "generate_content", "text_completion", "embeddings", "execute_tool", "invoke_agent", "llm", "embedding", "retriever", "reranker", "tool", "chain", "agent", "gateway") {
						v.Operation = operation
					}
					if v.Boundary == "" {
						v.Boundary = map[string]string{"CLIENT": "client", "SERVER": "server", "INTERNAL": "internal"}[v.Kind]
						if v.Boundary == "" {
							v.Boundary = "unknown"
						}
					}
					if v.Population == "" {
						v.Population = "unknown"
					}
					if v.TokenMethod == "" {
						v.TokenMethod = "unknown"
					}
					if v.Accounting == "" {
						v.Accounting = "none"
					}
					if v.Outcome == "" {
						v.Outcome = "unknown"
						if v.Status == "ERROR" {
							v.Outcome = "failed"
						} else if v.Status == "OK" {
							v.Outcome = "success"
						}
					}
					v.Start, e = llmTime(span.StartTimeUnixNano)
					if e != nil {
						return s, e
					}
					v.End, e = llmTime(span.EndTimeUnixNano)
					if e != nil {
						return s, e
					}
					v.InputTokens, e = count("input_tokens")
					if e != nil {
						return s, e
					}
					v.OutputTokens, e = count("output_tokens")
					if e != nil {
						return s, e
					}
					for key, target := range map[string]**string{"first_token": &v.FirstToken, "last_token": &v.LastToken} {
						if value, ok := attrs[profile.Attributes[key]]; ok {
							number := string(value.IntValue)
							if number == "" {
								number = value.StringValue
							}
							if _, e := m.Uint(number); e != nil {
								return s, fmt.Errorf("LLMTRACE_TOKEN_BOUNDARY_INVALID")
							}
							*target = m.Pointer(number)
						}
					}
					v.Flags, e = llmUint(span.Flags)
					if e != nil {
						return s, e
					}
					v.DroppedAttributes, e = llmUint(span.DroppedAttributes)
					if e != nil {
						return s, e
					}
					v.DroppedEvents, e = llmUint(span.DroppedEvents)
					if e != nil {
						return s, e
					}
					v.DroppedLinks, e = llmUint(span.DroppedLinks)
					if e != nil {
						return s, e
					}
					v.ID = l.SpanID(s.Source.SHA256, v)
					s.Spans = append(s.Spans, v)
					for li, link := range span.Links {
						if len(s.Links) >= l.MaxRelations {
							return s, fmt.Errorf("LLMTRACE_LINK_BUDGET")
						}
						flags, e := llmUint(link.Flags)
						if e != nil {
							return s, e
						}
						dropped, e := llmUint(link.DroppedAttributes)
						if e != nil {
							return s, e
						}
						s.Links = append(s.Links, l.Link{Span: v.ID, TraceID: strings.ToLower(link.TraceID), SpanID: strings.ToLower(link.SpanID), Flags: flags, DroppedAttributes: dropped, Position: position + "/links/" + strconv.Itoa(li)})
						omitted += len(link.Attributes)
					}
					for ei, event := range span.Events {
						omitted += len(event.Attributes)
						kind, ok := profiles.Events[event.Name]
						if !ok {
							omitted++
							continue
						}
						if len(s.Events) >= l.MaxRelations {
							return s, fmt.Errorf("LLMTRACE_EVENT_BUDGET")
						}
						t, e := llmTime(event.Time)
						if e != nil {
							return s, e
						}
						row := l.Event{Span: v.ID, Kind: kind, Time: t, Position: position + "/events/" + strconv.Itoa(ei)}
						row.ID = l.EventID(row)
						s.Events = append(s.Events, row)
					}
				}
			}
		}
		envelopes++
	}
	if envelopes == 0 {
		return s, fmt.Errorf("LLMTRACE_EMPTY_SOURCE")
	}
	s.Source.OmittedFields = strconv.Itoa(omitted)
	return s, s.Validate(ctx)
}

// RequiresLLMNormalization gates raw LLM carrier files before legacy analysis,
// case copies and text indexing. Canonical models contain no raw attribute keys.
func RequiresLLMNormalization(original []byte) bool {
	raw, e := llmExpanded(original)
	if e != nil {
		return bytes.HasPrefix(original, []byte{31, 139})
	}
	if bytes.Contains(raw, []byte(l.PrivacyVersion)) {
		if _, e := l.Decode(context.Background(), raw); e == nil {
			return false
		}
	}
	markers := []string{"gen_ai.", "openinference.", "diag.llm.", "diag.gateway.", "diag.lifecycle.", "llm.prompts", "llm.completions", "traceloop.", "langfuse.", "ai.prompt", "input.value", "output.value"}
	has := func(s string) bool {
		for _, m := range markers {
			if strings.Contains(s, m) {
				return true
			}
		}
		return false
	}
	if has(string(raw)) {
		return true
	}
	if !bytes.Contains(raw, []byte(`\u`)) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var root any
		if e := decoder.Decode(&root); e != nil {
			return false
		}
		var visit func(any) bool
		visit = func(v any) bool {
			switch x := v.(type) {
			case string:
				return has(x)
			case map[string]any:
				for k, v := range x {
					if has(k) || visit(v) {
						return true
					}
				}
			case []any:
				for _, v := range x {
					if visit(v) {
						return true
					}
				}
			}
			return false
		}
		if visit(root) {
			return true
		}
	}
}
