// Package executionmodel describes file-only benchmark and crash context.
package executionmodel

import (
	"context"
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"
)

const Version = "1.0.0"
const MaxBytes = 16 << 20
const MaxSamples = 50000
const MaxMetrics = 8192

func Hash(v any) string    { return m.Hash(v) }
func Ptr(v string) *string { return &v }

var digestRE = regexp.MustCompile(`^[a-f0-9]{64}$`)

func Digest(v string) bool { return digestRE.MatchString(v) }
func Text(v string) bool {
	return len(v) <= 1024 && utf8.ValidString(v) && !strings.ContainsAny(v, "\x00\r\n")
}
func Known(v string) bool { return Text(v) && v != "" && v != "unknown" }
func In(v string, vs ...string) bool {
	for _, s := range vs {
		if v == s {
			return true
		}
	}
	return false
}
func Uint(v *string) bool {
	if v == nil {
		return true
	}
	_, e := m.Uint(*v)
	return e == nil
}
func Number(v string) (*big.Rat, error) {
	if len(v) > 1024 {
		return nil, fmt.Errorf("EXECUTION_NUMBER_BUDGET")
	}
	if !strings.Contains(v, "/") {
		return m.Number(v)
	}
	parts := strings.Split(v, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("EXECUTION_NUMBER_INVALID")
	}
	for index, part := range parts {
		if len(part) == 0 || len(part) > 512 {
			return nil, fmt.Errorf("EXECUTION_NUMBER_INVALID")
		}
		for i, r := range part {
			if r == '-' && i == 0 && index == 0 {
				continue
			}
			if r < '0' || r > '9' {
				return nil, fmt.Errorf("EXECUTION_NUMBER_INVALID")
			}
		}
	}
	n, ok := new(big.Rat).SetString(v)
	if !ok {
		return nil, fmt.Errorf("EXECUTION_NUMBER_INVALID")
	}
	return n, nil
}
func Strict(raw []byte, out any, limit int) error {
	if len(raw) > limit {
		return fmt.Errorf("EXECUTION_BYTES_BUDGET: bytes=%d limit=%d; split original export", len(raw), limit)
	}
	return m.Strict(raw, out)
}

type Source struct {
	SHA      string `json:"sha256"`
	Format   string `json:"format"`
	Producer string `json:"producer"`
	Version  string `json:"producer_version"`
}

// Context fields are declarations, not authenticated capture provenance.
type Context struct {
	InputMean       *string `json:"requested_input_tokens_mean"`
	InputStddev     *string `json:"requested_input_tokens_stddev"`
	OutputMean      *string `json:"requested_output_tokens_mean"`
	OutputStddev    *string `json:"requested_output_tokens_stddev"`
	RunID           string  `json:"run_id"`
	Kind            string  `json:"benchmark_kind"`
	Scenario        string  `json:"scenario_id"`
	WorkloadSHA     string  `json:"workload_sha256"`
	ClientSHA       string  `json:"client_environment_sha256"`
	TargetSHA       string  `json:"target_environment_sha256"`
	Arrival         string  `json:"arrival_model"`
	ArrivalRate     *string `json:"arrival_rate_per_second"`
	Concurrency     *string `json:"concurrency"`
	Duration        *string `json:"configured_duration_ns"`
	Warmup          *string `json:"warmup_duration_ns"`
	Phase           string  `json:"phase"`
	Boundary        string  `json:"observation_boundary"`
	Protocol        string  `json:"protocol"`
	Streaming       *bool   `json:"streaming"`
	Model           string  `json:"model"`
	Revision        string  `json:"model_revision"`
	Tokenizer       string  `json:"tokenizer_revision"`
	InputTokensSHA  string  `json:"input_token_distribution_sha256"`
	OutputTokensSHA string  `json:"output_token_distribution_sha256"`
}
type Metric struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Statistic    string  `json:"statistic"`
	Unit         string  `json:"unit"`
	Population   string  `json:"population"`
	GroupSHA     string  `json:"group_sha256"`
	Policy       string  `json:"statistical_policy"`
	Value        *string `json:"value"`
	Original     *string `json:"original_value"`
	OriginalUnit string  `json:"original_unit"`
	Samples      *string `json:"supporting_records"`
	Position     string  `json:"source_position"`
}

