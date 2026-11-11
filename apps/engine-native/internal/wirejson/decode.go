// Package wirejson owns the bounded, precision-preserving JSON read boundary.
package wirejson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Limits struct {
	Bytes                                      int64
	Depth, StringBytes, ArrayItems, ObjectKeys int
}

// Operational allocation bounds, not diagnostic thresholds.
var DefaultLimits = Limits{64 << 20, 64, 4 << 20, 1_000_000, 100_000}

func Decode(data []byte, out any) error { return DecodeWithLimits(data, out, DefaultLimits) }
func DecodeWithLimits(data []byte, out any, l Limits) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("JSON must be UTF-8")
	}
	if int64(len(data)) > l.Bytes {
		return fmt.Errorf("JSON byte budget exceeded")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if e := walk(d, 0, l); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	target := reflect.ValueOf(out)
	if target.Kind() != reflect.Pointer || target.IsNil() {
		return fmt.Errorf("JSON destination must be a non-nil pointer")
	}
	next := reflect.New(target.Elem().Type())
	if e := d.Decode(next.Interface()); e != nil {
		return e
	}
	target.Elem().Set(next.Elem())
	return nil
}
func Read(r io.Reader, out any, l Limits) error {
	b, e := io.ReadAll(io.LimitReader(r, l.Bytes+1))
	if e != nil {
		return e
	}
	return DecodeWithLimits(b, out, l)
}
func walk(d *json.Decoder, depth int, l Limits) error {
	if depth > l.Depth {
		return fmt.Errorf("JSON depth budget exceeded")
	}
	t, e := d.Token()
	if e != nil {
		return e
	}
	if s, ok := t.(string); ok && len(s) > l.StringBytes {
		return fmt.Errorf("JSON string budget exceeded")
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	count := 0
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			count++
			if count > l.ObjectKeys {
				return fmt.Errorf("JSON key budget exceeded")
			}
			k, e := d.Token()
			if e != nil {
				return e
			}
			key, ok := k.(string)
			if !ok || seen[key] || len(key) > l.StringBytes {
				return fmt.Errorf("duplicate or invalid JSON key")
			}
			seen[key] = true
			if e = walk(d, depth+1, l); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			count++
			if count > l.ArrayItems {
				return fmt.Errorf("JSON array budget exceeded")
			}
			if e = walk(d, depth+1, l); e != nil {
				return e
			}
		}
	default:
		return fmt.Errorf("invalid JSON container")
	}
	_, e = d.Token()
	return e
}

// Float64Exact is an explicit compatibility projection. It rejects any value
// that cannot be represented, rather than silently rounding an integer/decimal.
func Float64Exact(n json.Number) (float64, error) {
	r, exactErr := RationalNumber(n)
	if exactErr != nil {
		return 0, exactErr
	}
	v, e := n.Float64()
	if e != nil {
		return 0, e
	}
	f := new(big.Rat).SetFloat64(v)
	if f == nil || f.Cmp(r) != 0 {
		return 0, fmt.Errorf("lossy numeric projection")
	}
	return v, nil
}

// CompatibleValues projects only exactly representable, JavaScript-safe numbers
// for legacy consumers. Other values stay json.Number; callers must use exact
// accessors or keep them opaque instead of defaulting failed assertions to zero.
func CompatibleValues(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			t[k] = CompatibleValues(x)
		}
	case []any:
		for i, x := range t {
			t[i] = CompatibleValues(x)
		}
	case json.Number:
		if f, e := Float64Exact(t); e == nil && f >= -9007199254740991 && f <= 9007199254740991 {
			return f
		}
	}
	return v
}

// RationalNumber bounds arithmetic work independently from opaque JSON storage.
// Huge exponents may remain json.Number data, but cannot force giant big.Int
// allocations during a compatibility projection or measurement calculation.
func RationalNumber(n json.Number) (*big.Rat, error) {
	text := string(n)
	if len(text) > 1024 {
		return nil, fmt.Errorf("numeric arithmetic budget exceeded")
	}
	if at := strings.IndexAny(text, "eE"); at >= 0 {
		exponent, e := strconv.ParseInt(text[at+1:], 10, 32)
		if e != nil || exponent < -4096 || exponent > 4096 {
			return nil, fmt.Errorf("numeric exponent exceeds arithmetic budget")
		}
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("invalid exact number")
	}
	return r, nil
}
