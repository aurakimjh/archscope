package analysis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const BaselineVersion = "1.0.0"

type BaselineRequest struct {
	CaseID   string `json:"case_id"`
	Revision string `json:"revision"`
	ResultID string `json:"result_id"`
	SourceID string `json:"source_id"`
}
type ContextMapping struct {
	BaselineSHA256 string `json:"baseline_sha256"`
	TargetSHA256   string `json:"target_sha256"`
	Reason         string `json:"reason"`
}
type BaselineBinding struct {
	ClockSHA256        string                      `json:"clock_sha256"`
	Contract           string                      `json:"contract"`
	Version            string                      `json:"version"`
	ResultSHA256       string                      `json:"result_sha256"`
	SourceID           string                      `json:"source_id"`
	SourceSHA256       string                      `json:"source_sha256"`
	ModelSHA256        string                      `json:"model_sha256"`
	OptionsSHA256      string                      `json:"options_sha256"`
	Selection          profilemodel.Selection      `json:"selection"`
	ComparisonIdentity string                      `json:"comparison_identity"`
	CaptureSHA256      string                      `json:"capture_sha256"`
	Policy             *profilemodel.CapturePolicy `json:"capture_policy"`
}
type BaselineComparisonRequest struct {
	ContextMapping *ContextMapping              `json:"context_mapping,omitempty"`
	Cursor         string                       `json:"cursor,omitempty"`
	Limit          int                          `json:"limit,omitempty"`
	CaseID         string                       `json:"case_id"`
	Revision       string                       `json:"revision"`
	BaselineID     string                       `json:"baseline_id"`
	TargetResultID string                       `json:"target_result_id"`
	TargetSourceID string                       `json:"target_source_id"`
	Mapping        []profilemodel.EntityMapping `json:"mapping,omitempty"`
}
type BaselineComparison struct {
	CacheHit           bool                         `json:"cache_hit"`
	VerificationScope  string                       `json:"verification_scope"`
	ContextMapping     *ContextMapping              `json:"context_mapping,omitempty"`
	Next               string                       `json:"next"`
	Total              int                          `json:"total"`
	QuerySHA256        string                       `json:"query_sha256"`
	Version            string                       `json:"version"`
	CaseID             string                       `json:"case_id"`
	Revision           string                       `json:"revision"`
	BaselineID         string                       `json:"baseline_id"`
	SourceVerification string                       `json:"source_verification"`
	Baseline           BaselineBinding              `json:"baseline"`
	Target             BaselineBinding              `json:"target"`
	Difference         profilemodel.Difference      `json:"difference"`
	Mapping            []profilemodel.EntityMapping `json:"mapping"`
}

