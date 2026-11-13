// Package inferencemodel binds device/serving semantics to original metric series.
// It owns no collector, vendor alias table, diagnostic threshold or trace engine.
package inferencemodel

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
	"strings"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

const Version = "1.1.0"
const LegacyVersion = "1.0.0"
const MaxBytes = 32 << 20
const MaxEntities = 1024
const MaxSignals = 2048
const MaxEvents = 20000

type Entity struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"` // device, mig, process, worker, replica
	Host         string  `json:"host_id"`
	Boot         string  `json:"boot_id"`
	Vendor       string  `json:"vendor,omitempty"`
	DeviceUUID   string  `json:"device_uuid,omitempty"`
	MIGUUID      string  `json:"mig_uuid,omitempty"`
	LocalIndex   string  `json:"local_index,omitempty"` // display only
	PCI          string  `json:"pci_bus_id,omitempty"`
	PID          string  `json:"pid,omitempty"`
	ProcessStart *string `json:"process_start_ns,omitempty"`
	Namespace    string  `json:"pid_namespace,omitempty"`
	Container    string  `json:"container_id,omitempty"`
	PodUID       string  `json:"pod_uid,omitempty"`
	Rank         string  `json:"rank,omitempty"`
}
type Link struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // uses_device, mig_parent, hosted_by
	Ref  m.Ref  `json:"ref"`
}
type Context struct {
	ID               string  `json:"id"`
	Engine           string  `json:"engine"`
	Version          string  `json:"engine_version"`
	Model            string  `json:"model"`
	Revision         string  `json:"model_revision"`
	DType            string  `json:"dtype"`
	Quantization     string  `json:"quantization"`
	Tokenizer        string  `json:"tokenizer_revision"`
	TP               *string `json:"tensor_parallel"`
	PP               *string `json:"pipeline_parallel"`
	DP               *string `json:"data_parallel"`
	ContextLength    *string `json:"context_length"`
	MaxSequences     *string `json:"max_sequences"`
	MaxBatchedTokens *string `json:"max_batched_tokens"`
	Replicas         *string `json:"replicas"`
	Cache            string  `json:"cache_configuration"`
	Speculative      string  `json:"speculative_configuration"`
	Adapter          string  `json:"adapter_revision"`
	Workload         string  `json:"workload_id"`
	Warmup           string  `json:"warmup"` // unknown, included, excluded, separate
}
type Signal struct {
	Declaration       *Declaration `json:"declared_metadata,omitempty"`
	ID                string       `json:"id"`
	SeriesID          string       `json:"series_id"`
	Semantic          string       `json:"semantic"`
	EntityID          string       `json:"entity_id"`
	ContextID         string       `json:"context_id,omitempty"`
	Boundary          string       `json:"boundary"`
	Population        string       `json:"population"`
	Form              string       `json:"statistical_form"` // gauge, counter, histogram, summary_quantile, precomputed_rate
	SourceCalculation string       `json:"source_calculation,omitempty"`
	Conversion        string       `json:"conversion"` // identity, mib_to_bytes, milliseconds_to_seconds, percent_to_ratio
}

// Sources describe structured runtime/error/request observations, not copied log bodies.
type Event struct {
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	EntityID     string  `json:"entity_id"`
	ContextID    string  `json:"context_id,omitempty"`
	Time         *string `json:"time_ns"`
	Code         string  `json:"code,omitempty"`
	RequestID    string  `json:"request_id,omitempty"`
	Boundary     string  `json:"boundary,omitempty"`
	Outcome      string  `json:"outcome,omitempty"`
	Population   string  `json:"population,omitempty"`
	RequestStart *string `json:"request_start_ns,omitempty"`
	FirstToken   *string `json:"first_token_ns,omitempty"`
	LastToken    *string `json:"last_token_ns,omitempty"`
	OutputTokens *string `json:"output_tokens,omitempty"`
	Ref          m.Ref   `json:"ref"`
}
type Quality struct {
	PointID string `json:"point_id"`
	Status  string `json:"status"`
	Raw     string `json:"raw"`
}
type Set struct {
	Version  string     `json:"schema_version"`
	Metrics  m.Set      `json:"metrics"`
	Entities []Entity   `json:"entities"`
	Links    []Link     `json:"links"`
	Contexts []Context  `json:"contexts"`
	Signals  []Signal   `json:"signals"`
	Sources  []m.Source `json:"event_sources"`
	Events   []Event    `json:"events"`
	Quality  []Quality  `json:"quality"`
}

