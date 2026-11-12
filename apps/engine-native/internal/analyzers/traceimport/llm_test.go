package traceimport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	l "github.com/aurakimjh/archscope/apps/engine-native/internal/llmmodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/traceimport"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"os"
	"strings"
	"testing"
)

func llmFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, e := os.ReadFile("../../catalog/contracts/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func llmSet(t *testing.T, name string) l.Set {
	t.Helper()
	s, e := l.Decode(context.Background(), llmFixture(t, name))
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestLLMSDKProfilesPrivacyAndCaseGuard(t *testing.T) {
	for _, tc := range []struct{ raw, opts, expected string }{{"sdk-python-1.38.0.otlp.json", "neutral-options.json", "llm-trace-set.json"}, {"edge-cases.otlp.json", "neutral-options.json", "llm-trace-edge-set.json"}, {"genai-subset.otlp.json", "genai-options.json", "llm-trace-genai-set.json"}, {"openinference-subset.otlp.json", "openinference-options.json", "llm-trace-openinference-set.json"}} {
		t.Run(tc.raw, func(t *testing.T) {
			raw := llmFixture(t, "llm-trace-"+tc.raw)
			o, e := p.DecodeLLMOptions(llmFixture(t, "llm-trace-"+tc.opts))
			if e != nil {
				t.Fatal(e)
			}
			s, e := p.ParseLLMBytes(context.Background(), raw, o)
			if e != nil {
				t.Fatal(e)
			}
			canonical, _ := json.Marshal(s)
			expected := llmSet(t, tc.expected)
			want, _ := json.Marshal(expected)
			if !bytes.Equal(canonical, want) {
				t.Fatal("fixture drift")
			}
			if bytes.Contains(canonical, []byte("SYNTHETIC_BODY_MARKER")) {
				t.Fatal("body leaked")
			}
			if !p.RequiresLLMNormalization(raw) || p.RequiresLLMNormalization(canonical) {
				t.Fatal("intake guard")
			}
			r, e := BuildLLM(context.Background(), s, "sanitized.json")
			if e != nil {
				t.Fatal(e)
			}
			saved, _ := json.Marshal(r)
			if bytes.Contains(saved, []byte("SYNTHETIC_BODY_MARKER")) {
				t.Fatal("result body leaked")
			}
			decoded, e := resultcontract.Decode(saved)
			if e != nil {
				t.Fatal(e)
			}
			if e = ValidateLLM(decoded); e != nil {
				t.Fatal(e)
			}
			t.Logf("spans=%d model=%d result=%d", len(s.Spans), len(canonical), len(saved))
		})
	}
	s := llmSet(t, "llm-trace-set.json")
	if s.Contexts[0].Scope != "synthetic-llm-sdk-fixture" || len(s.Links) != 1 {
		t.Fatal("SDK scope/link lost")
	}
}
func TestLLMGraphAttemptsAndTokenBoundaries(t *testing.T) {
	s := llmSet(t, "llm-trace-edge-set.json")
	p, e := ProjectLLM(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	if p.Duplicate != 2 || p.Missing != 1 || p.Cyclic != 2 || len(p.Attempts) != 2 || len(p.Requests) != 1 {
		t.Fatalf("graph %+v", p)
	}
	pair := false
	equal := false
	for _, a := range p.Attempts {
		if a.Status == "explicit_client_server_pair" {
			pair = true
			if a.Output == nil || *a.Output != "3" || a.Authority == "" {
				t.Fatal(a)
			}
		}
	}
	for i, v := range s.Spans {
		if v.SpanID == "0000000000000009" {
			equal = true
			if p.Spans[i].TPOT != nil || p.Spans[i].TokenStatus != "token_boundaries_not_distinct" {
				t.Fatal(p.Spans[i])
			}
		}
	}
	if !pair || !equal {
		t.Fatal(pair, equal)
	}
	// Two overlapping children cover 10..80 once, not the sum of their durations.
	base := s.Spans[0]
	s.Spans = nil
	s.Links = nil
	s.Events = nil
	for i, ts := range [][2]string{{"0", "100"}, {"10", "60"}, {"40", "80"}} {
		v := base
		v.Position = fmt.Sprint(i)
		v.SpanID = fmt.Sprintf("%016x", i+1)
		v.Parent = ""
		if i > 0 {
			v.Parent = "0000000000000001"
		}
		v.Start = m.Pointer(ts[0])
		v.End = m.Pointer(ts[1])
		v.FirstToken = nil
		v.LastToken = nil
		v.OutputTokens = nil
		v.Attempt = ""
		v.AttemptKind = ""
		v.Accounting = "none"
		v.ID = l.SpanID(s.Source.SHA256, v)
		s.Spans = append(s.Spans, v)
	}
	p, e = ProjectLLM(context.Background(), s)
	if e != nil || p.Spans[0].Self == nil || *p.Spans[0].Self != "30" {
		t.Fatal(p, e)
	}
	// A link is not included in the child union.
	s.Links = []l.Link{{Span: s.Spans[0].ID, TraceID: base.TraceID, SpanID: s.Spans[1].SpanID, Flags: "0", DroppedAttributes: "0", Position: "link:1"}}
	p, e = ProjectLLM(context.Background(), s)
	if e != nil || *p.Spans[0].Self != "30" {
		t.Fatal(e)
	}
}
func TestLLMQueryCursorCancellationAndAttemptCycles(t *testing.T) {
	s := llmSet(t, "llm-trace-edge-set.json")
	q := LLMQuery{Kind: "records", Limit: 1}
	digest := strings.Repeat("a", 64)
	page, e := QueryLLM(context.Background(), s, digest, q)
	if e != nil || page.Total != 10 || page.Next == "" {
		t.Fatal(page, e)
	}
	q.Cursor = page.Next
	q.Kind = "spans"
	if _, e = QueryLLM(context.Background(), s, digest, q); e == nil {
		t.Fatal("cursor selector not bound")
	}
	q.Kind = "records"
	if _, e = QueryLLM(context.Background(), s, strings.Repeat("b", 64), q); e == nil {
		t.Fatal("cursor digest not bound")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = QueryLLM(ctx, s, digest, q); e == nil {
		t.Fatal("cancel ignored")
	}
	for i := range s.Spans {
		v := &s.Spans[i]
		if v.Attempt == "attempt-1" {
			v.PreviousAttempt = "attempt-2"
			v.ID = l.SpanID(s.Source.SHA256, *v)
		}
	}
	s.Links = nil
	s.Events = nil
	p, e := ProjectLLM(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	for _, a := range p.Attempts {
		if a.PreviousStatus != "previous_attempt_cycle" {
			t.Fatal(a)
		}
	}
}
func TestLLMDeepScaleSavedReadAndNoRecursion(t *testing.T) {
	if testing.Short() {
		t.Skip("scale")
	}
	base := llmSet(t, "llm-trace-set.json")
	seed := base.Spans[0]
	base.Spans = nil
	base.Events = nil
	base.Links = nil
	for _, count := range []int{10000, 20000} {
		s := base
		s.Spans = make([]l.Span, count)
		for i := range s.Spans {
			v := seed
			v.Position = fmt.Sprint(i)
			v.SpanID = fmt.Sprintf("%016x", i+1)
			v.Parent = ""
			if i > 0 {
				v.Parent = fmt.Sprintf("%016x", i)
			}
			v.LogicalRequest = ""
			v.Attempt = ""
			v.PreviousAttempt = ""
			v.AttemptKind = ""
			v.Accounting = "none"
			v.Start = m.Pointer("1")
			v.End = m.Pointer("2")
			v.FirstToken = nil
			v.LastToken = nil
			v.ID = l.SpanID(s.Source.SHA256, v)
			s.Spans[i] = v
		}
		raw, _ := json.Marshal(s)
		if len(raw) > l.MaxBytes {
			t.Fatal("model budget", len(raw))
		}
		r, e := BuildLLM(context.Background(), s, "scale.json")
		if e != nil {
			t.Fatal(e)
		}
		saved, _ := json.Marshal(r)
		decoded, e := resultcontract.Decode(saved)
		if e != nil {
			t.Fatal(e)
		}
		if e = ValidateLLM(decoded); e != nil {
			t.Fatal(e)
		}
		t.Logf("spans=%d model_bytes=%d result_bytes=%d", count, len(raw), len(saved))
	}
}

func TestLLMUnknownClockMissingBoundaryAndStrictSourceRefusal(t *testing.T) {
	s := llmSet(t, "llm-trace-edge-set.json")
	s.Source.Clock = "unknown"
	project, e := ProjectLLM(context.Background(), s)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range project.Spans {
		if v.Duration != nil || v.Self != nil || v.TTFT != nil || v.TPOT != nil {
			t.Fatal("unknown clock derived", v)
		}
	}
	s.Source.Clock = "unix"
	s.Spans[0].Start = nil
	s.Spans[0].ID = l.SpanID(s.Source.SHA256, s.Spans[0])
	s.Links = nil
	s.Events = nil
	project, e = ProjectLLM(context.Background(), s)
	if e != nil || project.Spans[0].Duration != nil {
		t.Fatal(project, e)
	}
	raw := llmFixture(t, "llm-trace-sdk-python-1.38.0.otlp.json")
	escaped := bytes.ReplaceAll(raw, []byte("diag.llm."), []byte(`diag.\u006clm.`))
	if !p.RequiresLLMNormalization(escaped) {
		t.Fatal("escaped key bypass")
	}
	path := t.TempDir() + "/raw.json"
	os.WriteFile(path, raw, 0600)
	if _, e = Analyze(path, Options{}); e == nil || !strings.Contains(e.Error(), "LLMTRACE_SEMANTIC_REQUIRED") {
		t.Fatal("legacy path accepted raw body", e)
	}
}

func TestLLMLegacyGuardRefusesUninspectedTail(t *testing.T) {
	path := t.TempDir() + "/large.json"
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(64<<20 + 1); e != nil {
		t.Fatal(e)
	}
	f.Close()
	if _, e = Analyze(path, Options{}); e == nil || !strings.Contains(e.Error(), "TRACE_SOURCE_BUDGET") {
		t.Fatal("uninspected tail admitted", e)
	}
}
