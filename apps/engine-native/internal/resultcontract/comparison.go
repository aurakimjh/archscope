package resultcontract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

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
	Mode     string                `json:"mode"`
	Policy   *PopulationComparison `json:"policy,omitempty"`
	Baseline ComparisonReference   `json:"baseline"`
	Target   ComparisonReference   `json:"target"`
}

func comparePopulations(a, b models.AnalysisResult, aid, bid string) (*ComparisonEvidence, error) {
	leftBytes, e := json.Marshal(a)
	if e != nil {
		return nil, e
	}
	rightBytes, e := json.Marshal(b)
	if e != nil {
		return nil, e
	}
	lh, rh := sha256.Sum256(leftBytes), sha256.Sum256(rightBytes)
	out := &ComparisonEvidence{Mode: "same_input", Baseline: ComparisonReference{ResultSHA256: hex.EncodeToString(lh[:]), Population: aid}, Target: ComparisonReference{ResultSHA256: hex.EncodeToString(rh[:]), Population: bid}}
	if aid == "" {
		if !bytes.Equal(leftBytes, rightBytes) {
			return nil, fmt.Errorf("population reference required for cross-revision comparison")
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
	if bytes.Equal(lb, rb) {
		return out, nil
	}
	if left.SemanticRevision != right.SemanticRevision || !supportedComparison(left.Comparison) || !supportedComparison(right.Comparison) || *left.Comparison != *right.Comparison {
		return nil, fmt.Errorf("population comparison policy mismatch; original reanalysis may be required")
	}
	out.Policy = left.Comparison
	if !equalSources(left.SourceHashes, right.SourceHashes) {
		out.Mode = "baseline_target"
	}
	return out, nil
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
