package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	profile "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/systemtrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/systemtrace"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"strconv"
)

type SystemInventory struct {
	ResultSHA256 string `json:"result_sha256"`
	a.SourceInventory
}

func withSystem[T any](s *Service, parent context.Context, handle, digest string, run func(context.Context, m.Set, string) (T, error)) (out T, err error) {
	ctx, finish, e := s.beginLoad(parent)
	if e != nil {
		return out, e
	}
	defer finish()
	owner, e := s.AcquireResult(handle)
	if e != nil {
		return out, e
	}
	defer s.ReleaseResultLease(owner.ID)
	body, result, e := s.Read(handle)
	if e != nil {
		return out, e
	}
	actual := bytesDigest(body)
	if digest != "" && digest != actual {
		return out, fmt.Errorf("SYSTEM_RESULT_STALE")
	}
	if verdict := s.results.Check(result, "compute"); !verdict.Allowed {
		return out, fmt.Errorf("SYSTEM_RESULT_UNSUPPORTED: %s", verdict.Reason)
	}
	model, e := a.ReadCore(result)
	if e != nil {
		return out, e
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	return run(ctx, model, actual)
}
func (s *Service) SystemInventory(ctx context.Context, handle string) (SystemInventory, error) {
	return withSystem(s, ctx, handle, "", func(ctx context.Context, set m.Set, digest string) (SystemInventory, error) {
		return SystemInventory{digest, a.Inventory(set)}, ctx.Err()
	})
}
func (s *Service) MaterializeSystemProfile(ctx context.Context, handle, digest, metric string, entities []string) (ResultView, ResultLease, error) {
	var view ResultView
	var lease ResultLease
	if digest == "" || len(metric) > 512 || len(entities) > 1024 {
		return view, lease, fmt.Errorf("SYSTEM_PROJECTION_REQUEST_INVALID")
	}
	body, e := withSystem(s, ctx, handle, digest, func(ctx context.Context, set m.Set, actual string) ([]byte, error) {
		model, e := a.Project(ctx, set, metric, entities)
		if e != nil {
			return nil, e
		}
		selection := profilemodel.Selection{ProfileIDs: []string{}, MetricID: metric}
		for _, p := range model.Profiles {
			selection.ProfileIDs = append(selection.ProfileIDs, p.ID)
		}
		r, e := profile.BuildSet(ctx, model, selection, "explicit system evidence projection")
		if e != nil {
			return nil, e
		}
		r.Metadata.Extra["system_projection"] = map[string]any{"result_sha256": actual, "source_sha256": set.SourceSHA256, "metric": metric, "entities": entities, "identity_basis": "declared"}
		return json.Marshal(r)
	})
	if e != nil {
		return view, lease, e
	}
	return s.LoadOwnedSnapshot(ctx, body)
}

// Source opening verifies both case/result linkage and normalization against the
// immutable original. The caller cannot choose an arbitrary line or path.
func (s *Service) OpenSystemEvidence(ctx context.Context, store *cases.Store, caseID, revision, resultID, resultSHA, sourceID, sourceSHA, eventID string) (cases.TextPage, error) {
	ctx, finish, e := s.beginLoad(ctx)
	if e != nil {
		return cases.TextPage{}, e
	}
	defer finish()
	manifest, e := store.Get(caseID)
	if e != nil {
		return cases.TextPage{}, e
	}
	if manifest.Revision != revision {
		return cases.TextPage{}, fmt.Errorf("CASE_REVISION_STALE")
	}
	var ref *cases.ResultRef
	for i := range manifest.Results {
		if manifest.Results[i].ID == resultID {
			ref = &manifest.Results[i]
		}
	}
	if ref == nil || ref.Stale || ref.SHA256 != resultSHA || ref.SourceHashes[sourceID] != sourceSHA {
		return cases.TextPage{}, fmt.Errorf("CASE_LINK_STALE")
	}
	raw, e := store.ExportSnapshot(caseID, resultID)
	if e != nil {
		return cases.TextPage{}, e
	}
	r, e := resultcontract.Decode(raw)
	if e != nil {
		return cases.TextPage{}, e
	}
	if !s.results.Check(r, "compute").Allowed {
		return cases.TextPage{}, fmt.Errorf("SYSTEM_RESULT_UNSUPPORTED")
	}
	set, e := a.ReadCore(r)
	if e != nil {
		return cases.TextPage{}, e
	}
	if set.SourceSHA256 != sourceSHA {
		return cases.TextPage{}, fmt.Errorf("CASE_LINK_STALE")
	}
	line := 0
	for _, record := range set.Records {
		if record.ID == eventID {
			line = record.Line
			break
		}
	}
	if line == 0 {
		return cases.TextPage{}, fmt.Errorf("SYSTEM_EVENT_REFERENCE_MISSING")
	}
	original, e := store.SourceSnapshot(ctx, caseID, revision, sourceID)
	if e != nil {
		return cases.TextPage{}, e
	}
	if bytes.HasPrefix(original, []byte{31, 139}) {
		return cases.TextPage{}, fmt.Errorf("SYSTEM_GZIP_STRUCTURAL_REFERENCE: positions refer to decoded content; use explicit archive intake before text drilldown")
	}
	reparsed, e := p.ParseBytes(ctx, original, p.Options{Format: set.Format, Capture: set.Capture})
	if e != nil {
		return cases.TextPage{}, e
	}
	matched := false
	for _, record := range reparsed.Records {
		matched = matched || record.ID == eventID
	}
	if !matched {
		return cases.TextPage{}, fmt.Errorf("SYSTEM_ORIGINAL_REFERENCE_MISMATCH")
	}
	return store.OpenEvidence(ctx, caseID, revision, sourceID, sourceSHA, strconv.Itoa(line))
}
func (s *Service) SystemHistogram(ctx context.Context, handle, digest, eventID string, offset, limit int) (a.HistogramPage, error) {
	if digest == "" {
		return a.HistogramPage{}, fmt.Errorf("SYSTEM_RESULT_STALE")
	}
	return withSystem(s, ctx, handle, digest, func(ctx context.Context, set m.Set, actual string) (a.HistogramPage, error) {
		return a.Histogram(ctx, set, actual, eventID, offset, limit)
	})
}