func MetricID(v Metric) string {
	return "measurement-" + Hash([]string{v.Name, v.Statistic, v.Unit, v.Population, v.GroupSHA, v.Policy})
}

type Sample struct {
	ID           string  `json:"id"`
	GroupSHA     string  `json:"group_sha256"`
	Start        *string `json:"start_unix_ns"`
	Latency      *string `json:"latency_seconds"`
	FirstByte    *string `json:"first_byte_seconds"`
	Connect      *string `json:"connect_seconds"`
	Success      bool    `json:"success"`
	ResponseCode string  `json:"response_code"`
	Concurrency  *string `json:"observed_threads"`
	Position     string  `json:"source_position"`
}

func SampleID(v Sample) string { v.ID = ""; return "sample-" + Hash(v) }

type Frame struct {
	Kind     string `json:"kind"`
	Module   string `json:"module"`
	Symbol   string `json:"symbol"`
	Position string `json:"source_position"`
}
type Crash struct {
	Signal       string  `json:"signal"`
	PID          *string `json:"pid"`
	TID          *string `json:"native_tid_decimal"`
	Runtime      string  `json:"runtime"`
	Elapsed      *string `json:"elapsed_seconds"`
	Completeness string  `json:"completeness"`
	Frames       []Frame `json:"frames"`
}
type Set struct {
	Version        string         `json:"schema_version"`
	Kind           string         `json:"kind"`
	Source         Source         `json:"source"`
	Context        *Context       `json:"context"`
	Metrics        []Metric       `json:"metrics"`
	Samples        []Sample       `json:"samples"`
	Distributions  []Distribution `json:"distributions,omitempty"`
	Crash          *Crash         `json:"crash"`
	IgnoredMetrics int            `json:"ignored_metric_count"`
	Limitations    []string       `json:"limitations"`
}

