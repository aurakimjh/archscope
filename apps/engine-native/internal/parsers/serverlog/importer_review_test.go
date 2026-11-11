package serverlog_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/serverlog"
)

type faultSource struct{ reader io.Reader }

func (s faultSource) Info() ingestion.SourceInfo                  { return ingestion.SourceInfo{Name: "synthetic"} }
func (s faultSource) Open(context.Context) (io.ReadCloser, error) { return io.NopCloser(s.reader), nil }

type faultReader struct{}

func (faultReader) Read([]byte) (int, error) { return 0, errors.New("private/source/token") }

type discardSink struct{}

func (discardSink) Write(context.Context, ingestion.Batch) error { return nil }
func TestReviewR09ReadErrorIsNotBufferBudgetAndNeverLeaksCause(t *testing.T) {
	d := parser.NewImporter("tomcat", ingestion.ImporterDescriptor{RecordFamily: "server-log.record.v1"})
	for _, test := range []struct {
		reader io.Reader
		code   string
	}{
		{io.MultiReader(strings.NewReader("11-Nov-2026 10:00:00.000 INFO [main] org.apache.catalina.startup.Catalina Synthetic\n"), faultReader{}), "IMPORT_IO_FAILED"},
		{strings.NewReader(strings.Repeat("x", (1<<20)+1)), "IMPORT_BUDGET_EXCEEDED"},
	} {
		_, e := d.ParserFactory(nil).Parse(context.Background(), faultSource{test.reader}, discardSink{})
		var problem *ingestion.ImportError
		if !errors.As(e, &problem) || problem.Code != test.code || strings.Contains(e.Error(), "private/source") {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := d.ParserFactory(nil).Parse(ctx, faultSource{strings.NewReader("")}, discardSink{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
