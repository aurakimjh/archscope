package profilemodel

import "fmt"

// BudgetError is an operational refusal, never a rounded/truncated result.
type BudgetError struct {
	Reason   string `json:"reason"`
	Resource string `json:"resource,omitempty"`
	Actual   int64  `json:"actual,omitempty"`
	Limit    int64  `json:"limit,omitempty"`
}

func (e *BudgetError) Error() string { return e.Reason }
func budget(reason string) error     { return &BudgetError{Reason: reason} }
func limit(resource string, actual, ceiling int64) error {
	return &BudgetError{Reason: fmt.Sprintf("%s budget exceeded: %d > %d", resource, actual, ceiling), Resource: resource, Actual: actual, Limit: ceiling}
}
