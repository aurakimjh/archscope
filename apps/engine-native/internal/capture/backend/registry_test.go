package backend

import (
	"errors"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"testing"
)

func TestUnavailableBackendNeverFallsBack(t *testing.T) {
	registry := Default()
	for _, name := range []string{TransparentWindows, "unknown"} {
		if _, err := registry.Validate("windows", name); !errors.Is(err, capture.ErrModeUnavailable) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if name, err := registry.Validate("windows", ""); err != nil || name != ExplicitProxy {
		t.Fatal(name, err)
	}
	if _, err := registry.Validate("darwin", ExplicitProxy); !errors.Is(err, capture.ErrModeUnavailable) {
		t.Fatal(err)
	}
	caps := registry.Capabilities("windows")
	if len(caps) != 2 || !caps[0].Available || caps[0].Transparent || caps[1].Available || !caps[1].Transparent {
		t.Fatal(caps)
	}
}
