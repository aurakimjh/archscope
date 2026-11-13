package inference

import (
	"context"
	"encoding/json"
	"fmt"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"math/big"
)

// parseOllama accepts body-free allowlisted final-response counters from an
// explicitly run owner collector. It never calls an endpoint or infers TTFT.
func parseOllama(ctx context.Context, body []byte, digest, name string) (n.Set, error) {
	var f struct {
		Contract string `json:"contract"`
		Config   struct {
			Predict     string `json:"num_predict"`
			Temperature string `json:"temperature"`
			Stream      *bool  `json:"stream"`
		} `json:"config"`
		Producer string `json:"producer"`
		Version  string `json:"producer_version"`
		Scope    string `json:"scope"`
		Host     string `json:"host_id"`
		Boot     string `json:"boot_id"`
		SDK      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"sdk"`
		Observations []struct {
			Start   string            `json:"capture_start_ns"`
			End     string            `json:"capture_end_ns"`
			Elapsed string            `json:"client_elapsed_ns"`
			Metrics map[string]string `json:"metrics"`
		} `json:"observations"`
	}
	var s n.Set
	if err := n.Strict(body, &f, 64<<20); err != nil {
		return s, err
	}
	if f.Contract != "ollama-generate-final.v1" || f.Config.Predict != "8" || f.Config.Temperature != "0" || f.Config.Stream == nil || *f.Config.Stream || f.Producer != "ollama" || f.Version != "0.40.16" || len(f.Observations) < 1 || len(f.Observations) > 10000 || !wirejson.Text(f.Host, 128) || !wirejson.Text(f.Boot, 128) || f.Host == "" || f.Boot == "" {
		return s, fmt.Errorf("OLLAMA_SERVING_CAPTURE_UNQUALIFIED")
	}
	s = n.Set{Version: n.Version, Entities: []n.Entity{{ID: "declared-replica", Kind: "replica", Host: f.Host, Boot: f.Boot}}, Contexts: []n.Context{{ID: "declared-context", Engine: "ollama", Version: f.Version, Model: "redacted", Revision: "unknown", DType: "unknown", Quantization: "unknown", Tokenizer: "unknown", Cache: "unknown", Speculative: "unknown", Adapter: "unknown", Workload: "owned synthetic completion; num_predict8 temperature0 unstreamed", Warmup: "included"}}, Links: []n.Link{}, Signals: []n.Signal{}, Sources: []m.Source{}, Events: []n.Event{}, Quality: []n.Quality{}}
	s.Metrics = m.Set{Version: m.Version, Normalizer: m.ObservationNormalizer, Sources: []m.Source{{ID: "ollama-source", SHA256: digest, Name: name, Format: "ollama-generate-final.v1", Producer: f.Producer, ProducerVersion: f.Version, Query: json.RawMessage(`{"collector_version":"1.0.0","endpoint":"/api/generate","stream":false,"num_predict":8,"temperature":0}`)}}, Series: []m.Series{}, Points: []m.Point{}, Warnings: []string{"Actual Ollama0.40.16 local synthetic /api/generate final responses; warmup/load included. Client elapsed uses monotonic clock; capture bounds use wall clock. Model identity redacted/config fields unknown; do not treat as controlled comparison. No body/token timing/stream reconstruction, GPU attribution or vLLM Q03 qualification."}}
	defs := []struct {
		key, semantic, boundary string
		duration                bool
	}{{"client_elapsed_ns", "request_latency", "client", true}, {"total_duration", "request_latency", "server", true}, {"load_duration", "", "server", true}, {"prompt_eval_count", "input_tokens", "server", false}, {"prompt_eval_duration", "prefill_time", "server", true}, {"eval_count", "output_tokens", "server", false}, {"eval_duration", "decode_time", "server", true}}
	var previous uint64
	for i, row := range f.Observations {
		start, ok := m.Time(&row.Start)
		end, yes := m.Time(&row.End)
		if !ok || !yes || end < start || i > 0 && start < previous {
			return n.Set{}, fmt.Errorf("OLLAMA_SERVING_CAPTURE_TIME_INVALID")
		}
		previous = end
		allowed := map[string]bool{}
		for _, d := range defs {
			if d.key != "client_elapsed_ns" {
				allowed[d.key] = true
			}
		}
		for key := range row.Metrics {
			if !allowed[key] {
				return n.Set{}, fmt.Errorf("OLLAMA_SERVING_CAPTURE_FIELD_INVALID")
			}
		}
		for _, d := range defs {
			value, exists := row.Metrics[d.key]
			if d.key == "client_elapsed_ns" {
				value = row.Elapsed
				exists = true
			}
			if !exists {
				continue
			}
			integer, err := m.Uint(value)
			if err != nil {
				return n.Set{}, err
			}
			unit := "tokens"
			observed := value
			if d.duration {
				unit = "seconds"
				observed = m.Text(new(big.Rat).SetFrac(integer, big.NewInt(1000000000)))
			}
			series := m.Series{Name: "ollama." + d.key, Unit: unit, Kind: "gauge", Temporality: "unspecified", Resource: json.RawMessage(`{}`), Scope: m.Scope{Name: "manual.ollama.final", Version: "1.0.0", Attributes: json.RawMessage(`{}`), Dropped: "0"}, Attributes: json.RawMessage(fmt.Sprintf(`{"boundary":%q}`, d.boundary)), Origin: "ollama-final:0.40.16"}
			series.ID = m.SeriesID(series)
			if i == 0 || !hasSeries(s.Metrics.Series, series.ID) {
				s.Metrics.Series = append(s.Metrics.Series, series)
				if d.semantic != "" {
					s.Signals = append(s.Signals, n.Signal{ID: "signal-" + series.ID, SeriesID: series.ID, Semantic: d.semantic, EntityID: "declared-replica", ContextID: "declared-context", Boundary: d.boundary, Population: "successful observed unstreamed completion requests; warmup included", Form: "gauge", Conversion: "identity", SourceCalculation: "reported final-response integer; durations divided exactly by 1e9"})
				}
			}
			point := m.Point{SeriesID: series.ID, End: m.Pointer(row.End), Value: m.Pointer(observed), Refs: []m.Ref{{SourceID: "ollama-source", Position: fmt.Sprintf("observations/%d/%s", i, d.key)}}}
			point.ID = m.PointID(point)
			s.Metrics.Points = append(s.Metrics.Points, point)
		}
		event := n.Event{Kind: "request", EntityID: "declared-replica", ContextID: "declared-context", Time: m.Pointer(row.End), RequestStart: m.Pointer(row.Start), RequestID: fmt.Sprintf("observed-%d", i), Boundary: "client", Outcome: "success", Population: "observed unstreamed completion requests; warmup included", Ref: m.Ref{SourceID: "ollama-source", Position: fmt.Sprintf("observations/%d", i)}}
		if v, ok := row.Metrics["eval_count"]; ok {
			event.OutputTokens = m.Pointer(v)
		}
		event.ID = n.EventID(event)
		s.Events = append(s.Events, event)
	}
	var err error
	s.Metrics, err = m.Normalize(ctx, s.Metrics)
	if err != nil {
		return s, err
	}
	return s, s.Validate(ctx)
}
func hasSeries(series []m.Series, id string) bool {
	for _, s := range series {
		if s.ID == id {
			return true
		}
	}
	return false
}
