package resultcontract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// Projection owns a private decoded snapshot. Only bounded copied DTOs escape;
// a caller cannot change the content after its validation or mutate an index.
// Collection indexes are built once on demand; failed/canceled builds never cache.
type Projection struct {
	registry          *Registry
	result            models.AnalysisResult
	capabilities      Capabilities
	comparable        Verdict
	measurements      map[string]Measurement
	digest, canonical string
	weight            int64
	gate              chan struct{}
	indexes           map[string]*collectionIndex
	keyValues         map[string]*keyValueIndex
}
type collectionIndex struct {
	rows               reflect.Value
	keys, labels       []string
	truncated, escaped bool
}

func (r *Registry) PrepareBytes(ctx context.Context, body []byte, limits wirejson.Limits) (*Projection, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	result, e := DecodeWithLimits(body, limits)
	if e != nil {
		return nil, e
	}
	p, e := r.prepare(ctx, result)
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(body)
	p.digest = hex.EncodeToString(sum[:])
	p.weight, e = projectionWeight(ctx, reflect.ValueOf(result))
	if e != nil {
		return nil, e
	}
	// Reserve space for raw digest, capabilities and at most 128 column indexes.
	p.weight += int64(len(body)) + 16<<20
	return p, nil
}
func (r *Registry) prepare(ctx context.Context, result models.AnalysisResult) (*Projection, error) {
	caps, e := r.Capabilities(ctx, result)
	if e != nil {
		return nil, e
	}
	canonical, e := canonicalDigest(result)
	if e != nil {
		return nil, e
	}
	measurements, e := Measurements(result)
	if e != nil || !caps.Compatibility.Allowed {
		measurements = map[string]Measurement{}
	}
	p := &Projection{registry: r, result: result, capabilities: caps, comparable: r.check(result, "compare", caps.Compatibility.Allowed), canonical: canonical, measurements: measurements, gate: make(chan struct{}, 1), indexes: map[string]*collectionIndex{}, keyValues: map[string]*keyValueIndex{}}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	return p, nil
}
func (p *Projection) Digest() string         { return p.digest }
func (p *Projection) PayloadVersion() string { return p.result.Metadata.SchemaVersion }
func (p *Projection) Type() string           { return p.result.Type }
func (p *Projection) Weight() int64          { return p.weight }
func (p *Projection) Capabilities() Capabilities {
	out := p.capabilities
	out.Providers = append([]Capability{}, out.Providers...)
	return out
}
func (p *Projection) Sources() ([]Cell, bool) {
	out := []Cell{}
	truncated := false
	for i, source := range p.result.SourceFiles {
		if i == 32 {
			truncated = true
			break
		}
		cell := displayCell(source)
		out = append(out, cell)
		truncated = truncated || cell.Truncated
	}
	return out, truncated
}
func (p *Projection) Compare(ctx context.Context, other *Projection, path string) (Delta, error) {
	if e := ctx.Err(); e != nil {
		return Delta{}, e
	}
	if other == nil || p.registry != other.registry {
		return Delta{}, comparisonError(ComparisonResultUnsupported, "different result registries")
	}
	for _, candidate := range []*Projection{p, other} {
		if !candidate.comparable.Allowed {
			return Delta{}, comparisonError(ComparisonResultUnsupported, candidate.comparable.Reason)
		}
	}
	delta, e := p.registry.compareValidated(p.result, other.result, path, p.canonical, other.canonical)
	if canceled := ctx.Err(); canceled != nil {
		return Delta{}, canceled
	}
	return delta, e
}
func (p *Projection) collection(ctx context.Context, provider Provider) (*collectionIndex, error) {
	select {
	case p.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-p.gate }()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if cached := p.indexes[provider.Path]; cached != nil {
		return cached, nil
	}
	var rows reflect.Value
	var e error
	if provider.ID == "generic:findings" {
		rows = reflect.ValueOf(p.result.Metadata.Findings)
	} else {
		rows, e = rowsAt(p.result, provider.Path)
		if e != nil {
			return nil, e
		}
	}
	out := &collectionIndex{rows: rows, keys: []string{}, labels: []string{}}
	columns := map[string]bool{}
	for i := 0; i < rows.Len(); i++ {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		row, e := objectRow(rows.Index(i))
		if e != nil {
			return nil, e
		}
		for key := range row {
			if len(key) > MaxCellBytes {
				out.truncated = true
				continue
			}
			if columns[key] {
				continue
			}
			if len(columns) < MaxPageColumns {
				columns[key] = true
				continue
			}
			out.truncated = true
			largest := ""
			for k := range columns {
				if k > largest {
					largest = k
				}
			}
			if key < largest {
				delete(columns, largest)
				columns[key] = true
			}
		}
	}
	for key := range columns {
		out.keys = append(out.keys, key)
	}
	sort.Strings(out.keys)
	for _, key := range out.keys {
		label, truncated, escaped := DisplayText(key)
		out.labels = append(out.labels, label)
		out.truncated = out.truncated || truncated
		out.escaped = out.escaped || escaped
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	p.indexes[provider.Path] = out
	return out, nil
}

// Conservative structural admission units, not an exact RSS measurement. Stop
// counting after the service's entire budget: this projection won't be retained.
func projectionWeight(ctx context.Context, v reflect.Value) (int64, error) {
	const cap = int64(512<<20) + 1
	var total int64
	var visit func(reflect.Value) error
	visit = func(v reflect.Value) error {
		if total >= cap {
			return nil
		}
		if e := ctx.Err(); e != nil {
			return e
		}
		if !v.IsValid() {
			total += 16
			return nil
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			total += 16
			if !v.IsNil() {
				return visit(v.Elem())
			}
		case reflect.String:
			total += 32 + int64(v.Len())
		case reflect.Map:
			total += 128 + int64(v.Len())*64
			iter := v.MapRange()
			for iter.Next() {
				if e := visit(iter.Key()); e != nil {
					return e
				}
				if e := visit(iter.Value()); e != nil {
					return e
				}
				if total >= cap {
					break
				}
			}
		case reflect.Slice, reflect.Array:
			total += 64 + int64(v.Len())*16
			for i := 0; i < v.Len() && total < cap; i++ {
				if e := visit(v.Index(i)); e != nil {
					return e
				}
			}
		case reflect.Struct:
			total += 64
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					if e := visit(v.Field(i)); e != nil {
						return e
					}
				}
			}
		default:
			total += 32
		}
		return nil
	}
	if e := visit(v); e != nil {
		return 0, e
	}
	if total < 0 {
		return 0, fmt.Errorf("projection weight overflow")
	}
	return total, nil
}

