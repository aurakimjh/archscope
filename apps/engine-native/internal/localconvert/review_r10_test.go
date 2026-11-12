package localconvert

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR1001ExpandedOutputAbove100MiBStreamsAndKeepsHash(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	result, e := JFR(ctx, inputFile(t), buildTool(t, "large"))
	if e != nil {
		t.Fatal(e)
	}
	defer os.Remove(result.Path)
	if result.Provenance.OutputBytes <= 100<<20 {
		t.Fatal(result.Provenance)
	}
	output := filepath.Join(t.TempDir(), "large.json")
	if e = SaveNew(ctx, result, output); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(output)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, f)
	if e != nil || n != result.Provenance.OutputBytes || hex.EncodeToString(h.Sum(nil)) != result.Provenance.OutputSHA256 {
		t.Fatal(n, e)
	}
	t.Logf("expanded=%d elapsed=%s", n, time.Since(start))
}
func TestR1001ExpandedOutputCeilingAndMessages(t *testing.T) {
	result, e := JFR(context.Background(), inputFile(t), buildTool(t, "large-overlimit"))
	if result.Path != "" || Code(e) != "CONVERTER_OUTPUT_LIMIT" || !strings.Contains(PublicError(e).Error(), "512 MiB") {
		t.Fatal(result, e)
	}
	if !strings.Contains(PublicError(fail("CONVERTER_BUDGET_EXCEEDED")).Error(), "byte budget") {
		t.Fatal("budget cause hidden")
	}
}
func TestR1002OwnerSelectedToolHasUnverifiedIdentity(t *testing.T) {
	result, e := JFR(context.Background(), inputFile(t), buildTool(t, "notjfr"))
	if e != nil {
		t.Fatal(e)
	}
	defer os.Remove(result.Path)
	if result.Provenance.Tool != "declared_unverified" || result.Provenance.InvocationProfile != "jfr.print-json.v1" || result.Provenance.IdentityBasis != "owner_selected_hash_not_publisher_verified" {
		t.Fatal(result.Provenance)
	}
}
func TestR1003WarningsDoNotFailSuccessfulConversionOrExposeText(t *testing.T) {
	result, e := JFR(context.Background(), inputFile(t), buildTool(t, "warn"))
	if e != nil {
		t.Fatal(e)
	}
	defer os.Remove(result.Path)
	if result.Provenance.StderrBytes < 20000 || !result.Provenance.StderrOmitted {
		t.Fatal(result.Provenance)
	}
}
func TestR1004PreflightBeforeToolExecutionAndRecheckPublication(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out.json")
	os.WriteFile(out, []byte("original"), 0600)
	// Neither executable nor input exists: output collision must be found first.
	if _, e := ConvertNew(context.Background(), "missing.jfr", filepath.Join(root, "missing-tool"), out); Code(e) != "OUTPUT_EXISTS" || strings.Contains(e.Error(), ".input-assist-") {
		t.Fatal(e)
	}
	os.Remove(out)
	os.WriteFile(out+".conversion.json", []byte("original"), 0600)
	if _, e := ConvertNew(context.Background(), "missing.jfr", filepath.Join(root, "missing-tool"), out); Code(e) != "OUTPUT_EXISTS" {
		t.Fatal(e)
	}
	os.Remove(out + ".conversion.json")
	result, e := JFR(context.Background(), inputFile(t), buildTool(t, "ok"))
	if e != nil {
		t.Fatal(e)
	}
	defer os.Remove(result.Path)
	os.WriteFile(out, []byte("racing output"), 0600)
	if e = SaveNew(context.Background(), result, out); Code(e) != "OUTPUT_EXISTS" {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(out)
	if string(b) != "racing output" {
		t.Fatal("replaced competitor")
	}
}
func TestR1004StreamValidationShapeAndScalarBudgets(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		body  string
		valid bool
	}{{`{"recording":{"events":[]}}`, true}, {`{"events":[{"arbitrary":{"events":5}}]}`, true}, {`{"recording":{"events":0},"events":[]}`, false}, {`{"events":[]} {}`, false}, {`{"events":["` + strings.Repeat("x", maxScalarBytes+1) + `"]}`, false}, {`{"events":[` + strings.Repeat("1", maxScalarBytes+1) + `]}`, false}, {`{"events":[` + strings.Repeat("[", maxJSONDepth+1) + `0` + strings.Repeat("]", maxJSONDepth+1) + `]}`, false}} {
		path := filepath.Join(root, "output.json")
		os.WriteFile(path, []byte(tc.body), 0600)
		if e := validateJSON(context.Background(), path); (e == nil) != tc.valid {
			t.Fatal("validation mismatch", e, tc.valid)
		}
	}
}

func TestR1001VersionOutputBudgetHasVersionCause(t *testing.T) {
	_, e := JFR(context.Background(), inputFile(t), buildTool(t, "versionflood"))
	if Code(e) != "CONVERTER_VERSION_INVALID" || !strings.Contains(PublicError(e).Error(), "4 KiB") || strings.Contains(e.Error(), "512 MiB") {
		t.Fatal(e)
	}
}
