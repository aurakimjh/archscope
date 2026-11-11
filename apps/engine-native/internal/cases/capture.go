package cases

import (
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/common"
	"sort"
	"strings"
)

func validBasis(s string) bool {
	return s == "observed" || s == "user" || s == "estimated" || s == "unknown"
}
func claim(c *Claim) error {
	if c == nil {
		return nil
	}
	if !validBasis(c.Basis) || c.Value == nil && c.Basis != "unknown" || c.Value != nil && c.Basis == "unknown" {
		return problem("CAPTURE_BASIS_INVALID", "a known value needs observed/user/estimated basis; unknown remains null")
	}
	if len(c.Evidence) > 4096 || c.Value != nil && len(*c.Value) > 4096 {
		return problem("CASE_BUDGET_EXCEEDED", "claim byte limit")
	}
	return nil
}

// Validate checks declarations, never manufactures a clock anchor from file time.
func (c Capture) Validate() error {
	if c.SchemaVersion != Version {
		return problem("CAPTURE_VERSION_UNSUPPORTED", "unsupported capture sidecar")
	}
	if !validID(c.ID, "capture") || !validBasis(c.IdentityBasis) || len(c.Producer) > 256 || len(c.ProducerVersion) > 256 {
		return problem("CAPTURE_INVALID", "invalid producer/identity declaration")
	}
	if len(c.Files) > MaxEntries || len(c.Entities) > MaxEntries || len(c.Devices) > MaxEntries || len(c.Options) > 32 || len(c.Quality) > 32 || len(c.Conversions) > MaxEntries {
		return problem("CASE_BUDGET_EXCEEDED", "capture cardinality limit")
	}
	entities := map[string]bool{}
	for _, e := range c.Entities {
		if e.ID == "" || len(e.ID) > 256 || entities[e.ID] {
			return problem("CAPTURE_INVALID", "duplicate/empty entity identity")
		}
		entities[e.ID] = true
		for _, v := range []*Claim{e.Host, e.Boot, e.ProcessStart, e.PID, e.TIDNamespace, e.Container, e.Pod, e.Worker} {
			if err := claim(v); err != nil {
				return err
			}
		}
	}
	if c.Clock.Domain != "unknown" && c.Clock.Domain != "wall" && c.Clock.Domain != "monotonic" && c.Clock.Domain != "relative" {
		return problem("CAPTURE_INVALID", "explicit clock domain required")
	}
	for _, v := range []*Claim{c.Clock.Start, c.Clock.End, c.Clock.Timezone, c.Clock.AnchorUTC, c.Clock.RelativeTick, c.Clock.Offset, c.Clock.Drift, c.Clock.Uncertainty} {
		if err := claim(v); err != nil {
			return err
		}
	}
	devices := map[string]bool{}
	for _, d := range c.Devices {
		if d.ID == "" || len(d.ID) > 256 || devices[d.ID] || !validBasis(d.MappingBasis) || d.Vendor == "" || len(d.Vendor) > 256 || len(d.Kind) > 128 || len(d.Properties) > 64 {
			return problem("CAPTURE_INVALID", "invalid device declaration")
		}
		devices[d.ID] = true
		if d.MemoryModel != "unknown" && d.MemoryModel != "unified" && d.MemoryModel != "dedicated" {
			return problem("CAPTURE_INVALID", "explicit neutral memory model required")
		}
		for k, v := range d.Properties {
			if len(k) > 128 {
				return problem("CAPTURE_INVALID", "device property name limit")
			}
			if e := claim(&v); e != nil {
				return e
			}
		}
	}
	parents := map[string]string{}
	for _, d := range c.Devices {
		if d.ParentID != "" && !devices[d.ParentID] {
			return problem("CAPTURE_INVALID", "unresolved device parent")
		}
		parents[d.ID] = d.ParentID
	}
	for k := range parents {
		seen := map[string]bool{}
		for p := k; p != ""; p = parents[p] {
			if seen[p] {
				return problem("CAPTURE_INVALID", "device hierarchy cycle")
			}
			seen[p] = true
		}
	}
	names := map[string]bool{}
	for _, f := range c.Files {
		key := nameKey(f.Name)
		if !safeName(f.Name) || names[key] || !digestRE.MatchString(f.SHA256) || !validSize(f.Size) {
			return problem("CAPTURE_INVALID", "invalid/duplicate capture file")
		}
		names[key] = true
		for _, e := range f.EntityIDs {
			if !entities[e] {
				return problem("CAPTURE_INVALID", "unresolved file entity")
			}
		}
	}
	// These are semantic allowlisted values, never an invocation or environment dump.
	allowed := map[string]bool{"sample_interval": true, "include": true, "exclude": true, "workload": true, "warmup": true, "duration": true, "units": true}
	for k, v := range c.Options {
		if !allowed[k] {
			return problem("CAPTURE_OPTION_FORBIDDEN", "capture options use a bounded semantic allowlist")
		}
		if e := requireJSON(v); e != nil {
			return e
		}
		var value any
		if e := decodeJSON(v, &value); e != nil {
			return e
		}
		switch x := value.(type) {
		case nil, bool, json.Number:
		case string:
			if len(x) > 4096 {
				return problem("CASE_BUDGET_EXCEEDED", "option string limit")
			}
		case []any:
			if k != "include" && k != "exclude" || len(x) > 128 {
				return problem("CAPTURE_OPTION_FORBIDDEN", "only bounded include/exclude string lists supported")
			}
			for _, item := range x {
				str, ok := item.(string)
				if !ok || len(str) > 4096 {
					return problem("CAPTURE_OPTION_FORBIDDEN", "include/exclude values must be bounded strings")
				}
			}
		default:
			return problem("CAPTURE_OPTION_FORBIDDEN", "nested command/environment options unsupported")
		}

	}
	for _, v := range c.Quality {
		if e := claim(&v); e != nil {
			return e
		}
	}
	for _, v := range c.Conversions {
		if v.Tool == "" || v.Version == "" || !digestRE.MatchString(v.InputSHA256) || !digestRE.MatchString(v.OutputSHA256) || len(v.Location) > 4096 || len(v.LostCapabilities) > 128 {
			return problem("CAPTURE_INVALID", "conversion provenance requires tool/version/input/output digests")
		}
	}
	b, e := json.Marshal(c)
	if e != nil {
		return e
	}
	if len(b) > MaxManifestBytes {
		return problem("CASE_BUDGET_EXCEEDED", "capture byte limit")
	}
	return nil
}

