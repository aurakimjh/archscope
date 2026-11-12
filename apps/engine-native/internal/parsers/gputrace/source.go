// Package gputrace reads offline exports. It never runs a collector or converter.
package gputrace

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"io"
	"os"
	"time"
)

type Options struct {
	Format          string            `json:"format"`
	ProducerVersion string            `json:"producer_version,omitempty"`
	Capture         string            `json:"capture_id,omitempty"`
	Host            string            `json:"host_id,omitempty"`
	Boot            string            `json:"boot_id,omitempty"`
	Namespace       string            `json:"process_namespace,omitempty"`
	Run             string            `json:"run_id,omitempty"`
	Clock           string            `json:"clock_id,omitempty"`
	ClockGroup      string            `json:"clock_group,omitempty"`
	Offset          *string           `json:"clock_offset_ns,omitempty"`
	Uncertainty     *string           `json:"clock_uncertainty_ns,omitempty"`
	DeviceMap       map[string]string `json:"device_map,omitempty"`
	From            *string           `json:"from_ns,omitempty"`
	To              *string           `json:"to_ns,omitempty"`
}

var slots = make(chan struct{}, 2)

func DecodeOptions(b []byte) (Options, error) { var o Options; return o, g.Strict(b, &o, 64<<10) }
func Digest(b []byte) string                  { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func ParseFile(parent context.Context, path string, o Options) (g.Set, string, error) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return g.Set{}, "", fmt.Errorf("GPU_TRACE_IMPORT_BUSY")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	dir, e := os.MkdirTemp("", "gpu-trace-source-")
	if e != nil {
		return g.Set{}, "", e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "gpu-original", ingestion.DefaultBudgets())
	if e != nil {
		return g.Set{}, "", e
	}
	defer lease.Close()
	f, e := lease.Open(ctx)
	if e != nil {
		return g.Set{}, "", e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if e != nil {
		return g.Set{}, "", e
	}
	set, e := ParseBytes(ctx, raw, o)
	if e == nil {
		e = lease.Verify(ctx)
	}
	return set, Digest(raw), e
}
func ParseBytes(ctx context.Context, original []byte, o Options) (g.Set, error) {
	set := g.Empty()
	if len(original) > 64<<20 {
		return set, fmt.Errorf("GPU_TRACE_SOURCE_BYTES_BUDGET: bytes=%d limit=%d", len(original), 64<<20)
	}
	opts, _ := json.Marshal(o)
	if len(opts) > 64<<10 {
		return set, fmt.Errorf("GPU_TRACE_OPTIONS_BUDGET")
	}
	var isolated Options
	if e := g.Strict(opts, &isolated, 64<<10); e != nil {
		return set, e
	}
	o = isolated
	if e := ctx.Err(); e != nil {
		return set, e
	}
	body := original
	if bytes.HasPrefix(body, []byte{31, 139}) {
		z, e := gzip.NewReader(bytes.NewReader(body))
		if e != nil {
			return set, e
		}
		defer z.Close()
		body, e = io.ReadAll(io.LimitReader(z, 64<<20+1))
		if e != nil {
			return set, e
		}
		if len(body) > 64<<20 {
			return set, fmt.Errorf("GPU_TRACE_EXPANDED_BYTES_BUDGET")
		}
	}
	if o.Format == "gpu-execution-json" {
		if string(opts) != `{"format":"gpu-execution-json"}` {
			return set, fmt.Errorf("GPU_TRACE_CANONICAL_OPTIONS: only format is allowed")
		}
		return g.Decode(ctx, body)
	}
	source := g.Source{SHA: Digest(original), Format: o.Format, Version: o.ProducerVersion, Capture: o.Capture, Host: o.Host, Boot: o.Boot, Namespace: o.Namespace, Run: o.Run, Clock: o.Clock, DeviceMap: o.DeviceMap, ClockGroup: o.ClockGroup, Offset: o.Offset, Uncertainty: o.Uncertainty, From: o.From, To: o.To}
	for k, v := range o.DeviceMap {
		if !g.Known(k) || !g.Known(v) {
			return set, fmt.Errorf("GPU_TRACE_DEVICE_MAP_INVALID")
		}
	}
	b := builder{ctx: ctx, set: set, source: source, tracks: map[string]bool{}, o: o}
	var e error
	switch o.Format {
	case "pytorch-trace-json":
		e = b.pytorch(body)
	case "nsight-sqlite":
		e = b.sqlite(body)
	case "nsight-summary-csv":
		e = b.summary(body)
	default:
		e = fmt.Errorf("GPU_TRACE_FORMAT_REQUIRED")
	}
	if e != nil {
		return set, e
	}
	b.set.Sources = []g.Source{b.source}
	return b.set, b.set.Validate(ctx)
}

type builder struct {
	ctx    context.Context
	set    g.Set
	source g.Source
	tracks map[string]bool
	o      Options
}

func (b *builder) scope() error {
	b.source.ID = g.SourceID(b.source)
	s := g.Empty()
	s.Sources = []g.Source{b.source}
	return s.Validate(b.ctx)
}
func (b *builder) add(t g.Track, e g.Event) error {
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if b.source.From != nil {
		if !g.Intersects(e.From, e.To, *b.source.From, *b.source.To) {
			return nil
		}
	}
	if len(b.set.Events) >= g.MaxEvents {
		return fmt.Errorf("GPU_TRACE_EVENT_BUDGET: at_least=%d limit=%d; supply a smaller explicit from_ns/to_ns window; intervals are never silently truncated", len(b.set.Events)+1, g.MaxEvents)
	}
	t.Source = b.source.ID
	t.ID = g.TrackID(t)
	if !b.tracks[t.ID] {
		if len(b.tracks) >= g.MaxTracks {
			return fmt.Errorf("GPU_TRACE_TRACK_BUDGET")
		}
		b.tracks[t.ID] = true
		b.set.Tracks = append(b.set.Tracks, t)
	}
	e.Track = t.ID
	e.Kind = t.Kind
	e.ID = g.EventID(e)
	b.set.Events = append(b.set.Events, e)
	return nil
}
