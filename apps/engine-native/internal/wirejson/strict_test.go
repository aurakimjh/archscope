package wirejson

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStrictCallerBudgetAtomicPrecisionAndShape(t *testing.T) {
	type value struct {
		N   json.Number `json:"n"`
		Old string      `json:"old,omitempty"`
	}
	original := value{N: "7", Old: "retain-on-error"}
	for _, raw := range []string{`{"n":1,"n":2}`, `{"n":1,"extra":0}`, `{"n":1} {}`, `{"n":"bad"}`, "{\"n\":1,\"old\":\"\xff\"}"} {
		out := original
		if err := StrictWithLimits([]byte(raw), &out, DefaultLimits); err == nil || out != original {
			t.Fatalf("%q: %v %+v", raw, err, out)
		}
	}
	raw := []byte(`{"n":9007199254740993}`)
	limits := DefaultLimits
	limits.Bytes = int64(len(raw))
	var out value
	if err := StrictWithLimits(raw, &out, limits); err != nil || out.N != "9007199254740993" {
		t.Fatal(out, err)
	}
	limits.Bytes--
	var budget *LimitError
	if err := StrictWithLimits(raw, &out, limits); !errors.As(err, &budget) || budget.Resource != "json_bytes" || budget.Actual != int64(len(raw)) {
		t.Fatal(err)
	}
	if err := StrictWithLimits([]byte(`{"n":0}`), &out, DefaultLimits); err != nil || out.Old != "" {
		t.Fatal(out, err)
	}
	if err := StrictWithLimits([]byte(`{}`), nil, DefaultLimits); err == nil {
		t.Fatal("nil destination")
	}
}

func TestCommonValueCompatibility(t *testing.T) {
	for _, v := range []string{"0", "18446744073709551615"} {
		if _, err := Uint64(v); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []string{"00", "+1", "-0", "18446744073709551616", " 1", "1.0"} {
		if _, err := Uint64(v); err == nil {
			t.Fatal(v)
		}
	}
	if !IsDigest(strings.Repeat("a", 64)) || IsDigest(strings.Repeat("A", 64)) || Text("bad\ntext", 1024) || Text("\xff", 1024) {
		t.Fatal("validation boundary")
	}
	if Hash(map[string]string{"x": "y"}) != Digest([]byte(`{"x":"y"}`)) {
		t.Fatal("identity encoding changed")
	}
	if _, err := Uint64("18446744073709551615"); err != nil {
		t.Fatal(err)
	}
}

func TestStrictPreservesDefaultsWithoutMutatingOnFailure(t *testing.T) {
	type options struct {
		Width  int            `json:"width"`
		Values map[string]int `json:"values"`
	}
	out := options{Width: 2400, Values: map[string]int{"original": 1}}
	if err := StrictWithLimits([]byte(`{"values":{"new":2},"unknown":0}`), &out, DefaultLimits); err == nil || len(out.Values) != 1 || out.Values["original"] != 1 {
		t.Fatal(out, err)
	}
	if err := StrictWithLimits([]byte(`{"values":{"new":2}}`), &out, DefaultLimits); err != nil || out.Width != 2400 || len(out.Values) != 2 {
		t.Fatal(out, err)
	}
}
