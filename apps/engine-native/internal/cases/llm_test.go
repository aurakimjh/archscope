package cases

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
)

func TestLLMIntakeRequiresBodyFreeNormalization(t *testing.T) {
	ctx := context.Background()
	s, root := mustStore(t)
	manifest := mustCreate(t, s)
	raw, e := os.ReadFile("../catalog/contracts/llm-trace-sdk-python-1.38.0.otlp.json")
	if e != nil {
		t.Fatal(e)
	}
	p := file(t, "raw-llm.json", raw)
	for _, copy := range []bool{false, true} {
		_, e = s.Apply(ctx, Edit{CaseID: manifest.ID, ExpectedRevision: manifest.Revision, Operation: "source", Path: p, Copy: copy, Sanitize: true})
		if e == nil || !strings.Contains(e.Error(), "CASE_LLM_SANITIZED_SOURCE_REQUIRED") {
			t.Fatal(copy, e)
		}
	}
	canonical, e := os.ReadFile("../catalog/contracts/llm-trace-set.json")
	if e != nil {
		t.Fatal(e)
	}
	p = file(t, "llm-trace-set.json", canonical)
	manifest = apply(t, s, manifest, Edit{Operation: "source", Path: p, Copy: true})
	page, e := s.SourcePage(ctx, manifest.ID, manifest.Sources[0].ID, DecodeOptions{}, 0, 200)
	if e != nil {
		t.Fatal(e)
	}
	b := []byte{}
	for _, v := range page.Lines {
		b = append(b, []byte(v.Text)...)
	}
	if bytes.Contains(b, []byte("SYNTHETIC_BODY_MARKER")) {
		t.Fatal("body indexed")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) == 0 {
		t.Fatal("no case persistence")
	}
}
