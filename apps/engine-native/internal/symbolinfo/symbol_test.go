package symbolinfo

import (
	"strings"
	"testing"
)

func TestRustEvidenceAndRawIdentity(t *testing.T) {
	for _, raw := range []string{"__RNvCsftTivVSi7yj_17synthetic_symbols16synthetic_worker", "_ZN9synthetic4work17h0123456789abcdefE", "__ZN9synthetic4work17h1123456789abcdefE"} {
		v := Describe(raw, "", "")
		if v.Raw != raw || v.Display == raw || v.Language != "Rust" || v.Confidence != "encoded" {
			t.Fatal(v)
		}
	}
	for _, raw := range []string{"Math::work", "trust::work", "_Rinvalid", strings.Repeat("_R", 1024)} {
		v := Describe(raw, "", "")
		if v.Language == "Rust" || v.Raw != raw {
			t.Fatal(v)
		}
	}
	if Describe("worker", "worker.rs", "").Confidence != "declared" {
		t.Fatal("missing declared basis")
	}
}
