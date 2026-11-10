//go:build windows

package processlaunch

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSuspendedLaunchFixture(t *testing.T) {
	if path := os.Getenv("ARCHSCOPE_TEST_LAUNCH_MARKER"); path != "" {
		if err := os.WriteFile(path, []byte(os.Getenv("HTTP_PROXY")), 0600); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
}

func TestPreparedProcessWaitsForScope(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "started")
	t.Setenv("ARCHSCOPE_TEST_LAUNCH_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child, err := Prepare(Target{Path: executable, Args: []string{"-test.run=^TestSuspendedLaunchFixture$"}}, "127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		child.Abort()
		t.Fatal("child ran before scope assignment")
	}
	if child.Key.PID <= 0 || child.Key.StartTime == "" {
		child.Abort()
		t.Fatal("child identity missing")
	}
	if err := child.Resume(); err != nil {
		child.Abort()
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(marker); err == nil {
			if string(data) != "http://127.0.0.1:12345" {
				t.Fatalf("proxy=%q", data)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("resumed child did not run")
}
