package packs

import (
	"encoding/json"
	"fmt"
)

// Preview evaluates exactly one bounded row with the same mapping interpreter.
// Raw values stay text; callers own display escaping and must not infer support
// for unexamined rows or producers from this sample.
func (m Mapping) Preview(raw []byte) (Record, string, error) {
	record, e := m.Evaluate(raw)
	if e != nil {
		return record, "", e
	}
	var object any
	if e = Strict(raw, &object); e != nil {
		return record, "", e
	}
	value, e := aliases(object, m.ValuePaths)
	if e != nil {
		return record, "", e
	}
	switch v := value.(type) {
	case string:
		return record, v, nil
	case json.Number:
		return record, v.String(), nil
	default:
		return record, fmt.Sprint(value), nil
	}
}
