// Package inference normalizes offline device files or explicit metric bindings.
package inference

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

const Fields = "uuid,pci.bus_id,name,driver_version,utilization.gpu,utilization.memory,memory.used,memory.total,temperature.gpu,power.draw,clocks.sm"

type Capture struct {
	Contract string    `json:"contract"`
	Host     string    `json:"host_id"`
	Boot     string    `json:"boot_id"`
	Version  string    `json:"producer_version"`
	Fields   string    `json:"query_fields"`
	Times    []*string `json:"timestamps_ns"`
}
type Options struct {
	Format    string          `json:"format"`
	Capture   *Capture        `json:"capture,omitempty"`
	Bindings  json.RawMessage `json:"bindings,omitempty"`
	SeriesIDs []string        `json:"retained_series_ids,omitempty"`
}

var slots = make(chan struct{}, 2)

func DecodeOptions(raw []byte) (Options, error) {
	var o Options
	if len(raw) > 128<<10 {
		return o, fmt.Errorf("INFERENCE_OPTIONS_BUDGET")
	}
	return o, n.Strict(raw, &o)
}
func Digest(raw []byte) string { s := sha256.Sum256(raw); return hex.EncodeToString(s[:]) }
func ParseFile(parent context.Context, path string, o Options) (n.Set, string, error) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return n.Set{}, "", fmt.Errorf("INFERENCE_IMPORT_BUSY")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	dir, e := os.MkdirTemp("", "inference-source-")
	if e != nil {
		return n.Set{}, "", e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "inference-original", ingestion.DefaultBudgets())
	if e != nil {
		return n.Set{}, "", e
	}
	defer lease.Close()
	f, e := lease.Open(ctx)
	if e != nil {
		return n.Set{}, "", e
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if e != nil {
		return n.Set{}, "", e
	}
	s, e := ParseBytes(ctx, raw, filepath.Base(path), o)
	if e == nil {
		e = lease.Verify(ctx)
	}
	return s, Digest(raw), e
}
func ParseBytes(ctx context.Context, original []byte, name string, o Options) (n.Set, error) {
	var empty n.Set
	if len(original) > 64<<20 {
		return empty, fmt.Errorf("INFERENCE_SOURCE_BUDGET")
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	encoded, e := json.Marshal(o)
	if e != nil {
		return empty, e
	}
	if len(encoded) > 128<<10 {
		return empty, fmt.Errorf("INFERENCE_OPTIONS_BUDGET")
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
			return empty, fmt.Errorf("INFERENCE_EXPANDED_BUDGET")
		}
	}
	if !utf8.Valid(body) {
		return empty, fmt.Errorf("INFERENCE_ENCODING_INVALID: UTF-8 required")
	}
	switch o.Format {
	case "inference-evidence-json":
		if o.Capture != nil || len(o.Bindings) != 0 {
			return empty, fmt.Errorf("INFERENCE_OPTIONS_INVALID")
		}
		set, e := n.Decode(ctx, body)
		if e != nil {
			return empty, e
		}
		return n.SelectSeries(ctx, set, o.SeriesIDs)
	case "metric-bindings-json":
		if o.Capture != nil {
			return empty, fmt.Errorf("INFERENCE_OPTIONS_INVALID")
		}
		metrics, e := m.Decode(body)
		if e != nil {
			return empty, e
		}
		var keys map[string]json.RawMessage
		if e = n.Strict(o.Bindings, &keys); e != nil {
			return empty, e
		}
		if keys == nil || keys["metrics"] != nil {
			return empty, fmt.Errorf("INFERENCE_BINDING_INVALID: supply manifest without metrics")
		}
		var s n.Set
		retained := o.SeriesIDs
		if raw, ok := keys["retained_series_ids"]; ok {
			if len(retained) > 0 {
				return empty, fmt.Errorf("INFERENCE_SELECTION_DUPLICATE")
			}
			if e = n.Strict(raw, &retained); e != nil {
				return empty, e
			}
			delete(keys, "retained_series_ids")
		}
		bindingBody, _ := json.Marshal(keys)
		if e = n.Strict(bindingBody, &s); e != nil {
			return empty, e
		}
		if s.Version != n.Version && s.Version != n.LegacyVersion {
			return empty, fmt.Errorf("INFERENCE_VERSION_UNSUPPORTED")
		}
		s.Metrics = metrics
		for i := range s.Events {
			if s.Events[i].ID == "" {
				s.Events[i].ID = n.EventID(s.Events[i])
			}
		}
		for _, signal := range s.Signals {
			if signal.Declaration != nil {
				s.Version = n.Version
			}
		}
		if e = s.Validate(ctx); e != nil {
			return empty, e
		}
		return n.SelectSeries(ctx, s, retained)
	case "ollama-generate-final-json":
		if o.Capture != nil || len(o.Bindings) != 0 {
			return empty, fmt.Errorf("INFERENCE_OPTIONS_INVALID")
		}
		set, err := parseOllama(ctx, body, Digest(original), name)
		if err != nil {
			return empty, err
		}
		return n.SelectSeries(ctx, set, o.SeriesIDs)
	case "nvidia-smi-csv":
		if o.Capture == nil || len(o.Bindings) != 0 {
			return empty, fmt.Errorf("INFERENCE_CAPTURE_REQUIRED")
		}
		set, e := parseCSV(ctx, body, Digest(original), name, *o.Capture)
		if e != nil {
			return empty, e
		}
		return n.SelectSeries(ctx, set, o.SeriesIDs)
	default:
		return empty, fmt.Errorf("INFERENCE_FORMAT_UNSUPPORTED: select explicit source format")
	}
}

