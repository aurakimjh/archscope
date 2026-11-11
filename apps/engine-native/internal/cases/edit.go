package cases

import (
	"context"
	"encoding/json"
	"path"
	"path/filepath"
	"strconv"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/common"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

type Edit struct {
	Headers          map[string]string `json:"headers"`
	CaseID           string            `json:"case_id"`
	ExpectedRevision string            `json:"expected_revision"`
	Operation        string            `json:"operation"`
	Title            string            `json:"title"`
	Symptom          string            `json:"symptom"`
	RetentionDays    *string           `json:"retention_days"`
	Workspace        json.RawMessage   `json:"workspace"`
	Capture          *Capture          `json:"capture"`
	Path             string            `json:"path"`
	Name             string            `json:"name"`
	SourceID         string            `json:"source_id"`
	CaptureID        string            `json:"capture_id"`
	Copy             bool              `json:"copy"`
	Sanitize         bool              `json:"sanitize"`
	SharingReviewed  bool              `json:"sharing_reviewed"`
	Decode           DecodeOptions     `json:"decode"`
	ResultID         string            `json:"result_id"`
	Spec             json.RawMessage   `json:"spec"`
	Conditions       json.RawMessage   `json:"conditions"`
	ArchiveSHA256    string            `json:"archive_sha256"`
	Entries          []string          `json:"entries"`
	ExpectedHashes   map[string]string `json:"expected_hashes"`
}

func (s *Store) Apply(ctx context.Context, edit Edit) (Manifest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.expect(edit.CaseID, edit.ExpectedRevision)
	if e != nil {
		return m, e
	}
	if e = ctx.Err(); e != nil {
		return m, e
	}
	if m.State == "deleted" && edit.Operation != "restore" {
		return m, problem("CASE_DELETED", "restore logical deletion before editing")
	}
	if m.State == "archived" && edit.Operation != "restore" && edit.Operation != "delete" {
		return m, problem("CASE_ARCHIVED", "restore archived case before editing")
	}
	objects := map[string][]byte{}
	switch edit.Operation {
	case "details":
		m.Title = edit.Title
		m.Symptom = edit.Symptom
	case "retention":
		m.RetentionDays = edit.RetentionDays
	case "workspace":
		if e = requireJSON(edit.Workspace); e != nil {
			return m, e
		}
		m.Workspace = edit.Workspace
	case "archive":
		if m.State != "active" && m.State != "restored" {
			return m, problem("CASE_STATE_INVALID", "only active/restored cases can be archived")
		}
		m.State = "archived"
	case "restore":
		if m.State != "archived" && m.State != "deleted" {
			return m, problem("CASE_STATE_INVALID", "only archived/deleted cases can be restored")
		}
		m.State = "restored"
	case "delete":
		m.State = "deleted"
	case "capture":
		if edit.Capture == nil {
			return m, problem("CAPTURE_INVALID", "capture required")
		}
		if e = edit.Capture.Validate(); e != nil {
			return m, e
		}
		cleaned, e := edit.Capture.SanitizedOptions()
		if e != nil {
			return m, e
		}
		cleaned, e = MergeCaptureContext(cleaned, edit.Headers)
		if e != nil {
			return m, e
		}
		m.Captures = append(m.Captures, cleaned)
	case "source", "replace-source":
		var source Source
		source, e = prepareSource(edit)
		if e != nil {
			return m, e
		}
		body, e := readLocal(edit.Path, MaxObjectBytes)
		if e != nil {
			return m, e
		}
		if hash(body) != source.SHA256 {
			return m, problem("CASE_SOURCE_CHANGED", "source changed during intake")
		}
		if e = matchCapture(m, source); e != nil {
			return m, e
		}
		if edit.Sanitize {
			if !edit.Copy {
				return m, problem("CASE_REDACTION_INVALID", "sanitization creates an explicit managed copy")
			}
			decoded, e := DecodeText(body, edit.Decode)
			if e != nil {
				return m, e
			}
			source.ParentSHA256 = source.SHA256
			body = []byte(common.RedactText(decoded.Text).Text)
			source.SHA256 = hash(body)
			source.Size = strconv.Itoa(len(body))
			source.Redaction = Redaction{Profile: "text-mask", Version: common.RedactionVersion, Applied: true, ReviewedForSharing: edit.SharingReviewed}
			d, e := DecodeText(body, DecodeOptions{Encoding: "utf-8"})
			if e != nil {
				return m, e
			}
			source.Encoding = &d.Encoding
		} else if edit.Decode.Encoding != "" || len(edit.Decode.Sniff) > 0 {
			d, e := DecodeText(body, edit.Decode)
			if e != nil {
				return m, e
			}
			source.Encoding = &d.Encoding
		}
		if edit.Copy {
			source.Storage = "managed"
			source.ExternalPath = ""
			objects[source.SHA256] = body
		}
		source.Redaction.ReviewedForSharing = edit.SharingReviewed
		if edit.Operation == "source" {
			m.Sources = append(m.Sources, source)
		} else {
			found := false
			for i := range m.Sources {
				if m.Sources[i].ID == edit.SourceID {
					source.ID = edit.SourceID
					m.Sources[i] = source
					found = true
				}
			}
			if !found {
				return m, problem("CASE_SOURCE_MISSING", "source to replace not found")
			}
			markStale(&m, edit.SourceID)
		}
	case "encoding":
		found := false
		for i := range m.Sources {
			v := &m.Sources[i]
			if v.ID != edit.SourceID {
				continue
			}
			found = true
			var body []byte
			if v.Storage == "external" {
				body, e = readLocal(v.ExternalPath, MaxObjectBytes)
			} else if v.Storage == "managed" {
				body, e = s.read(path.Join("cases", m.ID, "objects", v.SHA256), MaxObjectBytes)
			} else {
				return m, problem("CASE_SOURCE_MISSING", "source not available")
			}
			if e != nil {
				return m, e
			}
			if hash(body) != v.SHA256 {
				return m, problem("CASE_HASH_MISMATCH", "source changed")
			}
			d, e := DecodeText(body, edit.Decode)
			if e != nil {
				return m, e
			}
			v.Encoding = &d.Encoding
		}
		if !found {
			return m, problem("CASE_SOURCE_MISSING", "source not found")
		}
		markStale(&m, edit.SourceID)
	case "omit-source":
		found := false
		for i := range m.Sources {
			v := &m.Sources[i]
			if v.ID == edit.SourceID {
				v.Storage = "omitted"
				v.ExternalPath = ""
				v.Status = "missing"
				found = true
			}
		}
		if !found {
			return m, problem("CASE_SOURCE_MISSING", "source not found")
		}
		markStale(&m, edit.SourceID)
	case "review-source":
		found := false
		for i := range m.Sources {
			if m.Sources[i].ID == edit.SourceID {
				m.Sources[i].Redaction.ReviewedForSharing = edit.SharingReviewed
				found = true
			}
		}
		if !found {
			return m, problem("CASE_SOURCE_MISSING", "source not found")
		}
	case "review-result":
		found := false
		for i := range m.Results {
			if m.Results[i].ID == edit.ResultID {
				m.Results[i].SharingReviewed = edit.SharingReviewed
				found = true
			}
		}
		if !found {
			return m, problem("CASE_RESULT_MISSING", "result not found")
		}
	case "baseline":
		if e = requireJSON(edit.Conditions); e != nil {
			return m, e
		}
		r, ok := findResult(m, edit.ResultID)
		if !ok {
			return m, problem("CASE_RESULT_MISSING", "baseline result not found")
		}
		key, e := id("baseline")
		if e != nil {
			return m, e
		}
		m.Baselines = append(m.Baselines, Baseline{ID: key, ResultID: r.ID, ResultSHA256: r.SHA256, CaptureID: r.CaptureID, Conditions: edit.Conditions})
	case "artifact":
		if e = requireJSON(edit.Spec); e != nil {
			return m, e
		}
		r, ok := findResult(m, edit.ResultID)
		if !ok {
			return m, problem("CASE_RESULT_MISSING", "artifact result not found")
		}
		key, e := id("artifact")
		if e != nil {
			return m, e
		}
		m.Artifacts = append(m.Artifacts, Artifact{ID: key, ResultID: r.ID, ResultSHA256: r.SHA256, Spec: edit.Spec, Stale: r.Stale})
	case "archive-intake":
		raw, e := readLocal(edit.Path, MaxArchiveBytes)
		if e != nil {
			return m, e
		}
		if !digestRE.MatchString(edit.ArchiveSHA256) || hash(raw) != edit.ArchiveSHA256 {
			return m, problem("ARCHIVE_PREVIEW_STALE", "preview archive again before intake")
		}
		selected := map[string]bool{}
		for _, name := range edit.Entries {
			if !safeName(name) || selected[name] {
				return m, problem("ARCHIVE_SELECTION_INVALID", "invalid/duplicate selected entry")
			}
			selected[name] = true
		}
		if len(selected) == 0 {
			return m, problem("ARCHIVE_SELECTION_REQUIRED", "select archive entries explicitly")
		}
		found := map[string]bool{}
		_, e = scanArchive(ctx, raw, func(entry ArchiveEntry, b []byte) error {
			if !selected[entry.Name] {
				return nil
			}
			found[entry.Name] = true
			key, e := id("source")
			if e != nil {
				return e
			}
			v := Source{ID: key, Name: entry.Name, SHA256: entry.SHA256, Size: entry.Size, Storage: "managed", Status: "available", CaptureID: edit.CaptureID}
			expected := edit.ExpectedHashes[entry.Name]
			if e = matchCapture(m, v); e != nil {
				v.Status = "failed"
				v.Storage = "omitted"
				v.Reason = "Capture manifest file hash/size/name did not match."
			}

			if expected != "" && !digestRE.MatchString(expected) {
				return problem("CASE_INVALID", "invalid expected entry digest")
			}
			if expected != "" && entry.SHA256 != expected {
				v.Status = "failed"
				v.Storage = "omitted"
				v.Reason = "Declared content hash did not match; source not published."
			} else if v.Status != "failed" {
				objects[v.SHA256] = b
			}
			m.Sources = append(m.Sources, v)
			return nil
		})
		if e != nil {
			return m, e
		}
		for name := range selected {
			if !found[name] {
				return m, problem("ARCHIVE_SELECTION_INVALID", "selected entry absent")
			}
		}
	default:
		return m, problem("CASE_OPERATION_UNSUPPORTED", "unknown case operation")
	}
	if e = ctx.Err(); e != nil {
		return m, e
	}
	if e = advance(&m); e != nil {
		return m, e
	}
	e = s.commitContext(ctx, m, objects)
	return m, e
}
func prepareSource(edit Edit) (Source, error) {
	name := edit.Name
	if name == "" {
		name = filepath.Base(edit.Path)
	}
	if !safeName(name) {
		return Source{}, problem("CASE_INVALID", "unsafe source name")
	}
	b, e := readLocal(edit.Path, MaxObjectBytes)
	if e != nil {
		return Source{}, e
	}
	key, e := id("source")
	if e != nil {
		return Source{}, e
	}
	p, e := filepath.Abs(edit.Path)
	if e != nil {
		return Source{}, e
	}
	return Source{ID: key, Name: name, SHA256: hash(b), Size: strconv.Itoa(len(b)), Storage: "external", ExternalPath: p, CaptureID: edit.CaptureID, Status: "available"}, nil
}
func markStale(m *Manifest, sourceID string) {
	for i := range m.Results {
		r := &m.Results[i]
		for _, key := range r.SourceIDs {
			if key == sourceID {
				r.Stale = true
			}
		}
	}
	for i := range m.Artifacts {
		r, ok := findResult(*m, m.Artifacts[i].ResultID)
		if ok && r.Stale {
			m.Artifacts[i].Stale = true
		}
	}
}
func findResult(m Manifest, resultID string) (ResultRef, bool) {
	for _, r := range m.Results {
		if r.ID == resultID {
			return r, true
		}
	}
	return ResultRef{}, false
}

type SaveResult struct {
	CaseID           string            `json:"case_id"`
	ExpectedRevision string            `json:"expected_revision"`
	SourceIDs        []string          `json:"source_ids"`
	CaptureID        string            `json:"capture_id"`
	Selection        json.RawMessage   `json:"selection"`
	OptionsHash      string            `json:"options_hash"`
	Versions         map[string]string `json:"versions"`
	SharingReviewed  bool              `json:"sharing_reviewed"`
}

func (s *Store) SaveResult(req SaveResult, body []byte) (Manifest, error) {
	if len(body) > MaxObjectBytes {
		return Manifest{}, problem("CASE_BUDGET_EXCEEDED", "result byte limit")
	}
	r, e := resultcontract.DecodeWithLimits(body, wirejson.ResultLimits(MaxObjectBytes, ingestion.DefaultBudgets().StackDepth))
	if e != nil {
		return Manifest{}, e
	}
	if r.Type == "" || r.Metadata.SchemaVersion == "" {
		return Manifest{}, problem("CASE_RESULT_INVALID", "AnalysisResult type/version required")
	}
	if e = requireJSON(req.Selection); e != nil {
		return Manifest{}, e
	}
	if req.OptionsHash != "" && !digestRE.MatchString(req.OptionsHash) {
		return Manifest{}, problem("CASE_INVALID", "invalid options digest")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.expect(req.CaseID, req.ExpectedRevision)
	if e != nil {
		return m, e
	}
	if m.State != "active" && m.State != "restored" {
		return m, problem("CASE_STATE_INVALID", "restore case before saving result")
	}
	sources := map[string]string{}
	for _, v := range m.Sources {
		sources[v.ID] = v.SHA256
	}
	pinned := map[string]string{}
	for _, key := range req.SourceIDs {
		if sources[key] == "" {
			return m, problem("CASE_SOURCE_MISSING", "result source is not in case")
		}
		pinned[key] = sources[key]
	}
	key, e := id("result")
	if e != nil {
		return m, e
	}
	m.Results = append(m.Results, ResultRef{ID: key, SHA256: hash(body), Type: r.Type, PayloadVersion: r.Metadata.SchemaVersion, SourceIDs: req.SourceIDs, SourceHashes: pinned, CaptureID: req.CaptureID, Selection: req.Selection, OptionsHash: req.OptionsHash, Versions: req.Versions, SharingReviewed: req.SharingReviewed, Availability: "snapshot-only"})
	if e = advance(&m); e != nil {
		return m, e
	}
	e = s.commit(m, map[string][]byte{hash(body): body})
	return m, e
}

// ExportSnapshot returns the exact saved AnalysisResult, including unknown JSON
// fields and decimal tokens; renderer projections never become source of truth.
func (s *Store) ExportSnapshot(caseID, resultID string) ([]byte, error) {
	m, e := s.Get(caseID)
	if e != nil {
		return nil, e
	}
	r, ok := findResult(m, resultID)
	if !ok {
		return nil, problem("CASE_RESULT_MISSING", "saved result not found")
	}
	return s.Object(caseID, r.SHA256)
}

func matchCapture(m Manifest, s Source) error {
	if s.CaptureID == "" {
		return nil
	}
	for _, c := range m.Captures {
		if c.ID != s.CaptureID {
			continue
		}
		if len(c.Files) == 0 {
			return nil
		}
		for _, f := range c.Files {
			if f.Name == s.Name {
				if f.SHA256 != s.SHA256 || f.Size != s.Size {
					return problem("CAPTURE_FILE_MISMATCH", "source differs from declared capture file")
				}
				return nil
			}
		}
		return problem("CAPTURE_FILE_MISMATCH", "source name not declared in selected capture")
	}
	return problem("CAPTURE_INVALID", "selected capture not found")
}
