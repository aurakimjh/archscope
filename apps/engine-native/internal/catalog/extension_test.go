package catalog

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/serverlog"
)

func TestC4SameFamilyAdapterNeedsNoServiceCLIRendererSwitch(t *testing.T) {
	base, r, e := Builtin()
	if e != nil {
		t.Fatal(e)
	}
	resolved, _ := base.Resolve("tomcat", nil, "test")
	d := resolved.Descriptor
	d.ID = "fixture.prefixed"
	d.Aliases = []string{"prefixed"}
	d.SupportStatus = "limited"
	d.Detector = func(_ context.Context, p ingestion.Probe) (ingestion.Candidate, error) {
		if strings.HasPrefix(string(p.Head), "fixture|") {
			return ingestion.Candidate{Grade: ingestion.Verified, Confidence: 1, Reason: "synthetic fixture prefix"}, nil
		}
		return ingestion.Candidate{}, nil
	}
	d.ValidateInput = func(ctx context.Context, s ingestion.Source) error { return ctx.Err() }
	d.ParserFactory = func(json.RawMessage) ingestion.Parser {
		return ingestion.ParserFunc(func(ctx context.Context, s ingestion.Source, k ingestion.RecordSink) (ingestion.ParseReport, error) {
			f, e := s.Open(ctx)
			if e != nil {
				return ingestion.ParseReport{}, e
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			var sequence uint64
			for scanner.Scan() {
				rec, err := parser.ParseLineForFormat(strings.TrimPrefix(scanner.Text(), "fixture|"), "tomcat")
				if err != nil {
					return ingestion.ParseReport{}, ingestion.Problem("IMPORT_INPUT_INVALID", "fixture parse failed")
				}
				payload, _ := json.Marshal(rec)
				sequence++
				if e = k.Write(ctx, ingestion.Batch{Family: d.RecordFamily, Records: []ingestion.Record{{Sequence: sequence, Position: ingestion.Position{Line: int64(sequence)}, Payload: payload}}}); e != nil {
					return ingestion.ParseReport{}, e
				}
			}
			return ingestion.ParseReport{Complete: true}, scanner.Err()
		})
	}
	custom, e := ingestion.NewCatalog([]ingestion.ImporterDescriptor{d}, Schemas(), []ingestion.Analyzer{Analyzer()})
	if e != nil {
		t.Fatal(e)
	}
	svc, e := analysis.New(t.TempDir(), custom, r)
	if e != nil {
		t.Fatal(e)
	}
	defer svc.Close()
	body, _ := Contracts.ReadFile("contracts/server.log")
	lines := strings.Split(strings.TrimSuffix(string(body), "\n"), "\n")
	for i := range lines {
		lines[i] = "fixture|" + lines[i]
	}
	path := filepath.Join(t.TempDir(), "new.fixture")
	os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600)
	j, e := svc.Start(analysis.Request{Path: path, Importer: "auto"})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	j, e = svc.Wait(ctx, j.ID, 1)
	if e != nil || j.State != "completed" {
		t.Fatal(j, e)
	}
	caps, e := svc.DescribeResult(ctx, j.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, provider := range caps.Providers {
		if provider.ID == "server:events" && provider.Availability == "available" {
			found = true
		}
	}
	page, e := svc.QueryResult(ctx, j.ResultHandle, "server:events", 0, 25)
	if e != nil || !found || len(page.Rows) != 3 || page.ResultSHA256 != caps.ResultSHA256 {
		t.Fatal("same-family provider not reused", page, e)
	}
	_, result, e := svc.Read(j.ResultHandle)
	if e != nil || result.Type != "server_log" || result.Summary["total_events"] != json.Number("3") {
		t.Fatal("same-family analyzer not reused", result.Summary, e)
	}
}
