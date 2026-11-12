package runtimemodel

import (
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"strconv"
	"strings"
)

func CompareTime(a, b *string) int {
	x, _ := m.Time(a)
	y, _ := m.Time(b)
	if x < y {
		return -1
	}
	if x > y {
		return 1
	}
	return 0
}
func NativeID(v Entity) (string, error) {
	if v.NativeEncoding == "os_tid_decimal" {
		if _, e := m.Uint(v.NativeID); e != nil {
			return "", e
		}
		return v.NativeID, nil
	}
	if v.NativeEncoding == "jvm_nid_hex" {
		raw := strings.TrimPrefix(strings.TrimPrefix(v.NativeID, "0x"), "0X")
		if len(raw) == 0 || len(raw) > 16 {
			return "", fmt.Errorf("RUNTIME_NATIVE_ID_INVALID")
		}
		n, e := strconv.ParseUint(raw, 16, 64)
		if e != nil {
			return "", e
		}
		return strconv.FormatUint(n, 10), nil
	}
	return "", fmt.Errorf("RUNTIME_NATIVE_ID_UNAVAILABLE")
}

type CPULink struct {
	CPU           string  `json:"cpu_observation"`
	Stack         string  `json:"stack_observation"`
	NativeID      string  `json:"native_tid_decimal"`
	Status        string  `json:"status"`
	Reason        string  `json:"reason"`
	From          *string `json:"from_ns"`
	To            *string `json:"to_ns"`
	Snapshot      *string `json:"snapshot_ns"`
	IdentityBasis string  `json:"identity_basis"`
}

// Correlate qualifies identity and time context, not causal attribution. Sources
// are immutable declared observations; this does not authenticate their origin.
func Correlate(s Set, cpu, stack Observation) CPULink {
	out := CPULink{CPU: cpu.ID, Stack: stack.ID, Status: "candidate", From: cpu.From, To: cpu.To, Snapshot: stack.Time, IdentityBasis: "source declarations; not provenance authentication"}
	entities := map[string]Entity{}
	processes := map[string]Process{}
	sources := map[string]Source{}
	for _, v := range s.Entities {
		entities[v.ID] = v
	}
	for _, v := range s.Processes {
		processes[v.ID] = v
	}
	for _, v := range s.Sources {
		sources[v.ID] = v
	}
	a, b := entities[cpu.Entity], entities[stack.Entity]
	pa, pb := processes[a.Process], processes[b.Process]
	at, ae := NativeID(a)
	bt, be := NativeID(b)
	out.NativeID = at
	switch {
	case cpu.Kind != "cpu" || stack.Kind != "thread_snapshot":
		out.Reason = "requires_cpu_and_os_thread_snapshot"
	case ae != nil || be != nil:
		out.Reason = "native_thread_id_unavailable"
	case at != bt:
		out.Status = "incompatible"
		out.Reason = "native_tid_mismatch"
	case !known(pa.Host) || !known(pa.Boot) || !known(pa.Namespace) || !known(pb.Host) || !known(pb.Boot) || !known(pb.Namespace) || pa.Start == nil || pb.Start == nil:
		out.Reason = "process_host_boot_namespace_lifetime_required"
	case pa.Host != pb.Host || pa.Boot != pb.Boot || pa.Namespace != pb.Namespace || pa.PID != pb.PID || *pa.Start != *pb.Start:
		out.Status = "incompatible"
		out.Reason = "process_identity_or_lifetime_mismatch"
	case sources[cpu.Ref.Source].Clock == "unknown" || sources[cpu.Ref.Source].Clock != sources[stack.Ref.Source].Clock:
		out.Reason = "clock_alignment_unavailable"
	case cpu.From != nil && CompareTime(pa.Start, cpu.From) > 0:
		out.Status = "incompatible"
		out.Reason = "cpu_window_precedes_process_lifetime"
	case cpu.Basis != "interval_average":
		out.Reason = "lifetime_average_is_not_interval_cpu"
	case cpu.From == nil || cpu.To == nil || stack.Time == nil || CompareTime(cpu.From, cpu.To) >= 0:
		out.Reason = "cpu_window_or_snapshot_time_required"
	case CompareTime(stack.Time, cpu.From) < 0 || CompareTime(stack.Time, cpu.To) > 0:
		out.Status = "incompatible"
		out.Reason = "snapshot_outside_cpu_window"
	case a.Start == nil || b.Start == nil:
		out.Reason = "thread_lifetime_required_to_rule_out_tid_reuse"
	case *a.Start != *b.Start:
		out.Status = "incompatible"
		out.Reason = "thread_lifetime_mismatch"
	case CompareTime(a.Start, cpu.From) > 0 || a.End != nil && CompareTime(a.End, cpu.To) < 0 || b.End != nil && CompareTime(b.End, cpu.To) < 0:
		out.Status = "incompatible"
		out.Reason = "cpu_window_exceeds_thread_lifetime"
	case a.Mapping == "unknown" || b.Mapping == "unknown":
		out.Reason = "mapping_basis_required"
	default:
		out.Status = "qualified_context"
		out.Reason = "same_lifetime_native_tid_and_containing_cpu_window; observation_only"
	}
	return out
}
