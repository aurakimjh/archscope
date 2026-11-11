package profilemodel

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// Explicit null timing and field absence are both unavailable, but distinct wire
// observations. Known zero is a non-nil decimal string and stays measured zero.
func (s Sample) MarshalJSON() ([]byte, error) {
	type plain Sample
	body, e := json.Marshal(plain(s))
	if e != nil {
		return nil, e
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(body, &fields); e != nil {
		return nil, e
	}
	if s.TimestampNull {
		if s.Timestamp != nil {
			return nil, fmt.Errorf("timestamp cannot be null and measured")
		}
		fields["timestamp"] = json.RawMessage(`null`)
	}
	if s.IntervalNull {
		if s.Interval != nil {
			return nil, fmt.Errorf("interval cannot be null and measured")
		}
		fields["interval"] = json.RawMessage(`null`)
	}
	return json.Marshal(fields)
}
func (s *Sample) UnmarshalJSON(body []byte) error {
	type plain Sample
	var next plain
	var fields map[string]json.RawMessage
	if e := wirejson.Decode(body, &fields); e != nil {
		return e
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&next); e != nil {
		return e
	}
	next.TimestampNull = bytes.Equal(bytes.TrimSpace(fields["timestamp"]), []byte("null"))
	next.IntervalNull = bytes.Equal(bytes.TrimSpace(fields["interval"]), []byte("null"))
	*s = Sample(next)
	return nil
}