// MergeCaptureContext keeps sidecar claims and records conflicts. Header evidence
// is not allowed to silently overwrite declared context (or vice versa).
func MergeCaptureContext(sidecar Capture, header map[string]string) (Capture, error) {
	if e := sidecar.Validate(); e != nil {
		return sidecar, e
	}

	if len(header) > 128 {
		return sidecar, problem("CASE_BUDGET_EXCEEDED", "header declaration limit")
	}
	known := map[string]string{"producer": sidecar.Producer, "producer_version": sidecar.ProducerVersion, "clock.domain": sidecar.Clock.Domain}
	add := func(key string, c *Claim) {
		if c != nil && c.Value != nil {
			known[key] = *c.Value
		}
	}
	for key, c := range map[string]*Claim{"start": sidecar.Clock.Start, "end": sidecar.Clock.End, "timezone": sidecar.Clock.Timezone, "anchor_utc": sidecar.Clock.AnchorUTC, "relative_tick": sidecar.Clock.RelativeTick, "offset": sidecar.Clock.Offset, "drift": sidecar.Clock.Drift, "uncertainty": sidecar.Clock.Uncertainty} {
		add(key, c)
	}
	for _, e := range sidecar.Entities {
		for key, c := range map[string]*Claim{"host": e.Host, "boot": e.Boot, "process_start": e.ProcessStart, "pid": e.PID, "tid_namespace": e.TIDNamespace, "container": e.Container, "pod": e.Pod, "worker": e.Worker} {
			add("entity."+e.ID+"."+key, c)
		}
	}
	for _, d := range sidecar.Devices {
		known["device."+d.ID+".vendor"] = d.Vendor
		known["device."+d.ID+".memory_model"] = d.MemoryModel
		for key, c := range d.Properties {
			add("device."+d.ID+"."+key, &c)
		}
	}
	keys := make([]string, 0, len(header))
	for k := range header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := header[k]
		if len(k) > 1024 || len(v) > 4096 {
			return sidecar, problem("CASE_BUDGET_EXCEEDED", "header byte limit")
		}
		if expected, ok := known[k]; ok && expected != "" && expected != "unknown" && expected != v {
			sidecar.Warnings = append(sidecar.Warnings, "header/sidecar conflict: "+k)
		}
	}

	return sidecar, nil
}

// RedactJSONStrings preserves numeric/null tokens. A reviewed sharing decision is
// separate from heuristic text masking, which cannot establish absence of secrets.
func RedactJSONStrings(raw json.RawMessage, redact func(string) string) (json.RawMessage, error) {
	var value any
	if e := decodeJSON(raw, &value); e != nil {
		return nil, e
	}
	var walk func(any) any
	walk = func(v any) any {
		switch x := v.(type) {
		case string:
			return redact(x)
		case []any:
			for i := range x {
				x[i] = walk(x[i])
			}
		case map[string]any:
			for k, v := range x {
				x[k] = walk(v)
			}
		}
		return v
	}
	return json.Marshal(walk(value))
}
func decodeJSON(raw []byte, out any) error {
	if e := requireJSON(raw); e != nil {
		return e
	}
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	return d.Decode(out)
}

// SanitizedOptions masks only descriptive option strings. Measurement scalars
// keep their tokens; no full command/environment object is accepted.
func (c Capture) SanitizedOptions() (Capture, error) {
	if e := c.Validate(); e != nil {
		return c, e
	}
	out := map[string]json.RawMessage{}
	for k, v := range c.Options {
		out[k] = append(json.RawMessage(nil), v...)
		if k == "workload" || k == "include" || k == "exclude" {
			b, e := RedactJSONStrings(v, func(s string) string { return common.RedactText(s).Text })
			if e != nil {
				return c, e
			}
			out[k] = b
		}
	}
	c.Options = out
	return c, nil
}
