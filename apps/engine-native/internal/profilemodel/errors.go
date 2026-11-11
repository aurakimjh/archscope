package profilemodel

// BudgetError is an operational refusal, never a rounded/truncated result.
type BudgetError struct{ Reason string }

func (e *BudgetError) Error() string { return e.Reason }
func budget(reason string) error     { return &BudgetError{reason} }
