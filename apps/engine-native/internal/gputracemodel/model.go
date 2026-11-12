// Package gputracemodel preserves scoped execution intervals; it owns no collector.
package gputracemodel

import (
	"context"
	"encoding/json"
	"fmt"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"math/big"
	"strings"
	"unicode/utf8"
)

const Version = "1.0.0"
const MaxBytes = 32 << 20
const MaxEvents = 50000
const MaxTracks = 4096

type Source struct {
	DeviceMap   map[string]string `json:"device_map"`
	ID          string            `json:"id"`
	SHA         string            `json:"sha256"`
	Format      string            `json:"format"`
	Producer    string            `json:"producer"`
	Version     string            `json:"producer_version"`
	Schema      string            `json:"export_schema"`
	Capture     string            `json:"capture_id"`
	Host        string            `json:"host_id"`
	Boot        string            `json:"boot_id"`
	Namespace   string            `json:"process_namespace"`
	Run         string            `json:"run_id"`
	Clock       string            `json:"clock_id"`
	ClockGroup  string            `json:"clock_group,omitempty"`
	Offset      *string           `json:"clock_offset_ns,omitempty"`
	Uncertainty *string           `json:"clock_uncertainty_ns,omitempty"`
	Rank        *string           `json:"rank"`
	WorldSize   *string           `json:"world_size"`
	Backend     string            `json:"distributed_backend"`
	From        *string           `json:"selection_from_ns"`
	To          *string           `json:"selection_to_ns"`
}
type Track struct {
	DeviceIndex string `json:"device_local_index"`
	ID          string `json:"id"`
	Source      string `json:"source_id"`
	Kind        string `json:"kind"`
	Process     string `json:"process_id"`
	Thread      string `json:"thread_id"`
	Device      string `json:"device_id"`
	Context     string `json:"context_id"`
	Stream      string `json:"stream_id"`
}
type Event struct {
	ID          string `json:"id"`
	Track       string `json:"track_id"`
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	From        string `json:"from_ns"`
	To          string `json:"to_ns"`
	Correlation string `json:"correlation_id"`
	External    string `json:"external_id"`
	Flow        string `json:"flow_key"`
	Phase       string `json:"flow_phase"`
	Position    string `json:"source_position"`
}
type Summary struct {
	Source    string `json:"source_id"`
	Name      string `json:"name"`
	Statistic string `json:"statistic"`
	Value     string `json:"value"`
	Unit      string `json:"unit"`
	Position  string `json:"source_position"`
}
type Set struct {
	Version   string    `json:"schema_version"`
	Sources   []Source  `json:"sources"`
	Tracks    []Track   `json:"tracks"`
	Events    []Event   `json:"events"`
	Warnings  []string  `json:"warnings"`
	Summaries []Summary `json:"summaries"`
}

