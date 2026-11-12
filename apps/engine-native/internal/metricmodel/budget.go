package metricmodel

import "fmt"

type BudgetError struct {
	Resource      string
	Actual, Limit int64
}

func (e *BudgetError) Error() string {
	return fmt.Sprintf("METRIC_BUDGET_EXCEEDED: %s=%d (limit %d); reduce export scope or select fewer series", e.Resource, e.Actual, e.Limit)
}
func budget(resource string, actual, limit int) error {
	return &BudgetError{resource, int64(actual), int64(limit)}
}