func Strict(raw []byte, out any, limits ...int) error {
	limit := MaxBytes
	if len(limits) > 0 {
		limit = limits[0]
	}
	if len(raw) > limit {
		return fmt.Errorf("INFERENCE_BUDGET: JSON exceeds %d bytes", limit)
	}
	return wirejson.StrictWithLimits(raw, out, wirejson.ResultLimits(int64(limit), 128))
}
func Decode(ctx context.Context, raw []byte) (Set, error) {
	var s Set
	if e := Strict(raw, &s); e != nil {
		return s, e
	}
	return s, s.Validate(ctx)
}
func EventID(e Event) string { e.ID = ""; return "inference-event-" + m.Hash(e) }
func text(s string) bool     { return len(s) <= 1024 && !strings.ContainsAny(s, "\x00\r\n") }
func known(s string) bool    { return s != "" && s != "unknown" }
func (s Set) Validate(ctx context.Context) error {
	if s.Version != Version && s.Version != LegacyVersion {
		return fmt.Errorf("INFERENCE_VERSION_UNSUPPORTED")
	}
	if len(s.Entities) == 0 || len(s.Entities) > MaxEntities || len(s.Signals) > MaxSignals || len(s.Contexts) > 1024 || len(s.Links) > 4096 || len(s.Events) > MaxEvents || len(s.Sources) > 128 || len(s.Quality) > m.MaxPoints {
		return fmt.Errorf("INFERENCE_BUDGET: entities/signals/contexts/links/events/sources/quality")
	}
	if e := s.Metrics.Validate(ctx); e != nil {
		return e
	}
	entities := map[string]Entity{}
	identities := map[string]bool{}
	for _, v := range s.Entities {
		if !known(v.ID) || !text(v.ID) || entities[v.ID].ID != "" || !known(v.Host) || !known(v.Boot) {
			return fmt.Errorf("INFERENCE_ENTITY_INVALID: stable host/boot and unique ID required")
		}
		for _, x := range []string{v.Host, v.Boot, v.Vendor, v.DeviceUUID, v.MIGUUID, v.LocalIndex, v.PCI, v.PID, v.Namespace, v.Container, v.PodUID, v.Rank} {
			if !text(x) {
				return fmt.Errorf("INFERENCE_ENTITY_INVALID")
			}
		}
		identity := []string{v.Kind, v.Host, v.Boot}
		switch v.Kind {
		case "device":
			if !known(v.DeviceUUID) {
				return fmt.Errorf("INFERENCE_DEVICE_ID_REQUIRED: local index is not identity")
			}
			identity = append(identity, v.DeviceUUID)
		case "mig":
			if !known(v.DeviceUUID) || !known(v.MIGUUID) {
				return fmt.Errorf("INFERENCE_MIG_ID_REQUIRED")
			}
			identity = append(identity, v.DeviceUUID, v.MIGUUID)
		case "process":
			if !known(v.Namespace) || v.ProcessStart == nil {
				return fmt.Errorf("INFERENCE_PROCESS_LIFETIME_REQUIRED")
			}
			if _, e := m.Uint(v.PID); e != nil {
				return e
			}
			if _, ok := m.Time(v.ProcessStart); !ok {
				return fmt.Errorf("INFERENCE_PROCESS_TIME_INVALID")
			}
			identity = append(identity, v.Namespace, v.PID, *v.ProcessStart)
		case "worker", "replica":
			identity = append(identity, v.ID) // explicit collector identity; no name-based join
		default:
			return fmt.Errorf("INFERENCE_ENTITY_KIND_INVALID")
		}
		key := m.Hash(identity)
		if identities[key] {
			return fmt.Errorf("INFERENCE_DUPLICATE_ENTITY")
		}
		identities[key] = true
		entities[v.ID] = v
	}
	contexts := map[string]bool{}
	for _, v := range s.Contexts {
		if !known(v.ID) || contexts[v.ID] {
			return fmt.Errorf("INFERENCE_CONTEXT_INVALID")
		}
		contexts[v.ID] = true
		for _, x := range []string{v.ID, v.Engine, v.Version, v.Model, v.Revision, v.DType, v.Quantization, v.Tokenizer, v.Cache, v.Speculative, v.Adapter, v.Workload} {
			if !text(x) {
				return fmt.Errorf("INFERENCE_CONTEXT_INVALID")
			}
		}
		for _, x := range []*string{v.TP, v.PP, v.DP, v.ContextLength, v.MaxSequences, v.MaxBatchedTokens, v.Replicas} {
			if x != nil {
				n, e := m.Uint(*x)
				if e != nil || n.Sign() == 0 {
					return fmt.Errorf("INFERENCE_CONTEXT_NUMBER_INVALID")
				}
			}
		}
		if v.Warmup != "unknown" && v.Warmup != "included" && v.Warmup != "excluded" && v.Warmup != "separate" {
			return fmt.Errorf("INFERENCE_WARMUP_INVALID")
		}
	}
	series := map[string]m.Series{}
	for _, v := range s.Metrics.Series {
		series[v.ID] = v
	}
	used := map[string]bool{}
	signals := map[string]bool{}
	for _, v := range s.Signals {
		if v.Declaration != nil && s.Version == LegacyVersion {
			return fmt.Errorf("INFERENCE_DECLARATION_VERSION_REQUIRED")
		}
		if !known(v.ID) || signals[v.ID] || series[v.SeriesID].ID == "" || used[v.SeriesID] || entities[v.EntityID].ID == "" || (v.ContextID != "" && !contexts[v.ContextID]) || !known(v.Population) || !text(v.Population) || !text(v.ID) || !text(v.SourceCalculation) {
			return fmt.Errorf("INFERENCE_SIGNAL_INVALID: one explicit meaning per series; no duplicated device/pod attribution")
		}
		if _, e := Definition(v, series[v.SeriesID], entities[v.EntityID]); e != nil {
			return fmt.Errorf("INFERENCE_SIGNAL_DEFINITION: signal=%s series=%s original_kind=%s original_unit=%q: %w; declare missing range metadata explicitly", v.ID, series[v.SeriesID].Name, series[v.SeriesID].Kind, series[v.SeriesID].Unit, e)
		}
		used[v.SeriesID] = true
		signals[v.ID] = true
	}
	// Unmapped original series remain browsable but are never summed or aliased.
	sources := map[string]m.Source{}
	for _, v := range s.Metrics.Sources {
		sources[v.ID] = v
	}
	for _, v := range s.Sources {
		if sources[v.ID].ID != "" {
			return fmt.Errorf("INFERENCE_SOURCE_DUPLICATE")
		}
		probe := m.Set{Version: m.Version, Normalizer: m.Normalizer, Sources: []m.Source{v}, Series: []m.Series{}, Points: []m.Point{}, Warnings: []string{}}
		if e := probe.Validate(ctx); e != nil {
			return e
		}
		sources[v.ID] = v
	}
	refOK := func(ref m.Ref) bool {
		return sources[ref.SourceID].ID != "" && known(ref.Position) && text(ref.Position)
	}
	parents := map[string]bool{}
	links := map[string]bool{}
	for _, v := range s.Links {
		from, to := entities[v.From], entities[v.To]
		key := m.Hash(v)
		if from.ID == "" || to.ID == "" || v.From == v.To || from.Host != to.Host || from.Boot != to.Boot || !refOK(v.Ref) || links[key] {
			return fmt.Errorf("INFERENCE_LINK_INVALID")
		}
		links[key] = true
		switch v.Kind {
		case "mig_parent":
			if from.Kind != "mig" || to.Kind != "device" || from.DeviceUUID != to.DeviceUUID || parents[from.ID] {
				return fmt.Errorf("INFERENCE_MIG_PARENT_INVALID")
			}
			parents[from.ID] = true
		case "uses_device":
			if (from.Kind != "worker" && from.Kind != "process" && from.Kind != "replica") || (to.Kind != "device" && to.Kind != "mig") {
				return fmt.Errorf("INFERENCE_DEVICE_LINK_INVALID")
			}
		case "hosted_by":
			if from.Kind != "worker" || to.Kind != "process" {
				return fmt.Errorf("INFERENCE_PROCESS_LINK_INVALID")
			}
		default:
			return fmt.Errorf("INFERENCE_LINK_KIND_INVALID")
		}
	}
	for _, v := range s.Entities {
		if v.Kind == "mig" && !parents[v.ID] {
			return fmt.Errorf("INFERENCE_MIG_PARENT_REQUIRED")
		}
	}
	events := map[string]bool{}
	for _, v := range s.Events {
		if e := ctx.Err(); e != nil {
			return e
		}
		if v.ID != EventID(v) || events[v.ID] || entities[v.EntityID].ID == "" || !refOK(v.Ref) || (v.ContextID != "" && !contexts[v.ContextID]) {
			return fmt.Errorf("INFERENCE_EVENT_INVALID")
		}
		events[v.ID] = true
		for _, x := range []string{v.Code, v.RequestID, v.Population} {
			if !text(x) {
				return fmt.Errorf("INFERENCE_EVENT_TEXT_INVALID")
			}
		}
		if v.Time != nil {
			if _, ok := m.Time(v.Time); !ok {
				return fmt.Errorf("INFERENCE_EVENT_TIME_INVALID")
			}
		}
		switch v.Kind {
		case "engine_start", "engine_restart", "model_load", "model_replace", "warmup_start", "warmup_end", "recovery", "cuda_oom", "framework_oom", "container_oom", "host_oom", "driver_xid", "ecc_error", "device_reset", "request":
		default:
			return fmt.Errorf("INFERENCE_EVENT_KIND_INVALID")
		}
		if v.Kind == "request" {
			if !known(v.RequestID) || !known(v.Population) || (v.Boundary != "server" && v.Boundary != "client" && v.Boundary != "gateway") || (v.Outcome != "success" && v.Outcome != "failed" && v.Outcome != "cancelled" && v.Outcome != "incomplete") {
				return fmt.Errorf("INFERENCE_REQUEST_INVALID")
			}
			for _, x := range []*string{v.RequestStart, v.FirstToken, v.LastToken} {
				if x != nil {
					if _, ok := m.Time(x); !ok {
						return fmt.Errorf("INFERENCE_REQUEST_TIME_INVALID")
					}
				}
			}
			if v.OutputTokens != nil {
				if _, e := m.Uint(*v.OutputTokens); e != nil {
					return e
				}
			}
		} else if v.RequestStart != nil || v.FirstToken != nil || v.LastToken != nil || v.OutputTokens != nil || v.RequestID != "" || v.Boundary != "" || v.Outcome != "" {
			return fmt.Errorf("INFERENCE_NONREQUEST_FIELDS")
		}
	}
	points := map[string]m.Point{}
	for _, v := range s.Metrics.Points {
		points[v.ID] = v
	}
	quality := map[string]bool{}
	for _, v := range s.Quality {
		point, ok := points[v.PointID]
		if !ok || quality[v.PointID] || point.Flags&1 == 0 || !text(v.Raw) || (v.Status != "not_available" && v.Status != "not_supported" && v.Status != "permission_denied") {
			return fmt.Errorf("INFERENCE_QUALITY_INVALID")
		}
		quality[v.PointID] = true
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(raw) > MaxBytes {
		return fmt.Errorf("INFERENCE_BUDGET: model exceeds %d bytes; select fewer original series/events", MaxBytes)
	}
	return ctx.Err()
}

type SemanticDefinition struct {
	Unit   string
	Factor *big.Rat
}

func Definition(v Signal, series m.Series, entity Entity) (SemanticDefinition, error) {
	out := SemanticDefinition{Factor: big.NewRat(1, 1)}
	effective, e := EffectiveSeries(v, series)
	if e != nil {
		return out, e
	}
	series = effective
	category := ""
	switch v.Semantic {
	case "gpu_utilization", "memory_io_utilization", "sm_activity", "tensor_activity", "dram_activity", "kv_cache_usage":
		category = "ratio"
		out.Unit = "ratio"
	case "device_memory_used", "device_memory_capacity", "framework_allocated", "framework_reserved", "pcie_bytes", "nvlink_bytes":
		category = "bytes"
		out.Unit = "bytes"
	case "temperature":
		category = "temperature"
		out.Unit = "celsius"
	case "power":
		category = "power"
		out.Unit = "watts"
	case "clock":
		category = "clock"
		out.Unit = "megahertz"
	case "queue_requests", "running_requests", "input_tokens", "output_tokens", "completed_requests", "preemptions", "ecc_count":
		category = "count"
		out.Unit = "count"
	case "ttft", "itl", "request_tpot", "request_latency", "queue_time", "prefill_time", "decode_time":
		category = "duration"
		out.Unit = "seconds"
	default:
		return out, fmt.Errorf("INFERENCE_SEMANTIC_UNSUPPORTED: %s", v.Semantic)
	}
	deviceRole := v.Semantic == "memory_io_utilization" || strings.HasPrefix(v.Semantic, "gpu_") || v.Semantic == "sm_activity" || v.Semantic == "tensor_activity" || v.Semantic == "dram_activity" || strings.HasPrefix(v.Semantic, "device_") || v.Semantic == "temperature" || v.Semantic == "power" || v.Semantic == "clock" || v.Semantic == "pcie_bytes" || v.Semantic == "nvlink_bytes" || v.Semantic == "ecc_count"
	if deviceRole {
		if (entity.Kind != "device" && entity.Kind != "mig") || v.Boundary != "device" {
			return out, fmt.Errorf("INFERENCE_SCOPE_INVALID: device telemetry cannot be attributed to a worker")
		}
	} else if v.Semantic == "framework_allocated" || v.Semantic == "framework_reserved" {
		if entity.Kind != "process" || v.Boundary != "process" {
			return out, fmt.Errorf("INFERENCE_SCOPE_INVALID: framework memory requires process lifetime")
		}
	} else {
		if (entity.Kind != "worker" && entity.Kind != "replica") || (v.Boundary != "server" && v.Boundary != "client" && v.Boundary != "gateway") {
			return out, fmt.Errorf("INFERENCE_SCOPE_INVALID: serving boundary required")
		}
	}
	switch v.Form {
	case "gauge", "summary_quantile", "precomputed_rate":
		if series.Kind != "gauge" {
			return out, fmt.Errorf("INFERENCE_FORM_MISMATCH")
		}
		if v.Form == "summary_quantile" && (category != "duration" || !known(v.SourceCalculation) || !text(v.SourceCalculation)) {
			return out, fmt.Errorf("INFERENCE_SUMMARY_PROVENANCE_REQUIRED")
		}
		if v.Form == "precomputed_rate" && (!known(v.SourceCalculation) || !text(v.SourceCalculation)) {
			return out, fmt.Errorf("INFERENCE_RATE_PROVENANCE_REQUIRED")
		}
	case "counter":
		if series.Kind != "sum" || !series.Monotonic || (category != "count" && category != "bytes") {
			return out, fmt.Errorf("INFERENCE_COUNTER_MISMATCH")
		}
	case "histogram":
		if (series.Kind != "histogram" && series.Kind != "exponential_histogram") || (category != "duration" && category != "count") {
			return out, fmt.Errorf("INFERENCE_HISTOGRAM_MISMATCH")
		}
	default:
		return out, fmt.Errorf("INFERENCE_FORM_UNSUPPORTED")
	}
	// Instantaneous resource/queue gauges cannot be reinterpreted as counters.
	switch v.Semantic {
	case "gpu_utilization", "memory_io_utilization", "sm_activity", "tensor_activity", "dram_activity", "kv_cache_usage", "device_memory_used", "device_memory_capacity", "framework_allocated", "framework_reserved", "temperature", "power", "clock", "queue_requests", "running_requests":
		if v.Form != "gauge" {
			return out, fmt.Errorf("INFERENCE_GAUGE_FORM_REQUIRED")
		}
	}
	switch v.Semantic {
	case "input_tokens", "output_tokens":
		out.Unit = "tokens"
	case "queue_requests", "running_requests", "completed_requests":
		out.Unit = "requests"
	}
	// Request gauges and duration aggregates retain their declared statistical form.
	unit := series.Unit
	if category == "count" && v.Form != "precomputed_rate" {
		allowed := []string{"1", "count"}
		switch v.Semantic {
		case "input_tokens", "output_tokens":
			allowed = append(allowed, "tokens", "{token}")
		case "queue_requests", "running_requests", "completed_requests":
			allowed = append(allowed, "requests", "{request}")
		default:
			allowed = append(allowed, "{event}", "{error}")
		}
		ok := false
		for _, x := range allowed {
			ok = ok || unit == x
		}
		if !ok {
			return out, fmt.Errorf("INFERENCE_COUNT_UNIT_MISMATCH")
		}
	}
	if v.Form == "precomputed_rate" {
		if category != "count" && category != "bytes" {
			return out, fmt.Errorf("INFERENCE_RATE_UNIT_INVALID")
		}
		if unit != "count/s" && unit != "tokens/s" && unit != "bytes/s" {
			return out, fmt.Errorf("INFERENCE_RATE_UNIT_INVALID")
		}
		if (category == "bytes") != (unit == "bytes/s") {
			return out, fmt.Errorf("INFERENCE_RATE_UNIT_INVALID")
		}
		out.Unit += "/second"
		if v.Conversion != "identity" {
			return out, fmt.Errorf("INFERENCE_RATE_CONVERSION_INVALID")
		}
		return out, nil
	}
	switch v.Conversion {
	case "identity":
		valid := map[string][]string{"ratio": {"1", "ratio"}, "bytes": {"By", "bytes"}, "duration": {"s", "seconds"}, "count": {"1", "count", "tokens", "requests", "{token}", "{request}", "{event}", "{error}"}, "temperature": {"Cel", "celsius"}, "power": {"W", "watts"}, "clock": {"MHz", "megahertz"}}
		ok := false
		for _, x := range valid[category] {
			ok = ok || unit == x
		}
		if !ok {
			return out, fmt.Errorf("INFERENCE_UNIT_MISMATCH: %s/%s", category, unit)
		}
	case "percent_to_ratio":
		if category != "ratio" || unit != "%" {
			return out, fmt.Errorf("INFERENCE_UNIT_MISMATCH")
		}
		out.Factor = big.NewRat(1, 100)
	case "mib_to_bytes":
		if category != "bytes" || unit != "MiB" {
			return out, fmt.Errorf("INFERENCE_UNIT_MISMATCH")
		}
		out.Factor = big.NewRat(1048576, 1)
	case "milliseconds_to_seconds":
		if category != "duration" || unit != "ms" {
			return out, fmt.Errorf("INFERENCE_UNIT_MISMATCH")
		}
		out.Factor = big.NewRat(1, 1000)
	default:
		return out, fmt.Errorf("INFERENCE_CONVERSION_UNSUPPORTED")
	}
	return out, nil
}
