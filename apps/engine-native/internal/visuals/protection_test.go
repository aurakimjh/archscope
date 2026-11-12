package visuals_test

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	v "github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
	"strings"
	"testing"
)

func TestR1401TransportProtectionPreservesPathsAndHeaders(t *testing.T) {
	view, spec, _, _ := sample(t)
	view.Columns = []string{"Frame / evidence", "Value"}
	paths := []string{"0.1", "1.5", "3.14", "12.5", "100.25", "-0.5", "runtime/pprof.StartCPUProfile", "compress/flate.NewWriter", "/usr/lib/module.so", "https://example.invalid/a/b", "한글 이름", `C:\a\b`}
	view.Rows = nil
	for _, path := range paths {
		view.Rows = append(view.Rows, []resultcontract.Cell{{Text: path}, {Text: "-5"}})
	}
	view.Total = len(view.Rows)
	spec.ViewSHA256 = v.Hash(view)
	spec.Query = view.Query
	tsv := string(v.TSV(view))
	html := v.TableHTML(view, spec)
	for _, path := range paths {
		if strings.Contains(tsv, "'"+path) || strings.Contains(html, "&#39;"+path) {
			t.Fatal("ordinary path changed", path)
		}
	}
	if strings.HasPrefix(tsv, "'Frame / evidence") || strings.Contains(html, "&#39;Frame / evidence") {
		t.Fatal("header changed")
	}
	if p := v.TableProtection(view); p.TSVCells != 0 || p.HTMLCells != 0 || p.Headers != 0 {
		t.Fatal(p)
	}
	risks := []string{"1/3", "2024-01-02", "1e999999", "-1.5e3", "9007199254740993", "=1+1", "@SUM(1)", "-", "0012"}
	view.Rows = nil
	for _, risk := range risks {
		view.Rows = append(view.Rows, []resultcontract.Cell{{Text: risk}, {Text: "literal"}})
	}
	view.Total = len(view.Rows)
	p := v.TableProtection(view)
	if p.TSVCells != len(risks) || p.HTMLCells != 3 {
		t.Fatal("policy counts", p)
	}
	html = v.TableHTML(view, spec)
	if strings.Contains(html, "&#39;1/3") || strings.Contains(html, "&#39;9007199254740993") || !strings.Contains(html, "&#39;=1+1") {
		t.Fatal("HTML did not separate text format from formula protection")
	}
	view.Columns[0] = "=1+1"
	if v.TableProtection(view).Headers != 1 {
		t.Fatal("formula header not protected")
	}
}
