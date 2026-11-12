package systemmodel

import (
	"encoding/json"
	"fmt"
)

// The in-memory API keeps expanded stacks for existing analyzers. Version 1.1
// stores each frame and each stack once; bounded integer indexes are not metrics.
type recordAlias Record
type compactRecord struct {
	recordAlias
	StackID *int `json:"stack_id,omitempty"`
}
type compactSet struct {
	Version      string          `json:"version"`
	Format       string          `json:"format"`
	SourceSHA256 string          `json:"source_sha256"`
	Capture      Capture         `json:"capture"`
	Frames       []Frame         `json:"frames"`
	Stacks       [][]int         `json:"stacks"`
	Records      []compactRecord `json:"records"`
}
type setAlias Set
type Pool struct {
	frames map[string]int
	stacks map[string]int
	Frames []Frame
	Stacks [][]int
	Bytes  int
}

func (p *Pool) record(r Record) compactRecord {
	if p.frames == nil {
		p.frames = map[string]int{}
		p.stacks = map[string]int{}
		p.Frames = []Frame{}
		p.Stacks = [][]int{}
	}
	out := compactRecord{recordAlias: recordAlias(r)}
	out.Stack = nil
	if len(r.Stack) == 0 {
		return out
	}
	indexes := make([]int, 0, len(r.Stack))
	for _, frame := range r.Stack {
		key := Hash(frame)
		index, ok := p.frames[key]
		if !ok {
			index = len(p.Frames)
			p.frames[key] = index
			p.Frames = append(p.Frames, frame)
			raw, _ := json.Marshal(frame)
			p.Bytes += len(raw) + 1
		}
		indexes = append(indexes, index)
	}
	key := Hash(indexes)
	index, ok := p.stacks[key]
	if !ok {
		index = len(p.Stacks)
		p.stacks[key] = index
		p.Stacks = append(p.Stacks, indexes)
		raw, _ := json.Marshal(indexes)
		p.Bytes += len(raw) + 1
	}
	out.StackID = &index
	return out
}
func (p *Pool) Add(r Record) {
	row := p.record(r)
	raw, _ := json.Marshal(row)
	p.Bytes += len(raw) + 1
}
func ModelBudget(actual, records int) error {
	return fmt.Errorf("SYSTEM_MODEL_BUDGET: bytes=%d limit=%d processed_records=%d; split capture with perf script --time <start>,<end> or reduce stacks/events; no records silently dropped", actual, MaxBytes, records)
}
func (s Set) MarshalJSON() ([]byte, error) {
	if s.Version == LegacyVersion {
		return json.Marshal(setAlias(s))
	}
	pool := Pool{}
	records := make([]compactRecord, 0, len(s.Records))
	for _, r := range s.Records {
		records = append(records, pool.record(r))
	}
	if pool.Frames == nil {
		pool.Frames = []Frame{}
		pool.Stacks = [][]int{}
	}
	return json.Marshal(compactSet{s.Version, s.Format, s.SourceSHA256, s.Capture, pool.Frames, pool.Stacks, records})
}
func (s *Set) UnmarshalJSON(raw []byte) error {
	var root map[string]json.RawMessage
	if e := Strict(raw, &root, MaxBytes); e != nil {
		return e
	}
	var version string
	if e := json.Unmarshal(root["version"], &version); e != nil {
		return e
	}
	if version == LegacyVersion {
		var legacy setAlias
		if e := Strict(raw, &legacy, MaxBytes); e != nil {
			return e
		}
		*s = Set(legacy)
		return nil
	}
	if version != Version {
		return fmt.Errorf("SYSTEM_MODEL_VERSION_UNSUPPORTED")
	}
	var wire compactSet
	if e := Strict(raw, &wire, MaxBytes); e != nil {
		return e
	}
	if len(wire.Frames) > 1_000_000 || len(wire.Stacks) > MaxRecords || len(wire.Records) > MaxRecords {
		return fmt.Errorf("SYSTEM_POOL_BUDGET")
	}
	refs := 0
	poolRefs := 0
	for _, stack := range wire.Stacks {
		poolRefs += len(stack)
		if poolRefs > 1_000_000 {
			return fmt.Errorf("SYSTEM_STACK_POOL_REFERENCE_BUDGET")
		}
		if len(stack) == 0 || len(stack) > 1024 {
			return fmt.Errorf("SYSTEM_STACK_INVALID")
		}
		for _, frame := range stack {
			if frame < 0 || frame >= len(wire.Frames) {
				return fmt.Errorf("SYSTEM_FRAME_REFERENCE_INVALID")
			}
		}
	}
	usedFrames := map[int]bool{}
	usedStacks := map[int]bool{}
	records := make([]Record, 0, len(wire.Records))
	for _, row := range wire.Records {
		if row.Stack != nil {
			return fmt.Errorf("SYSTEM_COMPACT_INLINE_STACK_FORBIDDEN")
		}
		r := Record(row.recordAlias)
		if row.StackID != nil {
			index := *row.StackID
			if index < 0 || index >= len(wire.Stacks) {
				return fmt.Errorf("SYSTEM_STACK_REFERENCE_INVALID")
			}
			usedStacks[index] = true
			refs += len(wire.Stacks[index])
			if refs > 1_000_000 {
				return fmt.Errorf("SYSTEM_STACK_REFERENCE_BUDGET")
			}
			r.Stack = make([]Frame, 0, len(wire.Stacks[index]))
			for _, f := range wire.Stacks[index] {
				usedFrames[f] = true
				r.Stack = append(r.Stack, wire.Frames[f])
			}
		}
		records = append(records, r)
	}
	if len(usedFrames) != len(wire.Frames) || len(usedStacks) != len(wire.Stacks) {
		return fmt.Errorf("SYSTEM_UNUSED_POOL_ENTRY")
	}
	*s = Set{Version: wire.Version, Format: wire.Format, SourceSHA256: wire.SourceSHA256, Capture: wire.Capture, Records: records}
	return nil
}
