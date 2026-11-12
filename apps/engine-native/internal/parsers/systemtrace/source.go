// Package systemtrace decodes explicitly selected file formats. It executes no tools.
package systemtrace

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	Format  string    `json:"format"`
	Capture m.Capture `json:"capture"`
}

var slots = make(chan struct{}, 2)

func DecodeOptions(b []byte) (Options, error) {
	var o Options
	e := m.Strict(b, &o, 128<<10)
	return o, e
}
func ParseFile(parent context.Context, path string, o Options) (m.Set, error) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return m.Set{}, fmt.Errorf("SYSTEM_IMPORT_BUSY")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	dir, e := os.MkdirTemp("", "system-source-")
	if e != nil {
		return m.Set{}, e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "system-original", ingestion.DefaultBudgets())
	if e != nil {
		return m.Set{}, e
	}
	defer lease.Close()
	f, e := lease.Open(ctx)
	if e != nil {
		return m.Set{}, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if e != nil {
		return m.Set{}, e
	}
	s, e := ParseBytes(ctx, b, o)
	if e != nil {
		return s, e
	}
	return s, lease.Verify(ctx)
}
func ParseBytes(ctx context.Context, original []byte, o Options) (m.Set, error) {
	s := m.Set{Version: m.Version, Format: o.Format, SourceSHA256: m.Digest(original), Capture: o.Capture, Records: []m.Record{}}
	if len(original) > 64<<20 {
		return s, fmt.Errorf("SYSTEM_SOURCE_BUDGET: original exceeds 64 MiB")
	}
	if e := o.Capture.Validate(); e != nil {
		return s, e
	}
	body := original
	if bytes.HasPrefix(body, []byte{31, 139}) {
		r, e := gzip.NewReader(bytes.NewReader(body))
		if e != nil {
			return s, e
		}
		defer r.Close()
		body, e = io.ReadAll(io.LimitReader(r, 64<<20+1))
		if e != nil {
			return s, e
		}
		if len(body) > 64<<20 {
			return s, fmt.Errorf("SYSTEM_SOURCE_BUDGET: expanded source exceeds 64 MiB")
		}
	}
	scan := bufio.NewScanner(bytes.NewReader(body))
	scan.Buffer(make([]byte, 4096), 1<<20)
	scan.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	line := 0
	offset := int64(0)
	var current *m.Record
	pool := m.Pool{}
	captureBytes, _ := json.Marshal(o.Capture)
	appendRecord := func(r m.Record) error {
		r.ID = m.RecordID(s.SourceSHA256, r)
		if len(s.Records) >= m.MaxRecords {
			return fmt.Errorf("SYSTEM_RECORD_BUDGET: more than %d; split original", m.MaxRecords)
		}
		pool.Add(r)
		if pool.Bytes+len(captureBytes)+512 > m.MaxBytes {
			return m.ModelBudget(pool.Bytes+len(captureBytes)+512, len(s.Records))
		}
		s.Records = append(s.Records, r)
		return nil
	}
	flush := func() error {
		if current == nil {
			return nil
		}
		for i, j := 0, len(current.Stack)-1; i < j; i, j = i+1, j-1 {
			current.Stack[i], current.Stack[j] = current.Stack[j], current.Stack[i]
		}
		e := appendRecord(*current)
		current = nil
		return e
	}
	for scan.Scan() {
		line++
		text := strings.TrimSuffix(scan.Text(), "\r")
		position := offset
		offset += int64(len(scan.Bytes())) + 1
		if e := ctx.Err(); e != nil {
			return s, e
		}
		fail := func(e error) (m.Set, error) {
			return s, fmt.Errorf("SYSTEM_SOURCE_INVALID at line %d byte %d: %w", line, position, e)
		}
		switch o.Format {
		case "perf-script":
			if strings.TrimSpace(text) == "" {
				if e := flush(); e != nil {
					return fail(e)
				}
				continue
			}
			if strings.HasPrefix(text, "#") {
				return fail(fmt.Errorf("SYSTEM_PERF_HEADER_UNSUPPORTED: export exactly the declared fields without --header; loss/clock/capture context belongs in the sidecar"))
			}
			if match := header.FindStringSubmatch(text); match != nil {
				if e := flush(); e != nil {
					return fail(e)
				}
				pid, tid, cpu, stamp, period, name, tail := match[2], match[3], match[4], match[5], match[6], match[7], match[8]
				cpuValue, _ := strconv.ParseUint(cpu, 10, 64)
				cpu = strconv.FormatUint(cpuValue, 10)
				ns, e := nanoSeconds(stamp)
				if e != nil {
					return fail(e)
				}
				entity, e := findEntity(o.Capture, pid, tid)
				if e != nil {
					return fail(e)
				}
				kind := ""
				for _, d := range o.Capture.Events {
					if d.Name == name {
						kind = d.Kind
					}
				}
				if kind == "sample" {
					if o.Capture.Fields != m.PerfFields {
						return fail(fmt.Errorf("SYSTEM_PERF_SAMPLE_FIELDS_REQUIRED"))
					}
					current = &m.Record{Kind: "sample", Name: name, Entity: entity, Time: &ns, Period: &period, CPU: &cpu, Context: "unknown", Fields: map[string]string{"comm": match[1]}, Line: line, Byte: position}
					if strings.TrimSpace(tail) != "" {
						f, e := parseFrame(tail)
						if e != nil {
							return fail(e)
						}
						current.Stack = append(current.Stack, f)
					}
				} else if kind == "sched_switch" {
					if o.Capture.Fields != m.PerfSchedulerFields {
						return fail(fmt.Errorf("SYSTEM_PERF_SCHEDULER_FIELDS_REQUIRED"))
					}
					switchData := switchPayload.FindStringSubmatch(tail)
					if switchData == nil {
						return fail(fmt.Errorf("SYSTEM_SCHED_SWITCH_LAYOUT_UNSUPPORTED"))
					}
					prev, e := findTID(o.Capture, switchData[1])
					if e != nil {
						return fail(e)
					}
					next, e := findTID(o.Capture, switchData[3])
					if e != nil {
						return fail(e)
					}
					state := "unknown"
					if switchData[2] == "R" || switchData[2] == "R+" {
						state = "runnable"
					} else if strings.ContainsAny(switchData[2], "SDITtXZP") {
						state = "sleeping"
					}
					for _, r := range []m.Record{{Kind: "sched_out", Name: name, Entity: prev, Time: &ns, CPU: &cpu, State: state, Fields: map[string]string{"raw_prev_state": switchData[2]}, Line: line, Byte: position}, {Kind: "sched_in", Name: name, Entity: next, Time: &ns, CPU: &cpu, Line: line, Byte: position}} {
						if e := appendRecord(r); e != nil {
							return fail(e)
						}
					}
				} else if kind == "wakeup" {
					if o.Capture.Fields != m.PerfSchedulerFields {
						return fail(fmt.Errorf("SYSTEM_PERF_SCHEDULER_FIELDS_REQUIRED"))
					}
					match := wakePayload.FindStringSubmatch(tail)
					if match == nil {
						return fail(fmt.Errorf("SYSTEM_WAKEUP_LAYOUT_UNSUPPORTED"))
					}
					target, e := findTID(o.Capture, match[1])
					if e != nil {
						return fail(e)
					}
					if e := appendRecord(m.Record{Kind: "wakeup", Name: name, Entity: target, Time: &ns, CPU: &cpu, Fields: map[string]string{"target_cpu": match[2]}, Line: line, Byte: position}); e != nil {
						return fail(e)
					}
				} else {
					return fail(fmt.Errorf("SYSTEM_EVENT_UNDECLARED_OR_UNSUPPORTED: supply a sample/scheduler event definition"))
				}
				continue
			}
			if current == nil {
				return fail(fmt.Errorf("SYSTEM_PERF_LAYOUT_UNSUPPORTED: require --ns and -F %s; binary perf.data and other field layouts require local conversion", m.PerfFields))
			}
			f, e := parseFrame(text)
			if e != nil {
				return fail(e)
			}
			current.Stack = append(current.Stack, f)
			if len(current.Stack) > 1024 {
				return fail(fmt.Errorf("SYSTEM_STACK_DEPTH_BUDGET"))
			}
		case "events-jsonl":
			if strings.TrimSpace(text) == "" {
				return fail(fmt.Errorf("SYSTEM_JSONL_EMPTY_RECORD"))
			}
			var r m.Record
			if e := m.Strict([]byte(text), &r, 1<<20); e != nil {
				return fail(e)
			}
			if r.ID != "" || r.Line != 0 || r.Byte != 0 {
				return fail(fmt.Errorf("SYSTEM_SOURCE_POSITION_OWNED"))
			}
			r.Line, r.Byte = line, position
			if e := appendRecord(r); e != nil {
				return fail(e)
			}
		case "bpftrace-jsonl":
			var msg struct {
				Type string                     `json:"type"`
				Data map[string]json.RawMessage `json:"data"`
			}
			if e := m.Strict([]byte(text), &msg, 1<<20); e != nil {
				return fail(e)
			}
			if msg.Type != "hist" || len(msg.Data) == 0 {
				return fail(fmt.Errorf("SYSTEM_BPFTRACE_MESSAGE_UNSUPPORTED: only pinned hist/lhist JSON; map/printf/syscall/diagnostic output requires an explicit event adapter"))
			}
			names := sortedKeys(msg.Data)
			for _, name := range names {
				raw := msg.Data[name]
				rows := map[string]json.RawMessage{"": raw}
				if bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
					rows = map[string]json.RawMessage{}
					if e := m.Strict(raw, &rows, 1<<20); e != nil {
						return fail(fmt.Errorf("SYSTEM_BPFTRACE_BUCKET_SHAPE: %w", e))
					}
				}
				for _, key := range sortedKeys(rows) {
					var input []struct {
						Min   json.RawMessage `json:"min"`
						Max   json.RawMessage `json:"max"`
						Count json.RawMessage `json:"count"`
					}
					if e := m.Strict(rows[key], &input, 1<<20); e != nil {
						return fail(fmt.Errorf("SYSTEM_BPFTRACE_BUCKET_SHAPE: %w", e))
					}
					h := m.Histogram{Name: name, Key: key, Buckets: []m.Bucket{}}
					for _, b := range input {
						lower, e := integerLiteral(b.Min, true)
						if e != nil {
							return fail(e)
						}
						upper, e := integerLiteral(b.Max, true)
						if e != nil {
							return fail(e)
						}
						count, e := integerLiteral(b.Count, false)
						if e != nil {
							return fail(e)
						}
						h.Buckets = append(h.Buckets, m.Bucket{Lower: lower, Upper: upper, Count: *count})
					}
					if e := appendRecord(m.Record{Kind: "histogram", Histogram: &h, Line: line, Byte: position}); e != nil {
						return fail(e)
					}
				}
			}
		default:
			return fail(fmt.Errorf("SYSTEM_FORMAT_REQUIRED: perf-script, events-jsonl or bpftrace-jsonl"))
		}
	}
	if e := scan.Err(); e != nil {
		return s, fmt.Errorf("SYSTEM_LINE_BUDGET_OR_IO at line %d: %w", line+1, e)
	}
	if e := flush(); e != nil {
		return s, e
	}
	return s, s.Validate(ctx)
}

