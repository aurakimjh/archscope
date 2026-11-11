package ingestion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBudgetDetailPreservesIntegersBeyondJavaScriptRange(t *testing.T) {
	e := BudgetProblem("input_bytes", "input_bytes", (1<<54)+1, 64<<20).(*ImportError)
	raw, err := json.Marshal(e.Detail)
	if err != nil || !strings.Contains(string(raw), `"actual":"18014398509481985"`) || !strings.Contains(string(raw), `"limit":"67108864"`) {
		t.Fatal(string(raw), err)
	}
}
