package runtimestack

import (
	"strings"
	"testing"
)

const pythonTracebackFixture = `Traceback (most recent call last):
  File "/srv/demo-site/app/api/orders.py", line 21, in get_order
    return repository.load_order(order_id)
  File "/srv/demo-site/app/repository.py", line 58, in load_order
    cursor.execute(sql, params)
TimeoutError: database query exceeded timeout

Traceback (most recent call last):
  File "/srv/demo-site/app/tasks.py", line 44, in run_job
    parse_payload(payload)
  File "/srv/demo-site/app/parser.py", line 12, in parse_payload
    return json.loads(payload)
ValueError: invalid payload
`

func TestParsePythonTracebackFileFixtureHappyPath(t *testing.T) {
	path := writeTempFile(t, "py.txt", pythonTracebackFixture)
	records, diags, err := ParsePythonTracebackFile(path, Options{})
	if err != nil {
		t.Fatalf("ParsePythonTracebackFile: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records = %d, want 2", len(records))
	}
	if records[0].Runtime != "python" {
		t.Errorf("Runtime = %q, want python", records[0].Runtime)
	}
	if records[0].RecordType != "TimeoutError" {
		t.Errorf("RecordType = %q, want TimeoutError", records[0].RecordType)
	}
	if records[0].Message == nil || *records[0].Message != "database query exceeded timeout" {
		t.Errorf("records[0].Message = %v", records[0].Message)
	}
	if len(records[0].Stack) != 2 {
		t.Fatalf("records[0].Stack len = %d, want 2 — got %v", len(records[0].Stack), records[0].Stack)
	}
	want0 := "/srv/demo-site/app/api/orders.py:21 in get_order"
	if records[0].Stack[0] != want0 {
		t.Errorf("records[0].Stack[0] = %q, want %q", records[0].Stack[0], want0)
	}
	// Top frame for signature is stack[-1] (deepest).
	wantSig := "TimeoutError|/srv/demo-site/app/repository.py:58 in load_order"
	if records[0].Signature != wantSig {
		t.Errorf("Signature = %q, want %q", records[0].Signature, wantSig)
	}

	if records[1].RecordType != "ValueError" {
		t.Errorf("records[1].RecordType = %q", records[1].RecordType)
	}
	if records[1].Message == nil || *records[1].Message != "invalid payload" {
		t.Errorf("records[1].Message = %v", records[1].Message)
	}

	if diags.ParsedRecords != 2 {
		t.Errorf("ParsedRecords = %d, want 2", diags.ParsedRecords)
	}
}

func TestParsePythonTracebackFileFlagsOutsideBlockLine(t *testing.T) {
	body := "stray line outside any traceback\n"
	path := writeTempFile(t, "stray.txt", body)
	_, diags, err := ParsePythonTracebackFile(path, Options{})
	if err != nil {
		t.Fatalf("ParsePythonTracebackFile: %v", err)
	}
	if diags.SkippedByReason[ReasonOutsidePythonTraceback] != 1 {
		t.Errorf("expected 1 OUTSIDE_PYTHON_TRACEBACK, got %v", diags.SkippedByReason)
	}
}

func TestParsePythonTracebackFileBlockWithoutTerminalException(t *testing.T) {
	// Traceback header but no terminal exception line → the block is
	// skipped with INVALID_PYTHON_TRACEBACK.
	body := "Traceback (most recent call last):\n  File \"/x.py\", line 1, in main\n    pass\n"
	path := writeTempFile(t, "noterm.txt", body)
	records, diags, err := ParsePythonTracebackFile(path, Options{})
	if err != nil {
		t.Fatalf("ParsePythonTracebackFile: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
	if diags.SkippedByReason[ReasonInvalidPythonTraceback] != 1 {
		t.Errorf("expected INVALID_PYTHON_TRACEBACK, got %v", diags.SkippedByReason)
	}
}

func TestParsePythonTracebackFileExceptionWithoutMessage(t *testing.T) {
	// Bare `KeyError` → message folded to nil per Python's `or None`.
	body := "Traceback (most recent call last):\n  File \"/x.py\", line 1, in main\n    pass\nKeyError\n"
	path := writeTempFile(t, "bare.txt", body)
	records, _, err := ParsePythonTracebackFile(path, Options{})
	if err != nil {
		t.Fatalf("ParsePythonTracebackFile: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].Message != nil {
		t.Errorf("Message should be nil, got %q", *records[0].Message)
	}
	if !strings.HasSuffix(records[0].Signature, "in main") {
		t.Errorf("Signature = %q", records[0].Signature)
	}
}
