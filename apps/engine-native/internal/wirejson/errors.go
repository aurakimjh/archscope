package wirejson

// LimitError distinguishes resource refusal from malformed JSON. Counts describe
// decoded UTF-8 bytes (strings/keys) or structural cardinality, never input text.
type LimitError struct {
	Resource string
	Actual   int64
	Limit    int64
	message  string
}

func (e *LimitError) Error() string { return e.message }

func exceeded(resource string, actual, limit int64, message string) error {
	return &LimitError{Resource: resource, Actual: actual, Limit: limit, message: message}
}
