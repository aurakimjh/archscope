package profilemodel_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	w "github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

func TestR5ProfileStringWireBoundary(t *testing.T) {
	set, _ := fixture(t)
	for _, size := range []int{w.DefaultLimits.StringBytes - 1, w.DefaultLimits.StringBytes, w.DefaultLimits.StringBytes + 1} {
		field, _ := json.Marshal(strings.Repeat("x", size))
		set.Extensions = map[string]json.RawMessage{"synthetic_padding": field}
		raw, e := json.Marshal(set)
		if e != nil {
			t.Fatal(e)
		}
		decoded, e := m.Decode(raw)
		if size <= w.DefaultLimits.StringBytes {
			if e != nil || string(decoded.Extensions["synthetic_padding"]) != string(field) {
				t.Fatal(size, e)
			}
		} else {
			var bound *m.BudgetError
			if !errors.As(e, &bound) || bound.Resource != "json_string_bytes" || bound.Actual != int64(size) || bound.Limit != int64(w.DefaultLimits.StringBytes) {
				t.Fatal(size, e)
			}
		}
	}
}

func TestR5OpaqueCaptureOptionsCannotDeclareTheirOwnIgnoreList(t *testing.T) {
	left, selection := fixture(t)
	right := copySet(t, left)
	left.CaptureOptions = json.RawMessage(`{"pid":1234,"duration_sec":30,"comparison_keys":["event"]}`)
	right.CaptureOptions = json.RawMessage(`{"pid":4321,"duration_sec":60,"comparison_keys":["event"]}`)
	_, e := m.Compare(context.Background(), left, selection, right, selection)
	var refusal *m.ComparisonError
	if !errors.As(e, &refusal) || refusal.Code != "capture_options_mismatch" {
		t.Fatal(e)
	}
}
