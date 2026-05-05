package profiler

import (
	"strings"
	"testing"
)

func TestRedactAuthorizationToken(t *testing.T) {
	result := RedactText(`Authorization: Bearer abc.def.ghi extra=1`)
	if !strings.Contains(result.Text, "<TOKEN len=11>") {
		t.Fatalf("expected token placeholder; got %q", result.Text)
	}
	if result.Summary["TOKEN"] == 0 {
		t.Fatalf("expected TOKEN counter > 0; got %+v", result.Summary)
	}
}

func TestRedactCookieHeader(t *testing.T) {
	result := RedactText(`Cookie: session=abc; csrf=xyz`)
	if !strings.Contains(result.Text, "<COOKIE len=") {
		t.Fatalf("expected cookie placeholder; got %q", result.Text)
	}
}

func TestRedactURLClassifiesQueryValues(t *testing.T) {
	result := RedactText(`GET https://api.example.com/v1/items?token=abcdef&page=42&q=hello 200`)
	if !strings.Contains(result.Text, "<TOKEN len=") {
		t.Fatalf("token query value should be classified; got %q", result.Text)
	}
	if !strings.Contains(result.Text, "<NUMBER len=") {
		t.Fatalf("numeric page should be classified as NUMBER; got %q", result.Text)
	}
	if !strings.Contains(result.Text, "<QUERY_VALUE len=") {
		t.Fatalf("plain string should be QUERY_VALUE; got %q", result.Text)
	}
}

func TestRedactEmailAndIPv4(t *testing.T) {
	result := RedactText("user contact: jane@example.com from 10.0.0.42")
	if !strings.Contains(result.Text, "<EMAIL>") {
		t.Fatalf("expected email placeholder; got %q", result.Text)
	}
	if !strings.Contains(result.Text, "<IPV4>") {
		t.Fatalf("expected IPv4 placeholder; got %q", result.Text)
	}
}

func TestRedactAbsolutePath(t *testing.T) {
	result := RedactText(`File "/Users/secret/project/app.py", line 4`)
	if !strings.Contains(result.Text, "<PATH>") {
		t.Fatalf("expected path placeholder; got %q", result.Text)
	}
	if !strings.Contains(result.Text, "app.py") {
		t.Fatalf("path leaf should be preserved; got %q", result.Text)
	}
}

func TestRedactLongIdentifier(t *testing.T) {
	result := RedactText("trace 1234567890123 succeeded")
	if !strings.Contains(result.Text, "<NUMBER len=") {
		t.Fatalf("expected long-number placeholder; got %q", result.Text)
	}
	if result.Summary["LONG_IDENTIFIER"] == 0 {
		t.Fatalf("expected LONG_IDENTIFIER counter > 0; got %+v", result.Summary)
	}
}

func TestRedactTextEmpty(t *testing.T) {
	result := RedactText("")
	if result.Text != "" {
		t.Fatalf("empty redaction should stay empty; got %q", result.Text)
	}
	if len(result.Summary) != 0 {
		t.Fatalf("expected empty summary; got %+v", result.Summary)
	}
}

func TestMergeRedactionSummaries(t *testing.T) {
	a := map[string]int{"TOKEN": 2, "EMAIL": 1}
	b := map[string]int{"TOKEN": 1, "IPV4": 3}
	merged := MergeRedactionSummaries(a, b)
	if merged["TOKEN"] != 3 || merged["EMAIL"] != 1 || merged["IPV4"] != 3 {
		t.Fatalf("merge mismatch; got %+v", merged)
	}
}
