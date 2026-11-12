package jfr

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestR1002LegacyMetadataKeepsOwnerToolUnverified(t *testing.T) {
	root := t.TempDir()
	name := "notjfr"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	tool := filepath.Join(root, name)
	if b, e := exec.Command("go", "build", "-o", tool, "../../localconvert/testdata/tool.go").CombinedOutput(); e != nil {
		t.Fatal(e, string(b))
	}
	input := filepath.Join(root, "synthetic.jfr")
	os.WriteFile(input, []byte("FLR\x00synthetic"), 0600)
	t.Setenv("ARCHSCOPE_JFR_CLI", tool)
	_, info, e := ParseRecordingContext(context.Background(), input, nil)
	if e != nil {
		t.Fatal(e)
	}
	if info.Conversion == nil || info.Conversion.Tool != "declared_unverified" || info.JFRCli == nil || *info.JFRCli != info.Conversion.Tool {
		t.Fatal(info)
	}
}
