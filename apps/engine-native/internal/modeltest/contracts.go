// Package modeltest provides model-boundary probes used only by external tests.
package modeltest

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type Mutation struct {
	Name   string
	Change func(map[string]any)
}

func Row(root map[string]any, key string, index int) map[string]any {
	return root[key].([]any)[index].(map[string]any)
}

func Boundaries(t *testing.T, fixture string, limit int, decode func([]byte) error, cases []Mutation) {
	t.Helper()
	original, err := os.ReadFile("../catalog/contracts/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err = decode(original); err != nil {
		t.Fatal("valid saved fixture", err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var document map[string]any
			reader := json.NewDecoder(bytes.NewReader(original))
			reader.UseNumber()
			if err := reader.Decode(&document); err != nil {
				t.Fatal(err)
			}
			c.Change(document)
			raw, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if err = decode(raw); err == nil {
				t.Fatal("invalid model accepted")
			}
		})
	}
	t.Run("exact_model_byte_limit", func(t *testing.T) {
		bounded := make([]byte, limit+1)
		for i := range bounded {
			bounded[i] = ' '
		}
		copy(bounded, original)
		if err := decode(bounded[:limit]); err != nil {
			t.Fatal("exact limit refused", err)
		}
		if err := decode(bounded); err == nil || !strings.Contains(strings.ToUpper(err.Error()), "BUDGET") {
			t.Fatal("over limit did not return a budget refusal", err)
		}
	})
}
