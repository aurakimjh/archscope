package aiinterpretation

import (
	"fmt"
	"strings"
)

type ValidationIssue struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("AI interpretation validation failed with %d issue(s)", len(e.Issues))
}

type AiFindingValidator struct {
	Registry *EvidenceRegistry
}

func (v AiFindingValidator) ValidateInterpretation(payload map[string]any) (map[string]any, error) {
	findings, ok := payload["findings"].([]any)
	if !ok {
		return nil, ValidationError{Issues: []ValidationIssue{{Code: "INVALID_SCHEMA", Message: "findings must be an array"}}}
	}
	issues := []ValidationIssue{}
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if !ok {
			issues = append(issues, ValidationIssue{Code: "INVALID_SCHEMA", Message: "finding must be an object"})
			continue
		}
		issues = append(issues, v.validateFinding(finding)...)
	}
	if len(issues) > 0 {
		return nil, ValidationError{Issues: issues}
	}
	return payload, nil
}

func (v AiFindingValidator) validateFinding(finding map[string]any) []ValidationIssue {
	issues := []ValidationIssue{}
	refs, ok := finding["evidence_refs"].([]any)
	if !ok || len(refs) == 0 {
		return []ValidationIssue{{Code: "EVIDENCE_REF_REQUIRED", Message: "evidence_refs must be a non-empty array"}}
	}
	quotes := map[string]any{}
	if raw, ok := finding["evidence_quotes"].(map[string]any); ok {
		quotes = raw
	}
	for _, rawRef := range refs {
		ref, ok := rawRef.(string)
		ref = strings.TrimSpace(ref)
		if !ok || ref == "" {
			issues = append(issues, ValidationIssue{Code: "EVIDENCE_REF_REQUIRED", Message: "evidence_refs must contain strings"})
			continue
		}
		if _, ok := ParseEvidenceRef(ref); !ok {
			issues = append(issues, ValidationIssue{Code: "EVIDENCE_REF_GRAMMAR", Message: "invalid evidence_ref grammar", EvidenceRef: ref})
			continue
		}
		item, ok := v.Registry.Get(ref)
		if !ok {
			issues = append(issues, ValidationIssue{Code: "EVIDENCE_REF_UNKNOWN", Message: "evidence_ref is not present in the input evidence set", EvidenceRef: ref})
			continue
		}
		if quote, ok := quotes[ref].(string); ok && quote != "" && !strings.Contains(item.Text, quote) {
			issues = append(issues, ValidationIssue{Code: "EVIDENCE_QUOTE_MISMATCH", Message: "evidence quote is not present in source evidence", EvidenceRef: ref})
		}
	}
	return issues
}
