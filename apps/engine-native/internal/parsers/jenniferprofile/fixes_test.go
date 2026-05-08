package jenniferprofile

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// TestStripApplicationHash covers the suffix Jennifer appends to
// APPLICATION URLs (`/path(-1059484346)` → `/path`). Both negative
// and positive hashes should round-trip out, and the cleaner has to
// be idempotent so a second pass leaves the value unchanged.
func TestStripApplicationHash(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/api/users(-1059484346)", "/api/users"},
		{"/api/users(123456)", "/api/users"},
		{"/api/users", "/api/users"},
		{"  /api/users(-1)  ", "/api/users"},
		{"/api/v1/orders(-9999)/path", "/api/v1/orders(-9999)/path"}, // suffix only
		{"", ""},
	}
	for _, c := range cases {
		got := stripApplicationHash(c.in)
		if got != c.want {
			t.Errorf("stripApplicationHash(%q) = %q; want %q", c.in, got, c.want)
		}
		if again := stripApplicationHash(got); again != got {
			t.Errorf("stripApplicationHash idempotent? %q → %q", got, again)
		}
	}
}

// TestExternalCallHyphenForm verifies that body events emitted as
// `EXTERNAL-CALL …` (hyphen) classify the same way as the
// underscore form. This was the bug behind "Ext Call count = 0" on
// the transaction profiles table.
func TestExternalCallHyphenForm(t *testing.T) {
	cases := []struct {
		msg  string
		want models.JenniferEventType
	}{
		{"EXTERNAL_CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=http://x/y)", models.JenniferEventExternalCall},
		{"EXTERNAL-CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=http://x/y)", models.JenniferEventExternalCall},
		{"external-call [HTTP] foo ( url=http://x/y)", models.JenniferEventExternalCall},
	}
	for _, c := range cases {
		ev := &models.JenniferProfileEvent{RawMessage: c.msg}
		got := classifyOne(ev)
		if got != c.want {
			t.Errorf("classifyOne(%q) = %q; want %q", c.msg, got, c.want)
		}
	}
}

// TestExternalCallURLExtractionHyphen drills one layer deeper —
// the EXTERNAL-CALL URL has to make it through fillExternalCall too
// so the matcher can compare it against the callee Application.
func TestExternalCallURLExtractionHyphen(t *testing.T) {
	ev := &models.JenniferProfileEvent{
		RawMessage: "EXTERNAL-CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=http://orders.svc/api/v1/place, meta=...)",
		EventType:  models.JenniferEventExternalCall,
	}
	fillExternalCall(ev)
	if ev.ExternalProtocol != "HTTP" {
		t.Errorf("protocol = %q; want HTTP", ev.ExternalProtocol)
	}
	if ev.ExternalClient != "APACHE_HTTP_CLIENT_V5" {
		t.Errorf("client = %q; want APACHE_HTTP_CLIENT_V5", ev.ExternalClient)
	}
	if ev.ExternalURL != "http://orders.svc/api/v1/place" {
		t.Errorf("url = %q; want http://orders.svc/api/v1/place", ev.ExternalURL)
	}
}

// TestJavaXAClassification — the SQL body marker JAVA_XA should
// promote a SQL_EXECUTE event to TWO_PC_UNKNOWN so the 2PC ledger
// counts the line.
func TestJavaXAClassification(t *testing.T) {
	// More-specific patterns (xa_start, xa_commit, …) win when both
	// markers are present in the SQL body — the 2PC unknown variant
	// is only emitted when JAVA_XA stands alone, which is the common
	// case for tracer-emitted wrapper rows that don't carry the
	// underlying xa_* call name.
	cases := []struct {
		msg     string
		details []string
		want    models.JenniferEventType
	}{
		{"SQL-EXECUTE-UPDATE", []string{"BEGIN JAVA_XA xa_start_new(...) END"}, models.JenniferEventTwoPCStart},
		{"SQL-EXECUTE-UPDATE", []string{"BEGIN JAVA_XA wrapping commit"}, models.JenniferEventTwoPCUnknown},
		{"JAVA_XA.start", nil, models.JenniferEventTwoPCUnknown},
		{"JAVA_XA wrapper", nil, models.JenniferEventTwoPCUnknown},
	}
	for _, c := range cases {
		ev := &models.JenniferProfileEvent{
			RawMessage:  c.msg,
			DetailLines: c.details,
		}
		got := classifyOne(ev)
		if got != c.want {
			t.Errorf("classifyOne(%q, %v) = %q; want %q", c.msg, c.details, got, c.want)
		}
	}
}
