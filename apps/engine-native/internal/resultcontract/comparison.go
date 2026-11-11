package resultcontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

const (
	ComparisonResultUnsupported        = "result_unsupported"
	ComparisonSelectionMismatch        = "selection_mismatch"
	ComparisonCoverageMismatch         = "coverage_mismatch"
	ComparisonPartialNotComparable     = "partial_not_comparable"
	ComparisonSemanticRevisionMismatch = "semantic_revision_mismatch"
	ComparisonPolicyUnsupported        = "policy_unsupported"
	ComparisonPolicyMissing            = "policy_missing"
	ComparisonIdentityMissing          = "identity_missing"
	ComparisonIdentityInvalid          = "identity_invalid"
	ComparisonMeasurementMismatch      = "measurement_mismatch"
)

// ComparisonError is machine-readable in Go and in Wails' default JSON error
// cause. It contains no raw source paths or exception text.
type ComparisonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ComparisonError) Error() string { return e.Code + ": " + e.Message }

func comparisonError(code, message string) error {
	return &ComparisonError{Code: code, Message: message}
}

// PopulationComparison is additive metadata. Old snapshots retain their exact
// same-population semantics; they do not acquire a cross-source policy by guess.
type PopulationComparison struct {
	Version       string `json:"version"`
	SelectionHash string `json:"selection_hash"`
	CoverageKind  string `json:"coverage_kind"`
}

type ComparisonReference struct {
	Handle       string   `json:"handle,omitempty"`
	ResultSHA256 string   `json:"result_sha256"`
	Population   string   `json:"population_ref,omitempty"`
	SourceHashes []string `json:"source_sha256,omitempty"`
}

// References record the caller's ordered baseline and target. Comparing never
// merges captures, equates entities, or certifies clock alignment/causality.
type ComparisonEvidence struct {
	// A digest identifies the result bytes, not the authenticity of its sources.
	// Never derive a verification claim from fields in a loaded result file.
	IdentityBasis string                `json:"identity_basis"`
	Mode          string                `json:"mode"`
	Policy        *PopulationComparison `json:"policy,omitempty"`
	Baseline      ComparisonReference   `json:"baseline"`
	Target        ComparisonReference   `json:"target"`
}

func canonicalDigest(result models.AnalysisResult) (string, error) {
	body, e := json.Marshal(result)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
func comparePopulations(a, b models.AnalysisResult, aid, bid string) (*ComparisonEvidence, error) {
	left, e := canonicalDigest(a)
	if e != nil {
		return nil, e
	}
	right, e := canonicalDigest(b)
	if e != nil {
		return nil, e
	}
	return comparePopulationsAt(a, b, aid, bid, left, right)
}
func comparePopulationsAt(a, b models.AnalysisResult, aid, bid, leftHash, rightHash string) (*ComparisonEvidence, error) {
	out := &ComparisonEvidence{IdentityBasis: "declared", Mode: "same_input", Baseline: ComparisonReference{ResultSHA256: leftHash, Population: aid}, Target: ComparisonReference{ResultSHA256: rightHash, Population: bid}}
	if aid == "" {
		if leftHash != rightHash {
			return nil, comparisonError(ComparisonIdentityMissing, "population reference required for cross-revision comparison")
		}
		return out, nil
	}

	left, e := populationIdentity(a, aid)
	if e != nil {
		return nil, e
	}
	right, e := populationIdentity(b, bid)
	if e != nil {
		return nil, e
	}
	out.Baseline.SourceHashes, out.Target.SourceHashes = left.SourceHashes, right.SourceHashes
	// An additive policy must not break old/new snapshots of the same input.
	legacyLeft, legacyRight := left, right
	legacyLeft.Comparison, legacyRight.Comparison = nil, nil
	lb, _ := json.Marshal(legacyLeft)
	rb, _ := json.Marshal(legacyRight)
	if string(lb) == string(rb) {
		return out, nil
	}
	const mismatch = "population comparison policy mismatch; original reanalysis may be required"
	// A deterministic order gives consumers one actionable refusal at a time.
	if left.SemanticRevision != right.SemanticRevision {
		return nil, comparisonError(ComparisonSemanticRevisionMismatch, mismatch)
	}
	if declaresPartial(a) || declaresPartial(b) {
		return nil, comparisonError(ComparisonPartialNotComparable, mismatch)
	}
	if left.Comparison == nil || right.Comparison == nil {
		return nil, comparisonError(ComparisonPolicyMissing, mismatch)
	}
	if !supportedComparison(left.Comparison) || !supportedComparison(right.Comparison) {
		return nil, comparisonError(ComparisonPolicyUnsupported, mismatch)
	}
	if left.Comparison.SelectionHash != right.Comparison.SelectionHash {
		return nil, comparisonError(ComparisonSelectionMismatch, mismatch)
	}
	if left.Comparison.CoverageKind != right.Comparison.CoverageKind {
		return nil, comparisonError(ComparisonCoverageMismatch, mismatch)
	}
	out.Policy = left.Comparison
	if !equalSources(left.SourceHashes, right.SourceHashes) {
		out.Mode = "baseline_target"
	}
	return out, nil
}

func declaresPartial(result models.AnalysisResult) bool {
	body, e := json.Marshal(result.Metadata.Extra["import_coverage"])
	if e != nil {
		return false
	}
	var coverage struct {
		Partial bool `json:"partial"`
	}
	return wirejson.Decode(body, &coverage) == nil && coverage.Partial
}

func equalSources(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func supportedComparison(p *PopulationComparison) bool {
	if p == nil || p.Version != "1.0.0" || !sha256Text.MatchString(p.SelectionHash) {
		return false
	}
	switch p.CoverageKind {
	case "complete_records", "selected_records:max_lines", "complete_selected_model":
		return true
	}
	return false
}
