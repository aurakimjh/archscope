package aiinterpretation

func EvaluateInterpretation(payload map[string]any, validator AiFindingValidator) map[string]any {
	_, err := validator.ValidateInterpretation(payload)
	total := 0
	if findings, ok := payload["findings"].([]any); ok {
		total = len(findings)
	}
	valid := err == nil
	ratio := 0.0
	if valid && total > 0 {
		ratio = 1.0
	}
	return map[string]any{
		"total_findings":           total,
		"evidence_integrity_ratio": ratio,
		"valid":                    valid,
	}
}
