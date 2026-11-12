//go:build ignore

// Generates project fixture expectations; the input directory is explicit.
package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"

	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/inference"
	history "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
)

func main() {
	base := os.Args[1]
	ctx := context.Background()
	read := func(name string) []byte {
		raw, e := os.ReadFile(filepath.Join(base, name))
		if e != nil {
			panic(e)
		}
		return raw
	}
	write := func(name string, v any) {
		raw, e := json.MarshalIndent(v, "", "  ")
		if e != nil {
			panic(e)
		}
		if e = os.WriteFile(filepath.Join(base, name), append(raw, '\n'), 0600); e != nil {
			panic(e)
		}
	}
	var capture p.Capture
	if e := n.Strict(read("nvidia-capture.json"), &capture); e != nil {
		panic(e)
	}
	s, e := p.ParseBytes(ctx, read("nvidia-query.csv"), "nvidia-query.csv", p.Options{Format: "nvidia-smi-csv", Capture: &capture})
	if e != nil {
		panic(e)
	}
	write("nvidia-set.json", s)
	metrics, e := history.ParseBytes(ctx, read("serving-snapshots.json"), "serving-snapshots.json", history.Options{Format: "snapshot-sequence-json", Producer: "synthetic explicit serving collector", ProducerVersion: "1.0.0"})
	if e != nil {
		panic(e)
	}
	s.Metrics.Sources = append(s.Metrics.Sources, metrics.Sources...)
	s.Metrics.Series = append(s.Metrics.Series, metrics.Series...)
	s.Metrics.Points = append(s.Metrics.Points, metrics.Points...)
	s.Metrics.Warnings = append(s.Metrics.Warnings, metrics.Warnings...)
	device := s.Entities[0]
	mig := n.Entity{ID: "synthetic-mig", Kind: "mig", Host: device.Host, Boot: device.Boot, Vendor: "nvidia", DeviceUUID: device.DeviceUUID, MIGUUID: "MIG-synthetic-1"}
	process := n.Entity{ID: "synthetic-process", Kind: "process", Host: device.Host, Boot: device.Boot, PID: "123", ProcessStart: m.Pointer("0"), Namespace: "synthetic-ns", Container: "synthetic-container", PodUID: "synthetic-pod"}
	worker := n.Entity{ID: "synthetic-worker-0", Kind: "worker", Host: device.Host, Boot: device.Boot, Rank: "0"}
	other := worker
	other.ID = "synthetic-worker-1"
	other.Rank = "1"
	s.Entities = append(s.Entities, mig, process, worker, other)
	s.Contexts = []n.Context{{ID: "synthetic-context", Engine: "vllm", Version: "unselected; synthetic 0.8.5-name example only", Model: "synthetic-model", Revision: "synthetic-revision-a", DType: "bfloat16", Quantization: "none", Tokenizer: "synthetic-tokenizer-a", TP: m.Pointer("2"), PP: m.Pointer("1"), DP: m.Pointer("1"), ContextLength: m.Pointer("4096"), MaxSequences: m.Pointer("8"), MaxBatchedTokens: m.Pointer("8192"), Replicas: m.Pointer("1"), Cache: "synthetic-declared-cache", Speculative: "disabled", Adapter: "none", Workload: "synthetic-workload-a", Warmup: "separate"}}
	for _, series := range metrics.Series {
		role, form, boundary, entity := "", "gauge", "server", worker.ID
		switch series.Name {
		case "vllm:num_requests_waiting":
			role = "queue_requests"
		case "vllm:num_requests_running":
			role = "running_requests"
		case "vllm:gpu_cache_usage_perc":
			role = "kv_cache_usage"
		case "vllm:generation_tokens_total":
			role = "output_tokens"
			form = "counter"
		case "vllm:time_to_first_token_seconds":
			role = "ttft"
			form = "histogram"
		case "vllm:time_per_output_token_seconds":
			role = "itl"
			form = "histogram"
		case "vllm:request_queue_time_seconds":
			role = "queue_time"
			form = "histogram"
		case "synthetic:mig_used_bytes":
			role = "device_memory_used"
			entity = mig.ID
			boundary = "device"
		case "framework_allocated":
			role = "framework_allocated"
			entity = process.ID
			boundary = "process"
		case "framework_reserved":
			role = "framework_reserved"
			entity = process.ID
			boundary = "process"
		}
		if role != "" {
			s.Signals = append(s.Signals, n.Signal{ID: "signal-" + role, SeriesID: series.ID, Semantic: role, EntityID: entity, ContextID: "synthetic-context", Boundary: boundary, Population: "synthetic-worker-a-declared", Form: form, Conversion: "identity"})
		}
	}
	s.Links = []n.Link{{From: mig.ID, To: device.ID, Kind: "mig_parent", Ref: m.Ref{SourceID: "mapping-source", Position: "line:1"}}, {From: worker.ID, To: mig.ID, Kind: "uses_device", Ref: m.Ref{SourceID: "mapping-source", Position: "line:2"}}, {From: worker.ID, To: process.ID, Kind: "hosted_by", Ref: m.Ref{SourceID: "mapping-source", Position: "line:3"}}, {From: other.ID, To: device.ID, Kind: "uses_device", Ref: m.Ref{SourceID: "mapping-source", Position: "line:4"}}}
	mapping := []byte{}
	for _, link := range s.Links {
		body, _ := json.Marshal(link)
		mapping = append(mapping, body...)
		mapping = append(mapping, '\n')
	}
	if e = os.WriteFile(filepath.Join(base, "mapping-observations.jsonl"), mapping, 0600); e != nil {
		panic(e)
	}
	s.Sources = []m.Source{{ID: "mapping-source", SHA256: p.Digest(mapping), Name: "mapping-observations.jsonl", Format: "explicit-mapping-jsonl.v1", Producer: "synthetic collector", ProducerVersion: "1.0.0"}}
	for i, kind := range []string{"engine_start", "warmup_start", "warmup_end", "cuda_oom", "framework_oom", "container_oom", "host_oom", "driver_xid", "device_reset", "engine_restart", "recovery"} {
		entity := worker.ID
		if kind == "driver_xid" || kind == "device_reset" {
			entity = device.ID
		}
		s.Events = append(s.Events, n.Event{Kind: kind, EntityID: entity, ContextID: "synthetic-context", Ref: m.Ref{SourceID: "runtime-source", Position: "line:" + strconv.Itoa(i+1)}})
		s.Events[len(s.Events)-1].Time = m.Pointer(strconv.FormatInt(int64(i+1)*1000000000, 10))
		if kind == "driver_xid" {
			s.Events[len(s.Events)-1].Code = "79"
		}
	}
	for i, count := range []string{"3", "1", "0"} {
		v := n.Event{Kind: "request", EntityID: worker.ID, ContextID: "synthetic-context", Time: m.Pointer("5000000000"), RequestID: "synthetic-request-" + count, Boundary: "server", Outcome: "success", Population: "synthetic-warm", RequestStart: m.Pointer("1000000000"), FirstToken: m.Pointer("1200000000"), LastToken: m.Pointer("1800000000"), OutputTokens: m.Pointer(count), Ref: m.Ref{SourceID: "runtime-source", Position: "line:" + strconv.Itoa(12+i)}}
		if count == "0" {
			v.Outcome = "cancelled"
			v.FirstToken = nil
			v.LastToken = nil
		}
		s.Events = append(s.Events, v)
	}
	runtime := []byte{}
	for i := range s.Events {
		s.Events[i].ID = n.EventID(s.Events[i])
		body, _ := json.Marshal(s.Events[i])
		runtime = append(runtime, body...)
		runtime = append(runtime, '\n')
	}
	if e = os.WriteFile(filepath.Join(base, "runtime-observations.jsonl"), runtime, 0600); e != nil {
		panic(e)
	}
	s.Sources = append(s.Sources, m.Source{ID: "runtime-source", SHA256: p.Digest(runtime), Name: "runtime-observations.jsonl", Format: "explicit-runtime-jsonl.v1", Producer: "synthetic collector", ProducerVersion: "1.0.0"})
	if e = s.Validate(ctx); e != nil {
		panic(e)
	}
	write("metric-model.json", s.Metrics)
	write("inference-evidence-set.json", s)
	raw, _ := json.Marshal(s)
	var binding map[string]json.RawMessage
	json.Unmarshal(raw, &binding)
	delete(binding, "metrics")
	write("bindings.json", binding)
}
