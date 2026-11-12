package execution

import (
	"context"
	"encoding/json"
	"fmt"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"sort"
	"strings"
)

func parseK6(ctx context.Context, raw []byte, s *e.Set) error {
	if s.Context.Kind != "http" {
		return fmt.Errorf("EXECUTION_K6_HTTP_CONTEXT_REQUIRED")
	}
	root, err := object(raw)
	if err != nil {
		return err
	}
	if state, ok := root["state"]; ok {
		obj, err := object(state)
		if err != nil {
			return err
		}
		if v, ok := obj["testRunDurationMs"]; ok {
			if err = metric(s, "observed_duration", "value", "seconds", "all_requests", "k6-1.3.0-source-summary", "/state/testRunDurationMs", v, 1000); err != nil {
				return err
			}
		}
	}
	metrics, err := object(root["metrics"])
	if err != nil {
		return fmt.Errorf("EXECUTION_K6_METRICS_REQUIRED")
	}
	names := []string{}
	for k := range metrics {
		names = append(names, k)
	}
	sort.Strings(names)
	defs := map[string]struct {
		name, unit, kind, contains, pop string
		scale                           int64
	}{"http_reqs": {"requests", "count", "counter", "default", "all_requests", 1}, "http_req_failed": {"error_ratio", "ratio", "rate", "default", "all_requests", 1}, "http_req_duration": {"request_latency", "seconds", "trend", "time", "all_requests", 1000}, "http_req_waiting": {"first_byte", "seconds", "trend", "time", "all_requests", 1000}, "http_req_connecting": {"connect", "seconds", "trend", "time", "all_requests", 1000}, "data_received": {"data_received", "bytes", "counter", "data", "all_requests", 1}, "data_sent": {"data_sent", "bytes", "counter", "data", "all_requests", 1}}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		def, ok := defs[name]
		if !ok {
			s.IgnoredMetrics++
			continue
		}
		obj, err := object(metrics[name])
		if err != nil {
			return err
		}
		var kind, contains string
		json.Unmarshal(obj["type"], &kind)
		json.Unmarshal(obj["contains"], &contains)
		if kind != def.kind || contains != def.contains {
			return fmt.Errorf("EXECUTION_K6_METRIC_SHAPE_MISMATCH")
		}
		values, err := object(obj["values"])
		if err != nil {
			return err
		}
		stats := map[string]string{"avg": "mean", "min": "min", "max": "max", "med": "p50", "p(90)": "p90", "p(95)": "p95", "p(99)": "p99"}
		keys := []string{}
		if kind == "trend" {
			for k := range stats {
				keys = append(keys, k)
			}
			sort.Strings(keys)
		} else if kind == "rate" {
			keys = []string{"rate"}
		} else {
			keys = []string{"count", "rate"}
		}
		for _, key := range keys {
			v, ok := values[key]
			if !ok {
				continue
			}
			stat := stats[key]
			if stat == "" {
				stat = key
			}
			metricName, unit := def.name, def.unit
			if key == "rate" && name == "http_reqs" {
				metricName = "request_throughput"
				unit = "requests/second"
			}
			if key == "rate" && name != "http_reqs" && kind == "counter" {
				continue
			}
			if err = metric(s, metricName, stat, unit, def.pop, "k6-1.3.0-source-summary", "/metrics/"+name+"/values/"+key, v, def.scale); err != nil {
				return err
			}
		}
	}
	if len(s.Metrics) == 0 {
		return fmt.Errorf("EXECUTION_K6_NO_SUPPORTED_METRICS")
	}
	return nil
}
func parseLLMPerf(ctx context.Context, raw []byte, s *e.Set) error {
	if s.Context.Kind != "llm" {
		return fmt.Errorf("EXECUTION_LLMPERF_LLM_CONTEXT_REQUIRED")
	}
	root, err := object(raw)
	if err != nil {
		return err
	}
	var version string
	json.Unmarshal(root["version"], &version)
	if version != "2023-08-31" {
		return fmt.Errorf("EXECUTION_LLMPERF_RESULTS_VERSION")
	}
	if rawConcurrency, ok := root["num_concurrent_requests"]; ok {
		v, err := value(rawConcurrency)
		if err != nil || v == nil || !e.Uint(v) {
			return fmt.Errorf("EXECUTION_LLMPERF_CONCURRENCY_INVALID")
		}
		if s.Context.Concurrency != nil && *s.Context.Concurrency != *v {
			return fmt.Errorf("EXECUTION_LLMPERF_CONCURRENCY_CONFLICT")
		}
		s.Context.Concurrency = v
	}
	if s.Context.Arrival != "closed" && s.Context.Arrival != "unknown" {
		return fmt.Errorf("EXECUTION_LLMPERF_ARRIVAL_MODEL_CONFLICT")
	}
	if s.Context.ArrivalRate != nil {
		return fmt.Errorf("EXECUTION_LLMPERF_ARRIVAL_RATE_UNSUPPORTED")
	}
	s.Context.Arrival = "closed"
	for _, binding := range []struct {
		key    string
		target **string
	}{{"mean_input_tokens", &s.Context.InputMean}, {"stddev_input_tokens", &s.Context.InputStddev}, {"mean_output_tokens", &s.Context.OutputMean}, {"stddev_output_tokens", &s.Context.OutputStddev}} {
		if raw, ok := root[binding.key]; ok {
			v, err := value(raw)
			if err != nil {
				return err
			}
			if *binding.target != nil && v != nil {
				a, _ := e.Number(**binding.target)
				b, _ := e.Number(*v)
				if a == nil || a.Cmp(b) != 0 {
					return fmt.Errorf("EXECUTION_LLMPERF_TOKEN_CONDITION_CONFLICT")
				}
			}
			*binding.target = v
		}
	}
	var sourceModel string
	json.Unmarshal(root["model"], &sourceModel)
	if e.Known(s.Context.Model) && s.Context.Model != sourceModel {
		return fmt.Errorf("EXECUTION_LLMPERF_MODEL_CONFLICT")
	}
	defs := []struct{ source, name, unit, pop string }{{"inter_token_latency_s", "itl", "seconds", "successful_requests"}, {"ttft_s", "ttft", "seconds", "successful_requests"}, {"end_to_end_latency_s", "request_latency", "seconds", "successful_requests"}, {"request_output_throughput_token_per_s", "per_request_output_throughput", "tokens/second/request", "successful_requests"}, {"number_input_tokens", "input_tokens", "tokens", "successful_requests"}, {"number_output_tokens", "output_tokens", "tokens", "successful_requests"}}
	recognized := map[string]bool{}
	for _, def := range defs {
		for _, stat := range []string{"mean", "min", "max", "stddev", "p25", "p50", "p75", "p90", "p95", "p99"} {
			if err := ctx.Err(); err != nil {
				return err
			}
			key := "results_" + def.source + "_" + stat
			if strings.HasPrefix(stat, "p") {
				key = "results_" + def.source + "_quantiles_" + stat
			}
			v, ok := root[key]
			if !ok {
				continue
			}
			recognized[key] = true
			if err = metric(s, def.name, stat, def.unit, def.pop, "llmperf-"+LLMPerfRevision+"-source-summary", "/"+key, v, 1); err != nil {
				return err
			}
		}
	}
	for _, def := range []struct{ key, name, stat, unit, pop string }{{"num_requests_started", "requests", "count", "count", "started_requests"}, {"num_completed_requests", "requests", "count", "count", "successful_requests"}, {"number_errors", "errors", "count", "count", "failed_requests"}, {"error_rate", "error_ratio", "mean", "ratio", "all_requests"}, {"mean_output_throughput_token_per_s", "output_throughput", "rate", "tokens/second", "successful_requests"}, {"num_completed_requests_per_min", "request_throughput", "rate", "requests/minute", "successful_requests"}} {
		key := "results_" + def.key
		if v, ok := root[key]; ok {
			recognized[key] = true
			if err = metric(s, def.name, def.stat, def.unit, def.pop, "llmperf-"+LLMPerfRevision+"-source-summary", "/"+key, v, 1); err != nil {
				return err
			}
		}
	}
	for key := range root {
		if strings.HasPrefix(key, "results_") && !recognized[key] {
			s.IgnoredMetrics++
		}
	}
	s.Limitations = append(s.Limitations, "LLMPerf ITL and per-request throughput preserve the pinned producer definitions; ITL is not relabeled TPOT. Error messages/code-frequency strings are excluded.")
	if len(s.Metrics) == 0 {
		return fmt.Errorf("EXECUTION_LLMPERF_NO_SUPPORTED_METRICS")
	}
	return nil
}
