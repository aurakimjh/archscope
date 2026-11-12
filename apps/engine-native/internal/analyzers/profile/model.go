package profile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const ModelPayloadVersion = "0.2.0"

type CoreEvidence struct {
	Version   string                  `json:"version"`
	Model     profilemodel.ProfileSet `json:"model"`
	Selection profilemodel.Selection  `json:"selection"`
}

func BuildSet(ctx context.Context, set profilemodel.ProfileSet, selection profilemodel.Selection, source string) (models.AnalysisResult, error) {
	if e := set.Validate(); e != nil {
		return models.AnalysisResult{}, e
	}
	encodedModel, e := json.Marshal(set)
	if e != nil {
		return models.AnalysisResult{}, e
	}
	set, e = profilemodel.Decode(encodedModel)
	if e != nil {
		return models.AnalysisResult{}, e
	}
	selection = profilemodel.CopySelection(selection)
	tree, e := profilemodel.BuildTree(ctx, set, selection)
	if e != nil {
		return models.AnalysisResult{}, e
	}
	result := models.New(ResultType, "profile_set")
	result.SourceFiles = []string{source}
	result.Metadata.SchemaVersion = modelPayload(set)
	result.Summary = map[string]any{"total_weight": tree.Root.Inclusive, "total_samples": nil, "sample_records": tree.SampleRecords, "metric_id": tree.Metric.ID, "measurement_kind": tree.Metric.Kind, "unit": tree.Metric.Unit, "scale": tree.Metric.Scale, "profile_count": len(selection.ProfileIDs)}
	if tree.Availability == "observed" && tree.Metric.Kind == "sample_count" {
		n, _ := profilemodel.Number(tree.Root.Inclusive)
		if n.IsInt() {
			result.Summary["total_samples"] = tree.Root.Inclusive
		}
	}
	rows := []map[string]any{}
	var walk func(profilemodel.TreeNode)
	walk = func(node profilemodel.TreeNode) {
		if node.FrameID != "" {
			rows = append(rows, map[string]any{"node_id": node.ID, "frame_id": node.FrameID, "weight": node.Inclusive, "self_weight": node.Self, "share": node.Share})
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(tree.Root)
	result.Series["frame_weights"] = rows
	result.Tables["frames"] = set.Frames
	result.Charts["flamegraph"] = tree.Root
	result.Metadata.Extra["envelope_schema_version"] = "1.0.0"
	result.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0", "profile_core": profilemodel.Version}
	result.Metadata.Extra["profile_core"] = CoreEvidence{profilemodel.Version, set, selection}
	result.Metadata.Extra["quality"] = set.Quality
	metric := resultcontract.Measurement{Kind: tree.Metric.Kind, Unit: tree.Metric.Unit, Scope: "selected_profile_set", Population: "selected", Aggregation: "sum", Encoding: "decimal_string", Availability: "observed"}
	if tree.Availability == "unavailable" {
		result.Summary["total_weight"] = nil
		metric.Availability = "unavailable"
		metric.Reason = tree.Reason
	}
	count := resultcontract.Measurement{Kind: "sample_count", Unit: "count", Scope: "selected_profile_set", Population: "selected", Aggregation: "sum", Encoding: "exact_integer_string", Availability: "observed"}
	if result.Summary["total_samples"] == nil {
		count.Availability = "unavailable"
		count.Reason = "selected_metric_does_not_supply_sample_count"
	}
	result.Metadata.Extra["measurement"] = map[string]resultcontract.Measurement{"/summary/total_weight": metric, "/summary/total_samples": count, "/series/frame_weights": {Columns: map[string]resultcontract.Measurement{"weight": metric, "self_weight": metric}}}
	result.Metadata.Extra["population_ids"] = []string{"selected"}
	if _, present, _ := set.SourceContract(); present {
		result.Metadata.Extra["capture_policy"] = set.CapturePolicy(selection.MetricID)
		result.Metadata.Extra["comparison_policy_scope"] = "canonical_profile_operations_only; generic scalar comparison retains strict capture equality"
	}
	rawMetric, _ := json.Marshal(tree.Metric)
	revision := sha256.Sum256(rawMetric)
	result.Metadata.Extra["populations"] = map[string]resultcontract.PopulationIdentity{"selected": {Comparison: &resultcontract.PopulationComparison{Version: "1.0.0", SelectionHash: strings.TrimPrefix(set.StrictComparisonIdentity(selection), "population-"), CoverageKind: "complete_selected_model"}, SourceHashes: append([]string(nil), set.SourceHashes...), SelectionHash: strings.TrimPrefix(tree.Population, "population-"), SemanticRevision: modelPayload(set) + "/" + profilemodel.Version + "/" + hex.EncodeToString(revision[:]), Coverage: "complete_selected_model"}}
	return result, nil
}
func ReadCore(result models.AnalysisResult) (CoreEvidence, error) {
	var core CoreEvidence
	if result.Type != ResultType || (result.Metadata.SchemaVersion != ModelPayloadVersion && result.Metadata.SchemaVersion != SourceModelPayloadVersion) {
		return core, fmt.Errorf("profile result requires original reanalysis into ProfileSet")
	}

	body, e := json.Marshal(result.Metadata.Extra["profile_core"])
	if e != nil {
		return core, e
	}
	var wire struct {
		Version   string          `json:"version"`
		Model     json.RawMessage `json:"model"`
		Selection json.RawMessage `json:"selection"`
	}
	if e = wirejson.Decode(body, &wire); e != nil {
		return core, e
	}
	core.Version = wire.Version
	if core.Model, e = profilemodel.Decode(wire.Model); e != nil {
		return core, e
	}
	if core.Selection, e = profilemodel.DecodeSelection(wire.Selection); e != nil {
		return core, e
	}

	if core.Version != profilemodel.Version {
		return core, fmt.Errorf("unsupported profile core revision")
	}
	if result.Metadata.SchemaVersion != modelPayload(core.Model) {
		return core, fmt.Errorf("profile semantic revision/model mismatch; reimport original")
	}
	return core, core.Model.ValidateSelection(core.Selection)
}

// ValidateModelResult verifies values against the retained canonical model. A
// schema-number edit or a forged summary cannot activate the new operations.
func ValidateModelResult(result models.AnalysisResult) error {
	core, e := ReadCore(result)
	if e != nil {
		return e
	}
	expected, e := BuildSet(context.Background(), core.Model, core.Selection, "")
	if e != nil {
		return e
	}
	var populations map[string]resultcontract.PopulationIdentity
	populationBody, e := json.Marshal(result.Metadata.Extra["populations"])
	if e != nil {
		return e
	}
	if e = wirejson.Decode(populationBody, &populations); e != nil {
		return e
	}
	if selected, exists := populations["selected"]; exists && selected.Comparison == nil {
		old := expected.Metadata.Extra["populations"].(map[string]resultcontract.PopulationIdentity)
		value := old["selected"]
		value.Comparison = nil
		old["selected"] = value
	}
	// Accept the exactly recomputed legacy policy of existing snapshots. It is
	// not rewritten or treated as a new source-independent scalar policy.
	if saved, ok := populations["selected"]; ok && saved.Comparison != nil {
		legacyHash := strings.TrimPrefix(core.Model.LegacyComparisonIdentity(core.Selection), "population-")
		if saved.Comparison.Version == "1.0.0" && saved.Comparison.SelectionHash == legacyHash && saved.Comparison.CoverageKind == "complete_selected_model" {
			expectedPop := expected.Metadata.Extra["populations"].(map[string]resultcontract.PopulationIdentity)
			item := expectedPop["selected"]
			copyPolicy := *saved.Comparison
			item.Comparison = &copyPolicy
			expectedPop["selected"] = item
		}
	}
	pairs := [][2]any{}
	if result.Metadata.SchemaVersion == SourceModelPayloadVersion {
		pairs = append(pairs, [2]any{result.Metadata.Extra["capture_policy"], expected.Metadata.Extra["capture_policy"]}, [2]any{result.Metadata.Extra["comparison_policy_scope"], expected.Metadata.Extra["comparison_policy_scope"]})
	}
	pairs = append(pairs, [][2]any{{result.Summary, expected.Summary}, {result.Series, expected.Series}, {result.Tables, expected.Tables}, {result.Charts, expected.Charts}, {result.Metadata.Extra["measurement"], expected.Metadata.Extra["measurement"]}, {result.Metadata.Extra["populations"], expected.Metadata.Extra["populations"]}, {result.Metadata.Extra["quality"], expected.Metadata.Extra["quality"]}}...)
	for _, pair := range pairs {
		a, e := json.Marshal(pair[0])
		if e != nil {
			return e
		}
		b, e := json.Marshal(pair[1])
		if e != nil {
			return e
		}
		var av, bv any
		if e = wirejson.DecodeWithLimits(a, &av, wirejson.LegacyResultLimits()); e != nil {
			return e
		}
		if e = wirejson.DecodeWithLimits(b, &bv, wirejson.LegacyResultLimits()); e != nil {
			return e
		}
		a, _ = json.Marshal(av)
		b, _ = json.Marshal(bv)
		if string(a) != string(b) {
			return fmt.Errorf("profile result/model projection mismatch")
		}
	}
	return nil
}
func ModelDescriptor() resultcontract.Descriptor {
	kinds := []string{"sample_count", "count", "allocation_count", "cpu_time", "wall_time", "duration", "bytes", "cycles", "signed_difference"}
	units := []string{"count", "samples", "objects", "seconds", "milliseconds", "microseconds", "nanoseconds", "ticks", "bytes", "cycles"}
	weight := resultcontract.ValueSchema{Nullable: true, Encodings: []string{"decimal_string"}, Kinds: kinds, Units: units, Aggregations: []string{"sum"}, RequirePopulation: true}
	count := resultcontract.ValueSchema{Nullable: true, Encodings: []string{"exact_integer_string"}, Kinds: []string{"sample_count"}, Units: []string{"count"}, Aggregations: []string{"sum"}, RequirePopulation: true}
	return resultcontract.Descriptor{Providers: []resultcontract.Provider{{ID: "profile:frame-weights", Version: "1.0.0", Kind: "detail", Path: "/series/frame_weights", Schema: "object-rows-v1"}}, Type: ResultType, Payload: resultcontract.Range{Min: ModelPayloadVersion, Before: "0.2.1"}, Envelope: resultcontract.Range{Min: "1.0.0", Before: "2.0.0"}, Extensions: map[string]resultcontract.Range{"measurement": {Min: "1.0.0", Before: "1.1.0"}, "profile_core": {Min: profilemodel.Version, Before: "1.0.1"}}, Required: map[string][]string{"save": {"profile_core", "measurement"}, "compute": {"profile_core", "measurement"}, "compare": {"profile_core", "measurement"}}, Scalars: map[string]resultcontract.ValueSchema{"/summary/total_weight": weight, "/summary/total_samples": count}, Collections: map[string]map[string]resultcontract.ValueSchema{"/series/frame_weights": {"weight": weight, "self_weight": weight}}, ValidatePayload: ValidateModelResult}
}

// ExactWeight is useful to consumers that need physical values without passing
// through JavaScript or a legacy scalar DTO.
func ExactWeight(result models.AnalysisResult) (*big.Rat, error) {
	core, e := ReadCore(result)
	if e != nil {
		return nil, e
	}
	tree, e := profilemodel.BuildTree(context.Background(), core.Model, core.Selection)
	if e != nil {
		return nil, e
	}
	if tree.Availability != "observed" {
		return nil, fmt.Errorf("weight unavailable: %s", tree.Reason)
	}
	return profilemodel.Number(tree.Root.Inclusive)
}
