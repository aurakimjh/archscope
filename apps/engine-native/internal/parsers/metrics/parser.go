package metrics

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

var errStopIteration = errors.New("stop metrics iteration")

type Sample struct {
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp string
	RawLine   string
}

type Options struct {
	MaxLines int
	Strict   bool
}

const (
	ReasonNoMetricMatch = "NO_METRIC_MATCH"
	ReasonInvalidNumber = "INVALID_NUMBER"
)

var metricRE = regexp.MustCompile(`^([A-Za-z_:][A-Za-z0-9_:]*)(?:\{([^}]*)\})?\s+([-+eE0-9.]+|NaN|Inf|\+Inf|-Inf)(?:\s+(\d+))?$`)

func ParseLine(line string) (*Sample, *parseError) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return nil, nil
	}
	if trimmed == "# EOF" {
		return nil, nil
	}
	m := metricRE.FindStringSubmatch(trimmed)
	if m == nil {
		return nil, &parseError{Reason: ReasonNoMetricMatch, Message: "line is not Prometheus/OpenMetrics sample syntax"}
	}
	value, err := strconv.ParseFloat(m[3], 64)
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidNumber, Message: err.Error()}
	}
	return &Sample{Name: m[1], Labels: parseLabels(m[2]), Value: value, Timestamp: m[4], RawLine: line}, nil
}

type parseError struct {
	Reason  string
	Message string
}

func ParseFile(path string, opts Options) ([]Sample, *diagnostics.ParserDiagnostics, error) {
	samples := []Sample{}
	diags, err := ForEachSample(path, opts, func(sample Sample) error {
		samples = append(samples, sample)
		return nil
	})
	return samples, diags, err
}

func ForEachSample(path string, opts Options, fn func(Sample) error) (*diagnostics.ParserDiagnostics, error) {
	if opts.MaxLines < 0 {
		return nil, fmt.Errorf("max_lines must be a positive integer")
	}
	diags := diagnostics.New("openmetrics")
	diags.SetSourceFile(path)
	err := textio.ForEachTextLine(path, "", func(lineNumber int, line string) error {
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			return errStopIteration
		}
		diags.TotalLines++
		sample, perr := ParseLine(line)
		if sample != nil {
			diags.ParsedRecords++
			return fn(*sample)
		}
		if perr != nil {
			diags.AddSkipped(lineNumber, perr.Reason, perr.Message, line)
			if opts.Strict {
				return fmt.Errorf("%s:%d: %s: %s", path, lineNumber, perr.Reason, perr.Message)
			}
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return diags, err
	}
	return diags, nil
}

func parseLabels(raw string) map[string]string {
	out := map[string]string{}
	for raw != "" {
		raw = strings.TrimSpace(raw)
		idx := strings.Index(raw, "=")
		if idx <= 0 {
			break
		}
		key := strings.TrimSpace(raw[:idx])
		raw = strings.TrimSpace(raw[idx+1:])
		if !strings.HasPrefix(raw, `"`) {
			break
		}
		raw = raw[1:]
		end := strings.Index(raw, `"`)
		if end < 0 {
			break
		}
		out[key] = raw[:end]
		raw = strings.TrimLeft(raw[end+1:], ", ")
	}
	return out
}
