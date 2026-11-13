package traceimport

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const FormatSentryTransaction = "sentry-transaction-json"

var sentryISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$`)
var sentryID = regexp.MustCompile(`^[0-9a-f]{16}$`)
var sentryTraceID = regexp.MustCompile(`^[0-9a-f]{32}$`)
var sentrySlots = make(chan struct{}, 2)

type sentrySpan struct {
	Trace  string          `json:"trace_id"`
	ID     string          `json:"span_id"`
	Parent string          `json:"parent_span_id"`
	Op     string          `json:"op"`
	Start  json.RawMessage `json:"start_timestamp"`
	End    json.RawMessage `json:"timestamp"`
	Status string          `json:"status"`
}

// Sentry timestamps may be RFC3339 strings or exact JSON epoch seconds. Float64
// conversion is deliberately avoided; sub-nanosecond precision is refused.
func sentryTime(raw json.RawMessage) (int64, error) {
	var text string
	if len(raw) == 0 {
		return 0, fmt.Errorf("SENTRY_TIME_REQUIRED")
	}
	if raw[0] == '"' {
		if json.Unmarshal(raw, &text) != nil {
			return 0, fmt.Errorf("SENTRY_TIME_INVALID")
		}
		if !sentryISO.MatchString(text) {
			return 0, fmt.Errorf("SENTRY_TIME_INVALID")
		}
		v, e := time.Parse(time.RFC3339Nano, text)
		if e != nil || v.Unix() < 0 {
			return 0, fmt.Errorf("SENTRY_TIME_INVALID")
		}
		ns := new(big.Int).Mul(big.NewInt(v.Unix()), big.NewInt(1000000000))
		ns.Add(ns, big.NewInt(int64(v.Nanosecond())))
		if !ns.IsInt64() {
			return 0, fmt.Errorf("SENTRY_TIME_INVALID")
		}
		return ns.Int64(), nil
	}
	if len(raw) > 128 {
		return 0, fmt.Errorf("SENTRY_TIME_INVALID")
	}
	if i := strings.IndexAny(string(raw), "eE"); i >= 0 {
		exp, e := strconv.Atoi(string(raw[i+1:]))
		if e != nil || exp < -128 || exp > 128 {
			return 0, fmt.Errorf("SENTRY_TIME_INVALID")
		}
	}
	v, ok := new(big.Rat).SetString(string(raw))
	if !ok {
		return 0, fmt.Errorf("SENTRY_TIME_INVALID")
	}
	v.Mul(v, big.NewRat(1000000000, 1))
	if !v.IsInt() || !v.Num().IsInt64() || v.Sign() < 0 {
		return 0, fmt.Errorf("SENTRY_TIME_INVALID")
	}
	return v.Num().Int64(), nil
}
func parseSentryFile(path string) (ParseResult, error) {
	out := ParseResult{Format: FormatSentryTransaction, Spans: []Span{}, Diagnostics: diagnostics.New("trace_import")}
	out.Diagnostics.SetSourceFile(path)
	select {
	case sentrySlots <- struct{}{}:
		defer func() { <-sentrySlots }()
	default:
		return out, fmt.Errorf("SENTRY_IMPORT_BUSY")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	err := ingestion.WithSourceBytes(ctx, path, "sentry-source-", "sentry-original", ingestion.DefaultBudgets(), func(raw []byte) error {
		body, e := ingestion.DecodeGzip(ctx, raw, 64<<20)
		if e != nil {
			return e
		}
		var f struct {
			Type  string          `json:"type"`
			Start json.RawMessage `json:"start_timestamp"`
			End   json.RawMessage `json:"timestamp"`
			SDK   struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"sdk"`
			Contexts struct {
				Trace sentrySpan `json:"trace"`
			} `json:"contexts"`
			Spans []sentrySpan `json:"spans"`
		}
		if e = wirejson.DecodeWithLimits(body, &f, wirejson.ResultLimits(64<<20, 128)); e != nil {
			return e
		}
		if f.Type != "transaction" || f.SDK.Name != "sentry.python" || f.SDK.Version != "2.69.2" || len(f.Spans) >= 50000 {
			return fmt.Errorf("SENTRY_TRANSACTION_UNQUALIFIED")
		}
		root := f.Contexts.Trace
		root.Start = f.Start
		root.End = f.End
		seen := map[string]bool{}
		for _, s := range append([]sentrySpan{root}, f.Spans...) {
			if e = ctx.Err(); e != nil {
				return e
			}
			if !sentryTraceID.MatchString(s.Trace) || s.Trace == strings.Repeat("0", 32) || s.Trace != root.Trace || !sentryID.MatchString(s.ID) || s.ID == strings.Repeat("0", 16) || seen[s.ID] || s.Parent != "" && !sentryID.MatchString(s.Parent) {
				return fmt.Errorf("SENTRY_SPAN_ID_INVALID")
			}
			seen[s.ID] = true
			start, e := sentryTime(s.Start)
			if e != nil {
				return e
			}
			end, e := sentryTime(s.End)
			if e != nil {
				return e
			}
			if end < start {
				return fmt.Errorf("SENTRY_SPAN_TIME_REVERSED")
			}
			op := "unclassified"
			kind := "INTERNAL"
			switch s.Op {
			case "http.server":
				op = s.Op
				kind = "SERVER"
			case "http.client", "db", "cache":
				op = s.Op
				kind = "CLIENT"
			}
			errorStatus := s.Status != "" && s.Status != "ok"
			status := "UNSET"
			if s.Status == "ok" {
				status = "OK"
			} else if errorStatus {
				status = "ERROR"
			}
			out.Spans = append(out.Spans, Span{TraceID: s.Trace, SpanID: s.ID, ParentSpanID: s.Parent, Name: "sentry." + op, ServiceName: "unknown-sentry-service", Kind: kind, StartUnixNanos: start, DurationNanos: end - start, StatusCode: status, Error: errorStatus, Attributes: map[string]string{"producer": "sentry.python", "producer_version": f.SDK.Version}, SourceFormat: FormatSentryTransaction})
		}
		return nil
	})
	if err != nil {
		out.Spans = nil
		return out, err
	}
	out.Diagnostics.ParsedRecords = len(out.Spans)
	return out, nil
}
