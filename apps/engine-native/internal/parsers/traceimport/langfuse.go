package traceimport

import (
	"context"
	"encoding/json"
	"fmt"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"strconv"
)

const LangfuseProfile = "langfuse-observations.v4.79"
const LangfuseFormat = "langfuse-observations-v2-json"

func parseLangfuse(ctx context.Context, original []byte, o LLMOptions) (l.Set, error) {
	s := l.Set{Version: l.Version, Contexts: []l.Context{}, Spans: []l.Span{}, Links: []l.Link{}, Events: []l.Event{}}
	if o.Producer != "langfuse" || o.ProducerVersion != "4.79.0" || o.Clock != "unix" {
		return s, fmt.Errorf("LANGFUSE_EXPORT_UNQUALIFIED: require local4.79.0 observations API v2, explicit namespace and unix clock")
	}
	body, err := llmExpanded(original)
	if err != nil {
		return s, err
	}
	var f struct {
		Data []json.RawMessage          `json:"data"`
		Meta map[string]json.RawMessage `json:"meta"`
	}

	if err = wirejson.DecodeWithLimits(body, &f, wirejson.ResultLimits(64<<20, 128)); err != nil {
		return s, err
	}
	if f.Data == nil || len(f.Data) == 0 || len(f.Data) > l.MaxSpans || len(f.Meta) != 0 {
		return s, fmt.Errorf("LANGFUSE_EXPORT_PAGE_INVALID: supply one explicit complete selected page, not a truncated cursor response")
	}
	s.Source = l.Source{TokenTimingPolicy: l.TokenTimingPolicy, SHA256: LLMDigest(original), Format: LangfuseFormat, Producer: o.Producer, ProducerVersion: o.ProducerVersion, Profile: o.Profile, Namespace: o.Namespace, Clock: o.Clock, Privacy: l.PrivacyVersion, OmittedFields: "0"}
	c := l.Context{Service: "unknown", Scope: "langfuse.observations.v2", ScopeVersion: "4.79.0", DroppedResource: "0", DroppedScope: "0"}
	c.ID = l.ContextID(c)
	s.Contexts = append(s.Contexts, c)
	omitted := 0
	for i, raw := range f.Data {
		if err = ctx.Err(); err != nil {
			return l.Set{}, err
		}
		var obs struct {
			ID     string                     `json:"id"`
			Trace  string                     `json:"traceId"`
			Parent string                     `json:"parentObservationId"`
			Type   string                     `json:"type"`
			Start  json.RawMessage            `json:"startTime"`
			End    json.RawMessage            `json:"endTime"`
			Level  string                     `json:"level"`
			Model  string                     `json:"model"`
			Usage  map[string]json.RawMessage `json:"usageDetails"`
		}
		if err = wirejson.DecodeWithLimits(raw, &obs, wirejson.ResultLimits(64<<20, 128)); err != nil {
			return l.Set{}, err
		}
		start, err := sentryTime(obs.Start)
		if err != nil {
			return l.Set{}, fmt.Errorf("LANGFUSE_START_INVALID")
		}
		var end *string
		if len(obs.End) > 0 && string(obs.End) != "null" {
			v, e := sentryTime(obs.End)
			if e != nil || v < start {
				return l.Set{}, fmt.Errorf("LANGFUSE_END_INVALID")
			}
			end = m.Pointer(strconv.FormatInt(v, 10))
		}
		role := "unclassified"
		switch obs.Type {
		case "GENERATION":
			role = "llm"
		case "RETRIEVER":
			role = "retriever"
		case "EMBEDDING":
			role = "embedding"
		case "SPAN":
		default:
			return l.Set{}, fmt.Errorf("LANGFUSE_OBSERVATION_TYPE_UNQUALIFIED")
		}
		status, outcome := "UNSET", "unknown"
		if obs.Level == "ERROR" {
			status = "ERROR"
			outcome = "failed"
		} else if obs.Level != "DEFAULT" && obs.Level != "DEBUG" && obs.Level != "WARNING" {
			return l.Set{}, fmt.Errorf("LANGFUSE_LEVEL_INVALID")
		}
		v := l.Span{Position: fmt.Sprintf("data/%d", i), TraceID: obs.Trace, SpanID: obs.ID, Parent: obs.Parent, ContextID: c.ID, Kind: "UNSPECIFIED", Role: role, Boundary: "unknown", Population: "unknown", Operation: obs.Type, Model: obs.Model, TokenMethod: "unknown", Start: m.Pointer(strconv.FormatInt(start, 10)), End: end, Status: status, Outcome: outcome, Accounting: "none", Flags: "0", DroppedAttributes: "0", DroppedEvents: "0", DroppedLinks: "0"}
		// API inputUsage/outputUsage may default missing measurements to zero. Only
		// explicit usageDetails keys carry observations in this qualified profile.
		for key, target := range map[string]**string{"input": &v.InputTokens, "output": &v.OutputTokens} {
			raw, present := obs.Usage[key]
			if !present || string(raw) == "null" {
				continue
			}
			var number json.Number
			if json.Unmarshal(raw, &number) != nil {
				return l.Set{}, fmt.Errorf("LANGFUSE_USAGE_INVALID")
			}
			text, err := llmUint(number)
			if err != nil {
				return l.Set{}, err
			}
			*target = m.Pointer(text)
			v.TokenMethod = "source_declared"
		}
		var fields map[string]json.RawMessage
		json.Unmarshal(raw, &fields)
		for key := range fields {
			switch key {
			case "id", "traceId", "parentObservationId", "type", "startTime", "endTime", "level", "model", "usageDetails":
			default:
				omitted++
			}
		}
		v.ID = l.SpanID(s.Source.SHA256, v)
		s.Spans = append(s.Spans, v)
	}
	s.Source.OmittedFields = strconv.Itoa(omitted)
	return s, s.Validate(ctx)
}
