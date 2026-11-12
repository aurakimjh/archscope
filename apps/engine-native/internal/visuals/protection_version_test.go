package visuals

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"testing"
)

func TestR1506HistoricalProtectionReceiptAndCurrentDecimalPolicy(t *testing.T) {
	view := View{Columns: []string{"value"}, Rows: [][]resultcontract.Cell{{{Text: "0.1"}}, {{Text: "1.5"}}, {{Text: "3.14"}}, {{Text: "2024.01.02"}}, {{Text: "2024-01-02"}}}}
	old := protectionForVersion(view, "spreadsheet-text-0.2.0")
	current := TableProtection(view)
	if old.TSVCells != 5 || current.TSVCells != 2 || current.HTMLCells != 0 || current.Version != "spreadsheet-text-0.2.1" {
		t.Fatal(old, current)
	}
}
