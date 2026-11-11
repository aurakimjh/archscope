package serverlog

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type failingSuffix struct{}

func (failingSuffix) Read([]byte) (int, error) { return 0, errors.New("private source exception") }

type cancellingSuffix struct{ cancel context.CancelFunc }

func (r cancellingSuffix) Read(p []byte) (int, error) { p[0] = ' '; r.cancel(); return 1, nil }

func TestR3SuffixWhitespaceAndReadFailures(t *testing.T) {
	for _, tc := range []struct {
		suffix   string
		nonblank bool
	}{
		{"", false}, {"\n\r\n \t\n", false}, {strings.Repeat("\u2003", 1025), false},
		{" \n\tX", true}, {"\u2003\xff", true},
	} {
		actual, e := nonblankSuffix(context.Background(), bufio.NewReaderSize(strings.NewReader(tc.suffix), 16))
		if e != nil || actual != tc.nonblank {
			t.Fatalf("suffix length %d: %v %v", len(tc.suffix), actual, e)
		}
	}
	if _, e := nonblankSuffix(context.Background(), bufio.NewReader(failingSuffix{})); e == nil || !strings.Contains(e.Error(), "IMPORT_IO_FAILED") || strings.Contains(e.Error(), "private") {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, e := nonblankSuffix(ctx, bufio.NewReader(cancellingSuffix{cancel})); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if _, e := nonblankSuffix(context.Background(), bufio.NewReader(io.LimitReader(strings.NewReader("   X"), 3))); e != nil {
		t.Fatal(e)
	}
}
