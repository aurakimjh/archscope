package packs

import (
	"context"
	"testing"
)

func TestApplicationPrereleaseRangeKeepsSignedStableBoundary(t *testing.T) {
	r := Range{"0.3.0", "0.4.0"}
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"0.3.5", true}, {"0.4.0-rc.1", true}, {"0.4.0-rc.2", true}, {"0.4.0-rc.1+build.1", true},
		{"0.4.0", false}, {"0.4.1", false}, {"0.5.0-alpha.1", false},
		{"0.3.0-rc.1", false}, {"0.2.9", false}, {"0.4", false},
		{"v0.4.0-rc.1", false}, {"0.4.0-rc.01", false}, {"0.4.0-", false},
	} {
		if got := r.acceptsAppVersion(tc.version); got != tc.want {
			t.Errorf("%s accepted=%v, want %v", tc.version, got, tc.want)
		}
	}
	if (Range{"0.3", "0.4.0"}).acceptsAppVersion(AppVersion) {
		t.Fatal("shortened contract range must remain invalid")
	}
	if r.accepts("0.4.0-rc.1") {
		t.Fatal("data/adapter/extension contract versions must remain numeric")
	}
}

func TestReleaseInstallsOriginalSignedBootstrap(t *testing.T) {
	m, err := OpenDefault(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	state, err := m.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Active) != 1 {
		t.Fatalf("release must activate the signed bootstrap, got %d scopes", len(state.Active))
	}
}
