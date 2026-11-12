package metrichistory

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestR1304PreservesResourceRefusalsAndCancellation(t *testing.T) {
	budget := errors.New("METRIC_BUDGET_EXCEEDED: model_bytes actual=33554433 limit=33554432")
	for _, err := range []error{budget, context.Canceled, context.DeadlineExceeded} {
		if diagnostic("/point", "numeric", err) != err {
			t.Fatal("resource/cancellation contract lost")
		}
	}
	issue := &SourceError{Code: strings.Repeat("X", 300), Path: strings.Repeat("X", 300), Rule: strings.Repeat("X", 300), Offset: 9223372036854775807}
	if len(issue.Error()) > 512 {
		t.Fatal("diagnostic exceeds512 bytes", len(issue.Error()))
	}
}
