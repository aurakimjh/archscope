package aiinterpretation

func EvaluateInterpretation(payload map[string]any, validator AiFindingValidator) map[string]any {
	_, err := validator.ValidateInterpretation(payload)
	total := 0
	if findings, ok := payload["findings"].([]any); ok {
		total = len(findings)
	}
	if err == nil {
		return map[string]any{
			"total_findings":           total,
			"valid_findings":           total,
			"rejected_findings":        0,
			"evidence_integrity_ratio": 1.0,
			"valid":                    true,
			"gate_status":              "passed",
			"issue_codes":              []string{},
			"issues":                   []ValidationIssue{},
		}
	}
	issues := []ValidationIssue{}
	if validationErr, ok := err.(ValidationError); ok {
		issues = validationErr.Issues
	}
	issueCodes := make([]string, 0, len(issues))
	evidenceIssues := 0
	for _, issue := range issues {
		issueCodes = append(issueCodes, issue.Code)
		if isEvidenceIssue(issue.Code) {
			evidenceIssues += 1
		}
	}
	ratio := 0.0
	if total > 0 {
		ratio = 1.0 - float64(evidenceIssues)/float64(total)
		if ratio < 0 {
			ratio = 0
		}
	}
	return map[string]any{
		"total_findings":           total,
		"valid_findings":           0,
		"rejected_findings":        total,
		"evidence_integrity_ratio": ratio,
		"valid":                    false,
		"gate_status":              "blocked",
		"issue_codes":              issueCodes,
		"issues":                   issues,
	}
}

func isEvidenceIssue(code string) bool {
	switch code {
	case "EVIDENCE_REFS_REQUIRED",
		"EVIDENCE_REF_BLANK",
		"EVIDENCE_REF_GRAMMAR",
		"EVIDENCE_REF_UNKNOWN",
		"EVIDENCE_QUOTES_REQUIRED",
		"EVIDENCE_QUOTE_REQUIRED",
		"EVIDENCE_QUOTE_MISMATCH":
		return true
	default:
		return false
	}
}
