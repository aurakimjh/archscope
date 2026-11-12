package metrichistory

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type keyValue struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}
type otlpFile struct {
	Resources []struct {
		Resource struct {
			Attributes []keyValue `json:"attributes"`
			Dropped    uint32     `json:"droppedAttributesCount"`
		} `json:"resource"`
		SchemaURL string `json:"schemaUrl"`
		Scopes    []struct {
			Scope struct {
				Name       string     `json:"name"`
				Version    string     `json:"version"`
				Attributes []keyValue `json:"attributes"`
				Dropped    uint32     `json:"droppedAttributesCount"`
			} `json:"scope"`
			SchemaURL string `json:"schemaUrl"`
			Metrics   []struct {
				Name        string          `json:"name"`
				Description string          `json:"description"`
				Unit        string          `json:"unit"`
				Gauge       *otlpData       `json:"gauge"`
				Sum         *otlpData       `json:"sum"`
				Histogram   *otlpData       `json:"histogram"`
				Exponential *otlpData       `json:"exponentialHistogram"`
				Summary     json.RawMessage `json:"summary"`
			} `json:"metrics"`
		} `json:"scopeMetrics"`
	} `json:"resourceMetrics"`
}
type otlpData struct {
	Points      []otlpPoint `json:"dataPoints"`
	Temporality int         `json:"aggregationTemporality"`
	Monotonic   bool        `json:"isMonotonic"`
}
type otlpPoint struct {
	Attributes    []keyValue        `json:"attributes"`
	Start         json.RawMessage   `json:"startTimeUnixNano"`
	End           json.RawMessage   `json:"timeUnixNano"`
	Int           json.RawMessage   `json:"asInt"`
	Double        json.RawMessage   `json:"asDouble"`
	Count         json.RawMessage   `json:"count"`
	Sum           json.RawMessage   `json:"sum"`
	Min           json.RawMessage   `json:"min"`
	Max           json.RawMessage   `json:"max"`
	Bounds        []json.RawMessage `json:"explicitBounds"`
	Counts        []json.RawMessage `json:"bucketCounts"`
	Scale         int32             `json:"scale"`
	ZeroCount     json.RawMessage   `json:"zeroCount"`
	ZeroThreshold json.RawMessage   `json:"zeroThreshold"`
	Positive      *struct {
		Offset int32             `json:"offset"`
		Counts []json.RawMessage `json:"bucketCounts"`
	} `json:"positive"`
	Negative *struct {
		Offset int32             `json:"offset"`
		Counts []json.RawMessage `json:"bucketCounts"`
	} `json:"negative"`
	Flags     uint32            `json:"flags"`
	Exemplars []json.RawMessage `json:"exemplars"`
}

