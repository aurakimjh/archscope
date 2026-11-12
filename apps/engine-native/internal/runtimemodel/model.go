// Package runtimemodel stores file-only, scoped runtime observations. It never
// converts a task snapshot to a wait duration or a stack state to CPU use.
package runtimemodel

import (
	"context"
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"
)

const Version = "1.0.0"
const MaxBytes = 32 << 20
const MaxObservations = 50000
const MaxEntities = 4096
const MaxFrames = 50000

type Source struct {
	ID       string `json:"id"`
	SHA      string `json:"sha256"`
	Format   string `json:"format"`
	Producer string `json:"producer"`
	Version  string `json:"producer_version"`
	Clock    string `json:"clock_id"`
}
type Process struct {
	ID        string  `json:"id"`
	Host      string  `json:"host_id"`
	Boot      string  `json:"boot_id"`
	Namespace string  `json:"pid_namespace"`
	PID       string  `json:"pid"`
	Start     *string `json:"process_start_ns"`
	Runtime   string  `json:"runtime"`
}
type Entity struct {
	ID             string  `json:"id"`
	Process        string  `json:"process_id"`
	Kind           string  `json:"kind"`
	RuntimeID      string  `json:"runtime_id"`
	NativeID       string  `json:"native_id"`
	NativeEncoding string  `json:"native_encoding"`
	Start          *string `json:"thread_start_ns"`
	End            *string `json:"thread_end_ns"`
	Name           string  `json:"name"`
	Mapping        string  `json:"mapping_basis"`
}
type Ref struct {
	Source   string `json:"source_id"`
	Position string `json:"position"`
}
type Observation struct {
	Inferred       string  `json:"inferred_category,omitempty"`
	InferenceBasis string  `json:"inference_basis,omitempty"`
	ID             string  `json:"id"`
	Entity         string  `json:"entity_id"`
	Kind           string  `json:"kind"`
	Metric         string  `json:"metric"`
	Value          *string `json:"value"`
	Unit           string  `json:"unit"`
	Basis          string  `json:"measurement_basis"`
	State          string  `json:"observed_state"`
	Population     string  `json:"population"`
	Time           *string `json:"time_ns"`
	From           *string `json:"from_ns"`
	To             *string `json:"to_ns"`
	Stack          string  `json:"stack_id"`
	Ref            Ref     `json:"ref"`
}
type Relation struct {
	From string `json:"from_entity"`
	To   string `json:"to_entity"`
	Kind string `json:"kind"`
	Ref  Ref    `json:"ref"`
}
type ProfileRef struct {
	Entity      string `json:"entity_id"`
	ProfileID   string `json:"profile_id"`
	OriginalSHA string `json:"original_sha256"`
	ModelSHA    string `json:"model_sha256"`
	Clock       string `json:"clock_id"`
	Ref         Ref    `json:"ref"`
}
type PlanNode struct {
	ID            string  `json:"id"`
	Parent        string  `json:"parent_id"`
	NodeType      string  `json:"node_type"`
	EstimatedRows *string `json:"estimated_rows"`
	ActualRows    *string `json:"actual_rows_per_loop"`
	Loops         *string `json:"actual_loops"`
	TotalMS       *string `json:"actual_total_ms_per_loop"`
	Ref           Ref     `json:"ref"`
}
type Set struct {
	Version      string        `json:"schema_version"`
	Sources      []Source      `json:"sources"`
	Processes    []Process     `json:"processes"`
	Entities     []Entity      `json:"entities"`
	Frames       []p.Frame     `json:"frames"`
	Stacks       []p.Stack     `json:"stacks"`
	Observations []Observation `json:"observations"`
	Relations    []Relation    `json:"relations"`
	Profiles     []ProfileRef  `json:"profile_refs"`
	Plans        []PlanNode    `json:"plan_nodes"`
}