func canonicalDigest(value any) (string, error) {
	body, e := json.Marshal(value)
	if e != nil {
		return "", e
	}
	var v any
	if e = wirejson.DecodeWithLimits(body, &v, wirejson.ResultLimits(cases.MaxObjectBytes, 1024)); e != nil {
		return "", e
	}
	body, e = json.Marshal(v)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
func bytesDigest(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func (s *Service) reimportCaseProfile(ctx context.Context, store *cases.Store, req BaselineRequest) (analyzer.CoreEvidence, BaselineBinding, error) {
	var empty analyzer.CoreEvidence
	var binding BaselineBinding
	m, e := store.Get(req.CaseID)
	if e != nil {
		return empty, binding, e
	}
	if m.Revision != req.Revision {
		return empty, binding, fmt.Errorf("CASE_REVISION_STALE")
	}
	var ref *cases.ResultRef
	for i := range m.Results {
		if m.Results[i].ID == req.ResultID {
			ref = &m.Results[i]
		}
	}
	if ref == nil || ref.Stale {
		return empty, binding, fmt.Errorf("BASELINE_RESULT_MISSING_OR_STALE")
	}
	if ref.SourceHashes[req.SourceID] == "" {
		return empty, binding, fmt.Errorf("BASELINE_ORIGINAL_NOT_BOUND: save the result with its original source reference")
	}
	raw, e := store.ExportSnapshot(m.ID, ref.ID)
	if e != nil {
		return empty, binding, e
	}
	if bytesDigest(raw) != ref.SHA256 {
		return empty, binding, fmt.Errorf("BASELINE_RESULT_CHANGED")
	}
	result, e := resultcontract.DecodeWithLimits(raw, wirejson.ResultLimits(cases.MaxObjectBytes, 1024))
	if e != nil {
		return empty, binding, e
	}
	if v := s.results.Check(result, "compare"); !v.Allowed {
		return empty, binding, fmt.Errorf("BASELINE_RESULT_UNSUPPORTED: %s", v.Reason)
	}
	core, e := analyzer.ReadCore(result)
	if e != nil {
		return empty, binding, e
	}
	contract, present, e := core.Model.SourceContract()
	if e != nil || !present || result.Metadata.SchemaVersion != analyzer.SourceModelPayloadVersion {
		return empty, binding, fmt.Errorf("BASELINE_REIMPORT_REQUIRED: use an original-source 0.4.0 result; old snapshots stay unchanged")
	}
	if len(core.Model.SourceHashes) != 1 || core.Model.SourceHashes[0] != ref.SourceHashes[req.SourceID] {
		return empty, binding, fmt.Errorf("BASELINE_SOURCE_BINDING_MISMATCH")
	}
	original, e := store.SourceSnapshot(ctx, m.ID, m.Revision, req.SourceID)
	if e != nil {
		return empty, binding, e
	}
	if bytesDigest(original) != core.Model.SourceHashes[0] {
		return empty, binding, fmt.Errorf("BASELINE_SOURCE_BINDING_MISMATCH")
	}
	var capture map[string]json.RawMessage
	if e = wirejson.Decode(core.Model.CaptureOptions, &capture); e != nil {
		return empty, binding, e
	}
	// These keys are owned by the normalizer and recomputed from original bytes.
	if core.Model.Format == "speedscope-json" {
		delete(capture, "source_profile_bounds")
	}
	if core.Model.Format == "pprof" {
		delete(capture, "source_period")
		delete(capture, "source_period_type")
		delete(capture, "source_duration")
	}
	optionsJSON, _ := json.Marshal(capture)
	options := parser.SourceOptions{Format: core.Model.Format, Producer: core.Model.Producer, ProducerVersion: core.Model.ProducerVersion, CaptureOptions: optionsJSON}
	if contract.ProducerBasis == "unknown" {
		options.Producer = ""
	}
	replay, e := parser.ParseSourceBytes(ctx, original, options)
	if e != nil {
		return empty, binding, e
	}
	expected, e := canonicalDigest(core.Model)
	if e != nil {
		return empty, binding, e
	}
	actual, e := canonicalDigest(replay)
	if e != nil {
		return empty, binding, e
	}
	if expected != actual {
		return empty, binding, fmt.Errorf("BASELINE_REIMPORT_MISMATCH: original/options/normalizer do not reconstruct the saved model")
	}
	// Supplied SaveResult metadata cannot assert verification. If present it must
	// agree with the independently reconstructed selection/options binding.
	optionHash, e := canonicalDigest(options)
	if e != nil {
		return empty, binding, e
	}
	if len(ref.Selection) > 0 && string(ref.Selection) != "null" {
		var selection profilemodel.Selection
		if e = wirejson.Decode(ref.Selection, &selection); e != nil {
			return empty, binding, e
		}
		a, _ := canonicalDigest(selection)
		b, _ := canonicalDigest(core.Selection)
		if a != b {
			return empty, binding, fmt.Errorf("BASELINE_SELECTION_BINDING_MISMATCH")
		}
	}
	if ref.OptionsHash != "" && ref.OptionsHash != optionHash {
		return empty, binding, fmt.Errorf("BASELINE_OPTIONS_BINDING_MISMATCH")
	}
	clockHash := ""
	captureHash := ""
	if ref.CaptureID != "" {
		for _, c := range m.Captures {
			if c.ID == ref.CaptureID {
				clockHash, e = canonicalDigest(c.Clock)
				if e != nil {
					return empty, binding, e
				}
				captureHash, e = canonicalDigest(c)
				if e != nil {
					return empty, binding, e
				}
			}
		}
	}
	binding = BaselineBinding{ClockSHA256: clockHash, Contract: "profile-baseline", Version: BaselineVersion, ResultSHA256: ref.SHA256, SourceID: req.SourceID, SourceSHA256: bytesDigest(original), ModelSHA256: actual, OptionsSHA256: optionHash, Selection: profilemodel.CopySelection(core.Selection), ComparisonIdentity: replay.ComparisonIdentity(core.Selection), CaptureSHA256: captureHash, Policy: replay.CapturePolicy(core.Selection.MetricID)}
	// All subsequent computation uses the reimported model, not edited saved fields.
	core.Model = replay
	return core, binding, nil
}
func (s *Service) PinCaseBaseline(parent context.Context, store *cases.Store, req BaselineRequest) (cases.Manifest, error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return cases.Manifest{}, e
	}
	defer finish()
	_, binding, e := s.reimportCaseProfile(ctx, store, req)
	if e != nil {
		return cases.Manifest{}, e
	}
	body, _ := json.Marshal(binding)
	return store.Apply(ctx, cases.Edit{CaseID: req.CaseID, ExpectedRevision: req.Revision, Operation: "baseline", ResultID: req.ResultID, Conditions: body})
}
func (s *Service) computeCaseBaseline(ctx context.Context, store *cases.Store, req BaselineComparisonRequest) (BaselineComparison, error) {
	out := BaselineComparison{Version: BaselineVersion, CaseID: req.CaseID, Revision: req.Revision, BaselineID: req.BaselineID}
	m, e := store.Get(req.CaseID)
	if e != nil {
		return out, e
	}
	if m.Revision != req.Revision {
		return out, fmt.Errorf("CASE_REVISION_STALE")
	}
	var baseline *cases.Baseline
	for i := range m.Baselines {
		if m.Baselines[i].ID == req.BaselineID {
			baseline = &m.Baselines[i]
		}
	}
	if baseline == nil {
		return out, fmt.Errorf("BASELINE_MISSING")
	}
	var pinned BaselineBinding
	if e = strictQueryJSON(baseline.Conditions, &pinned); e != nil || pinned.Contract != "profile-baseline" || pinned.Version != BaselineVersion {
		return out, fmt.Errorf("BASELINE_REIMPORT_REQUIRED: create a new qualified baseline pin")
	}
	left, verified, e := s.reimportCaseProfile(ctx, store, BaselineRequest{req.CaseID, req.Revision, baseline.ResultID, pinned.SourceID})
	if e != nil {
		return out, e
	}
	a, _ := canonicalDigest(pinned)
	b, _ := canonicalDigest(verified)
	if a != b || baseline.ResultSHA256 != verified.ResultSHA256 {
		return out, fmt.Errorf("BASELINE_BINDING_CHANGED: preserve old pin and explicitly create a new one")
	}
	right, target, e := s.reimportCaseProfile(ctx, store, BaselineRequest{req.CaseID, req.Revision, req.TargetResultID, req.TargetSourceID})
	if e != nil {
		return out, e
	}
	if verified.ClockSHA256 != target.ClockSHA256 {
		return out, fmt.Errorf("clock_class_mismatch: capture clocks differ; no implicit time alignment")
	}
	if verified.CaptureSHA256 != target.CaptureSHA256 {
		mapping := req.ContextMapping
		if mapping == nil || mapping.BaselineSHA256 != verified.CaptureSHA256 || mapping.TargetSHA256 != target.CaptureSHA256 || len(strings.TrimSpace(mapping.Reason)) == 0 || len(mapping.Reason) > 512 {
			return out, fmt.Errorf("capture_context_mapping_required: explicitly bind both capture digests and explain the mapping")
		}
	}
	difference, e := profilemodel.CompareMapped(ctx, left.Model, left.Selection, right.Model, right.Selection, req.Mapping)
	if e != nil {
		return out, e
	}
	difference.Baseline.ResultSHA256 = verified.ResultSHA256
	difference.Target.ResultSHA256 = target.ResultSHA256
	if _, e = store.SourceSnapshot(ctx, m.ID, m.Revision, verified.SourceID); e != nil {
		return out, e
	}
	if _, e = store.SourceSnapshot(ctx, m.ID, m.Revision, target.SourceID); e != nil {
		return out, e
	}
	query := req
	query.Cursor = ""
	out.QuerySHA256, _ = canonicalDigest(query)
	out.Total = len(difference.Rows)
	out.SourceVerification = "verified_by_reimport"
	out.Baseline = verified
	out.Target = target
	out.Difference = difference
	out.ContextMapping = req.ContextMapping
	out.Mapping = req.Mapping
	return out, nil
}

func (s *Service) OpenProfileEvidence(ctx context.Context, store *cases.Store, caseID, revision, resultID, resultSHA, sourceID, sourceSHA, reference string) (cases.TextPage, error) {
	m, e := store.Get(caseID)
	if e != nil {
		return cases.TextPage{}, e
	}
	if m.Revision != revision {
		return cases.TextPage{}, fmt.Errorf("CASE_REVISION_STALE")
	}
	var ref *cases.ResultRef
	for i := range m.Results {
		if m.Results[i].ID == resultID {
			ref = &m.Results[i]
		}
	}
	if ref == nil || ref.Stale || ref.SHA256 != resultSHA || ref.SourceHashes[sourceID] != sourceSHA {
		return cases.TextPage{}, fmt.Errorf("CASE_LINK_STALE")
	}
	raw, e := store.ExportSnapshot(caseID, resultID)
	if e != nil {
		return cases.TextPage{}, e
	}
	result, e := resultcontract.DecodeWithLimits(raw, wirejson.ResultLimits(cases.MaxObjectBytes, 1024))
	if e != nil {
		return cases.TextPage{}, e
	}
	if v := s.results.Check(result, "compute"); !v.Allowed {
		return cases.TextPage{}, fmt.Errorf("PROFILE_RESULT_UNSUPPORTED")
	}
	core, e := analyzer.ReadCore(result)
	if e != nil {
		return cases.TextPage{}, e
	}
	sourceMatches := false
	for _, sha := range core.Model.SourceHashes {
		sourceMatches = sourceMatches || sha == sourceSHA
	}
	found := false
	for _, p := range core.Model.Profiles {
		for _, sample := range p.Samples {
			if e = ctx.Err(); e != nil {
				return cases.TextPage{}, e
			}
			found = found || sample.SourceRef == reference
		}
	}
	if !sourceMatches || !found {
		return cases.TextPage{}, fmt.Errorf("CASE_LINK_STALE")
	}
	if core.Model.Format != "pyspy-raw" || !strings.HasPrefix(reference, "line:") {
		return cases.TextPage{}, fmt.Errorf("PROFILE_STRUCTURAL_REFERENCE: inspect the original format coordinate; no text line is inferred")
	}
	return store.OpenEvidence(ctx, caseID, revision, sourceID, sourceSHA, strings.TrimPrefix(reference, "line:"))
}
