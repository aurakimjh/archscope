package producerpolicy

import "testing"

func TestExplicitQualification(t *testing.T) {
	for _, c := range []struct {
		version                        string
		allow                          bool
		wantVersion, wantQualification string
		fail                           bool
	}{
		{"1.2.3", false, "1.2.3", "", false},
		{"1.2.3", true, "1.2.3", "", false},
		{"1.2.4", false, "", "", true},
		{"", false, "", "", true},
		{"1.2.4", true, "1.2.4", Unverified, false},
		{"", true, "unknown", Unverified, false},
		{"private/path", true, "", "", true},
		{"bad\nversion", true, "", "", true},
	} {
		v, q, err := Select(c.version, "1.2.3", c.allow)
		if (err != nil) != c.fail || v != c.wantVersion || q != c.wantQualification {
			t.Fatalf("%+v -> %q %q %v", c, v, q, err)
		}
	}
	if Valid("1.2.3", "verified") || Valid("path/private", Unverified) {
		t.Fatal("invalid persisted qualification")
	}
}
