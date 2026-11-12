package traceimport

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"

	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const LLMPayloadVersion = "0.3.0"

type LLMSpanView struct {
	ID           string  `json:"record_id"`
	TraceID      string  `json:"trace_id"`
	SpanID       string  `json:"span_id"`
	Role         string  `json:"role"`
	Boundary     string  `json:"boundary"`
	Population   string  `json:"population"`
	Model        string  `json:"model"`
	Duration     *string `json:"duration_ns"`
	TimeStatus   string  `json:"time_status"`
	Self         *string `json:"observed_self_ns"`
	SelfStatus   string  `json:"self_status"`
	ParentStatus string  `json:"parent_status"`
	TTFT         *string `json:"ttft_ns"`
	TPOT         *string `json:"tpot_ns"`
	TokenStatus  string  `json:"token_timing_status"`
	Input        *string `json:"input_tokens"`
	Output       *string `json:"output_tokens"`
	Outcome      string  `json:"outcome"`
}
type LLMRelation struct {
	Record       string `json:"record_id"`
	Kind         string `json:"kind"`
	TargetTrace  string `json:"target_trace_id"`
	TargetSpan   string `json:"target_span_id"`
	TargetRecord string `json:"target_record_id"`
	Status       string `json:"status"`
}
type LLMAttempt struct {
	Key            string  `json:"attempt_key"`
	Namespace      string  `json:"request_namespace"`
	Logical        string  `json:"logical_request_id"`
	Attempt        string  `json:"attempt_id"`
	Previous       string  `json:"previous_attempt_id"`
	Kind           string  `json:"attempt_kind"`
	Status         string  `json:"status"`
	PreviousStatus string  `json:"previous_status"`
	Records        string  `json:"span_record_ids"`
	Accounting     string  `json:"token_accounting_status"`
	Authority      string  `json:"token_authority_record"`
	Input          *string `json:"input_tokens"`
	Output         *string `json:"output_tokens"`
	Models         string  `json:"models"`
	Populations    string  `json:"populations"`
}
type LLMLogical struct {
	Namespace   string `json:"request_namespace"`
	ID          string `json:"logical_request_id"`
	Spans       string `json:"span_observation_count"`
	Attempts    string `json:"explicit_attempt_count"`
	TokenPolicy string `json:"token_accounting"`
	Models      string `json:"models"`
	Populations string `json:"populations"`
}
type LLMProjection struct {
	Spans                      []LLMSpanView
	Relations                  []LLMRelation
	Attempts                   []LLMAttempt
	Requests                   []LLMLogical
	Duplicate, Missing, Cyclic int
}