func Empty() Set {
	return Set{Version: Version, Sources: []Source{}, Processes: []Process{}, Entities: []Entity{}, Frames: []p.Frame{}, Stacks: []p.Stack{}, Observations: []Observation{}, Relations: []Relation{}, Profiles: []ProfileRef{}, Plans: []PlanNode{}}
}
func Hash(v any) string                  { return m.Hash(v) }
func ObservationID(v Observation) string { v.ID = ""; return "runtime-observation-" + Hash(v) }
func Text(s string) bool {
	return len(s) <= 1024 && utf8.ValidString(s) && !strings.ContainsAny(s, "\x00\r\n")
}
func In(s string, vs ...string) bool {
	for _, v := range vs {
		if s == v {
			return true
		}
	}
	return false
}
func Strict(raw []byte, out any, limits ...int) error {
	limit := MaxBytes
	if len(limits) > 0 {
		limit = min(limit, limits[0])
	}
	if len(raw) > limit {
		return fmt.Errorf("RUNTIME_BYTES_BUDGET: bytes=%d limit=%d; split original observations", len(raw), limit)
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

var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)

func Digest(s string) bool { return digest.MatchString(s) }
func uintp(s *string) bool {
	if s == nil {
		return true
	}
	_, e := m.Uint(*s)
	return e == nil
}
func Number(s *string) bool {
	if s == nil {
		return true
	}
	_, e := m.Number(*s)
	return e == nil
}
func known(s string) bool { return s != "" && s != "unknown" && Text(s) }

// Metric contracts are physical data definitions, not diagnostic thresholds.
func Contract(v Observation) bool {
	switch v.Kind {
	case "task_snapshot", "thread_snapshot", "worker_event", "handle_snapshot", "panic_snapshot":
		return v.Metric == "" && v.Value == nil && v.Unit == "" && v.Basis == "snapshot" && In(v.State, "pending", "running", "suspended", "cancelled", "done", "unknown", "RUNNABLE", "BLOCKED", "WAITING", "TIMED_WAITING", "NEW", "TERMINATED", "started", "stopped", "active", "inactive", "panicked")
	case "allocation":
		return (In(v.Metric, "live_bytes", "allocated_bytes") && v.Unit == "bytes" || v.Metric == "live_blocks" && v.Unit == "count") && v.Basis == "snapshot" || v.Metric == "allocation_diff_bytes" && v.Unit == "bytes" && v.Basis == "snapshot_difference"
	case "event_loop":
		return (In(v.Metric, "delay_mean_ns", "delay_p95_ns", "delay_max_ns") && v.Unit == "nanoseconds" && v.Basis == "distribution_statistic" || v.Metric == "utilization_ratio" && v.Unit == "ratio" && v.Basis == "interval")
	case "cpu":
		return v.Metric == "cpu_usage_percent" && v.Unit == "percent" && In(v.Basis, "interval_average", "lifetime_average")
	case "gc":
		return In(v.Metric, "pause_ns", "concurrent_ns") && v.Unit == "nanoseconds" && v.Basis == "observed_interval"
	case "access", "database":
		return In(v.Metric, "latency_ns", "upstream_latency_ns", "pool_wait_ns", "lock_wait_ns", "db_execution_ns") && v.Unit == "nanoseconds" && v.Basis == "observed_interval"
	default:
		return false
	}
}
func (s Set) Validate(ctx context.Context) error {
	if s.Version != Version {
		return fmt.Errorf("RUNTIME_VERSION_UNSUPPORTED")
	}
	if len(s.Sources) == 0 || len(s.Sources) > 128 || len(s.Processes) > 1024 || len(s.Entities) > MaxEntities || len(s.Frames) > MaxFrames || len(s.Stacks) > MaxFrames || len(s.Observations) > MaxObservations || len(s.Relations) > MaxObservations || len(s.Profiles) > 1024 || len(s.Plans) > 20000 {
		return fmt.Errorf("RUNTIME_RECORD_BUDGET: split sources; no silent truncation")
	}
	sources := map[string]Source{}
	for _, v := range s.Sources {
		if !known(v.ID) || sources[v.ID].ID != "" || !Digest(v.SHA) || !known(v.Format) || !known(v.Producer) || !known(v.Version) || !known(v.Clock) && v.Clock != "unknown" {
			return fmt.Errorf("RUNTIME_SOURCE_INVALID")
		}
		sources[v.ID] = v
	}
	ref := func(r Ref) bool { return sources[r.Source].ID != "" && known(r.Position) }
	processes := map[string]Process{}
	for _, v := range s.Processes {
		if !known(v.ID) || processes[v.ID].ID != "" || !Text(v.Host) || !Text(v.Boot) || !Text(v.Namespace) || !Text(v.Runtime) || !uintp(v.Start) {
			return fmt.Errorf("RUNTIME_PROCESS_INVALID")
		}
		if _, e := m.Uint(v.PID); e != nil {
			return e
		}
		processes[v.ID] = v
	}
	entities := map[string]Entity{}
	for _, v := range s.Entities {
		if !known(v.ID) || entities[v.ID].ID != "" || processes[v.Process].ID == "" || !In(v.Kind, "process", "os_thread", "runtime_thread", "worker", "async_task", "event_loop") || !Text(v.RuntimeID) || !Text(v.Name) || !uintp(v.Start) || !uintp(v.End) || !In(v.NativeEncoding, "none", "os_tid_decimal", "jvm_nid_hex") || !In(v.Mapping, "observed", "declared", "unknown") {
			return fmt.Errorf("RUNTIME_ENTITY_INVALID")
		}
		if v.NativeEncoding == "none" {
			if v.NativeID != "" {
				return fmt.Errorf("RUNTIME_NATIVE_ID_ENCODING_REQUIRED")
			}
		} else if _, e := NativeID(v); e != nil {
			return e
		}
		if v.Start != nil && v.End != nil && CompareTime(v.Start, v.End) > 0 {
			return fmt.Errorf("RUNTIME_THREAD_LIFETIME_INVALID")
		}
		entities[v.ID] = v
	}
	frames := map[string]bool{}
	for _, v := range s.Frames {
		if v.ID != p.FrameID(v) || frames[v.ID] || !Text(v.Symbol) || !Text(v.Display) || !Text(v.Module) || !Text(v.BuildID) || !Text(v.File) || v.Line != nil && *v.Line < 0 {
			return fmt.Errorf("RUNTIME_FRAME_INVALID")
		}
		frames[v.ID] = true
	}
	stacks := map[string]bool{}
	refs := 0
	for _, v := range s.Stacks {
		refs += len(v.Frames)
		if v.ID != p.StackID(v.Frames) || stacks[v.ID] || len(v.Frames) > 1024 || refs > 1000000 {
			return fmt.Errorf("RUNTIME_STACK_INVALID")
		}
		for _, id := range v.Frames {
			if !frames[id] {
				return fmt.Errorf("RUNTIME_STACK_FRAME_MISSING")
			}
		}
		stacks[v.ID] = true
	}
	ids := map[string]bool{}
	for _, v := range s.Observations {
		if e := ctx.Err(); e != nil {
			return e
		}
		if v.ID != ObservationID(v) || ids[v.ID] || entities[v.Entity].ID == "" || !ref(v.Ref) || !Contract(v) || !Text(v.Population) || v.Population == "" || !Text(v.State) || !Text(v.Inferred) || !Text(v.InferenceBasis) || !Number(v.Value) || !uintp(v.Time) || !uintp(v.From) || !uintp(v.To) || v.Stack != "" && !stacks[v.Stack] {
			return fmt.Errorf("RUNTIME_OBSERVATION_INVALID: id/entity/ref/physical contract/clock/stack")
		}
		if v.Value != nil {
			n, _ := m.Number(*v.Value)
			if n.Sign() < 0 && v.Metric != "allocation_diff_bytes" {
				return fmt.Errorf("RUNTIME_NEGATIVE_MEASUREMENT")
			}
			if v.Unit == "ratio" && n.Cmp(big.NewRat(1, 1)) > 0 {
				return fmt.Errorf("RUNTIME_RATIO_INVALID")
			}
		}
		if (v.From == nil) != (v.To == nil) || v.From != nil && CompareTime(v.From, v.To) > 0 {
			return fmt.Errorf("RUNTIME_INTERVAL_INVALID")
		}
		if v.Kind == "cpu" && entities[v.Entity].Kind != "os_thread" {
			return fmt.Errorf("RUNTIME_CPU_OS_THREAD_REQUIRED")
		}
		if v.Kind == "thread_snapshot" && entities[v.Entity].Kind == "async_task" || v.Kind == "task_snapshot" && entities[v.Entity].Kind != "async_task" {
			return fmt.Errorf("RUNTIME_TASK_THREAD_BOUNDARY_INVALID")
		}
		ids[v.ID] = true
	}
	for _, v := range s.Relations {
		if entities[v.From].ID == "" || entities[v.To].ID == "" || !ref(v.Ref) || !In(v.Kind, "task_parent", "runs_on", "worker_process") {
			return fmt.Errorf("RUNTIME_RELATION_INVALID")
		}
	}
	for _, v := range s.Profiles {
		if entities[v.Entity].ID == "" || !known(v.ProfileID) || !Digest(v.OriginalSHA) || !Digest(v.ModelSHA) || !ref(v.Ref) || !known(v.Clock) && v.Clock != "unknown" {
			return fmt.Errorf("RUNTIME_PROFILE_REFERENCE_INVALID")
		}
	}
	plans := map[string]bool{}
	for _, v := range s.Plans {
		if !known(v.ID) || plans[v.ID] || !known(v.NodeType) || !Text(v.Parent) || !ref(v.Ref) {
			return fmt.Errorf("RUNTIME_PLAN_INVALID")
		}
		for _, n := range []*string{v.EstimatedRows, v.ActualRows, v.Loops, v.TotalMS} {
			if !Number(n) {
				return fmt.Errorf("RUNTIME_PLAN_NUMBER_INVALID")
			}
			if n != nil {
				x, _ := m.Number(*n)
				if x.Sign() < 0 {
					return fmt.Errorf("RUNTIME_PLAN_NEGATIVE")
				}
			}
		}
		plans[v.ID] = true
	}
	for _, v := range s.Plans {
		if v.Parent != "" && !plans[v.Parent] {
			return fmt.Errorf("RUNTIME_PLAN_PARENT_MISSING")
		}
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(raw) > MaxBytes {
		return fmt.Errorf("RUNTIME_MODEL_BUDGET: bytes=%d limit=%d; split source", len(raw), MaxBytes)
	}

	byPlan := map[string]string{}
	for _, v := range s.Plans {
		byPlan[v.ID] = v.Parent
	}
	done := map[string]bool{}
	for id := range byPlan {
		path := map[string]bool{}
		at := id
		for at != "" && !done[at] {
			if path[at] {
				return fmt.Errorf("RUNTIME_PLAN_CYCLE")
			}
			path[at] = true
			at = byPlan[at]
		}
		for key := range path {
			done[key] = true
		}
	}
	return ctx.Err()
}
