package inference

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	n "github.com/aurakimjh/archscope/apps/engine-native/internal/inferencemodel"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	history "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

//go:embed contracts/genai-ollama-profile.json
var genaiProfileJSON []byte

const GenAIProfile = "otel-genai-ollama.v1.37"

// Explicit profile/version/scope checks precede aliasing; generic OTLP remains
// available without this opt-in and never gains these serving semantics.
func parseGenAI(ctx context.Context, original []byte, name string, bindings json.RawMessage) (n.Set, error) {
	var c struct {
		Profile       string `json:"semantic_profile"`
		Producer      string `json:"producer"`
		Version       string `json:"producer_version"`
		Host          string `json:"host_id"`
		Boot          string `json:"boot_id"`
		EngineVersion string `json:"engine_version"`
	}
	var profile struct {
		ID                string `json:"id"`
		Producer          string `json:"producer"`
		Version           string `json:"producer_version"`
		Engine            string `json:"engine_version"`
		Scope             string `json:"scope_name"`
		ScopeVersion      string `json:"scope_version"`
		Schema            string `json:"schema_url"`
		ProviderAttribute string `json:"provider_attribute"`
		Provider          string `json:"provider"`
		Operation         string `json:"operation"`
		Duration          struct {
			Metric   string `json:"metric"`
			Unit     string `json:"unit"`
			Semantic string `json:"semantic"`
		} `json:"duration"`
		Tokens struct {
			Metric string `json:"metric"`
			Unit   string `json:"unit"`
			Input  string `json:"input_semantic"`
			Output string `json:"output_semantic"`
		} `json:"tokens"`
	}
	var s n.Set
	if err := n.Strict(genaiProfileJSON, &profile, 64<<10); err != nil {
		return s, err
	}
	if profile.ID != GenAIProfile {
		return s, fmt.Errorf("GENAI_PROFILE_REGISTRATION_INVALID")
	}
	if err := n.Strict(bindings, &c, 64<<10); err != nil {
		return s, err
	}
	if c.Profile != profile.ID || c.EngineVersion != profile.Engine || c.Producer != profile.Producer || c.Version != profile.Version || c.Host == "" || c.Boot == "" || !wirejson.Text(c.Host, 128) || !wirejson.Text(c.Boot, 128) {
		return s, fmt.Errorf("GENAI_CAPTURE_UNQUALIFIED")
	}
	metrics, err := history.ParseBytes(ctx, original, name, history.Options{Format: "otlp-json", Producer: c.Producer, ProducerVersion: c.Version})
	if err != nil {
		return s, err
	}
	s = n.Set{Version: n.Version, Metrics: metrics, Entities: []n.Entity{{ID: "declared-client", Kind: "replica", Host: c.Host, Boot: c.Boot}}, Links: []n.Link{}, Contexts: []n.Context{{ID: "declared-manual-otel", Engine: "ollama", Version: "0.40.16", Model: "redacted", Revision: "unknown", DType: "unknown", Quantization: "unknown", Tokenizer: "unknown", Cache: "unknown", Speculative: "unknown", Adapter: c.Profile, Workload: "manual synthetic text_completion metrics; no request records", Warmup: "included"}}, Signals: []n.Signal{}, Sources: []m.Source{}, Events: []n.Event{}, Quality: []n.Quality{}}
	if len(metrics.Series) == 0 {
		return s, fmt.Errorf("GENAI_SERIES_REQUIRED")
	}
	for _, series := range metrics.Series {
		if series.Scope.Name != profile.Scope || series.Scope.Version != profile.ScopeVersion || series.Scope.SchemaURL != profile.Schema || series.Kind != "histogram" || series.Temporality != "cumulative" {
			return n.Set{}, fmt.Errorf("GENAI_SCOPE_SCHEMA_OR_TEMPORALITY_UNQUALIFIED")
		}
		var attrs map[string]struct {
			String *string `json:"stringValue"`
		}
		if err = wirejson.StrictWithLimits(series.Attributes, &attrs, wirejson.ResultLimits(64<<10, 16)); err != nil {
			return n.Set{}, fmt.Errorf("GENAI_ATTRIBUTE_INVALID")
		}
		text := func(key string) string {
			if v := attrs[key].String; v != nil {
				return *v
			}
			return ""
		}
		if text("gen_ai.operation.name") != profile.Operation || text(profile.ProviderAttribute) != profile.Provider || text("gen_ai.request.model") == "" {
			return n.Set{}, fmt.Errorf("GENAI_OPERATION_OR_PROVIDER_UNQUALIFIED")
		}
		semantic := ""
		population := "observed client text_completion calls; warmup included"
		switch series.Name {
		case profile.Duration.Metric:
			if series.Unit != profile.Duration.Unit || len(attrs) != 3 {
				return n.Set{}, fmt.Errorf("GENAI_DURATION_UNIT_OR_ATTRIBUTES")
			}
			semantic = profile.Duration.Semantic
		case profile.Tokens.Metric:
			if series.Unit != profile.Tokens.Unit || len(attrs) != 4 {
				return n.Set{}, fmt.Errorf("GENAI_TOKEN_UNIT_OR_ATTRIBUTES")
			}
			switch text("gen_ai.token.type") {
			case "input":
				semantic = profile.Tokens.Input
			case "output":
				semantic = profile.Tokens.Output
			default:
				return n.Set{}, fmt.Errorf("GENAI_TOKEN_TYPE_UNQUALIFIED")
			}
			population += "; " + text("gen_ai.token.type") + " token usage observations"
		default:
			return n.Set{}, fmt.Errorf("GENAI_METRIC_UNQUALIFIED: use generic OTLP for other metrics")
		}
		// Preserve histogram buckets/count/sum as observations; never infer request
		// spans, token arrivals, quantiles, totals across series or server populations.
		s.Signals = append(s.Signals, n.Signal{ID: "signal-" + series.ID, SeriesID: series.ID, Semantic: semantic, EntityID: "declared-client", ContextID: "declared-manual-otel", Boundary: "client", Population: population, Form: "histogram", Conversion: "identity", SourceCalculation: "manual.ollama.genai1.0.0 / OTel Python1.38.0 / semconv1.37.0 cumulative explicit histogram"})
	}
	s.Metrics.Warnings = append(s.Metrics.Warnings, "Qualified manual Ollama0.40.16 + OTel Python1.38.0 JSON encoder scope manual.ollama.genai1.0.0 / schema1.37.0 only. No automatic vendor instrumentation or other semantic revision qualification. Cumulative histograms preserve original populations and buckets; no quantile/request/token-boundary reconstruction, cross-series sum or client/server deduplication.")
	return s, s.Validate(ctx)
}
