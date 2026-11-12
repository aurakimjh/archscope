package metrichistory

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const MaxInputBytes = 64 << 20

var slots = make(chan struct{}, 2)
var numeric = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

type Options struct {
	Format          string          `json:"format"`
	Producer        string          `json:"producer"`
	ProducerVersion string          `json:"producer_version"`
	ExportID        string          `json:"export_id"`
	Namespace       string          `json:"namespace"`
	Query           json.RawMessage `json:"query"`
}
type Inventory struct {
	SourceSHA256 string     `json:"source_sha256"`
	Sources      []m.Source `json:"sources"`
	Series       []m.Series `json:"series"`
	PointCount   int        `json:"point_count"`
	Duplicates   int        `json:"deduplicated_points"`
	Warnings     []string   `json:"warnings"`
}

func Inspect(set m.Set) Inventory {
	return Inventory{set.Sources[0].SHA256, set.Sources, set.Series, len(set.Points), set.Duplicates, InspectionWarnings(set)}
}
func DecodeOptions(raw []byte) (Options, error) {
	var out Options
	if len(raw) > 64<<10 {
		return out, fmt.Errorf("METRIC_OPTIONS_BUDGET")
	}
	return out, m.Strict(raw, &out)
}
func ParseFile(ctx context.Context, path string, options Options) (m.Set, error) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return m.Set{}, fmt.Errorf("METRIC_IMPORT_BUSY")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if stat, err := os.Stat(path); err == nil && stat.Size() > MaxInputBytes {
		return m.Set{}, fmt.Errorf("METRIC_BUDGET_EXCEEDED: original is %d bytes; limit is %d", stat.Size(), MaxInputBytes)
	}
	dir, e := os.MkdirTemp("", "metric-history-")
	if e != nil {
		return m.Set{}, e
	}
	defer os.RemoveAll(dir)
	lease, e := ingestion.Acquire(ctx, path, dir, "metric-source", ingestion.DefaultBudgets())
	if e != nil {
		return m.Set{}, e
	}
	defer lease.Close()
	r, e := lease.Open(ctx)
	if e != nil {
		return m.Set{}, e
	}
	defer r.Close()
	body, e := io.ReadAll(io.LimitReader(r, MaxInputBytes+1))
	if e != nil {
		return m.Set{}, e
	}
	set, e := ParseBytes(ctx, body, filepath.Base(path), options)
	if e != nil {
		return set, e
	}
	return set, lease.Verify(ctx)
}
func ParseBytes(ctx context.Context, original []byte, name string, options Options) (m.Set, error) {
	if len(original) > MaxInputBytes {
		return m.Set{}, fmt.Errorf("METRIC_BUDGET_EXCEEDED: original exceeds 64 MiB")
	}
	sum := sha256.Sum256(original)
	body := original
	if bytes.HasPrefix(body, []byte{31, 139}) {
		z, e := gzip.NewReader(bytes.NewReader(body))
		if e != nil {
			return m.Set{}, e
		}
		defer z.Close()
		body, e = io.ReadAll(io.LimitReader(z, MaxInputBytes+1))
		if e != nil {
			return m.Set{}, e
		}
		if len(body) > MaxInputBytes {
			return m.Set{}, fmt.Errorf("METRIC_BUDGET_EXCEEDED: decoded source exceeds 64 MiB")
		}
	}
	format := options.Format
	if format == "" || format == "auto" {
		var root map[string]json.RawMessage
		if e := wirejson.DecodeWithLimits(body, &root, wirejson.ResultLimits(MaxInputBytes, 128)); e != nil {
			return m.Set{}, e
		}
		if _, ok := root["contract"]; ok && string(root["contract"]) == `"apple-ollama-capture.v1"` {
			format = "apple-ollama-capture-json"
		} else if _, ok := root["resourceMetrics"]; ok {
			format = "otlp-json"
		} else if _, ok := root["snapshots"]; ok {
			format = "snapshot-sequence-json"
		} else if _, ok := root["data"]; ok {
			format = "prometheus-range-json"
		} else {
			return m.Set{}, fmt.Errorf("METRIC_FORMAT_REQUIRED")
		}
	}
	if len(options.Producer) > 256 || len(options.ProducerVersion) > 256 || len(options.Namespace) > 1024 || len(options.ExportID) > 1024 || len(options.Query) > 64<<10 {
		return m.Set{}, fmt.Errorf("METRIC_OPTIONS_BUDGET")
	}
	producer := options.Producer
	if producer == "" {
		producer = "unknown"
	}
	if len(options.Query) == 0 {
		options.Query = json.RawMessage(`{}`)
	}
	query, e := m.Canonical(options.Query)
	if e != nil {
		return m.Set{}, e
	}
	b := builder{ctx: ctx, set: m.Set{Version: m.Version, Normalizer: m.Normalizer, Sources: []m.Source{{ID: "source-0", SHA256: hex.EncodeToString(sum[:]), Name: name, Format: format, Producer: producer, ProducerVersion: options.ProducerVersion, ExportID: options.ExportID, Query: query}}, Series: []m.Series{}, Points: []m.Point{}, Warnings: []string{"Source producer/export/query identity is declared; no original-publisher authentication or GenAI alias qualification."}}, series: map[string]bool{}, origin: options.Namespace}
	if b.origin == "" {
		b.origin = format + ":" + producer + ":" + options.ProducerVersion
	} else {
		b.origin = format + ":" + b.origin
	}
	if format == "prometheus-range-json" {
		b.origin += ":query:" + m.Hash(query)
	}
	switch format {
	case "apple-ollama-capture-json":
		e = b.apple(body, options)
	case "otlp-json":
		e = b.otlp(body)
	case "snapshot-sequence-json":
		e = b.snapshots(body)
	case "prometheus-range-json":
		e = b.rangeQuery(body)
	default:
		e = fmt.Errorf("METRIC_FORMAT_UNSUPPORTED")
	}
	if e != nil {
		return b.set, e
	}
	return m.Normalize(ctx, b.set)
}

