package aiinterpretation

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ValidationIssue struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	FindingID   string `json:"finding_id,omitempty"`
	EvidenceRef string `json:"evidence_ref,omitempty"`
}

type ValidationError struct {
	Issues []ValidationIssue
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("AI interpretation validation failed with %d issue(s)", len(e.Issues))
}

type AiFindingValidator struct {
	Registry              *EvidenceRegistry
	MinConfidence         float64
	RequireEvidenceQuotes bool
}

func (v AiFindingValidator) ValidateInterpretation(payload map[string]any) (map[string]any, error) {
	issues := []ValidationIssue{}
	if stringValue(payload["schema_version"]) != "0.1.0" {
		issues = append(issues, ValidationIssue{Code: "SCHEMA_VERSION", Message: "schema_version must be 0.1.0"})
	}
	findings, ok := payload["findings"].([]any)
	if !ok {
		issues = append(issues, ValidationIssue{Code: "FINDINGS_REQUIRED", Message: "findings must be an array"})
		return nil, ValidationError{Issues: issues}
	}
	validFindings := []any{}
	for _, raw := range findings {
		finding, ok := raw.(map[string]any)
		if !ok {
			issues = append(issues, ValidationIssue{Code: "FINDING_SHAPE", Message: "finding must be an object"})
			continue
		}
		findingIssues := v.validateFinding(finding)
		if len(findingIssues) > 0 {
			issues = append(issues, findingIssues...)
			continue
		}
		validFindings = append(validFindings, finding)
	}
	if len(issues) > 0 {
		return nil, ValidationError{Issues: issues}
	}
	payload["findings"] = validFindings
	return payload, nil
}

func (v AiFindingValidator) validateFinding(finding map[string]any) []ValidationIssue {
	issues := []ValidationIssue{}
	findingID := stringValue(finding["id"])
	for _, field := range []string{"id", "label", "model", "summary", "reasoning"} {
		if stringValue(finding[field]) == "" {
			issues = append(issues, ValidationIssue{
				Code:      strings.ToUpper(field) + "_REQUIRED",
				Message:   field + " must be a non-empty string",
				FindingID: findingID,
			})
		}
	}
	if stringValue(finding["generated_by"]) != "ai" {
		issues = append(issues, ValidationIssue{
			Code:      "GENERATED_BY_REQUIRED",
			Message:   "generated_by must be ai",
			FindingID: findingID,
		})
	}
	severity := stringValue(finding["severity"])
	if !validSeverity(severity) {
		issues = append(issues, ValidationIssue{
			Code:      "SEVERITY_INVALID",
			Message:   "severity must be info, warning, or critical",
			FindingID: findingID,
		})
	}
	confidence, ok := numberValue(finding["confidence"])
	if !ok || confidence < 0 || confidence > 1 {
		issues = append(issues, ValidationIssue{
			Code:      "CONFIDENCE_INVALID",
			Message:   "confidence must be a number between 0 and 1",
			FindingID: findingID,
		})
	} else if confidence < v.minConfidence() {
		issues = append(issues, ValidationIssue{
			Code:      "CONFIDENCE_TOO_LOW",
			Message:   fmt.Sprintf("confidence must be at least %.2f", v.minConfidence()),
			FindingID: findingID,
		})
	}
	if !stringSliceOK(finding["limitations"]) {
		issues = append(issues, ValidationIssue{
			Code:      "LIMITATIONS_INVALID",
			Message:   "limitations must be an array of strings",
			FindingID: findingID,
		})
	}
	refs, ok := finding["evidence_refs"].([]any)
	if !ok || len(refs) == 0 {
		return append(issues, ValidationIssue{
			Code:      "EVIDENCE_REFS_REQUIRED",
			Message:   "evidence_refs must be a non-empty array",
			FindingID: findingID,
		})
	}
	quotes := map[string]any{}
	if raw, exists := finding["evidence_quotes"]; exists {
		if typed, ok := raw.(map[string]any); ok {
			quotes = typed
		} else {
			issues = append(issues, ValidationIssue{
				Code:      "EVIDENCE_QUOTES_INVALID",
				Message:   "evidence_quotes must be an object",
				FindingID: findingID,
			})
		}
	} else if v.RequireEvidenceQuotes {
		issues = append(issues, ValidationIssue{
			Code:      "EVIDENCE_QUOTES_REQUIRED",
			Message:   "evidence_quotes must be present when quote matching is required",
			FindingID: findingID,
		})
	}
	for _, rawRef := range refs {
		ref, ok := rawRef.(string)
		ref = strings.TrimSpace(ref)
		if !ok || ref == "" {
			issues = append(issues, ValidationIssue{
				Code:      "EVIDENCE_REF_BLANK",
				Message:   "evidence_refs must contain non-empty strings",
				FindingID: findingID,
			})
			continue
		}
		if _, ok := ParseEvidenceRef(ref); !ok {
			issues = append(issues, ValidationIssue{
				Code:        "EVIDENCE_REF_GRAMMAR",
				Message:     "invalid evidence_ref grammar",
				FindingID:   findingID,
				EvidenceRef: ref,
			})
			continue
		}
		item, ok := v.Registry.Get(ref)
		if !ok {
			issues = append(issues, ValidationIssue{
				Code:        "EVIDENCE_REF_UNKNOWN",
				Message:     "evidence_ref is not present in the input evidence set",
				FindingID:   findingID,
				EvidenceRef: ref,
			})
			continue
		}
		quote, ok := quotes[ref].(string)
		if v.RequireEvidenceQuotes && (!ok || strings.TrimSpace(quote) == "") {
			issues = append(issues, ValidationIssue{
				Code:        "EVIDENCE_QUOTE_REQUIRED",
				Message:     "evidence_quotes must include a non-empty quote for every evidence_ref",
				FindingID:   findingID,
				EvidenceRef: ref,
			})
			continue
		}
		if ok && strings.TrimSpace(quote) != "" && !containsNormalized(item.Text, quote) {
			issues = append(issues, ValidationIssue{
				Code:        "EVIDENCE_QUOTE_MISMATCH",
				Message:     "evidence quote is not present in source evidence",
				FindingID:   findingID,
				EvidenceRef: ref,
			})
		}
	}
	return issues
}

func (v AiFindingValidator) minConfidence() float64 {
	if v.MinConfidence > 0 {
		return v.MinConfidence
	}
	return 0.3
}

func validSeverity(value string) bool {
	switch value {
	case "info", "warning", "critical":
		return true
	default:
		return false
	}
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func stringSliceOK(value any) bool {
	switch items := value.(type) {
	case []any:
		for _, item := range items {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	case []string:
		return true
	default:
		return false
	}
}

func containsNormalized(text, quote string) bool {
	return strings.Contains(normalizeText(text), normalizeText(quote))
}

func normalizeText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