type keyValueIndex struct {
	values map[string]any
	keys   []string
}

func (p *Projection) values(ctx context.Context, provider Provider) (*keyValueIndex, error) {
	select {
	case p.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-p.gate }()
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if cached := p.keyValues[provider.Path]; cached != nil {
		return cached, nil
	}
	values := p.result.Summary
	if provider.Kind == "diagnostics" {
		values = map[string]any{}
		if p.result.Metadata.Diagnostics != nil {
			body, e := json.Marshal(p.result.Metadata.Diagnostics)
			if e != nil {
				return nil, e
			}
			decoder := json.NewDecoder(strings.NewReader(string(body)))
			decoder.UseNumber()
			if e = decoder.Decode(&values); e != nil {
				return nil, e
			}
		}
	}
	out := &keyValueIndex{values: values, keys: []string{}}
	for key := range values {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		out.keys = append(out.keys, key)
	}
	sort.Strings(out.keys)
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	p.keyValues[provider.Path] = out
	return out, nil
}

// ExtensionJSON copies a bounded extension for internal semantic readers. The
// private prepared result and indexes never escape to callers or frontend IPC.
func (p *Projection) ExtensionJSON(ctx context.Context, name string, limit int) (json.RawMessage, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if verdict := p.registry.check(p.result, "compute", p.capabilities.Compatibility.Allowed); !verdict.Allowed {
		return nil, fmt.Errorf("RESULT_COMPUTE_UNSUPPORTED: %s", verdict.Reason)
	}
	value, ok := p.result.Metadata.Extra[name]
	if !ok {
		return nil, fmt.Errorf("RESULT_EXTENSION_MISSING")
	}
	raw, e := json.Marshal(value)
	if e != nil {
		return nil, e
	}
	if limit < 1 || len(raw) > limit {
		return nil, fmt.Errorf("RESULT_EXTENSION_BUDGET")
	}
	return raw, ctx.Err()
}
