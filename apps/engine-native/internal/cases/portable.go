package cases

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"path"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type ExportOptions struct {
	SourceIDs        []string `json:"source_ids"`
	ResultIDs        []string `json:"result_ids"`
	IncludeWorkspace bool     `json:"include_workspace"`
	ManifestReviewed bool     `json:"manifest_reviewed"`
}

func (s *Store) Portable(ctx context.Context, caseID string, opts ExportOptions) ([]byte, error) {
	if !opts.ManifestReviewed {
		return nil, problem("CASE_SHARING_REVIEW_REQUIRED", "review case context/title/specifications before portable export")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.get(caseID)
	if e != nil {
		return nil, e
	}
	if m.State == "deleted" {
		return nil, problem("CASE_DELETED", "restore case before sharing")
	}
	sources := map[string]bool{}
	results := map[string]bool{}
	for _, key := range opts.SourceIDs {
		sources[key] = true
	}
	for _, key := range opts.ResultIDs {
		results[key] = true
	}
	objects := map[string][]byte{}
	for i := range m.Sources {
		v := &m.Sources[i]
		if sources[v.ID] {
			delete(sources, v.ID)
			if !v.Redaction.ReviewedForSharing {
				return nil, problem("CASE_SHARING_REVIEW_REQUIRED", "selected source has no sharing review")
			}
			var body []byte
			if v.Storage == "managed" {
				body, e = s.read(path.Join("cases", caseID, "objects", v.SHA256), MaxObjectBytes)
			} else if v.Storage == "external" {
				body, e = readLocal(v.ExternalPath, MaxObjectBytes)
			} else {
				return nil, problem("CASE_SOURCE_MISSING", "selected source unavailable")
			}
			if e != nil {
				return nil, e
			}
			if hash(body) != v.SHA256 || strconv.Itoa(len(body)) != v.Size {
				return nil, problem("CASE_HASH_MISMATCH", "selected source changed")
			}
			if nested(v.Name, body) {
				return nil, problem("ARCHIVE_NESTED_FORBIDDEN", "source archive cannot be nested in a portable case")
			}
			objects[v.SHA256] = body
			v.Storage = "managed"
		} else {
			v.Storage = "omitted"
			v.Status = "missing"
		}
		v.ExternalPath = ""
	}
	chosen := []ResultRef{}
	for _, v := range m.Results {
		if !results[v.ID] {
			continue
		}
		delete(results, v.ID)
		if !v.SharingReviewed {
			return nil, problem("CASE_SHARING_REVIEW_REQUIRED", "selected AnalysisResult has no sharing review")
		}
		body, e := s.read(path.Join("cases", caseID, "objects", v.SHA256), MaxObjectBytes)
		if e != nil {
			return nil, e
		}
		if hash(body) != v.SHA256 {
			return nil, problem("CASE_HASH_MISMATCH", "selected result changed")
		}
		objects[v.SHA256] = body
		chosen = append(chosen, v)
	}
	if len(sources) > 0 || len(results) > 0 {
		return nil, problem("CASE_REFERENCE_INVALID", "unknown source/result selection")
	}
	m.Results = chosen
	artifacts := []Artifact{}
	for _, v := range m.Artifacts {
		if _, ok := findResult(m, v.ResultID); ok {
			artifacts = append(artifacts, v)
		}
	}
	m.Artifacts = artifacts
	baselines := []Baseline{}
	for _, v := range m.Baselines {
		if _, ok := findResult(m, v.ResultID); ok {
			v.OriginalAvailable = false
			baselines = append(baselines, v)
		}
	}
	m.Baselines = baselines
	if !opts.IncludeWorkspace {
		m.Workspace = json.RawMessage("null")
	}
	if e = m.validate(); e != nil {
		return nil, e
	}
	manifest, e := json.Marshal(m)
	if e != nil {
		return nil, e
	}
	if len(objects)+1 > MaxEntries {
		return nil, problem("CASE_BUDGET_EXCEEDED", "portable entry count limit")
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	add := func(name string, b []byte) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0600)
		w, e := zw.CreateHeader(h)
		if e != nil {
			return e
		}
		_, e = w.Write(b)
		return e
	}
	if e = add("case.json", manifest); e != nil {
		return nil, e
	}
	expanded := len(manifest)
	for key, b := range objects {
		expanded += len(b)
		if expanded > MaxArchiveBytes {
			return nil, problem("CASE_BUDGET_EXCEEDED", "portable expanded byte limit")
		}
		if e = add("objects/"+key, b); e != nil {
			return nil, e
		}
	}
	if e = zw.Close(); e != nil {
		return nil, e
	}
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if out.Len() > MaxArchiveBytes {
		return nil, problem("CASE_BUDGET_EXCEEDED", "portable archive byte limit")
	}
	return out.Bytes(), nil
}
func (s *Store) Restore(ctx context.Context, file, previewSHA256 string) (Manifest, error) {
	raw, e := readLocal(file, MaxArchiveBytes)
	if e != nil {
		return Manifest{}, e
	}
	if !digestRE.MatchString(previewSHA256) || hash(raw) != previewSHA256 {
		return Manifest{}, problem("ARCHIVE_PREVIEW_STALE", "preview portable archive before restoring")
	}
	objects := map[string][]byte{}
	var manifest []byte
	_, e = scanArchive(ctx, raw, func(entry ArchiveEntry, b []byte) error {
		if entry.Name == "case.json" {
			if len(b) > MaxManifestBytes {
				return problem("CASE_BUDGET_EXCEEDED", "manifest byte limit")
			}
			manifest = b
			return nil
		}
		if !strings.HasPrefix(entry.Name, "objects/") || !digestRE.MatchString(strings.TrimPrefix(entry.Name, "objects/")) {
			return problem("CASE_ARCHIVE_INVALID", "unexpected portable entry")
		}
		key := strings.TrimPrefix(entry.Name, "objects/")
		if key != entry.SHA256 {
			return problem("CASE_HASH_MISMATCH", "portable object digest mismatch")
		}
		objects[key] = b
		return nil
	})
	if e != nil {
		return Manifest{}, e
	}
	if manifest == nil {
		return Manifest{}, problem("CASE_ARCHIVE_INVALID", "missing case manifest")
	}
	var m Manifest
	if e = decodeStrict(manifest, &m); e != nil {
		return m, e
	}
	if e = m.validate(); e != nil {
		return m, e
	}
	required := map[string]bool{}
	for i := range m.Sources {
		v := &m.Sources[i]
		if v.ExternalPath != "" || v.Storage == "external" {
			return m, problem("CASE_ARCHIVE_INVALID", "portable manifest contains external paths")
		}
		v.Redaction.ReviewedForSharing = false
		if v.Storage == "managed" {
			b, ok := objects[v.SHA256]
			if !ok || strconv.Itoa(len(b)) != v.Size {
				return m, problem("CASE_HASH_MISMATCH", "portable source absent or size mismatch")
			}
			required[v.SHA256] = true
		}
	}
	for i := range m.Results {
		v := &m.Results[i]
		b, ok := objects[v.SHA256]
		if !ok {
			return m, problem("CASE_HASH_MISMATCH", "portable result missing")
		}
		r, e := resultcontract.DecodeWithLimits(b, wirejson.ResultLimits(MaxObjectBytes, ingestion.DefaultBudgets().StackDepth))
		if e != nil {
			return m, e
		}
		if r.Type != v.Type || r.Metadata.SchemaVersion != v.PayloadVersion {
			return m, problem("CASE_ARCHIVE_INVALID", "result envelope/reference mismatch")
		}
		required[v.SHA256] = true
		v.SharingReviewed = false
		v.Availability = "snapshot-only"
	}
	for key := range objects {
		if !required[key] {
			return m, problem("CASE_ARCHIVE_INVALID", "unreferenced portable object")
		}
	}
	key, e := id("case")
	if e != nil {
		return m, e
	}
	m.ID = key
	m.Revision = "1"
	m.State = "restored"
	m.UpdatedAt = now()
	s.mu.Lock()
	defer s.mu.Unlock()
	list, e := s.list()
	if e != nil {
		return m, e
	}
	if len(list) >= MaxCases {
		return m, problem("CASE_BUDGET_EXCEEDED", "case count limit")
	}
	if e = ctx.Err(); e != nil {
		return m, e
	}
	e = s.commitContext(ctx, m, objects)
	if e != nil {
		if _, ge := s.get(m.ID); ge != nil {
			s.root.RemoveAll(path.Join("cases", m.ID))
		}
	}
	return m, e
}