func llmKey(trace, span string) string { return trace + "/" + span }
func duration(a, b *string) (*string, string) {
	x, ok := m.Time(a)
	y, yes := m.Time(b)
	if !ok || !yes {
		return nil, "timestamp_unavailable"
	}
	if y < x {
		return nil, "reversed_time"
	}
	return m.Pointer(strconv.FormatUint(y-x, 10)), "observed_endpoints"
}
func jsonText(v any) string { b, _ := json.Marshal(v); return string(b) }
func sortedValues(values map[string]bool) []string {
	out := []string{}
	for k := range values {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func ProjectLLM(ctx context.Context, s l.Set) (LLMProjection, error) {
	out := LLMProjection{Spans: []LLMSpanView{}, Relations: []LLMRelation{}, Attempts: []LLMAttempt{}, Requests: []LLMLogical{}}
	if e := s.Validate(ctx); e != nil {
		return out, e
	}
	byWire := map[string][]int{}
	for i, v := range s.Spans {
		byWire[llmKey(v.TraceID, v.SpanID)] = append(byWire[llmKey(v.TraceID, v.SpanID)], i)
	}
	parent := make([]int, len(s.Spans))
	children := make([][]int, len(s.Spans))
	parentStatus := make([]string, len(s.Spans))
	for i := range parent {
		parent[i] = -1
	}
	for i, v := range s.Spans {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if len(byWire[llmKey(v.TraceID, v.SpanID)]) > 1 {
			out.Duplicate++
		}
		if v.Parent == "" {
			parentStatus[i] = "root"
			continue
		}
		targets := byWire[llmKey(v.TraceID, v.Parent)]
		status, target := "missing_parent", ""
		if len(targets) == 1 {
			status = "explicit_parent"
			parent[i] = targets[0]
			children[targets[0]] = append(children[targets[0]], i)
			target = s.Spans[targets[0]].ID
		} else if len(targets) > 1 {
			status = "ambiguous_parent"
		} else {
			out.Missing++
		}
		parentStatus[i] = status
		out.Relations = append(out.Relations, LLMRelation{v.ID, "parent", v.TraceID, v.Parent, target, status})
	}
	// Iterative single-parent cycle audit is linear even for a 20k-deep chain.
	color := make([]byte, len(s.Spans))
	cyclic := make([]bool, len(s.Spans))
	for i := range s.Spans {
		if color[i] != 0 {
			continue
		}
		path := []int{}
		at := i
		for at >= 0 && color[at] == 0 {
			color[at] = 1
			path = append(path, at)
			at = parent[at]
		}
		if at >= 0 && color[at] == 1 {
			for j := len(path) - 1; j >= 0; j-- {
				cyclic[path[j]] = true
				out.Cyclic++
				if path[j] == at {
					break
				}
			}
		}
		for _, v := range path {
			color[v] = 2
		}
	}
	events := map[string][]l.Event{}
	for _, e := range s.Events {
		events[e.Span] = append(events[e.Span], e)
	}
	for i, v := range s.Spans {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		d, status := duration(v.Start, v.End)
		if s.Source.Clock == "unknown" {
			d = nil
			status = "clock_unavailable"
		}
		r := LLMSpanView{ID: v.ID, TraceID: v.TraceID, SpanID: v.SpanID, Role: v.Role, Boundary: v.Boundary, Population: v.Population, Model: v.Model, Duration: d, TimeStatus: status, ParentStatus: parentStatus[i], Input: v.InputTokens, Output: v.OutputTokens, Outcome: v.Outcome, SelfStatus: "observed_child_union_subtraction", TokenStatus: "token_boundaries_unavailable"}
		if cyclic[i] {
			r.ParentStatus = "cycle"
		}
		if len(byWire[llmKey(v.TraceID, v.SpanID)]) > 1 {
			r.ParentStatus = "duplicate_span_id"
		}
		if d == nil {
			r.SelfStatus = status
		} else if cyclic[i] || r.ParentStatus == "duplicate_span_id" || r.ParentStatus == "ambiguous_parent" {
			r.SelfStatus = "graph_ambiguous"
		} else {
			type interval struct{ start, end uint64 }
			ranges := []interval{}
			start, _ := m.Time(v.Start)
			end, _ := m.Time(v.End)
			for _, ci := range children[i] {
				child := s.Spans[ci]
				a, ok := m.Time(child.Start)
				b, yes := m.Time(child.End)
				if !ok || !yes || b < a || a < start || b > end || cyclic[ci] || len(byWire[llmKey(child.TraceID, child.SpanID)]) > 1 {
					r.SelfStatus = "child_interval_unqualified"
					break
				}
				ranges = append(ranges, interval{a, b})
			}
			if r.SelfStatus == "observed_child_union_subtraction" {
				sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
				covered := uint64(0)
				var lo, hi uint64
				for j, x := range ranges {
					if j == 0 {
						lo, hi = x.start, x.end
					} else if x.start <= hi {
						if x.end > hi {
							hi = x.end
						}
					} else {
						covered += hi - lo
						lo, hi = x.start, x.end
					}
				}
				if len(ranges) > 0 {
					covered += hi - lo
				}
				r.Self = m.Pointer(strconv.FormatUint(end-start-covered, 10))
			}
		}
		first, last := v.FirstToken, v.LastToken
		conflict := false
		for _, kind := range []string{"first_token", "last_token"} {
			values := []*string{}
			for _, event := range events[v.ID] {
				if event.Kind == kind {
					values = append(values, event.Time)
				}
			}
			target := &first
			if kind == "last_token" {
				target = &last
			}
			if len(values) == 1 {
				if *target == nil {
					*target = values[0]
				} else if values[0] == nil || **target != *values[0] {
					conflict = true
				}
			} else if len(values) > 1 {
				conflict = true
			}
		}
		if s.Source.Clock == "unknown" {
			r.TokenStatus = "clock_unavailable"
		} else if conflict {
			r.TokenStatus = "token_boundary_conflict"
		} else {
			start, sk := m.Time(v.Start)
			end, ek := m.Time(v.End)
			f, fk := m.Time(first)
			z, zk := m.Time(last)
			if fk && sk && f >= start && (!ek || f <= end) {
				r.TTFT = m.Pointer(strconv.FormatUint(f-start, 10))
				r.TokenStatus = "observed_first_token"
			}
			if v.OutputTokens != nil {
				count, _ := m.Uint(*v.OutputTokens)
				if count.Cmp(big.NewInt(2)) < 0 {
					r.TokenStatus = "tpot_requires_at_least_two_tokens"
				} else if fk && zk && z > f && (!sk || f >= start) && (!ek || z <= end) {
					r.TPOT = m.Pointer(m.Text(new(big.Rat).SetFrac(new(big.Int).SetUint64(z-f), new(big.Int).Sub(count, big.NewInt(1)))))
					r.TokenStatus = "observed_token_boundaries"
				} else if fk && zk && z == f {
					r.TokenStatus = "token_boundaries_not_distinct"
				} else if fk && zk {
					r.TokenStatus = "token_boundary_order_invalid"
				}
			}
		}
		if s.Source.TokenTimingPolicy == l.TokenTimingPolicy && s.Source.Clock != "unknown" && !conflict {
			start, sk := m.Time(v.Start)
			end, ek := m.Time(v.End)
			f, fk := m.Time(first)
			z, zk := m.Time(last)
			invalid := (fk || zk) && ((sk && ek && start > end) || (fk && sk && f < start) || (fk && ek && f > end) || (zk && sk && z < start) || (zk && ek && z > end) || (fk && zk && f > z))
			if invalid {
				r.TTFT = nil
				r.TPOT = nil
				r.TokenStatus = "token_boundary_order_invalid"
			}
		}
		out.Spans = append(out.Spans, r)
	}
	for _, link := range s.Links {
		targets := byWire[llmKey(link.TraceID, link.SpanID)]
		status, target := "external_or_missing_link", ""
		if len(targets) == 1 {
			status = "explicit_link"
			target = s.Spans[targets[0]].ID
		} else if len(targets) > 1 {
			status = "ambiguous_link"
		}
		out.Relations = append(out.Relations, LLMRelation{link.Span, "link", link.TraceID, link.SpanID, target, status})
	}
	out.Attempts, out.Requests = llmAttempts(s)
	return out, ctx.Err()
}
func requestKey(v l.Span) string { return l.Hash([]string{v.RequestNamespace, v.LogicalRequest}) }
func attemptKey(v l.Span) string {
	return l.Hash([]string{v.RequestNamespace, v.LogicalRequest, v.Attempt})
}
func llmAttempts(s l.Set) ([]LLMAttempt, []LLMLogical) {
	wireCounts := map[string]int{}
	for _, v := range s.Spans {
		wireCounts[llmKey(v.TraceID, v.SpanID)]++
	}
	groups := map[string][]l.Span{}
	requests := map[string][]l.Span{}
	for _, v := range s.Spans {
		if v.LogicalRequest != "" {
			key := requestKey(v)
			requests[key] = append(requests[key], v)
		}
		if v.Attempt != "" {
			key := attemptKey(v)
			groups[key] = append(groups[key], v)
		}
	}
	attempts := []LLMAttempt{}
	for _, key := range sortedMapKeys(groups) {
		spans := groups[key]
		v := spans[0]
		ids := []string{}
		models, populations, authorities, previous, kinds := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, x := range spans {
			ids = append(ids, x.ID)
			models[x.Model] = true
			populations[x.Population] = true
			if x.Accounting != "none" {
				authorities[x.Accounting] = true
			}
			previous[x.PreviousAttempt] = true
			kinds[x.AttemptKind] = true
		}
		r := LLMAttempt{Key: key, Namespace: v.RequestNamespace, Logical: v.LogicalRequest, Attempt: v.Attempt, Kind: v.AttemptKind, Previous: v.PreviousAttempt, Status: "ambiguous_attempt_observations", Accounting: "not_aggregated", Records: jsonText(ids), Models: jsonText(sortedValues(models)), Populations: jsonText(sortedValues(populations))}
		qualified := len(spans) == 1
		if qualified {
			r.Status = "single_observation"
		} else if len(spans) == 2 {
			a, b := spans[0], spans[1]
			if a.Kind == "SERVER" {
				a, b = b, a
			}
			if a.Kind == "CLIENT" && b.Kind == "SERVER" && a.TraceID == b.TraceID && b.Parent == a.SpanID && a.Role == b.Role {
				r.Status = "explicit_client_server_pair"
				qualified = true
			}
		}
		for _, v := range spans {
			if wireCounts[llmKey(v.TraceID, v.SpanID)] != 1 {
				qualified = false
				r.Status = "duplicate_span_identity"
			}
		}
		if len(previous) > 1 || len(kinds) > 1 {
			qualified = false
			r.Status = "attempt_metadata_conflict"
		}
		if qualified {
			var authority *l.Span
			if len(spans) == 1 && len(authorities) == 0 {
				authority = &spans[0]
			} else if len(authorities) == 1 {
				kind := "CLIENT"
				if authorities["server"] {
					kind = "SERVER"
				}
				for i := range spans {
					if spans[i].Kind == kind {
						if authority != nil {
							authority = nil
							break
						}
						authority = &spans[i]
					}
				}
			}
			if authority != nil {
				r.Authority = authority.ID
				r.Input = authority.InputTokens
				r.Output = authority.OutputTokens
				r.Accounting = "one_explicit_observation"
			}
		}
		if r.Previous == "" {
			r.PreviousStatus = "none"
		} else if groups[l.Hash([]string{v.RequestNamespace, v.LogicalRequest, r.Previous})] == nil {
			r.PreviousStatus = "previous_attempt_missing"
		} else if r.Previous == r.Attempt {
			r.PreviousStatus = "previous_attempt_cycle"
		} else {
			r.PreviousStatus = "explicit_previous_attempt"
		}
		attempts = append(attempts, r)
	}

	// Audit declared previous-attempt edges iteratively, including multi-node cycles.
	byKey := map[string]int{}
	for i, v := range attempts {
		byKey[v.Key] = i
	}
	colors := make([]uint8, len(attempts))
	for i := range attempts {
		if colors[i] != 0 {
			continue
		}
		path := []int{}
		positions := map[int]int{}
		at := i
		for at >= 0 && colors[at] == 0 {
			if start, ok := positions[at]; ok {
				for _, j := range path[start:] {
					attempts[j].PreviousStatus = "previous_attempt_cycle"
				}
				break
			}
			positions[at] = len(path)
			path = append(path, at)
			v := attempts[at]
			next, ok := byKey[l.Hash([]string{v.Namespace, v.Logical, v.Previous})]
			if v.Previous == "" || !ok || v.Status == "attempt_metadata_conflict" {
				at = -1
			} else {
				at = next
			}
		}
		for _, j := range path {
			colors[j] = 2
		}
	}
	logical := []LLMLogical{}
	for _, key := range sortedMapKeys(requests) {
		spans := requests[key]
		v := spans[0]
		models, populations, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, x := range spans {
			models[x.Model] = true
			populations[x.Population] = true
			if x.Attempt != "" {
				ids[x.Attempt] = true
			}
		}
		logical = append(logical, LLMLogical{v.RequestNamespace, v.LogicalRequest, strconv.Itoa(len(spans)), strconv.Itoa(len(ids)), "no cross-attempt/client-server token sum", jsonText(sortedValues(models)), jsonText(sortedValues(populations))})
	}
	return attempts, logical
}
func sortedMapKeys[T any](v map[string]T) []string {
	keys := []string{}
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func BuildLLM(ctx context.Context, s l.Set, source string) (models.AnalysisResult, error) {
	r := models.New(ResultType, "llm-trace.v1")
	r.Metadata.SchemaVersion = LLMPayloadVersion
	r.SourceFiles = []string{source}
	p, e := ProjectLLM(ctx, s)
	if e != nil {
		return r, e
	}
	traces := map[string]bool{}
	for _, v := range s.Spans {
		traces[v.TraceID] = true
	}
	r.Summary = map[string]any{"total_spans": strconv.Itoa(len(s.Spans)), "unique_traces": strconv.Itoa(len(traces)), "logical_requests": strconv.Itoa(len(p.Requests)), "explicit_attempts": strconv.Itoa(len(p.Attempts)), "duplicate_span_observations": strconv.Itoa(p.Duplicate), "missing_parent_spans": strconv.Itoa(p.Missing), "cyclic_spans": strconv.Itoa(p.Cyclic), "span_links": strconv.Itoa(len(s.Links)), "lifecycle_and_token_events": strconv.Itoa(len(s.Events))}
	events := []map[string]any{}
	timeline := []map[string]any{}
	for _, v := range s.Events {
		events = append(events, map[string]any{"event_id": v.ID, "record_id": v.Span, "kind": v.Kind, "time_ns": v.Time, "source_position": v.Position})
		if t := m.Timestamp(v.Time); t != nil && s.Source.Clock == "unix" {
			timeline = append(timeline, map[string]any{"time": *t, "clock": "unix", "event_id": v.ID, "record_id": v.Span, "kind": v.Kind})
		}
	}
	r.Tables = map[string]any{"llm_spans": p.Spans, "relations": p.Relations, "logical_requests": p.Requests, "attempts": p.Attempts, "llm_events": events, "timeline": timeline}
	r.Series = map[string]any{}
	r.Charts = map[string]any{}
	r.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	r.Metadata.Extra["extensions"] = map[string]string{"llm_core": l.Version, "measurement": "1.0.0"}
	r.Metadata.Extra["llm_core"] = s
	r.Metadata.Extra["privacy_policy"] = l.PrivacyVersion
	r.Metadata.Extra["identity_basis"] = "declared"
	r.Metadata.Extra["clock_ids"] = []string{s.Source.Clock}
	r.Metadata.Extra["limitations"] = []string{"Body-free structural allowlist; raw names, prompt/completion/tool arguments, status messages and arbitrary attributes are omitted. Technical IDs still require sanitized input.", "Span links and parents are different relations. Duplicate/missing/cyclic graphs remain explicit; observed child-union subtraction is not CPU self time or a causal critical path.", "Logical request and attempt identity require explicit request namespace/IDs; time proximity never links requests. Token observations are not summed across retries or client/server.", "Lifecycle populations are explicit declarations, never inferred from nearby events; no requests are silently excluded.", "OTLP carrier and selected semantic subset only; Q01 Langfuse/LangSmith JSON exports and online APIs remain gated."}
	measurements := map[string]resultcontract.Measurement{}
	for key := range r.Summary {
		measurements["/summary/"+key] = resultcontract.Measurement{Kind: "record_count", Unit: "count", Scope: "llm_trace_observations", Encoding: "exact_integer_string", Aggregation: "count", Availability: "observed"}
	}
	r.Metadata.Extra["measurement"] = measurements
	raw, e := json.Marshal(r)
	if e != nil {
		return r, e
	}
	if len(raw) > 64<<20 {
		return r, fmt.Errorf("LLMTRACE_RESULT_BUDGET: bytes=%d limit=67108864 spans=%d; split source traces; no silent truncation", len(raw), len(s.Spans))
	}
	return r, ctx.Err()
}
func ReadLLMCore(r models.AnalysisResult) (l.Set, error) {
	var s l.Set
	if r.Type != ResultType || r.Metadata.SchemaVersion != LLMPayloadVersion {
		return s, fmt.Errorf("LLMTRACE_RESULT_VERSION_UNSUPPORTED")
	}
	raw, e := json.Marshal(r.Metadata.Extra["llm_core"])
	if e != nil {
		return s, e
	}
	return l.Decode(context.Background(), raw)
}
func ValidateLLM(r models.AnalysisResult) error {
	s, e := ReadLLMCore(r)
	if e != nil {
		return e
	}
	expected, e := BuildLLM(context.Background(), s, "")
	if e != nil {
		return e
	}
	pairs := [][2]any{{r.Summary, expected.Summary}, {r.Tables, expected.Tables}, {r.Series, expected.Series}, {r.Charts, expected.Charts}}
	for _, key := range []string{"privacy_policy", "identity_basis", "clock_ids", "limitations", "measurement"} {
		pairs = append(pairs, [2]any{r.Metadata.Extra[key], expected.Metadata.Extra[key]})
	}
	for _, pair := range pairs {
		a, _ := json.Marshal(pair[0])
		b, _ := json.Marshal(pair[1])
		var x, y any
		if wirejson.DecodeWithLimits(a, &x, wirejson.ResultLimits(64<<20, 128)) != nil || wirejson.DecodeWithLimits(b, &y, wirejson.ResultLimits(64<<20, 128)) != nil || !reflect.DeepEqual(x, y) {
			return fmt.Errorf("LLMTRACE_RESULT_MODEL_MISMATCH")
		}
	}
	return nil
}
func LLMDescriptor() resultcontract.Descriptor {
	scalars := map[string]resultcontract.ValueSchema{}
	for _, key := range []string{"total_spans", "unique_traces", "logical_requests", "explicit_attempts", "duplicate_span_observations", "missing_parent_spans", "cyclic_spans", "span_links", "lifecycle_and_token_events"} {
		scalars["/summary/"+key] = resultcontract.ValueSchema{Encodings: []string{"exact_integer_string"}, Kinds: []string{"record_count"}, Units: []string{"count"}, Aggregations: []string{"count"}}
	}
	providers := []resultcontract.Provider{}
	for _, key := range []string{"llm_spans", "relations", "logical_requests", "attempts", "llm_events"} {
		providers = append(providers, resultcontract.Provider{ID: "llm:" + key, Version: "1.0.0", Kind: "detail", Path: "/tables/" + key, Schema: "object-rows-v1"})
	}
	providers = append(providers, resultcontract.Provider{ID: "llm:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/timeline", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "event_id"})
	return resultcontract.Descriptor{Type: ResultType, Payload: resultcontract.Range{Min: LLMPayloadVersion, Before: "0.3.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Operations: []string{"display", "save", "compute", "json", "csv", "html", "report-pack"}, Extensions: map[string]resultcontract.Range{"llm_core": {Min: l.Version, Before: "1.0.1"}, "measurement": {Min: "1.0.0", Before: "1.1.0"}}, Required: map[string][]string{"save": {"llm_core", "measurement"}, "compute": {"llm_core", "measurement"}}, Scalars: scalars, Providers: providers, ValidatePayload: ValidateLLM}
}
