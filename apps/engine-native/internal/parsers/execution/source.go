package execution

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/executionmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"
)

const LLMPerfRevision = "f1d6bed47e4501b0e371082b41601b59ab55269f"

type Options struct {
	Format          string     `json:"format"`
	ProducerVersion string     `json:"producer_version,omitempty"`
	Context         *e.Context `json:"context,omitempty"`
}

func DecodeOptions(raw []byte) (Options, error) { var o Options; return o, e.Strict(raw, &o, 64<<10) }
func Digest(raw []byte) string                  { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }

var slots = make(chan struct{}, 2)

func ParseFile(parent context.Context, path string, o Options) (e.Set, string, error) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		return e.Set{}, "", fmt.Errorf("EXECUTION_IMPORT_BUSY")
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "execution-source-")
	if err != nil {
		return e.Set{}, "", err
	}
	defer os.RemoveAll(dir)
	lease, err := ingestion.Acquire(ctx, path, dir, "original", ingestion.DefaultBudgets())
	if err != nil {
		return e.Set{}, "", err
	}
	defer lease.Close()
	f, err := lease.Open(ctx)
	if err != nil {
		return e.Set{}, "", err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if err != nil {
		return e.Set{}, "", err
	}
	set, err := ParseBytes(ctx, raw, o)
	if err == nil {
		err = lease.Verify(ctx)
	}
	return set, Digest(raw), err
}
func ParseBytes(ctx context.Context, original []byte, o Options) (e.Set, error) {
	s := e.Empty()
	if len(original) > 64<<20 {
		return s, fmt.Errorf("EXECUTION_SOURCE_BYTES_BUDGET")
	}
	raw := original
	if bytes.HasPrefix(raw, []byte{31, 139}) {
		z, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return s, err
		}
		defer z.Close()
		raw, err = io.ReadAll(io.LimitReader(z, 64<<20+1))
		if err != nil {
			return s, err
		}
		if len(raw) > 64<<20 {
			return s, fmt.Errorf("EXECUTION_EXPANDED_BYTES_BUDGET")
		}
	}
	if o.Format == "execution-evidence-json" {
		if o.Context != nil || o.ProducerVersion != "" {
			return s, fmt.Errorf("EXECUTION_CANONICAL_OPTIONS_INVALID")
		}
		return e.Decode(ctx, raw)
	}
	s.Source = e.Source{SHA: Digest(original), Format: o.Format, Version: o.ProducerVersion}
	s.Limitations = []string{"Source/caller conditions are declarations, not authenticated provenance or experimental control.", "No load generator, endpoint calls, core/heap dump engine or causal/root-cause diagnosis.", "Only selected numeric fields are normalized. Bodies, URLs, arbitrary names, environment, arguments and error messages are excluded; sanitize technical identity first.", "Percentiles are retained per source population/policy; never averaged or added. Missing measurements remain null."}
	var err error
	if o.Format == "hs-err-text" {
		s.Kind = "jvm_crash"
		if o.Context != nil {
			return s, fmt.Errorf("EXECUTION_CRASH_NOT_BENCHMARK")
		}
		err = parseCrash(ctx, raw, &s)
		if err == nil && o.ProducerVersion != "" && s.Source.Version != o.ProducerVersion {
			return s, fmt.Errorf("EXECUTION_CRASH_VERSION_CONFLICT")
		}
	} else {
		if o.Context == nil {
			return s, fmt.Errorf("EXECUTION_COLLECTION_CONTEXT_REQUIRED")
		}
		s.Kind = "benchmark"
		c := e.JSONCopy(*o.Context)
		s.Context = &c
		switch o.Format {
		case "k6-handle-summary-json":
			s.Source.Producer = "k6"
			if o.ProducerVersion != "1.3.0" {
				return s, fmt.Errorf("EXECUTION_K6_VERSION_UNQUALIFIED: pinned 1.3.0 handleSummary only")
			}
			err = parseK6(ctx, raw, &s)
		case "llmperf-summary-json":
			s.Source.Producer = "llmperf"
			if o.ProducerVersion != LLMPerfRevision {
				return s, fmt.Errorf("EXECUTION_LLMPERF_VERSION_UNQUALIFIED")
			}
			err = parseLLMPerf(ctx, raw, &s)
		case "jmeter-jtl-csv":
			s.Source.Producer = "Apache JMeter"
			if o.ProducerVersion != "5.6.3" {
				return s, fmt.Errorf("EXECUTION_JTL_VERSION_UNQUALIFIED: pinned synthetic 5.6.3 layout")
			}
			err = parseJTL(ctx, raw, &s)
		default:
			return s, fmt.Errorf("EXECUTION_SOURCE_FORMAT_UNSUPPORTED")
		}
	}
	if err != nil {
		return s, err
	}
	if err = ctx.Err(); err != nil {
		return s, err
	}
	return s, s.Validate(ctx)
}
func value(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if bytes.HasPrefix(bytes.TrimSpace(raw), []byte(`"`)) {
		return nil, fmt.Errorf("EXECUTION_SOURCE_NUMBER_INVALID")
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("EXECUTION_SOURCE_NUMBER_INVALID")
	}
	return decimal(n.String())
}
func decimal(raw string) (*string, error) {
	if len(raw) > 256 {
		return nil, fmt.Errorf("EXECUTION_SOURCE_NUMBER_BUDGET")
	}
	if idx := strings.IndexAny(raw, "eE"); idx >= 0 {
		power, err := strconv.Atoi(raw[idx+1:])
		if err != nil || power < -128 || power > 128 {
			return nil, fmt.Errorf("EXECUTION_SOURCE_EXPONENT_BUDGET")
		}
	}
	n, ok := new(big.Rat).SetString(raw)
	if !ok || n.Sign() < 0 {
		return nil, fmt.Errorf("EXECUTION_SOURCE_NUMBER_INVALID")
	}
	return e.Ptr(m.Text(n)), nil
}
func metric(s *e.Set, name, stat, unit, pop, policy, position string, raw json.RawMessage, scale int64) error {
	v, err := value(raw)
	if err != nil {
		return fmt.Errorf("EXECUTION_METRIC_VALUE_INVALID: %s", position)
	}
	original := v
	if v != nil && scale != 1 {
		n, _ := e.Number(*v)
		v = e.Ptr(m.Text(new(big.Rat).Quo(n, big.NewRat(scale, 1))))
	}
	originalUnit := unit
	if scale == 1000 {
		originalUnit = "milliseconds"
	}
	row := e.Metric{Name: name, Statistic: stat, Unit: unit, Population: pop, GroupSHA: e.Hash("whole selected export"), Policy: policy, Value: v, Original: original, OriginalUnit: originalUnit, Position: position}
	row.ID = e.MetricID(row)
	s.Metrics = append(s.Metrics, row)
	return nil
}
func object(raw []byte) (map[string]json.RawMessage, error) {
	var v map[string]json.RawMessage
	err := e.Strict(raw, &v, 64<<20)
	if v == nil && err == nil {
		err = fmt.Errorf("EXECUTION_OBJECT_REQUIRED")
	}
	return v, err
}
