package visuals_test

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	v "github.com/aurakimjh/archscope/apps/engine-native/internal/visuals"
)

func TestOfficeClipboardKeepsCompleteSelectionAndProvenanceInOneTable(t *testing.T) {
	view, spec, _, _ := sample(t)
	spec.Presentation.RowsPerPage = 1 // Clipboard must include both image pages.
	fragment := v.ClipboardTableHTML(view, spec)
	decoder := xml.NewDecoder(strings.NewReader(fragment))
	depth, tables, rows := 0, 0, 0
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if depth == 0 && token.Name.Local != "table" {
				t.Fatal("independent clipboard shape", token.Name.Local)
			}
			depth++
			if token.Name.Local == "table" {
				tables++
			}
			if token.Name.Local == "tr" {
				rows++
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(token)) != "" {
				t.Fatal("text outside clipboard table")
			}
			text.Write(token)
		}
	}
	if tables != 1 || rows != len(view.Rows)+3 {
		t.Fatal("missing data or metadata rows", tables, rows)
	}
	for _, want := range []string{spec.Presentation.Title, "rows 1–2 / 2", view.Columns[0], "<script>literal</script>", "9007199254740993", "'=HYPERLINK", "1/3", view.Sources[0].SHA256, spec.ViewSHA256, view.Quality, view.Warnings[0]} {
		if !strings.Contains(text.String(), want) {
			t.Fatal("clipboard lost exact text", want)
		}
	}
	if strings.Contains(fragment, "<script>") || strings.Contains(fragment, "<caption>") || strings.Contains(fragment, "<p>") {
		t.Fatal("unsafe or independently positioned clipboard content")
	}
	spec.Presentation.Title = "=1+1"
	if !strings.Contains(v.ClipboardTableHTML(view, spec), "&#39;=1+1") {
		t.Fatal("clipboard metadata row did not protect formula-like title")
	}
}
