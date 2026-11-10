package sessionfile

import (
	"bytes"
	"context"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/halog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/httpcapture"
)

func fixture() Document {
	return Document{Type: Kind, Version: 1, Scope: "filtered", SelectedID: "original", Transactions: []models.CaptureTransaction{{ID: "original", ConnectionID: "tcp-one", Sequence: 4, Method: "POST", URL: "https://example.test/a?x=1&token=private", StatusCode: 302, StartedAt: "2026-11-10T00:00:00Z", TotalMS: 42, State: models.TxComplete, Fidelity: "semantic", StreamProvenance: "reconstructed_semantic", Informational: []models.InformationalResponse{{Status: 103, Headers: []models.HeaderField{{Name: "Link", Value: "</a>"}}}}, Request: models.HTTPMessage{ContentType: "application/x-www-form-urlencoded", BodyStorage: "inline", BodyPreview: "x=1&x=2&password=private", Headers: []models.HeaderField{{Name: "Authorization", Value: "Bearer private"}}}, Response: models.HTTPMessage{ContentType: "text/plain", BodyStorage: "inline", BodyPreview: "한글 </script><script>alert(1)</script>"}, Process: &models.ProcessInstance{Name: "fixture", Key: models.ProcessKey{PID: 42, StartTime: "born"}, Attribution: "confirmed"}}}}
}
func TestPortableRoundTripsAndAllFormats(t *testing.T) {
	root := t.TempDir()
	original := fixture()
	for _, format := range []string{"session", "har", "xml", "xml_compact", "urls", "text", "html", "xls", "responses"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(root, "session."+format)
			if format == "responses" {
				path = root
			}
			if err := Write(context.Background(), path, original, format); err != nil {
				t.Fatal(err)
			}
			if format == "responses" {
				dirs, _ := filepath.Glob(filepath.Join(root, "http-responses-*"))
				if len(dirs) != 1 {
					t.Fatal(dirs)
				}
				data, _ := os.ReadFile(filepath.Join(dirs[0], "000001.response.txt"))
				if !strings.Contains(string(data), "한글") {
					t.Fatal(string(data))
				}
				return
			}
			data, _ := os.ReadFile(path)
			if bytes.Contains(data, []byte("private")) {
				t.Fatal("secret leaked")
			}
			switch format {
			case "session", "har", "xml", "xml_compact":
				doc, _, err := Read(path)
				if err != nil {
					t.Fatal(err)
				}
				if doc.SelectedID != "original" || len(doc.Transactions) != 1 || doc.Transactions[0].ConnectionID != "tcp-one" || doc.Transactions[0].Sequence != 4 || doc.Transactions[0].Informational[0].Status != 103 || doc.Transactions[0].Process.Key.StartTime != "born" {
					t.Fatalf("identity lost: %+v", doc)
				}
				if format == "xml_compact" && (doc.Transactions[0].Response.BodyPreview != "" || doc.Transactions[0].Response.BodyStorage != "export_omitted") {
					t.Fatal("compact omission lost")
				}
			case "html":
				if bytes.Contains(data, []byte("<script>")) {
					t.Fatal("HTML injection")
				}
			}
			if format == "har" {
				parsed, err := parser.ParseFile(path, parser.Options{})
				if err != nil || len(parsed.Entries) != 1 {
					t.Fatalf("standard HAR parser: %v", err)
				}
			}
			if format == "xls" {
				python := os.Getenv("ARCHSCOPE_XLS_READER")
				if python != "" {
					out, err := exec.Command(python, "-c", `import xlrd,sys; s=xlrd.open_workbook(sys.argv[1]).sheet_by_index(0); assert s.nrows==2; assert s.cell_value(1,0)=='original'; assert '한글' in s.cell_value(1,14); assert s.cell_value(1,2)=='42'`, path).CombinedOutput()
					if err != nil {
						t.Fatalf("independent XLS reader: %v %s", err, out)
					}
				}
			}
		})
	}
	if original.Transactions[0].Response.BodyPreview == "" || !strings.Contains(original.Transactions[0].URL, "private") {
		t.Fatal("source mutated")
	}
}
func TestCancellationAndInvalidImports(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "keep")
	os.WriteFile(path, []byte("unchanged"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Write(ctx, path, fixture(), "session"); err != context.Canceled {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "unchanged" {
		t.Fatal("canceled write changed destination")
	}
	doc := fixture()
	doc.Transactions = append(doc.Transactions, doc.Transactions[0])
	if _, err := Sanitize(&doc); err == nil {
		t.Fatal("duplicate IDs accepted")
	}
	if _, err := encodeXLS([][]string{{strings.Repeat("a", 4001)}}); err == nil {
		t.Fatal("XLS silently truncated")
	}
}

func TestLargeXLSIndependentReader(t *testing.T) {
	python := os.Getenv("ARCHSCOPE_XLS_READER")
	if python == "" {
		t.Skip("optional independent XLS reader")
	}
	rows := [][]string{{"ID", "Value"}}
	for i := 0; i < 1000; i++ {
		rows = append(rows, []string{"=not-a-formula", strings.Repeat("한글", 30)})
	}
	data, err := encodeXLS(rows)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "large.xls")
	os.WriteFile(path, data, 0600)
	out, err := exec.Command(python, "-c", `import xlrd,sys; s=xlrd.open_workbook(sys.argv[1]).sheet_by_index(0); assert s.nrows==1001; assert s.cell_value(1000,0)=='=not-a-formula'; assert s.cell_value(1000,1)=='한글'*30`, path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
}

func TestLegacyTreeRoundTripAndCompactOmission(t *testing.T) {
	original := fixture()
	original.Transactions[0].Response.LegacyPreview = &models.LegacyPreview{Format: "amf0", State: "available", Root: &models.ContentNode{Kind: "object", Children: []models.ContentNode{{Name: "password", Kind: "string", Value: "private"}, {Name: "x", Kind: "integer", Value: "2"}}}}
	for _, format := range []string{"session", "xml", "har", "xml_compact"} {
		path := filepath.Join(t.TempDir(), "legacy."+format)
		if err := Write(context.Background(), path, original, format); err != nil {
			t.Fatal(err)
		}
		doc, _, err := Read(path)
		if err != nil {
			t.Fatal(err)
		}
		tree := doc.Transactions[0].Response.LegacyPreview
		if format == "xml_compact" {
			if tree != nil {
				t.Fatal("compact kept structured payload")
			}
			continue
		}
		if tree == nil || tree.Root.Children[0].Value != "[REDACTED]" || tree.Root.Children[1].Value != "2" {
			t.Fatalf("%+v", tree)
		}
	}
	if original.Transactions[0].Response.LegacyPreview.Root.Children[0].Value != "private" {
		t.Fatal("original tree mutated")
	}
}

func TestLegacyExtensionCannotMasqueradeAsNativeJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keep.halog")
	os.WriteFile(path, []byte("original"), 0600)
	if err := Write(context.Background(), path, fixture(), "session"); err == nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Fatal("legacy output overwritten")
	}
	os.WriteFile(path, []byte(`{"type":"archscope-http-session","schemaVersion":1,"transactions":[]}`), 0600)
	if _, _, err := Read(path); err == nil {
		t.Fatal("JSON disguised as .halog accepted")
	}
	data, err := Encode(fixture(), "native")
	if err != nil {
		t.Fatal(err)
	}
	file, err := halog.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := file.Transactions()
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("private")) {
		t.Fatal("native export leaked a secret")
	}
}