type builder struct {
	ctx             context.Context
	set             m.Set
	series          map[string]bool
	origin          string
	charge, buckets int
}

func (b *builder) addSeries(s m.Series) (string, error) {
	s.Origin = b.origin
	if len(s.Resource) == 0 {
		s.Resource = json.RawMessage(`{}`)
	}
	if len(s.Attributes) == 0 {
		s.Attributes = json.RawMessage(`{}`)
	}
	if len(s.Scope.Attributes) == 0 {
		s.Scope.Attributes = json.RawMessage(`{}`)
	}
	if s.Scope.Dropped == "" {
		s.Scope.Dropped = "0"
	}
	s.ID = m.SeriesID(s)
	if !b.series[s.ID] {
		if len(b.set.Series) >= m.MaxSeries {
			return "", fmt.Errorf("METRIC_BUDGET_EXCEEDED: series exceeds %d", m.MaxSeries)
		}
		raw, _ := json.Marshal(s)
		b.charge += len(raw)
		b.series[s.ID] = true
		b.set.Series = append(b.set.Series, s)
	}
	return s.ID, nil
}
func (b *builder) addPoint(p m.Point, position string) error {
	if e := b.ctx.Err(); e != nil {
		return e
	}
	if len(b.set.Points) >= m.MaxPoints {
		return fmt.Errorf("METRIC_BUDGET_EXCEEDED: points exceed %d; reduce export scope", m.MaxPoints)
	}
	p.Refs = []m.Ref{{SourceID: "source-0", Position: position}}
	if p.Histogram != nil {
		b.buckets += len(p.Histogram.Counts)
		for _, side := range []*m.Buckets{p.Histogram.Positive, p.Histogram.Negative} {
			if side != nil {
				b.buckets += len(side.Counts)
			}
		}
		if b.buckets > m.MaxTotalBuckets {
			return fmt.Errorf("METRIC_BUDGET_EXCEEDED: bucket total")
		}
	}
	raw, _ := json.Marshal(p)
	b.charge += len(raw)
	if b.charge > m.MaxBytes {
		return fmt.Errorf("METRIC_BUDGET_EXCEEDED: normalized model exceeds 32 MiB")
	}
	b.set.Points = append(b.set.Points, p)
	return nil
}
func decimal(raw json.RawMessage) (*string, string, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil, "", fmt.Errorf("METRIC_NUMBER_REQUIRED")
	}
	if strings.HasPrefix(text, `"`) {
		if e := json.Unmarshal(raw, &text); e != nil {
			return nil, "", e
		}
	}
	switch text {
	case "NaN":
		return nil, "nan", nil
	case "+Inf", "Infinity", "+Infinity", "Inf":
		return nil, "+inf", nil
	case "-Inf", "-Infinity":
		return nil, "-inf", nil
	}
	if len(text) > 1024 || !numeric.MatchString(text) {
		return nil, "", fmt.Errorf("METRIC_NUMBER_INVALID")
	}
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		n, e := strconv.Atoi(text[i+1:])
		if e != nil || n < -128 || n > 128 {
			return nil, "", fmt.Errorf("METRIC_NUMBER_EXPONENT_BUDGET")
		}
	}
	number, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, "", fmt.Errorf("METRIC_NUMBER_INVALID")
	}
	out := m.Text(number)
	if _, e := m.Number(out); e != nil {
		return nil, "", e
	}
	return &out, "", nil
}
func integer(raw json.RawMessage, signed bool, zeroDefault bool) (string, error) {
	if len(raw) == 0 && zeroDefault {
		return "0", nil
	}
	value, special, e := decimal(raw)
	if e != nil || special != "" || value == nil {
		return "", fmt.Errorf("METRIC_INTEGER_INVALID")
	}
	n, _ := m.Number(*value)
	if !n.IsInt() {
		return "", fmt.Errorf("METRIC_INTEGER_INVALID")
	}
	text := n.Num().String()
	if signed {
		if _, e = strconv.ParseInt(text, 10, 64); e != nil {
			return "", e
		}
	} else {
		if _, e = strconv.ParseUint(text, 10, 64); e != nil {
			return "", e
		}
	}
	return text, nil
}

// InspectionWarnings adds a read-time coverage notice without changing stored
// model/result projections or their hashes.
func InspectionWarnings(set m.Set) []string {
	out := append([]string(nil), set.Warnings...)
	for _, source := range set.Sources {
		if source.Format != "apple-ollama-capture-json" {
			continue
		}
		var provenance struct {
			Collector string `json:"collector_version"`
		}
		_ = json.Unmarshal(source.Query, &provenance)
		if provenance.Collector == "1.0.0" {
			out = append(out, "Legacy Apple collector1.0.0 captures contain a selected model subset; excluded-model count and selected-model load state are unknown. Do not read models as the complete loaded inventory.")
			break
		}
	}
	return out
}
