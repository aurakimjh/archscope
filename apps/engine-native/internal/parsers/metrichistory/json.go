package metrichistory

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// Snapshot sequences declare metric semantics rather than guessing from a name.
type historyFile struct {
	Schema    string `json:"schema_version"`
	Snapshots []struct {
		Time    *string `json:"time_unix_nano"`
		Metrics []struct {
			Name        string          `json:"name"`
			Unit        string          `json:"unit"`
			Kind        string          `json:"kind"`
			Temporality string          `json:"temporality"`
			Monotonic   bool            `json:"monotonic"`
			Start       *string         `json:"start_time_unix_nano"`
			Attributes  json.RawMessage `json:"attributes"`
			Resource    json.RawMessage `json:"resource"`
			Scope       m.Scope         `json:"scope"`
			Value       json.RawMessage `json:"value"`
			Flags       uint32          `json:"flags"`
			Histogram   *m.Histogram    `json:"histogram"`
		} `json:"metrics"`
	} `json:"snapshots"`
}

func (b *builder) snapshots(body []byte) error {
	var input historyFile
	if e := m.Strict(body, &input); e != nil {
		return e
	}
	if input.Schema != "1.0.0" || input.Snapshots == nil {
		return fmt.Errorf("METRIC_HISTORY_VERSION_UNSUPPORTED")
	}
	for i, snapshot := range input.Snapshots {
		for j, metric := range snapshot.Metrics {
			attrs := metric.Attributes
			resource := metric.Resource
			var e error
			if len(attrs) > 0 {
				attrs, e = m.Canonical(attrs)
				if e != nil {
					return e
				}
			}
			if len(resource) > 0 {
				resource, e = m.Canonical(resource)
				if e != nil {
					return e
				}
			}
			scope := metric.Scope
			if len(scope.Attributes) > 0 {
				scope.Attributes, e = m.Canonical(scope.Attributes)
				if e != nil {
					return e
				}
			}
			kind := metric.Kind
			if kind == "" {
				return fmt.Errorf("METRIC_DECLARATION_REQUIRED: kind is explicit")
			}
			temporality := metric.Temporality
			if kind == "gauge" && temporality == "" {
				temporality = "unspecified"
			}
			id, e := b.addSeries(m.Series{Name: metric.Name, Unit: metric.Unit, Kind: kind, Temporality: temporality, Monotonic: metric.Monotonic, Resource: resource, Scope: scope, Attributes: attrs})
			if e != nil {
				return e
			}
			p := m.Point{SeriesID: id, Start: metric.Start, End: snapshot.Time, Histogram: metric.Histogram, Flags: metric.Flags}
			if metric.Histogram == nil {
				if len(metric.Value) > 0 {
					p.Value, p.Special, e = decimal(metric.Value)
					if e != nil {
						return e
					}
				} else if p.Flags&1 != 0 {
					p.Special = "no_recorded_value"
				} else {
					return fmt.Errorf("METRIC_NUMBER_REQUIRED")
				}
			}
			if e = b.addPoint(p, fmt.Sprintf("snapshots/%d/metrics/%d", i, j)); e != nil {
				return e
			}
		}
	}
	return nil
}
func (b *builder) rangeQuery(body []byte) error {
	b.set.Normalizer = m.ObservationNormalizer
	var input struct {
		Status   string   `json:"status"`
		Warnings []string `json:"warnings"`
		Infos    []string `json:"infos"`
		Data     struct {
			Type   string `json:"resultType"`
			Result []struct {
				Metric     map[string]string   `json:"metric"`
				Values     [][]json.RawMessage `json:"values"`
				Histograms json.RawMessage     `json:"histograms"`
			} `json:"result"`
		} `json:"data"`
	}
	if e := wirejson.DecodeWithLimits(body, &input, wirejson.ResultLimits(MaxInputBytes, 128)); e != nil {
		return e
	}
	if input.Status != "success" || input.Data.Type != "matrix" {
		return fmt.Errorf("METRIC_RANGE_MATRIX_REQUIRED")
	}
	var provenance map[string]json.RawMessage
	if e := wirejson.Decode(b.set.Sources[0].Query, &provenance); e != nil {
		return e
	}
	var query string
	if json.Unmarshal(provenance["query"], &query) != nil || query == "" {
		return fmt.Errorf("METRIC_RANGE_QUERY_PROVENANCE_REQUIRED: preserve the executed query")
	}
	var step *string
	if raw := provenance["step_ns"]; raw != nil {
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return fmt.Errorf("METRIC_STEP_INVALID")
		}
		if _, ok := m.Time(&text); !ok {
			return fmt.Errorf("METRIC_STEP_INVALID")
		}
		step = &text
	}
	for i, series := range input.Data.Result {
		if len(series.Histograms) > 0 && string(series.Histograms) != "[]" {
			return fmt.Errorf("METRIC_PROMETHEUS_NATIVE_HISTOGRAM_UNSUPPORTED: not an OTLP exponential histogram")
		}
		name := series.Metric["__name__"]
		if name == "" {
			name = query
		}
		attrs, _ := json.Marshal(series.Metric)
		attrs, e := m.Canonical(attrs)
		if e != nil {
			return e
		}
		id, e := b.addSeries(m.Series{Name: name, Unit: "", Kind: "gauge", Temporality: "unspecified", Attributes: attrs, ExpectedStep: step})
		if e != nil {
			return e
		}
		for j, pair := range series.Values {
			if len(pair) != 2 {
				return fmt.Errorf("METRIC_RANGE_PAIR_INVALID")
			}
			timeValue, special, e := decimal(pair[0])
			if e != nil || special != "" || timeValue == nil {
				return fmt.Errorf("METRIC_RANGE_TIME_INVALID")
			}
			seconds, _ := m.Number(*timeValue)
			nanos := new(big.Rat).Mul(seconds, big.NewRat(1_000_000_000, 1))
			if !nanos.IsInt() || nanos.Sign() < 0 || nanos.Num().BitLen() > 64 {
				return fmt.Errorf("METRIC_RANGE_TIME_PRECISION: exact nonzero uint64 nanoseconds required")
			}
			timestamp := nanos.Num().String()
			value, special, e := decimal(pair[1])
			if e != nil {
				return e
			}
			if e = b.addPoint(m.Point{SeriesID: id, End: &timestamp, Value: value, Special: special}, "data/result/"+strconv.Itoa(i)+"/values/"+strconv.Itoa(j)); e != nil {
				return e
			}
		}
	}
	for _, warning := range append(input.Warnings, input.Infos...) {
		if len(b.set.Warnings) < 128 {
			b.set.Warnings = append(b.set.Warnings, warning)
		}
	}
	b.set.Warnings = append(b.set.Warnings, "Range-query values retain query-result semantics; no counter type, physical unit, raw-instrument identity or GenAI alias is inferred.")
	return nil
}