var header = regexp.MustCompile(`^\s*(.*?)\s+([0-9]+)/([0-9]+)\s+\[([0-9]+)\]\s+([0-9]+\.[0-9]{9}):\s+([0-9]+)\s+(\S+):\s*(.*)$`)
var frame = regexp.MustCompile(`^\s*([a-fA-F0-9]+)\s+(.+?)\s+\((.*)\)\s*$`)
var switchPayload = regexp.MustCompile(`^prev_comm=.* prev_pid=([0-9]+) prev_prio=[0-9]+ prev_state=(\S+) ==> next_comm=.* next_pid=([0-9]+) next_prio=[0-9]+\s*$`)
var wakePayload = regexp.MustCompile(`^comm=.* pid=([0-9]+) prio=[0-9]+ target_cpu=([0-9]+)\s*$`)

func nanoSeconds(s string) (string, error) {
	parts := strings.Split(s, ".")
	seconds, e := strconv.ParseUint(parts[0], 10, 64)
	if e != nil || seconds > 18446744073 {
		return "", fmt.Errorf("SYSTEM_TIME_RANGE")
	}
	n := strings.TrimLeft(parts[0]+parts[1], "0")
	if n == "" {
		n = "0"
	}
	if !m.UInt(n) {
		return "", fmt.Errorf("SYSTEM_TIME_RANGE")
	}
	return n, nil
}
func parseFrame(text string) (m.Frame, error) {
	match := frame.FindStringSubmatch(text)
	if match == nil {
		return m.Frame{}, fmt.Errorf("SYSTEM_STACK_LAYOUT_UNSUPPORTED")
	}
	context := "unknown"
	if match[3] == "[kernel.kallsyms]" {
		context = "kernel"
	}
	return m.Frame{Address: strings.ToLower(match[1]), Symbol: match[2], Module: match[3], Context: context}, nil
}
func findEntity(c m.Capture, pid, tid string) (string, error) {
	for _, e := range c.Entities {
		if e.PID == pid && e.TID == tid {
			return e.ID, nil
		}
	}
	return "", fmt.Errorf("SYSTEM_ENTITY_REQUIRED: declare PID/TID %s/%s and its capture lifetime", pid, tid)
}
func findTID(c m.Capture, tid string) (string, error) {
	found := ""
	for _, e := range c.Entities {
		if e.TID == tid {
			if found != "" {
				return "", fmt.Errorf("SYSTEM_TID_AMBIGUOUS")
			}
			found = e.ID
		}
	}
	if found == "" {
		return "", fmt.Errorf("SYSTEM_ENTITY_REQUIRED: declare scheduler TID %s", tid)
	}
	return found, nil
}
func integerLiteral(b json.RawMessage, optional bool) (*string, error) {
	if len(b) == 0 {
		if optional {
			return nil, nil
		}
		return nil, fmt.Errorf("SYSTEM_BUCKET_COUNT_REQUIRED")
	}
	s := string(b)
	if strings.HasPrefix(s, `"`) {
		if e := json.Unmarshal(b, &s); e != nil {
			return nil, e
		}
	}
	if optional && !m.SI(s) || !optional && !m.UInt(s) {
		return nil, fmt.Errorf("SYSTEM_BUCKET_NUMBER_INVALID")
	}
	return &s, nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
