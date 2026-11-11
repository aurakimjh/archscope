package profilemodel

import (
	"errors"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

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

// Preserve the wire boundary's resource identity so import orchestration can
// return a budget refusal with exact detail instead of a format mismatch.
func decodeModelJSON(body []byte, out any) error {
	e := wirejson.Decode(body, out)
	var bound *wirejson.LimitError
	if errors.As(e, &bound) {
		return limit(bound.Resource, bound.Actual, bound.Limit)
	}
	return e
}