var number = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func parseCSV(ctx context.Context, body []byte, digest, name string, c Capture) (n.Set, error) {
	s := n.Set{Version: n.Version, Entities: []n.Entity{}, Links: []n.Link{}, Contexts: []n.Context{}, Signals: []n.Signal{}, Sources: []m.Source{}, Events: []n.Event{}, Quality: []n.Quality{}}
	if c.Contract != "nvidia-smi.query-csv.v1" || c.Fields != Fields || c.Host == "" || c.Boot == "" || c.Version == "" || c.Version == "unknown" {
		return s, fmt.Errorf("INFERENCE_CAPTURE_INVALID: exact query/format, declared version and host/boot required")
	}
	provenance, _ := json.Marshal(c)
	s.Metrics = m.Set{Version: m.Version, Normalizer: m.Normalizer, Sources: []m.Source{{ID: "nvidia-source", SHA256: digest, Name: name, Format: "nvidia-smi.query-csv.v1", Producer: "nvidia-smi", ProducerVersion: c.Version, Query: provenance}}, Series: []m.Series{}, Points: []m.Point{}, Warnings: []string{"Declared driver/version/query layout; synthetic fixture qualification only, no native GPU execution. No mtime clock, workload attribution or utilization efficiency inference."}}
	definitions := []struct{ name, semantic, unit, conversion string }{{"utilization.gpu", "gpu_utilization", "%", "percent_to_ratio"}, {"utilization.memory", "memory_io_utilization", "%", "percent_to_ratio"}, {"memory.used", "device_memory_used", "MiB", "mib_to_bytes"}, {"memory.total", "device_memory_capacity", "MiB", "mib_to_bytes"}, {"temperature.gpu", "temperature", "Cel", "identity"}, {"power.draw", "power", "W", "identity"}, {"clocks.sm", "clock", "MHz", "identity"}}
	seen := map[string]bool{}
	seriesSeen := map[string]bool{}
	entityPCI := map[string]string{}
	pciDevice := map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	line := 0
	dataRow := 0
	trailingBlank := false
	for scanner.Scan() {
		line++
		if strings.TrimSpace(scanner.Text()) == "" {
			trailingBlank = true
			continue
		}
		if trailingBlank {
			return s, fmt.Errorf("INFERENCE_CSV_BLANK_INTERIOR line=%d", line)
		}
		dataRow++
		if e := ctx.Err(); e != nil {
			return s, e
		}
		if len(s.Metrics.Points)+len(definitions) > m.MaxPoints {
			return s, fmt.Errorf("INFERENCE_POINT_BUDGET: split source")
		}
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		reader := csv.NewReader(strings.NewReader(raw))
		reader.TrimLeadingSpace = true
		reader.FieldsPerRecord = 11
		fields, e := reader.Read()
		if e != nil {
			return s, fmt.Errorf("INFERENCE_CSV_INVALID line=%d: %w", line, e)
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if !strings.HasPrefix(fields[0], "GPU-") || len(fields[0]) <= 4 || fields[1] == "" || fields[1] == "N/A" || fields[2] == "" || fields[3] != c.Version {
			return s, fmt.Errorf("INFERENCE_CSV_IDENTITY_INVALID line=%d", line)
		}
		entityID := "device-" + m.Hash([]string{c.Host, c.Boot, fields[0]})
		if previous, ok := entityPCI[entityID]; ok && previous != fields[1] {
			return s, fmt.Errorf("INFERENCE_DEVICE_MAPPING_CONFLICT line=%d", line)
		}
		if id, ok := pciDevice[fields[1]]; ok && id != entityID {
			return s, fmt.Errorf("INFERENCE_DEVICE_MAPPING_CONFLICT line=%d", line)
		}
		entityPCI[entityID] = fields[1]
		pciDevice[fields[1]] = entityID
		if !seen[entityID] {
			s.Entities = append(s.Entities, n.Entity{ID: entityID, Kind: "device", Host: c.Host, Boot: c.Boot, Vendor: "nvidia", DeviceUUID: fields[0], PCI: fields[1]})
			seen[entityID] = true
			if len(s.Entities) > n.MaxEntities {
				return s, fmt.Errorf("INFERENCE_ENTITY_BUDGET")
			}
		}
		var end *string
		if len(c.Times) > 0 {
			if dataRow > len(c.Times) {
				return s, fmt.Errorf("INFERENCE_TIMESTAMP_ARITY")
			}
			end = c.Times[dataRow-1]
		}
		for j, d := range definitions {
			resource, _ := json.Marshal(map[string]string{"host_id": c.Host, "boot_id": c.Boot, "device_uuid": fields[0], "pci_bus_id": fields[1], "device_name": fields[2]})
			series := m.Series{Name: d.name, Unit: d.unit, Kind: "gauge", Temporality: "unspecified", Origin: "nvidia-smi.query-csv.v1:" + c.Version, Resource: resource, Attributes: json.RawMessage(`{}`), Scope: m.Scope{Name: "nvidia-smi.query-csv.v1", Version: c.Version, Attributes: json.RawMessage(`{}`), Dropped: "0"}}
			series.ID = m.SeriesID(series)
			if !seriesSeen[series.ID] {
				s.Metrics.Series = append(s.Metrics.Series, series)
				s.Signals = append(s.Signals, n.Signal{ID: "signal-" + series.ID, SeriesID: series.ID, Semantic: d.semantic, EntityID: entityID, Boundary: "device", Population: "whole-device", Form: "gauge", Conversion: d.conversion})
				seriesSeen[series.ID] = true
			}
			p := m.Point{SeriesID: series.ID, End: end, Refs: []m.Ref{{SourceID: "nvidia-source", Position: fmt.Sprintf("line:%d/column:%d", line, j+5)}}}
			value := fields[j+4]
			status := ""
			switch value {
			case "N/A", "[N/A]":
				status = "not_available"
			case "[Not Supported]":
				status = "not_supported"
			case "[Insufficient Permissions]":
				status = "permission_denied"
			default:
				if len(value) > 128 || !number.MatchString(value) {
					return s, fmt.Errorf("INFERENCE_CSV_VALUE_INVALID line=%d column=%d field=%s value=%.128q", line, j+5, d.name, value)
				}
				if _, e = m.Number(value); e != nil {
					return s, e
				}
				p.Value = m.Pointer(value)
			}
			if status != "" {
				p.Flags = 1
				p.Special = "no_recorded_value"
			}
			p.ID = m.PointID(p)
			s.Metrics.Points = append(s.Metrics.Points, p)
			if status != "" {
				s.Quality = append(s.Quality, n.Quality{PointID: p.ID, Status: status, Raw: value})
			}
		}
	}
	if e := scanner.Err(); e != nil {
		return s, e
	}
	if dataRow == 0 || len(c.Times) > 0 && len(c.Times) != dataRow {
		return s, fmt.Errorf("INFERENCE_TIMESTAMP_ARITY: %s rows", strconv.Itoa(dataRow))
	}
	return s, s.Validate(ctx)
}
