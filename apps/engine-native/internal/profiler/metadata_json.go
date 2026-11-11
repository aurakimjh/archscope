package profiler

import (
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

var metadataKeys = []string{"parser", "schema_version", "diagnostics", "timeline_scope", "progress_log_path", "diff_summary", "diff_tables"}

func (m Metadata) MarshalJSON() ([]byte, error) {
	type fields Metadata
	b, e := json.Marshal(fields(m))
	if e != nil {
		return nil, e
	}
	var out map[string]json.RawMessage
	if e = wirejson.Decode(b, &out); e != nil {
		return nil, e
	}
	for k, v := range m.Extra {
		reserved := false
		for _, key := range metadataKeys {
			reserved = reserved || k == key
		}
		if !reserved {
			out[k] = v
		}
	}
	return json.Marshal(out)
}
func (m *Metadata) UnmarshalJSON(b []byte) error {
	type fields Metadata
	var next fields
	if e := wirejson.Decode(b, &next); e != nil {
		return e
	}
	var raw map[string]json.RawMessage
	if e := wirejson.Decode(b, &raw); e != nil {
		return e
	}
	for _, k := range metadataKeys {
		delete(raw, k)
	}
	next.Extra = raw
	*m = Metadata(next)
	return nil
}
