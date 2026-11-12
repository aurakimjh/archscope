// Package runtimeevidence normalizes bounded offline runtime/TA/plan exports.
package runtimeevidence

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/symbolinfo"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"io"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	ThreadFormat    string            `json:"thread_format,omitempty"`
	Format          string            `json:"format"`
	Producer        string            `json:"producer,omitempty"`
	Version         string            `json:"producer_version,omitempty"`
	Clock           string            `json:"clock_id,omitempty"`
	Process         *r.Process        `json:"process,omitempty"`
	Entities        []r.Entity        `json:"entities,omitempty"`
	Time            *string           `json:"time_ns,omitempty"`
	From            *string           `json:"from_ns,omitempty"`
	To              *string           `json:"to_ns,omitempty"`
	Basis           string            `json:"measurement_basis,omitempty"`
	Population      string            `json:"population,omitempty"`
	ProfileEntities map[string]string `json:"profile_entities,omitempty"`
}

func DecodeOptions(raw []byte) (Options, error) { var o Options; return o, r.Strict(raw, &o, 128<<10) }
func Digest(raw []byte) string                  { x := sha256.Sum256(raw); return hex.EncodeToString(x[:]) }

var slots = make(chan struct{}, 2)

func ParseFile(parent context.Context, path string, o Options) (r.Set, string, error) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return r.Set{}, "", fmt.Errorf("RUNTIME_SOURCE_BUSY")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	dir, e := os.MkdirTemp("", "runtime-original-")
	if e != nil {
		return r.Set{}, "", e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "original", ingestion.DefaultBudgets())
	if e != nil {
		return r.Set{}, "", e
	}
	defer lease.Close()
	f, e := lease.Open(ctx)
	if e != nil {
		return r.Set{}, "", e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if e != nil {
		return r.Set{}, "", e
	}
	s, e := ParseBytes(ctx, raw, o)
	if e == nil {
		e = lease.Verify(ctx)
	}
	return s, Digest(raw), e
}

type builder struct {
	prefix   string
	failure  error
	set      r.Set
	ctx      context.Context
	source   r.Source
	options  Options
	frames   map[string]bool
	stacks   map[string]bool
	entities map[string]r.Entity
}

