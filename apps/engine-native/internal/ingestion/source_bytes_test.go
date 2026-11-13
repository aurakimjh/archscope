package ingestion

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotConsumerAndBoundedDecode(t *testing.T) {
	ctx := context.Background()
	p := filepath.Join(t.TempDir(), "source")
	raw := []byte("observed bytes")
	if e := os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	calls := 0
	e := WithSourceBytes(ctx, p, "acquire-test-", "original", DefaultBudgets(), func(got []byte) error {
		calls++
		if !bytes.Equal(got, raw) {
			t.Fatal("wrong bytes")
		}
		return nil
	})
	if e != nil || calls != 1 {
		t.Fatal(e, calls)
	}
	sentinel := errors.New("parse refusal")
	if e = WithSourceBytes(ctx, p, "acquire-test-", "original", DefaultBudgets(), func([]byte) error { return sentinel }); !errors.Is(e, sentinel) {
		t.Fatal(e)
	}
	var buf bytes.Buffer
	z := gzip.NewWriter(&buf)
	z.Write(raw)
	z.Close()
	got, e := DecodeGzip(ctx, buf.Bytes(), int64(len(raw)))
	if e != nil || !bytes.Equal(got, raw) {
		t.Fatal(e)
	}
	if _, e = DecodeGzip(ctx, buf.Bytes(), int64(len(raw)-1)); e == nil {
		t.Fatal("accepted expansion overflow")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = DecodeGzip(canceled, buf.Bytes(), 64); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	b := DefaultBudgets()
	b.InputBytes = int64(len(raw) - 1)
	calls = 0
	e = WithSourceBytes(ctx, p, "acquire-test-", "original", b, func([]byte) error { calls++; return nil })
	if e == nil || calls != 0 {
		t.Fatal("overflow reached parser")
	}
}
