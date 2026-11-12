package traceimport

import (
	"bytes"
	"compress/gzip"
	"testing"
)

func TestR17VendorRawCarrierGate(t *testing.T) {
	for _, marker := range []string{"traceloop.entity.input", "langfuse.observation.input", "input.value", "output.value", "ai.prompt", "traceloop\\u002eentity.input"} {
		raw := []byte(`{"resourceSpans":[{"attributes":{"` + marker + `":"synthetic"}}]}`)
		if !RequiresLLMNormalization(raw) {
			t.Fatal(marker)
		}
		var b bytes.Buffer
		z := gzip.NewWriter(&b)
		z.Write(raw)
		z.Close()
		if !RequiresLLMNormalization(b.Bytes()) {
			t.Fatal("gzip", marker)
		}
	}
	if RequiresLLMNormalization([]byte(`{"service":"ordinary"}`)) {
		t.Fatal("ordinary carrier")
	}
}