func ParseBytes(ctx context.Context, original []byte, o Options) (r.Set, error) {
	empty := r.Empty()
	if len(original) > 64<<20 {
		return empty, fmt.Errorf("RUNTIME_SOURCE_BYTES_BUDGET")
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	encoded, _ := json.Marshal(o)
	if len(encoded) > 128<<10 {
		return empty, fmt.Errorf("RUNTIME_OPTIONS_BUDGET")
	}
	body := original
	if bytes.HasPrefix(body, []byte{31, 139}) {
		z, e := gzip.NewReader(bytes.NewReader(body))
		if e != nil {
			return empty, e
		}
		defer z.Close()
		body, e = io.ReadAll(io.LimitReader(z, 64<<20+1))
		if e != nil {
			return empty, e
		}
		if len(body) > 64<<20 {
			return empty, fmt.Errorf("RUNTIME_EXPANDED_BUDGET")
		}
	}
	if o.Format == "runtime-evidence-json" {
		if o.Process != nil || len(o.Entities) > 0 || len(o.ProfileEntities) > 0 {
			return empty, fmt.Errorf("RUNTIME_CANONICAL_OPTIONS_INVALID")
		}
		return r.Decode(ctx, body)
	}
	if o.Clock == "" {
		o.Clock = "unknown"
	}
	if o.Population == "" {
		o.Population = "unknown"
	}
	if !r.Text(o.Clock) || !r.Text(o.Population) {
		return empty, fmt.Errorf("RUNTIME_CONTEXT_INVALID")
	}
	b := builder{set: empty, ctx: ctx, options: o, frames: map[string]bool{}, stacks: map[string]bool{}, entities: map[string]r.Entity{}}
	b.source = r.Source{SHA: Digest(original), Format: o.Format, Producer: o.Producer, Version: o.Version, Clock: o.Clock}
	if o.Process != nil {
		b.set.Processes = append(b.set.Processes, *o.Process)
	}
	b.set.Entities = append(b.set.Entities, o.Entities...)
	for _, v := range o.Entities {
		b.entities[v.ID] = v
	}
	var e error
	switch o.Format {
	case "python-runtime-json":
		e = b.python(body)
	case "node-report-json":
		e = b.node(body, false)
	case "node-runtime-json":
		e = b.node(body, true)
	case "ps-l-csv":
		e = b.cpuCSV(body)
	case "thread-dump":
		e = b.threadDump(body)
	case "thread-snapshots-json":
		e = b.threads(body)
	case "profile-source-set-json":
		e = b.profile(body)
	case "rust-backtrace":
		e = b.rustBacktrace(body)
	case "postgres-explain-json":
		e = b.postgres(body)
	default:
		return empty, fmt.Errorf("RUNTIME_FORMAT_UNSUPPORTED")
	}
	if e != nil {
		return empty, e
	}
	if b.failure != nil {
		return empty, b.failure
	}
	if b.source.Producer == "" || b.source.Version == "" {
		return empty, fmt.Errorf("RUNTIME_PRODUCER_VERSION_REQUIRED")
	}
	old := "source"
	b.source.ID = "runtime-source-" + r.Hash(b.source)
	b.set.Sources = []r.Source{b.source}
	for i := range b.set.Observations {
		if b.set.Observations[i].Ref.Source == old {
			b.set.Observations[i].Ref.Source = b.source.ID
		}
		b.set.Observations[i].ID = r.ObservationID(b.set.Observations[i])
	}
	for i := range b.set.Relations {
		b.set.Relations[i].Ref.Source = b.source.ID
	}
	for i := range b.set.Profiles {
		b.set.Profiles[i].Ref.Source = b.source.ID
	}
	for i := range b.set.Plans {
		b.set.Plans[i].Ref.Source = b.source.ID
	}
	return b.set, b.set.Validate(ctx)
}
func decode(raw []byte, v any) error {
	return wirejson.DecodeWithLimits(raw, v, wirejson.ResultLimits(64<<20, 128))
}
func (b *builder) version(producer, version string) error {
	if b.source.Producer == "" {
		b.source.Producer = producer
	}
	if b.source.Version != "" && b.source.Version != version {
		return fmt.Errorf("RUNTIME_PRODUCER_VERSION_MISMATCH")
	}
	b.source.Version = version
	return nil
}
func (b *builder) process() error {
	if b.options.Process == nil {
		return fmt.Errorf("RUNTIME_PROCESS_CONTEXT_REQUIRED: declare sanitized host/boot/namespace/PID/lifetime")
	}
	return nil
}
func (b *builder) entity(kind, id, name, native, encoding string) string {
	v := r.Entity{Process: b.options.Process.ID, Kind: kind, RuntimeID: id, Name: name, NativeID: native, NativeEncoding: encoding, Mapping: "unknown"}
	for _, declared := range b.options.Entities {
		if declared.RuntimeID == id && declared.Process == v.Process && declared.Kind == kind {
			if native != "" {
				a, ae := r.NativeID(v)
				d, de := r.NativeID(declared)
				if ae != nil || de != nil || a != d {
					b.failure = fmt.Errorf("RUNTIME_NATIVE_MAPPING_CONFLICT")
				}
			}
			return declared.ID
		}
	}
	v.ID = "entity-" + r.Hash(v)
	if _, ok := b.entities[v.ID]; !ok {
		b.entities[v.ID] = v
		b.set.Entities = append(b.set.Entities, v)
	}
	return v.ID
}

type frame struct {
	Symbol   string `json:"symbol"`
	File     string `json:"file"`
	Line     *int64 `json:"line"`
	Language string `json:"language,omitempty"`
	Module   string `json:"module,omitempty"`
	Build    string `json:"build_id,omitempty"`
}

func (b *builder) stack(fs []frame) string {
	if len(fs) > 1024 {
		b.failure = fmt.Errorf("RUNTIME_STACK_DEPTH_BUDGET")
		return ""
	}
	if b.failure != nil {
		return ""
	}
	if len(fs) == 0 {
		return ""
	}
	ids := []string{}
	for _, f := range fs {
		if e := b.ctx.Err(); e != nil {
			b.failure = e
			return ""
		}
		if len(b.frames) >= r.MaxFrames {
			b.failure = fmt.Errorf("RUNTIME_FRAME_BUDGET")
			return ""
		}
		info := symbolinfo.Describe(f.Symbol, f.File, f.Language)
		v := p.Frame{Symbol: f.Symbol, Display: info.Display, File: f.File, Line: f.Line, Module: f.Module, BuildID: f.Build, Language: f.Language}
		if info.Runtime == "Rust" {
			v.Runtime = info.Runtime
			v.Language = info.Language
		}
		v.ID = p.FrameID(v)
		if !b.frames[v.ID] {
			b.frames[v.ID] = true
			b.set.Frames = append(b.set.Frames, v)
		}
		ids = append(ids, v.ID)
	}
	v := p.Stack{ID: p.StackID(ids), Frames: ids}
	if !b.stacks[v.ID] {
		b.stacks[v.ID] = true
		b.set.Stacks = append(b.set.Stacks, v)
	}
	return v.ID
}
func (b *builder) add(v r.Observation) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	if len(b.set.Observations) >= r.MaxObservations || len(b.set.Entities) > r.MaxEntities || len(b.set.Frames) > r.MaxFrames {
		return fmt.Errorf("RUNTIME_RECORD_BUDGET")
	}
	v.Ref.Source = "source"
	if v.Population == "" {
		v.Population = b.options.Population
	}
	b.set.Observations = append(b.set.Observations, v)
	return nil
}

var numeric = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func number(v json.Number) (*string, error) {
	if v == "" {
		return nil, nil
	}
	raw := string(v)
	if len(raw) > 256 || !numeric.MatchString(raw) {
		return nil, fmt.Errorf("RUNTIME_NUMBER_INVALID")
	}
	if i := strings.IndexAny(raw, "eE"); i >= 0 {
		n, e := strconv.Atoi(raw[i+1:])
		if e != nil || n > 128 || n < -128 {
			return nil, fmt.Errorf("RUNTIME_NUMBER_EXPONENT_BUDGET")
		}
	}
	n, ok := new(big.Rat).SetString(raw)
	if !ok {
		return nil, fmt.Errorf("RUNTIME_NUMBER_INVALID")
	}
	s := m.Text(n)
	if _, e := m.Number(s); e != nil {
		return nil, e
	}
	return &s, nil
}
func ptr(s string) *string { return &s }
