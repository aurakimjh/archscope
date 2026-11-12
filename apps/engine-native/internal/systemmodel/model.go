// Package systemmodel stores exact file-only perf/BPF observations, not live probes.
package systemmodel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const Version = "1.1.0"
const LegacyVersion = "1.0.0"
const MaxBytes = 16 << 20
const MaxRecords = 50000
const PerfFields = "comm,pid,tid,cpu,time,period,event,ip,sym,dso"
const PerfSchedulerFields = "comm,pid,tid,cpu,time,period,event,trace"

type Entity struct {
	ID           string  `json:"id"`
	PID          string  `json:"pid"`
	TID          string  `json:"tid"`
	ProcessStart *string `json:"process_start_ns"`
}
type EventDefinition struct {
	Phase      string `json:"phase,omitempty"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	PeriodKind string `json:"period_kind,omitempty"`
	PeriodUnit string `json:"period_unit,omitempty"`
}
type HistogramDefinition struct {
	Name        string  `json:"name"`
	Unit        string  `json:"unit"`
	Population  string  `json:"population"`
	Aggregation string  `json:"aggregation"`
	ResetID     string  `json:"reset_id"`
	From        *string `json:"from_ns"`
	To          *string `json:"to_ns"`
}
type Capture struct {
	IgnoredTIDs    []string              `json:"ignored_tids,omitempty"`
	ImportedSHA256 string                `json:"imported_sha256,omitempty"`
	OriginalSHA256 string                `json:"original_sha256,omitempty"`
	ExportOptions  []string              `json:"export_options,omitempty"`
	Schema         string                `json:"schema"`
	ID             string                `json:"id"`
	Tool           string                `json:"tool"`
	ToolVersion    string                `json:"tool_version"`
	ScriptSHA256   string                `json:"script_sha256,omitempty"`
	Fields         string                `json:"fields,omitempty"`
	Clock          string                `json:"clock"`
	BootID         string                `json:"boot_id"`
	Namespace      string                `json:"pid_namespace"`
	Lost           *string               `json:"lost"`
	UnwindFailures *string               `json:"unwind_failures"`
	KernelSymbols  string                `json:"kernel_symbols"`
	Frequency      *string               `json:"frequency_hz"`
	Entities       []Entity              `json:"entities"`
	Events         []EventDefinition     `json:"events"`
	Histograms     []HistogramDefinition `json:"histograms"`
}
type Frame struct {
	Address string `json:"address"`
	Symbol  string `json:"symbol"`
	Module  string `json:"module"`
	Context string `json:"context"`
}
type Bucket struct {
	Lower *string `json:"lower"`
	Upper *string `json:"upper"`
	Count string  `json:"count"`
}
type Histogram struct {
	Name    string   `json:"name"`
	Key     string   `json:"key"`
	Buckets []Bucket `json:"buckets"`
}
type Record struct {
	ID        string            `json:"id,omitempty"`
	Line      int               `json:"line,omitempty"`
	Byte      int64             `json:"byte,omitempty"`
	Kind      string            `json:"kind"`
	Name      string            `json:"name,omitempty"`
	Entity    string            `json:"entity_id,omitempty"`
	Time      *string           `json:"time_ns,omitempty"`
	End       *string           `json:"end_ns,omitempty"`
	Period    *string           `json:"period,omitempty"`
	CPU       *string           `json:"cpu,omitempty"`
	State     string            `json:"state,omitempty"`
	Context   string            `json:"context,omitempty"`
	Stack     []Frame           `json:"stack,omitempty"`
	Fields    map[string]string `json:"fields,omitempty"`
	Histogram *Histogram        `json:"histogram,omitempty"`
}
type Set struct {
	Version      string   `json:"version"`
	Format       string   `json:"format"`
	SourceSHA256 string   `json:"source_sha256"`
	Capture      Capture  `json:"capture"`
	Records      []Record `json:"records"`
}

var unsigned = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var signed = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func Hash(v any) string      { b, _ := json.Marshal(v); return Digest(b) }
func Digest(b []byte) string { x := sha256.Sum256(b); return hex.EncodeToString(x[:]) }
func Strict(b []byte, out any, limit int) error {
	if len(b) > limit {
		return fmt.Errorf("SYSTEM_BUDGET: bytes=%d limit=%d", len(b), limit)
	}
	if e := wirejson.DecodeWithLimits(b, new(any), wirejson.ResultLimits(int64(limit), 128)); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}
func UInt(s string) bool {
	_, e := strconv.ParseUint(s, 10, 64)
	return len(s) <= 20 && unsigned.MatchString(s) && e == nil
}
func SI(s string) bool {
	_, e := strconv.ParseInt(s, 10, 64)
	return len(s) <= 20 && signed.MatchString(s) && e == nil
}
func Ptr(s string) *string { return &s }
func bound(s string, n int) bool {
	return utf8.ValidString(s) && len(s) <= n && !strings.ContainsRune(s, 0)
}
func (c Capture) Validate() error {
	if c.ImportedSHA256 != "" && !hashPattern.MatchString(c.ImportedSHA256) {
		return fmt.Errorf("SYSTEM_IMPORTED_HASH_INVALID")
	}
	if c.OriginalSHA256 != "" && !hashPattern.MatchString(c.OriginalSHA256) {
		return fmt.Errorf("SYSTEM_ORIGINAL_HASH_INVALID")
	}
	if len(c.ExportOptions) > 32 {
		return fmt.Errorf("SYSTEM_EXPORT_OPTIONS_BUDGET")
	}
	for _, arg := range c.ExportOptions {
		if !bound(arg, 1024) {
			return fmt.Errorf("SYSTEM_EXPORT_OPTIONS_BUDGET")
		}
	}
	if c.Schema != "system-evidence.v1" || c.ID == "" || c.Tool == "" || c.ToolVersion == "" || c.Clock == "" || c.BootID == "" || c.Namespace == "" || len(c.Entities) > 1024 || len(c.Events) > 64 || len(c.Histograms) > 64 {
		return fmt.Errorf("SYSTEM_CAPTURE_INVALID: explicit schema/tool/version/capture/clock/boot/namespace required")
	}
	for _, v := range []string{c.ID, c.Tool, c.ToolVersion, c.Clock, c.BootID, c.Namespace, c.KernelSymbols} {
		if !bound(v, 256) {
			return fmt.Errorf("SYSTEM_CAPTURE_BUDGET")
		}
	}
	if c.KernelSymbols != "unknown" && c.KernelSymbols != "partial" && c.KernelSymbols != "resolved" {
		return fmt.Errorf("SYSTEM_SYMBOL_QUALITY_REQUIRED")
	}
	for _, p := range []*string{c.Lost, c.UnwindFailures, c.Frequency} {
		if p != nil && !UInt(*p) {
			return fmt.Errorf("SYSTEM_CAPTURE_COUNT_INVALID")
		}
	}
	if c.Frequency != nil && *c.Frequency == "0" {
		return fmt.Errorf("SYSTEM_FREQUENCY_INVALID")
	}
	if c.ScriptSHA256 != "" && !hashPattern.MatchString(c.ScriptSHA256) {
		return fmt.Errorf("SYSTEM_SCRIPT_HASH_INVALID")
	}
	ids, pairs := map[string]bool{}, map[string]bool{}
	for _, e := range c.Entities {
		pair := e.PID + "/" + e.TID
		if e.ID == "" || !bound(e.ID, 256) || !UInt(e.PID) || !UInt(e.TID) || ids[e.ID] || pairs[pair] || e.ProcessStart != nil && !UInt(*e.ProcessStart) {
			return fmt.Errorf("SYSTEM_ENTITY_INVALID: unique declared lifetime per PID/TID; split reused IDs")
		}
		ids[e.ID] = true
		pairs[pair] = true
	}
	if len(c.IgnoredTIDs) > 1024 {
		return fmt.Errorf("SYSTEM_IGNORED_TIDS_INVALID")
	}
	ignored := map[string]bool{}
	declared := map[string]bool{}
	for _, entity := range c.Entities {
		declared[entity.TID] = true
	}
	for _, tid := range c.IgnoredTIDs {
		if !UInt(tid) || ignored[tid] || !declared[tid] {
			return fmt.Errorf("SYSTEM_IGNORED_TIDS_INVALID: unique canonical TIDs require a declared lifetime")
		}
		ignored[tid] = true
	}
	names := map[string]bool{}
	for _, e := range c.Events {
		if e.Name == "" || !bound(e.Name, 256) || names[e.Name] {
			return fmt.Errorf("SYSTEM_EVENT_DEFINITION_INVALID")
		}
		names[e.Name] = true
		switch e.Kind {
		case "sample":
			if e.Phase != "" {
				return fmt.Errorf("SYSTEM_DURATION_PHASE_INVALID")
			}
			if e.PeriodKind != "" || e.PeriodUnit != "" {
				if !((e.PeriodKind == "cycles" && e.PeriodUnit == "cycles") || (e.PeriodKind == "count" && e.PeriodUnit == "count") || (e.PeriodKind == "cpu_time" && e.PeriodUnit == "nanoseconds")) {
					return fmt.Errorf("SYSTEM_PERIOD_SEMANTICS_INVALID")
				}
			}
		case "sched_switch", "wakeup", "block_io", "syscall", "network":
			if e.Kind == "block_io" || e.Kind == "syscall" || e.Kind == "network" {
				switch e.Phase {
				case "queue", "service", "total", "observed_local", "unclassified":
				default:
					return fmt.Errorf("SYSTEM_DURATION_PHASE_REQUIRED")
				}
			} else if e.Phase != "" {
				return fmt.Errorf("SYSTEM_DURATION_PHASE_INVALID")
			}
			if e.PeriodKind != "" || e.PeriodUnit != "" {
				return fmt.Errorf("SYSTEM_PERIOD_SEMANTICS_INVALID")
			}
		default:
			return fmt.Errorf("SYSTEM_EVENT_KIND_UNSUPPORTED")
		}
	}
	names = map[string]bool{}
	for _, h := range c.Histograms {
		if h.Name == "" || !bound(h.Name, 256) || names[h.Name] || h.Unit == "" || !bound(h.Unit, 128) || h.Population == "" || !bound(h.Population, 256) || h.ResetID == "" || !bound(h.ResetID, 256) || (h.Aggregation != "delta" && h.Aggregation != "cumulative") {
			return fmt.Errorf("SYSTEM_HISTOGRAM_CONTEXT_REQUIRED")
		}
		names[h.Name] = true
		if (h.From == nil) != (h.To == nil) || h.From != nil && (!UInt(*h.From) || !UInt(*h.To) || Cmp(*h.From, *h.To) >= 0) {
			return fmt.Errorf("SYSTEM_HISTOGRAM_WINDOW_INVALID")
		}
	}
	return nil
}
func Cmp(a, b string) int {
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	return x.Cmp(y)
}
func Sub(a, b string) string {
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	return x.Sub(x, y).String()
}
func (s Set) Validate(ctx context.Context) error {
	if s.Version != Version && s.Version != LegacyVersion || !hashPattern.MatchString(s.SourceSHA256) || len(s.Records) > MaxRecords {
		return fmt.Errorf("SYSTEM_MODEL_INVALID_OR_BUDGET")
	}
	if e := s.Capture.Validate(); e != nil {
		return e
	}
	if s.Capture.ImportedSHA256 != "" && s.Capture.ImportedSHA256 != s.SourceSHA256 {
		return fmt.Errorf("SYSTEM_CAPTURE_IMPORTED_MISMATCH: imported_sha256 must match actual input bytes, not the upstream binary")
	}
	switch s.Format {
	case "perf-script":
		if s.Capture.Tool != "perf" || (s.Capture.Fields != PerfFields && s.Capture.Fields != PerfSchedulerFields) {
			return fmt.Errorf("SYSTEM_PERF_FIELDS_REQUIRED")
		}
	case "events-jsonl":
		if s.Capture.ScriptSHA256 == "" {
			return fmt.Errorf("SYSTEM_SCRIPT_HASH_REQUIRED")
		}
	case "bpftrace-jsonl":
		if s.Capture.Tool != "bpftrace" || s.Capture.ToolVersion != "0.22.0" || s.Capture.ScriptSHA256 == "" {
			return fmt.Errorf("SYSTEM_BPFTRACE_REVISION_UNSUPPORTED")
		}
	default:
		return fmt.Errorf("SYSTEM_FORMAT_UNSUPPORTED")
	}
	entities := map[string]bool{}
	defs := map[string]EventDefinition{}
	hist := map[string]bool{}
	for _, e := range s.Capture.Entities {
		entities[e.ID] = true
	}
	for _, d := range s.Capture.Events {
		defs[d.Name] = d
	}
	for _, h := range s.Capture.Histograms {
		hist[h.Name] = true
	}
	ids := map[string]bool{}
	refs, buckets := 0, 0
	for i, r := range s.Records {
		if e := ctx.Err(); e != nil {
			return e
		}
		if r.Line < 1 || r.Line > 64<<20 || r.Byte < 0 || r.Byte > 64<<20 || r.ID != RecordID(s.SourceSHA256, r) || ids[r.ID] {
			return fmt.Errorf("SYSTEM_RECORD_ID_INVALID: record %d", i+1)
		}
		ids[r.ID] = true
		if !bound(r.Name, 256) || !bound(r.Entity, 256) || !bound(r.State, 32) || !bound(r.Context, 32) || len(r.Fields) > 16 || len(r.Stack) > 1024 {
			return fmt.Errorf("SYSTEM_RECORD_BUDGET: line %d", r.Line)
		}
		for k, v := range r.Fields {
			if !bound(k, 128) || !bound(v, 1024) {
				return fmt.Errorf("SYSTEM_FIELD_BUDGET: line %d", r.Line)
			}
		}
		for _, p := range []*string{r.Time, r.End, r.Period, r.CPU} {
			if p != nil && !UInt(*p) {
				return fmt.Errorf("SYSTEM_NUMBER_INVALID: line %d", r.Line)
			}
		}
		for _, f := range r.Stack {
			if !bound(f.Address, 32) || !bound(f.Symbol, 1024) || !bound(f.Module, 1024) || (f.Context != "user" && f.Context != "kernel" && f.Context != "unknown") {
				return fmt.Errorf("SYSTEM_FRAME_INVALID: line %d", r.Line)
			}
		}
		refs += len(r.Stack)
		if refs > 1000000 {
			return fmt.Errorf("SYSTEM_STACK_REFERENCE_BUDGET")
		}
		if r.Kind == "histogram" {
			if r.Histogram == nil || !hist[r.Histogram.Name] || r.Time != nil || r.End != nil || r.Period != nil || r.CPU != nil || r.Entity != "" || r.Name != "" || r.State != "" || r.Context != "" || len(r.Stack) > 0 || len(r.Fields) > 0 || !bound(r.Histogram.Key, 1024) || len(r.Histogram.Buckets) == 0 || len(r.Histogram.Buckets) > 4096 {
				return fmt.Errorf("SYSTEM_HISTOGRAM_INVALID: line %d", r.Line)
			}
			for j, b := range r.Histogram.Buckets {
				buckets++
				if buckets > 100000 || !UInt(b.Count) || b.Lower != nil && !SI(*b.Lower) || b.Upper != nil && !SI(*b.Upper) || (b.Lower == nil && j != 0) || (b.Upper == nil && j != len(r.Histogram.Buckets)-1) || b.Lower != nil && b.Upper != nil && Cmp(*b.Lower, *b.Upper) > 0 {
					return fmt.Errorf("SYSTEM_BUCKET_INVALID: line %d", r.Line)
				}
				if j > 0 {
					prev := r.Histogram.Buckets[j-1]
					if prev.Upper == nil || b.Lower == nil || Cmp(*prev.Upper, *b.Lower) >= 0 {
						return fmt.Errorf("SYSTEM_BUCKET_OVERLAP: line %d", r.Line)
					}
				}
			}
			continue
		}
		if r.Histogram != nil || !entities[r.Entity] || r.Time == nil {
			return fmt.Errorf("SYSTEM_EVENT_CONTEXT_REQUIRED: line %d", r.Line)
		}
		def, ok := defs[r.Name]
		if !ok {
			return fmt.Errorf("SYSTEM_EVENT_UNDECLARED: line %d", r.Line)
		}
		switch r.Kind {
		case "sample":
			if def.Kind != "sample" || r.End != nil || r.State != "" || (r.Context != "user" && r.Context != "kernel" && r.Context != "unknown") {
				return fmt.Errorf("SYSTEM_SAMPLE_INVALID: line %d", r.Line)
			}
		case "sched_out", "sched_in", "wakeup":
			want := "sched_switch"
			if r.Kind == "wakeup" {
				want = "wakeup"
			}
			if def.Kind != want || r.End != nil || r.Period != nil || r.Kind != "sched_out" && r.State != "" || r.Kind == "sched_out" && r.State != "runnable" && r.State != "sleeping" && r.State != "unknown" {
				return fmt.Errorf("SYSTEM_SCHEDULER_INVALID: line %d", r.Line)
			}
		case "duration":
			if (def.Kind != "block_io" && def.Kind != "syscall" && def.Kind != "network") || r.Fields["phase"] != def.Phase || r.End == nil || Cmp(*r.Time, *r.End) > 0 || r.Period != nil || r.State != "" {
				return fmt.Errorf("SYSTEM_DURATION_INVALID: line %d", r.Line)
			}
		default:
			return fmt.Errorf("SYSTEM_RECORD_KIND_UNSUPPORTED: line %d", r.Line)
		}
	}
	raw, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(raw) > MaxBytes {
		return ModelBudget(len(raw), len(s.Records))
	}
	return ctx.Err()
}
func RecordID(source string, r Record) string { r.ID = ""; return "event-" + Hash([]any{source, r}) }
func Decode(ctx context.Context, b []byte) (Set, error) {
	var s Set
	if e := Strict(b, &s, MaxBytes); e != nil {
		return s, e
	}
	return s, s.Validate(ctx)
}