func Empty() Set {
	return Set{Version: Version, Metrics: []Metric{}, Samples: []Sample{}, Limitations: []string{}}
}
func Decode(ctx context.Context, raw []byte) (Set, error) {
	var s Set
	if e := Strict(raw, &s, MaxBytes); e != nil {
		return s, e
	}
	return s, s.Validate(ctx)
}
func (c Context) Validate() error {
	for _, v := range []string{c.RunID, c.Scenario, c.Model, c.Revision, c.Tokenizer, c.Protocol} {
		if !Text(v) {
			return fmt.Errorf("EXECUTION_CONTEXT_TEXT_INVALID")
		}
	}
	for _, v := range []string{c.WorkloadSHA, c.ClientSHA, c.TargetSHA, c.InputTokensSHA, c.OutputTokensSHA} {
		if v != "" && v != "unknown" && !Digest(v) {
			return fmt.Errorf("EXECUTION_CONTEXT_HASH_INVALID")
		}
	}
	if !In(c.Kind, "http", "llm") || !In(c.Arrival, "open", "closed", "unknown") || !In(c.Phase, "cold", "steady", "mixed", "unknown") || !In(c.Boundary, "client", "server", "gateway", "unknown") || !Uint(c.Concurrency) || !Uint(c.Duration) || !Uint(c.Warmup) {
		return fmt.Errorf("EXECUTION_CONTEXT_INVALID")
	}
	for _, raw := range []*string{c.InputMean, c.InputStddev, c.OutputMean, c.OutputStddev} {
		if raw != nil {
			n, err := Number(*raw)
			if err != nil || n.Sign() < 0 {
				return fmt.Errorf("EXECUTION_TOKEN_CONDITION_INVALID")
			}
		}
	}
	if c.ArrivalRate != nil {
		n, e := Number(*c.ArrivalRate)
		if e != nil || n.Sign() <= 0 {
			return fmt.Errorf("EXECUTION_ARRIVAL_RATE_INVALID")
		}
	}
	return nil
}
func (s Set) Validate(ctx context.Context) error {
	if !In(s.Version, Version, DistributionVersion) || !In(s.Kind, "benchmark", "jvm_crash") || !Digest(s.Source.SHA) || !Known(s.Source.Format) || !Known(s.Source.Producer) || !Known(s.Source.Version) || len(s.Metrics) > MaxMetrics || len(s.Samples) > MaxSamples || s.IgnoredMetrics < 0 || len(s.Limitations) > 32 {
		return fmt.Errorf("EXECUTION_MODEL_INVALID_OR_BUDGET")
	}
	for _, v := range s.Limitations {
		if !Text(v) {
			return fmt.Errorf("EXECUTION_LIMITATION_INVALID")
		}
	}
	if s.Kind == "benchmark" {
		if s.Context == nil || s.Crash != nil {
			return fmt.Errorf("EXECUTION_BENCHMARK_CONTEXT_REQUIRED")
		}
		if e := s.Context.Validate(); e != nil {
			return e
		}
	} else {
		if s.Context != nil || s.Crash == nil || len(s.Metrics) != 0 || len(s.Samples) != 0 {
			return fmt.Errorf("EXECUTION_CRASH_CONTRACT")
		}
		c := s.Crash
		if !In(c.Signal, "SIGSEGV", "SIGBUS", "SIGILL", "SIGFPE", "SIGABRT", "EXCEPTION_ACCESS_VIOLATION", "INTERNAL_ERROR", "unknown") || !Uint(c.PID) || !Uint(c.TID) || !Text(c.Runtime) || !In(c.Completeness, "end_marker_present", "partial_or_unknown") || len(c.Frames) > 128 {
			return fmt.Errorf("EXECUTION_CRASH_INVALID")
		}
		if c.Elapsed != nil {
			n, e := Number(*c.Elapsed)
			if e != nil || n.Sign() < 0 {
				return fmt.Errorf("EXECUTION_CRASH_TIME_INVALID")
			}
		}
		for _, f := range c.Frames {
			if !In(f.Kind, "C", "J", "j", "V", "v") || !Text(f.Module) || !Text(f.Symbol) || !Known(f.Position) {
				return fmt.Errorf("EXECUTION_CRASH_FRAME_INVALID")
			}
		}
	}
	ids := map[string]bool{}
	for _, v := range s.Metrics {
		if e := ctx.Err(); e != nil {
			return e
		}
		if v.ID != MetricID(v) || ids[v.ID] || !In(v.Name, "requests", "errors", "samples", "sample_errors", "sample_latency", "request_latency", "first_byte", "connect", "ttft", "itl", "input_tokens", "output_tokens", "request_throughput", "output_throughput", "per_request_output_throughput", "error_ratio", "data_received", "data_sent", "observed_duration") || !In(v.Statistic, "count", "sum", "min", "max", "mean", "p25", "p50", "p75", "p90", "p95", "p99", "rate", "stddev", "value") || !In(v.Unit, "seconds", "count", "tokens", "ratio", "bytes", "requests/second", "requests/minute", "tokens/second", "tokens/second/request") || !In(v.Population, "all_requests", "successful_requests", "failed_requests", "started_requests", "all_samples", "failed_samples", "unknown") || !Digest(v.GroupSHA) || !Known(v.Policy) || !Known(v.Position) || !Text(v.OriginalUnit) || !Uint(v.Samples) {
			return fmt.Errorf("EXECUTION_METRIC_INVALID")
		}
		ids[v.ID] = true
		if !metricContract(v) {
			return fmt.Errorf("EXECUTION_PHYSICAL_CONTRACT_INVALID")
		}
		for _, raw := range []*string{v.Value, v.Original} {
			if raw != nil {
				n, e := Number(*raw)
				if e != nil || n.Sign() < 0 {
					return fmt.Errorf("EXECUTION_METRIC_NUMBER_INVALID")
				}
			}
		}
		if v.Original != nil {
			if v.Value == nil {
				return fmt.Errorf("EXECUTION_NORMALIZATION_VALUE_MISSING")
			}
			original, err := Number(*v.Original)
			if err != nil {
				return err
			}
			value, err := Number(*v.Value)
			if err != nil {
				return err
			}
			switch {
			case v.OriginalUnit == v.Unit:
			case v.OriginalUnit == "milliseconds" && v.Unit == "seconds":
				original.Quo(original, big.NewRat(1000, 1))
			default:
				return fmt.Errorf("EXECUTION_UNIT_CONVERSION_UNDECLARED")
			}
			if original.Cmp(value) != 0 {
				return fmt.Errorf("EXECUTION_UNIT_CONVERSION_MISMATCH")
			}
		}
		if v.Unit == "ratio" && v.Value != nil {
			n, _ := Number(*v.Value)
			if n.Cmp(big.NewRat(1, 1)) > 0 {
				return fmt.Errorf("EXECUTION_RATIO_INVALID")
			}
		}
	}
	ids = map[string]bool{}
	for _, v := range s.Samples {
		if e := ctx.Err(); e != nil {
			return e
		}
		if v.ID != SampleID(v) || ids[v.ID] || !Digest(v.GroupSHA) || !Uint(v.Start) || !Uint(v.Concurrency) || !Known(v.Position) || !Text(v.ResponseCode) {
			return fmt.Errorf("EXECUTION_SAMPLE_INVALID")
		}
		for _, raw := range []*string{v.Latency, v.FirstByte, v.Connect} {
			if raw != nil {
				n, e := Number(*raw)
				if e != nil || n.Sign() < 0 {
					return fmt.Errorf("EXECUTION_SAMPLE_DURATION_INVALID")
				}
			}
		}
		ids[v.ID] = true
	}
	if err := s.validateDistributions(ctx); err != nil {
		return err
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(raw) > MaxBytes {
		return fmt.Errorf("EXECUTION_MODEL_BYTES_BUDGET: bytes=%d limit=%d; reduce original run scope; split-file percentiles cannot be combined", len(raw), MaxBytes)
	}
	return ctx.Err()
}

func metricContract(v Metric) bool {
	switch v.Name {
	case "requests", "errors", "samples", "sample_errors":
		if v.Unit != "count" || !In(v.Statistic, "count", "sum") {
			return false
		}
	case "observed_duration":
		if v.Unit != "seconds" || v.Statistic != "value" {
			return false
		}
	case "request_latency", "sample_latency", "first_byte", "connect", "ttft", "itl":
		if v.Unit != "seconds" || In(v.Statistic, "count", "rate", "value") {
			return false
		}
	case "input_tokens", "output_tokens":
		if v.Unit != "tokens" {
			return false
		}
	case "error_ratio":
		if v.Unit != "ratio" {
			return false
		}
	case "request_throughput":
		if !In(v.Unit, "requests/second", "requests/minute") {
			return false
		}
	case "output_throughput":
		if v.Unit != "tokens/second" {
			return false
		}
	case "per_request_output_throughput":
		if v.Unit != "tokens/second/request" {
			return false
		}
	case "data_received", "data_sent":
		if v.Unit != "bytes" {
			return false
		}
	default:
		return false
	}
	if v.Value != nil && In(v.Statistic, "count", "sum") && In(v.Unit, "count", "tokens", "bytes") {
		n, err := Number(*v.Value)
		if err != nil || !n.IsInt() {
			return false
		}
	}
	return true
}