func attributes(values []keyValue, depth int, path string) (outBody json.RawMessage, failure error) {
	defer func() { failure = diagnostic(path, "bounded_typed_attributes", failure) }()
	if depth > 16 || len(values) > 128 {
		return nil, fmt.Errorf("METRIC_ATTRIBUTE_BUDGET")
	}
	out := map[string]json.RawMessage{}
	for index, kv := range values {
		if kv.Key == "" || len(kv.Key) > 4096 || out[kv.Key] != nil {
			return nil, fmt.Errorf("METRIC_ATTRIBUTE_KEY_INVALID")
		}
		value, e := anyValue(kv.Value, depth+1, fmt.Sprintf("%s/%d/value", path, index))
		if e != nil {
			return nil, e
		}
		out[kv.Key] = value
	}
	body, _ := json.Marshal(out)
	if len(body) > 64<<10 {
		return nil, fmt.Errorf("METRIC_ATTRIBUTE_BUDGET")
	}
	return body, nil
}
func anyValue(raw json.RawMessage, depth int, path string) (outBody json.RawMessage, failure error) {
	defer func() { failure = diagnostic(path, "one_declared_typed_attribute_value", failure) }()
	if len(raw) == 0 {
		return json.RawMessage(`{}`), nil
	}
	if depth > 16 {
		return nil, fmt.Errorf("METRIC_ATTRIBUTE_DEPTH")
	}
	var fields map[string]json.RawMessage
	if e := wirejson.Decode(raw, &fields); e != nil {
		return nil, e
	}
	known := []string{"stringValue", "boolValue", "intValue", "doubleValue", "arrayValue", "kvlistValue", "bytesValue"}
	present := []string{}
	for _, key := range known {
		if fields[key] != nil {
			present = append(present, key)
		}
	}
	if len(present) > 1 {
		return nil, fmt.Errorf("METRIC_ANYVALUE_ONEOF")
	}
	if len(present) == 0 {
		return json.RawMessage(`{}`), nil
	}
	key := present[0]
	value := fields[key]
	switch key {
	case "stringValue":
		var s string
		if e := json.Unmarshal(value, &s); e != nil || len(s) > 16<<10 {
			return nil, fmt.Errorf("METRIC_ATTRIBUTE_STRING")
		}
	case "boolValue":
		var b bool
		if string(value) == "null" || json.Unmarshal(value, &b) != nil {
			return nil, fmt.Errorf("METRIC_ATTRIBUTE_BOOL")
		}
	case "intValue":
		text, e := integerAt(value, true, false, path+"/intValue")
		if e != nil {
			return nil, e
		}
		value, _ = json.Marshal(text)
	case "doubleValue":
		text, special, e := decimalAt(value, path+"/doubleValue")
		if e != nil {
			return nil, e
		}
		if text != nil {
			value = json.RawMessage(*text)
		} else {
			value, _ = json.Marshal(special)
		}
	case "bytesValue":
		var s string
		if json.Unmarshal(value, &s) != nil {
			return nil, fmt.Errorf("METRIC_ATTRIBUTE_BYTES")
		}
		if _, e := base64.StdEncoding.DecodeString(s); e != nil {
			return nil, e
		}
	case "arrayValue":
		var array struct {
			Values []json.RawMessage `json:"values"`
		}
		if e := wirejson.Decode(value, &array); e != nil || len(array.Values) > 128 {
			return nil, fmt.Errorf("METRIC_ATTRIBUTE_ARRAY")
		}
		next := []json.RawMessage{}
		for index, v := range array.Values {
			v, e := anyValue(v, depth+1, fmt.Sprintf("%s/arrayValue/values/%d", path, index))
			if e != nil {
				return nil, e
			}
			next = append(next, v)
		}
		value, _ = json.Marshal(map[string]any{"values": next})
	case "kvlistValue":
		var list struct {
			Values []keyValue `json:"values"`
		}
		if e := wirejson.Decode(value, &list); e != nil {
			return nil, e
		}
		nested, e := attributes(list.Values, depth+1, path+"/kvlistValue/values")
		if e != nil {
			return nil, e
		}
		value = nested
	}
	body, _ := json.Marshal(map[string]json.RawMessage{key: value})
	return m.Canonical(body)
}
func timeField(raw json.RawMessage) (*string, error) {
	text, e := integer(raw, false, true)
	if e != nil {
		return nil, e
	}
	if text == "0" {
		return nil, nil
	}
	return &text, nil
}
func finiteOptional(raw json.RawMessage) (*string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	n, s, e := decimal(raw)
	if e != nil {
		return nil, e
	}
	if s != "" {
		return nil, fmt.Errorf("METRIC_HISTOGRAM_NONFINITE")
	}
	return n, nil
}
func (b *builder) otlp(body []byte) error {
	var file otlpFile
	if e := decodeSource(b.ctx, body, &file, false); e != nil {
		return e
	}
	if file.Resources == nil {
		return &SourceError{Code: "METRIC_OTLP_ROOT_REQUIRED", Path: "/resourceMetrics", Rule: "use_lowerCamelCase_resourceMetrics"}
	}
	for ri, resource := range file.Resources {
		resourcePath := fmt.Sprintf("/resourceMetrics/%d", ri)
		b.path = resourcePath
		attrs, e := attributes(resource.Resource.Attributes, 0, resourcePath+"/resource/attributes")
		if e != nil {
			return e
		}
		resourceBody, _ := json.Marshal(map[string]any{"attributes": json.RawMessage(attrs), "dropped_attributes": strconv.FormatUint(uint64(resource.Resource.Dropped), 10)})
		for si, scope := range resource.Scopes {
			scopePath := fmt.Sprintf("%s/scopeMetrics/%d", resourcePath, si)
			b.path = scopePath
			scopeAttrs, e := attributes(scope.Scope.Attributes, 0, scopePath+"/scope/attributes")
			if e != nil {
				return e
			}
			descriptor := m.Scope{Name: scope.Scope.Name, Version: scope.Scope.Version, SchemaURL: scope.SchemaURL, Attributes: scopeAttrs, Dropped: strconv.FormatUint(uint64(scope.Scope.Dropped), 10)}
			for mi, metric := range scope.Metrics {
				metricPath := fmt.Sprintf("%s/metrics/%d", scopePath, mi)
				b.path = metricPath
				data := metric.Gauge
				kind := "gauge"
				count := 0
				for _, value := range []*otlpData{metric.Gauge, metric.Sum, metric.Histogram, metric.Exponential} {
					if value != nil {
						count++
					}
				}
				if len(metric.Summary) > 0 {
					return fmt.Errorf("METRIC_SUMMARY_UNSUPPORTED: quantiles are not merged or converted to histograms")
				}
				if count != 1 {
					return fmt.Errorf("METRIC_OTLP_ONEOF: exactly one supported metric data kind required")
				}
				if metric.Sum != nil {
					data = metric.Sum
					kind = "sum"
				}
				if metric.Histogram != nil {
					data = metric.Histogram
					kind = "histogram"
				}
				if metric.Exponential != nil {
					data = metric.Exponential
					kind = "exponential_histogram"
				}
				temporality := "unspecified"
				if kind != "gauge" {
					switch data.Temporality {
					case 0:
					case 1:
						temporality = "delta"
					case 2:
						temporality = "cumulative"
					default:
						return fmt.Errorf("METRIC_TEMPORALITY_UNSUPPORTED")
					}
				}
				jsonKind := kind
				if kind == "exponential_histogram" {
					jsonKind = "exponentialHistogram"
				}
				for pi, p := range data.Points {
					pointPath := fmt.Sprintf("%s/%s/dataPoints/%d", metricPath, jsonKind, pi)
					b.path = pointPath
					if e = b.ctx.Err(); e != nil {
						return e
					}
					pointAttrs, e := attributes(p.Attributes, 0, pointPath+"/attributes")
					if e != nil {
						return e
					}
					series := m.Series{Name: metric.Name, Description: metric.Description, Unit: metric.Unit, Kind: kind, Temporality: temporality, Monotonic: kind == "sum" && data.Monotonic, Resource: resourceBody, ResourceSchemaURL: resource.SchemaURL, Scope: descriptor, Attributes: pointAttrs}
					id, e := b.addSeries(series)
					if e != nil {
						return e
					}
					start, e := timeAt(p.Start, pointPath+"/startTimeUnixNano")
					if e != nil {
						return e
					}
					end, e := timeAt(p.End, pointPath+"/timeUnixNano")
					if e != nil {
						return e
					}
					if a, ok := m.Time(start); ok {
						if z, known := m.Time(end); known && a > z {
							return diagnostic(pointPath+"/startTimeUnixNano", "start_not_after_end", fmt.Errorf("METRIC_TIME_INVALID"))
						}
					}
					point := m.Point{SeriesID: id, Start: start, End: end, Flags: p.Flags}
					if kind == "gauge" || kind == "sum" {
						if len(p.Int) > 0 && len(p.Double) > 0 {
							return diagnostic(pointPath, "exactly_one_asInt_or_asDouble", fmt.Errorf("METRIC_NUMBER_ONEOF"))
						}
						if len(p.Int) > 0 {
							text, e := integerAt(p.Int, true, false, pointPath+"/asInt")
							if e != nil {
								return e
							}
							point.Value = &text
						} else if len(p.Double) > 0 {
							point.Value, point.Special, e = decimalAt(p.Double, pointPath+"/asDouble")
							if e != nil {
								return e
							}
						} else if p.Flags&1 != 0 {
							point.Special = "no_recorded_value"
						} else {
							return diagnostic(pointPath, "asInt_or_asDouble_or_no_recorded_value_flag", fmt.Errorf("METRIC_NUMBER_REQUIRED"))
						}
					} else {
						hist := &m.Histogram{}
						hist.Count, e = integerAt(p.Count, false, true, pointPath+"/count")
						if e != nil {
							return e
						}
						hist.Sum, e = finiteAt(p.Sum, pointPath+"/sum")
						if e != nil {
							return e
						}
						hist.Min, e = finiteAt(p.Min, pointPath+"/min")
						if e != nil {
							return e
						}
						hist.Max, e = finiteAt(p.Max, pointPath+"/max")
						if e != nil {
							return e
						}
						if kind == "histogram" {
							for index, v := range p.Bounds {
								n, e := finiteAt(v, fmt.Sprintf("%s/explicitBounds/%d", pointPath, index))
								if e != nil {
									return e
								}
								if n == nil {
									return diagnostic(fmt.Sprintf("%s/explicitBounds/%d", pointPath, index), "finite_histogram_bound", fmt.Errorf("METRIC_HISTOGRAM_BOUND_INVALID"))
								}
								hist.Bounds = append(hist.Bounds, *n)
							}
							for index, v := range p.Counts {
								n, e := integerAt(v, false, false, fmt.Sprintf("%s/bucketCounts/%d", pointPath, index))
								if e != nil {
									return e
								}
								hist.Counts = append(hist.Counts, n)
							}
						} else {
							scale := p.Scale
							hist.Scale = &scale
							hist.ZeroCount, e = integerAt(p.ZeroCount, false, true, pointPath+"/zeroCount")
							if e != nil {
								return e
							}
							hist.ZeroThreshold, e = finiteAt(p.ZeroThreshold, pointPath+"/zeroThreshold")
							if e != nil {
								return e
							}
							if hist.ZeroThreshold == nil {
								hist.ZeroThreshold = m.Pointer("0")
							}
							for _, entry := range []struct {
								side string
								data *struct {
									Offset int32             `json:"offset"`
									Counts []json.RawMessage `json:"bucketCounts"`
								}
							}{{"positive", p.Positive}, {"negative", p.Negative}} {
								side, data := entry.side, entry.data
								if data == nil {
									continue
								}
								buckets := &m.Buckets{Offset: data.Offset, Counts: []string{}}
								for index, v := range data.Counts {
									n, e := integerAt(v, false, false, fmt.Sprintf("%s/%s/bucketCounts/%d", pointPath, side, index))
									if e != nil {
										return e
									}
									buckets.Counts = append(buckets.Counts, n)
								}
								if side == "positive" {
									hist.Positive = buckets
								} else {
									hist.Negative = buckets
								}
							}
						}
						point.Histogram = hist
					}
					if series.Monotonic && point.Value != nil {
						n, _ := m.Number(*point.Value)
						if n.Sign() < 0 {
							field := "asDouble"
							if len(p.Int) > 0 {
								field = "asInt"
							}
							return diagnostic(pointPath+"/"+field, "nonnegative_monotonic_sum", fmt.Errorf("METRIC_MONOTONIC_NEGATIVE"))
						}
					}
					if len(p.Exemplars) > 128 {
						return fmt.Errorf("METRIC_EXEMPLAR_BUDGET")
					}
					if len(p.Exemplars) > 0 {
						for _, raw := range p.Exemplars {
							var exemplar map[string]json.RawMessage
							if e = wirejson.Decode(raw, &exemplar); e != nil {
								return e
							}
							for key, size := range map[string]int{"traceId": 32, "spanId": 16} {
								if value := exemplar[key]; len(value) > 0 {
									var text string
									if json.Unmarshal(value, &text) != nil || text != "" && len(text) != size {
										return fmt.Errorf("METRIC_EXEMPLAR_ID_INVALID")
									}
									if _, e = hex.DecodeString(text); e != nil {
										return e
									}
								}
							}
						}
						point.Exemplars, _ = json.Marshal(p.Exemplars)
						point.Exemplars, e = m.Canonical(point.Exemplars)
						if e != nil {
							return e
						}
					}
					if e = b.addPoint(point, fmt.Sprintf("resourceMetrics/%d/scopeMetrics/%d/metrics/%d/dataPoints/%d", ri, si, mi, pi)); e != nil {
						return e
					}
				}
			}
		}
	}
	return nil
}

// Canonical keys used by scope/resource attributes preserve typed values and do
// not depend on the producer's attribute-array ordering.
func sortedKeys(values map[string]json.RawMessage) []string {
	keys := []string{}
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
