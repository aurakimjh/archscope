package runtimestack

import (
	"strings"
	"testing"
)

const nodejsFixture = `TypeError: Cannot read properties of undefined (reading 'id')
    at findOrder (/srv/demo-site/src/orders/service.js:42:18)
    at async handleRequest (/srv/demo-site/src/orders/controller.js:17:5)
    at async Layer.handle [as handle_request] (/srv/demo-site/node_modules/express/lib/router/layer.js:95:5)

Error: upstream payment API timed out
    at PaymentClient.charge (/srv/demo-site/src/payment/client.js:88:11)
    at async checkout (/srv/demo-site/src/payment/checkout.js:31:7)
`

func TestParseNodejsFileFixtureHappyPath(t *testing.T) {
	path := writeTempFile(t, "node.txt", nodejsFixture)
	records, diags, err := ParseNodejsFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseNodejsFile: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if records[0].RecordType != "TypeError" {
		t.Errorf("records[0].RecordType = %q", records[0].RecordType)
	}
	if records[0].Runtime != "nodejs" {
		t.Errorf("Runtime = %q", records[0].Runtime)
	}
	if records[0].Message == nil || !strings.Contains(*records[0].Message, "Cannot read properties") {
		t.Errorf("records[0].Message = %v", records[0].Message)
	}
	if len(records[0].Stack) != 3 {
		t.Errorf("records[0].Stack len = %d, want 3 — got %v", len(records[0].Stack), records[0].Stack)
	}
	if !strings.HasPrefix(records[0].Stack[0], "findOrder") {
		t.Errorf("records[0].Stack[0] = %q", records[0].Stack[0])
	}
	wantSig0 := "TypeError|" + records[0].Stack[0]
	if records[0].Signature != wantSig0 {
		t.Errorf("Signature = %q, want %q", records[0].Signature, wantSig0)
	}

	if records[1].RecordType != "Error" {
		t.Errorf("records[1].RecordType = %q, want Error", records[1].RecordType)
	}
	if records[1].Message == nil || *records[1].Message != "upstream payment API timed out" {
		t.Errorf("records[1].Message = %v", records[1].Message)
	}

	if diags.ParsedRecords != 2 {
		t.Errorf("ParsedRecords = %d, want 2", diags.ParsedRecords)
	}
}

func TestParseNodejsFileFlagsLineWithoutHeader(t *testing.T) {
	body := "garbage line\n"
	path := writeTempFile(t, "noise.txt", body)
	_, diags, err := ParseNodejsFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseNodejsFile: %v", err)
	}
	if diags.SkippedByReason[ReasonNoNodejsErrorHeader] != 1 {
		t.Errorf("expected 1 NO_NODEJS_ERROR_HEADER, got %v", diags.SkippedByReason)
	}
}

func TestParseNodejsFileTimestampPrefix(t *testing.T) {
	// The header regex accepts an optional ISO-ish timestamp prefix.
	body := "2026-04-27T10:00:01.123Z RangeError: bad index\n    at foo (/srv/x.js:1:1)\n"
	path := writeTempFile(t, "ts.txt", body)
	records, _, err := ParseNodejsFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseNodejsFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].RecordType != "RangeError" {
		t.Errorf("RecordType = %q", records[0].RecordType)
	}
	if len(records[0].Stack) != 1 {
		t.Errorf("Stack len = %d, want 1", len(records[0].Stack))
	}
}

func TestParseNodejsFileBareErrorHeader(t *testing.T) {
	// Bare `Error` (just the word, no prefix) is allowed by the regex.
	body := "Error: simple\n    at run (/x.js:1:1)\n"
	path := writeTempFile(t, "bare.txt", body)
	records, _, err := ParseNodejsFile(path, Options{})
	if err != nil {
		t.Fatalf("ParseNodejsFile: %v", err)
	}
	if len(records) != 1 || records[0].RecordType != "Error" {
		t.Fatalf("records = %+v", records)
	}
}