func Empty() Set {
	return Set{Version: Version, Sources: []Source{}, Tracks: []Track{}, Events: []Event{}, Warnings: []string{}, Summaries: []Summary{}}
}
func Hash(v any) string { return m.Hash(v) }
func Text(s string) bool {
	return utf8.ValidString(s) && len(s) <= 1024 && !strings.ContainsAny(s, "\x00\r\n")
}
func Known(s string) bool { return Text(s) && s != "" && s != "unknown" }
func Uint(s string) bool {
	if len(s) > 20 {
		return false
	}
	_, e := m.Uint(s)
	return e == nil
}
func In(s string, items ...string) bool {
	for _, v := range items {
		if s == v {
			return true
		}
	}
	return false
}
func Strict(b []byte, v any, limit int) error {
	if len(b) > limit {
		return fmt.Errorf("GPU_TRACE_BYTES_BUDGET: bytes=%d limit=%d", len(b), limit)
	}
	return m.Strict(b, v)
}
func SourceID(v Source) string { v.ID = ""; return "gpu-source-" + Hash(v) }
func TrackID(v Track) string   { v.ID = ""; return "gpu-track-" + Hash(v) }
func EventID(v Event) string   { v.ID = ""; return "gpu-event-" + Hash(v) }
func Decode(ctx context.Context, b []byte) (Set, error) {
	var s Set
	if e := Strict(b, &s, MaxBytes); e != nil {
		return s, e
	}
	return s, s.Validate(ctx)
}
func (s Set) Validate(ctx context.Context) error {
	if s.Version != Version || len(s.Sources) < 1 || len(s.Sources) > 16 || len(s.Tracks) > MaxTracks || len(s.Events) > MaxEvents || len(s.Warnings) > 128 || len(s.Summaries) > 4096 {
		return fmt.Errorf("GPU_TRACE_MODEL_BUDGET_OR_VERSION")
	}
	sources := map[string]Source{}
	tracks := map[string]Track{}
	events := map[string]bool{}
	positions := map[string]bool{}
	for _, v := range s.Sources {
		if v.ID != SourceID(v) || sources[v.ID].ID != "" || len(v.SHA) != 64 || !Known(v.Format) || !Known(v.Producer) || !Known(v.Version) {
			return fmt.Errorf("GPU_TRACE_SOURCE_INVALID")
		}
		for _, r := range v.SHA {
			if !strings.ContainsRune("0123456789abcdef", r) {
				return fmt.Errorf("GPU_TRACE_SOURCE_HASH")
			}
		}
		for _, x := range []string{v.Schema, v.Capture, v.Host, v.Boot, v.Namespace, v.Run, v.Clock} {
			if !Known(x) {
				return fmt.Errorf("GPU_TRACE_SOURCE_SCOPE_REQUIRED")
			}
		}
		if v.Rank != nil || v.WorldSize != nil {
			if v.Rank == nil || v.WorldSize == nil || !Uint(*v.Rank) || !Uint(*v.WorldSize) || *v.WorldSize == "0" || Cmp(*v.Rank, *v.WorldSize) >= 0 || !Known(v.Backend) {
				return fmt.Errorf("GPU_TRACE_RANK_INVALID")
			}
		}
		if (v.From == nil) != (v.To == nil) || v.From != nil && (!Uint(*v.From) || !Uint(*v.To) || Cmp(*v.From, *v.To) >= 0) {
			return fmt.Errorf("GPU_TRACE_SELECTION_INVALID")
		}
		if v.ClockGroup != "" || v.Offset != nil || v.Uncertainty != nil {
			if !Known(v.ClockGroup) || v.Offset == nil || v.Uncertainty == nil || !Uint(*v.Uncertainty) {
				return fmt.Errorf("GPU_TRACE_CLOCK_MAPPING_REQUIRED")
			}
			if len(*v.Offset) > 21 {
				return fmt.Errorf("GPU_TRACE_CLOCK_OFFSET_INVALID")
			}
			n, ok := new(big.Int).SetString(*v.Offset, 10)
			if !ok || n.String() != *v.Offset || n.BitLen() > 64 {
				return fmt.Errorf("GPU_TRACE_CLOCK_OFFSET_INVALID")
			}
		}
		if !Text(v.Backend) {
			return fmt.Errorf("GPU_TRACE_BACKEND_INVALID")
		}
		if len(v.DeviceMap) > 128 {
			return fmt.Errorf("GPU_TRACE_DEVICE_MAP_BUDGET")
		}
		for index, id := range v.DeviceMap {
			if !Uint(index) || !Known(id) {
				return fmt.Errorf("GPU_TRACE_DEVICE_MAP_INVALID")
			}
		}
		sources[v.ID] = v
	}
	for _, v := range s.Tracks {
		if v.ID != TrackID(v) || tracks[v.ID].ID != "" || sources[v.Source].ID == "" || !In(v.Kind, "cpu", "cuda_api", "gpu_kernel", "memcpy", "annotation", "communication", "flow") || !Known(v.Process) {
			return fmt.Errorf("GPU_TRACE_TRACK_INVALID")
		}
		for _, x := range []string{v.Thread, v.Device, v.Context, v.Stream, v.Process} {
			if !Text(x) {
				return fmt.Errorf("GPU_TRACE_TRACK_IDENTITY")
			}
		}
		if In(v.Kind, "gpu_kernel", "memcpy") && (!Known(v.Device) || !Known(v.Context) || !Known(v.Stream) || sources[v.Source].DeviceMap[v.DeviceIndex] != v.Device) {
			return fmt.Errorf("GPU_TRACE_GPU_SCOPE_REQUIRED")
		}
		tracks[v.ID] = v
	}
	for _, v := range s.Events {
		if err := ctx.Err(); err != nil {
			return err
		}
		t := tracks[v.Track]
		if v.ID != EventID(v) || events[v.ID] || t.ID == "" || !In(v.Kind, "cpu", "cuda_api", "gpu_kernel", "memcpy", "annotation", "communication", "flow") || v.Kind != t.Kind || !Text(v.Name) || !Uint(v.From) || !Uint(v.To) || Cmp(v.From, v.To) > 0 || !Known(v.Position) {
			return fmt.Errorf("GPU_TRACE_EVENT_INVALID")
		}
		for _, x := range []string{v.Correlation, v.External, v.Flow} {
			if !Text(x) {
				return fmt.Errorf("GPU_TRACE_EVENT_IDENTITY")
			}
		}
		if v.Kind == "flow" {
			if v.From != v.To || !Known(v.Flow) || !In(v.Phase, "s", "f") {
				return fmt.Errorf("GPU_TRACE_FLOW_INVALID")
			}
		} else if v.Phase != "" || v.Flow != "" {
			return fmt.Errorf("GPU_TRACE_FLOW_SCOPE")
		}
		src := sources[t.Source]
		if src.From != nil && !Intersects(v.From, v.To, *src.From, *src.To) {
			return fmt.Errorf("GPU_TRACE_SELECTION_MISMATCH")
		}
		ref := t.Source + "/" + v.Position
		if positions[ref] {
			return fmt.Errorf("GPU_TRACE_DUPLICATE_SOURCE_POSITION")
		}
		positions[ref] = true
		events[v.ID] = true
	}
	for _, v := range s.Summaries {
		if len(v.Value) > 256 {
			return fmt.Errorf("GPU_TRACE_SUMMARY_NUMBER_BUDGET")
		}
		n, e := m.Number(v.Value)
		if e != nil || n.Sign() < 0 || v.Unit == "count" && !Uint(v.Value) || v.Unit == "%" && n.Cmp(big.NewRat(100, 1)) > 0 || sources[v.Source].ID == "" || !Known(v.Name) || !Known(v.Position) || !In(v.Unit, "ns", "count", "%") || !In(v.Statistic, "total", "count", "mean", "median", "min", "max", "stddev", "duration_share") {
			return fmt.Errorf("GPU_TRACE_SUMMARY_INVALID")
		}
	}
	for _, w := range s.Warnings {
		if !Text(w) {
			return fmt.Errorf("GPU_TRACE_WARNING_BUDGET")
		}
	}
	raw, _ := json.Marshal(s)
	if len(raw) > MaxBytes {
		return fmt.Errorf("GPU_TRACE_MODEL_BYTES_BUDGET: bytes=%d limit=%d; select a smaller source window", len(raw), MaxBytes)
	}
	return ctx.Err()
}
func Cmp(a, b string) int {
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	return x.Cmp(y)
}
func Difference(a, b string) string {
	x, _ := new(big.Int).SetString(a, 10)
	y, _ := new(big.Int).SetString(b, 10)
	return new(big.Int).Sub(x, y).String()
}
func Merge(ctx context.Context, sets []Set) (Set, error) {
	out := Empty()
	seen := map[string]bool{}
	for _, s := range sets {
		if e := s.Validate(ctx); e != nil {
			return out, e
		}
		for _, src := range s.Sources {
			if seen[src.ID] {
				return out, fmt.Errorf("GPU_TRACE_DUPLICATE_SOURCE: no implicit cross-export deduplication")
			}
			seen[src.ID] = true
		}
		if len(out.Sources)+len(s.Sources) > 16 || len(out.Events)+len(s.Events) > MaxEvents || len(out.Tracks)+len(s.Tracks) > MaxTracks || len(out.Summaries)+len(s.Summaries) > 4096 {
			return out, fmt.Errorf("GPU_TRACE_MERGE_BUDGET")
		}
		out.Sources = append(out.Sources, s.Sources...)
		out.Tracks = append(out.Tracks, s.Tracks...)
		out.Events = append(out.Events, s.Events...)
		out.Summaries = append(out.Summaries, s.Summaries...)
		for _, w := range s.Warnings {
			if len(out.Warnings) < 128 {
				out.Warnings = append(out.Warnings, w)
			}
		}
	}
	return out, out.Validate(ctx)
}

// Intersects uses a half-open selection; zero-duration observations are points.
func Intersects(from, to, low, high string) bool {
	if from == to {
		return Cmp(from, low) >= 0 && Cmp(from, high) < 0
	}
	return Cmp(from, high) < 0 && Cmp(to, low) > 0
}
