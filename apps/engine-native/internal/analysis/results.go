package analysis

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"io"
	"os"
	"path/filepath"
	"time"

	csvexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/csv"
	htmlexport "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/html"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

type ResultView struct {
	Handle        string                 `json:"handle"`
	Type          string                 `json:"type"`
	SummaryJSON   string                 `json:"summaryJSON"`
	Truncated     bool                   `json:"truncated"`
	Compatibility resultcontract.Verdict `json:"compatibility"`
}

func (s *Service) View(handle string) (ResultView, error) {
	_, r, e := s.Read(handle)
	if e != nil {
		return ResultView{}, e
	}
	body, e := json.Marshal(r.Summary)
	if e != nil {
		return ResultView{}, e
	}
	truncated := len(body) > 16<<10
	if truncated {
		body = []byte(`{"notice":"summary exceeds bounded view; use backend export"}`)
	}
	return ResultView{handle, r.Type, string(body), truncated, s.results.Check(r, "save")}, nil
}
func (s *Service) Load(ctx context.Context, path string) (ResultView, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(s.policy.TimeoutMS)*time.Millisecond)
	defer cancel()
	directory, e := os.MkdirTemp(filepath.Join(s.root, "staging"), "load-")
	if e != nil {
		return ResultView{}, e
	}
	defer os.RemoveAll(directory)
	handle, e := randomID("result-")
	if e != nil {
		return ResultView{}, e
	}
	lease, e := ingestion.Acquire(ctx, path, directory, handle+"-source", s.policy)
	if e != nil {
		return ResultView{}, e
	}
	defer lease.Close()
	f, e := lease.Open(ctx)
	if e != nil {
		return ResultView{}, e
	}
	defer f.Close()
	body, e := io.ReadAll(io.LimitReader(f, s.policy.DecodedBytes+1))
	if e != nil {
		return ResultView{}, e
	}
	if int64(len(body)) > s.policy.DecodedBytes {
		return ResultView{}, fmt.Errorf("result byte budget exceeded")
	}
	if _, e = resultcontract.Decode(body); e != nil {
		return ResultView{}, e
	}
	if e = ctx.Err(); e != nil {
		return ResultView{}, e
	}
	if e = lease.Verify(ctx); e != nil {
		return ResultView{}, e
	}
	staged := filepath.Join(directory, "result.json")
	if e = writeDurable(staged, body); e != nil {
		return ResultView{}, e
	}
	if e = ctx.Err(); e != nil {
		return ResultView{}, e
	}
	if e = os.Rename(staged, filepath.Join(s.root, "results", handle+".json")); e != nil {
		return ResultView{}, e
	}
	return s.View(handle)
}
func (s *Service) ResultJSON(handle string) (string, error) {
	body, _, e := s.Read(handle)
	return string(body), e
}
func (s *Service) Scalar(handle, path string) (resultcontract.ScalarView, error) {
	_, r, e := s.Read(handle)
	if e != nil {
		return resultcontract.ScalarView{}, e
	}
	if v := s.results.Check(r, "display"); !v.Allowed {
		return resultcontract.ScalarView{}, fmt.Errorf("%s", v.Reason)
	}
	return resultcontract.Scalar(r, path)
}
func (s *Service) Compare(a, b, path string) (resultcontract.Delta, error) {
	_, left, e := s.Read(a)
	if e != nil {
		return resultcontract.Delta{}, e
	}
	_, right, e := s.Read(b)
	if e != nil {
		return resultcontract.Delta{}, e
	}
	return s.results.Compare(left, right, path)
}
func (s *Service) Export(handle, format string) ([]byte, error) {
	body, r, e := s.Read(handle)
	if e != nil {
		return nil, e
	}
	if format == "raw-json" {
		return body, nil
	}
	if v := s.results.Check(r, format); !v.Allowed {
		return nil, fmt.Errorf("%s", v.Reason)
	}
	switch format {
	case "json":
		return body, nil
	case "csv":
		view, e := resultcontract.ExportPresentation(r, false)
		if e != nil {
			return nil, e
		}
		return csvexport.Marshal(view)
	case "html":
		view, e := resultcontract.ExportPresentation(r, true)
		if e != nil {
			return nil, e
		}
		return htmlexport.Marshal(view)
	case "report-pack":
		return s.ReportPack([]string{handle})
	default:
		return nil, fmt.Errorf("unsupported export format")
	}
}

// ReportPack creates a source appendix from backend revisions. It does not
// recompute the Evidence Board narrative or parse opaque JSON in JavaScript.
func (s *Service) ReportPack(handles []string) ([]byte, error) {
	if len(handles) == 0 || len(handles) > 32 {
		return nil, fmt.Errorf("pack requires 1..32 results")
	}
	var buffer bytes.Buffer
	z := zip.NewWriter(&buffer)
	entries := []map[string]string{}
	var total int64
	for _, handle := range handles {
		body, r, e := s.Read(handle)
		if e != nil {
			return nil, e
		}
		if v := s.results.Check(r, "report-pack"); !v.Allowed {
			return nil, fmt.Errorf("%s", v.Reason)
		}
		total += int64(len(body))
		if total > s.policy.DecodedBytes {
			return nil, fmt.Errorf("pack byte budget exceeded")
		}
		name := "sources/" + handle + ".json"
		w, e := z.Create(name)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(body); e != nil {
			return nil, e
		}
		sum := sha256.Sum256(body)
		entries = append(entries, map[string]string{"path": name, "sha256": hex.EncodeToString(sum[:])})
	}
	manifest, _ := json.Marshal(map[string]any{"type": "archscope_report_pack", "schema_version": "0.1.0", "scope": "backend_source_appendix", "source_results": entries})
	w, e := z.Create("report-pack.json")
	if e != nil {
		return nil, e
	}
	if _, e = w.Write(manifest); e != nil {
		return nil, e
	}
	if e = z.Close(); e != nil {
		return nil, e
	}
	return buffer.Bytes(), nil
}
func AtomicExport(path string, body []byte) error {
	directory := filepath.Dir(path)
	f, e := os.CreateTemp(directory, ".evidence-export-")
	if e != nil {
		return e
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(body)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	return os.Rename(temporary, path)
}
