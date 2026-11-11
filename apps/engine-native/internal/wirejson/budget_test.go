package wirejson_test

import (
	"errors"
	"testing"

	w "github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

func TestTypedJSONBudgetsKeepResourceAndDestination(t *testing.T) {
	for _, tc := range []struct {
		body, resource  string
		actual, ceiling int64
		set             func(*w.Limits)
	}{
		{`"123456789"`, "json_string_bytes", 9, 8, func(l *w.Limits) { l.StringBytes = 8 }},
		{`{"123456789":0}`, "json_string_bytes", 9, 8, func(l *w.Limits) { l.StringBytes = 8 }},
		{`"ééééé"`, "json_string_bytes", 10, 8, func(l *w.Limits) { l.StringBytes = 8 }},
		{`[1,2,3]`, "json_array_items", 3, 2, func(l *w.Limits) { l.ArrayItems = 2 }},
		{`{"a":1,"b":2,"c":3}`, "json_object_keys", 3, 2, func(l *w.Limits) { l.ObjectKeys = 2 }},
		{`[[0]]`, "json_depth", 2, 1, func(l *w.Limits) { l.Depth = 1 }},
		{`[0]`, "json_bytes", 3, 2, func(l *w.Limits) { l.Bytes = 2 }},
	} {
		t.Run(tc.resource, func(t *testing.T) {
			limits := w.DefaultLimits
			tc.set(&limits)
			var target any = "preserved"
			e := w.DecodeWithLimits([]byte(tc.body), &target, limits)
			var bound *w.LimitError
			if !errors.As(e, &bound) || bound.Resource != tc.resource || bound.Actual != tc.actual || bound.Limit != tc.ceiling || target != "preserved" {
				t.Fatal(e, target)
			}
		})
	}
	var value string
	l := w.DefaultLimits
	l.StringBytes = 8
	if e := w.DecodeWithLimits([]byte(`"éééé"`), &value, l); e != nil || len(value) != 8 {
		t.Fatal(value, e)
	}
	for _, invalid := range []string{`{"a":1,"a":2}`, `[1,`, string([]byte{0xff})} {
		var out any
		var bound *w.LimitError
		if e := w.Decode([]byte(invalid), &out); e == nil || errors.As(e, &bound) {
			t.Fatal("malformed JSON classified as resource", e)
		}
	}
}
