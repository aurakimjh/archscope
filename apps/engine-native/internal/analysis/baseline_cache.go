package analysis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/cases"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

// This process-local cache can only be populated by original reimport. Case
// objects, imported receipts and disk caches never establish this authority.
type baselineCacheEntry struct {
	key     string
	body    []byte
	expires time.Time
}

const baselineCacheBytes = 128 << 20

func (s *Service) cachedBaseline(key string) (BaselineComparison, bool) {
	s.baselineMu.Lock()
	defer s.baselineMu.Unlock()
	for _, entry := range s.baselineCache {
		if entry.key == key && time.Now().Before(entry.expires) {
			var out BaselineComparison
			if wirejson.DecodeWithLimits(entry.body, &out, wirejson.ResultLimits(baselineCacheBytes, 1024)) == nil {
				return out, true
			}
		}
	}
	return BaselineComparison{}, false
}
func (s *Service) rememberBaseline(key string, out BaselineComparison) {
	body, err := json.Marshal(out)
	if err != nil || len(body) > baselineCacheBytes {
		return
	}
	s.baselineMu.Lock()
	defer s.baselineMu.Unlock()
	kept := []baselineCacheEntry{}
	charge := len(body)
	for _, entry := range s.baselineCache {
		if entry.key != key && time.Now().Before(entry.expires) && len(kept) < 1 && charge+len(entry.body) <= baselineCacheBytes {
			kept = append(kept, entry)
			charge += len(entry.body)
		}
	}
	s.baselineCache = append(kept, baselineCacheEntry{key, body, time.Now().Add(2 * time.Minute)})
}
func checkBaselineBytes(ctx context.Context, store *cases.Store, req BaselineComparisonRequest, baselineResult string, out BaselineComparison) error {
	for _, pair := range []struct {
		id      string
		binding BaselineBinding
	}{{baselineResult, out.Baseline}, {req.TargetResultID, out.Target}} {
		body, err := store.ExportSnapshot(req.CaseID, pair.id)
		if err != nil {
			return err
		}
		if bytesDigest(body) != pair.binding.ResultSHA256 {
			return fmt.Errorf("BASELINE_RESULT_CHANGED")
		}
		original, err := store.SourceSnapshot(ctx, req.CaseID, req.Revision, pair.binding.SourceID)
		if err != nil {
			return err
		}
		if bytesDigest(original) != pair.binding.SourceSHA256 {
			return fmt.Errorf("BASELINE_SOURCE_BINDING_MISMATCH")
		}
	}
	return ctx.Err()
}

func (s *Service) CompareCaseBaseline(parent context.Context, store *cases.Store, req BaselineComparisonRequest) (BaselineComparison, error) {
	ctx, finish, err := s.beginLoad(parent)
	if err != nil {
		return BaselineComparison{}, err
	}
	defer finish()
	limit := req.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 || len(req.Cursor) > 2048 {
		return BaselineComparison{}, fmt.Errorf("BASELINE_QUERY_INVALID")
	}
	manifest, err := store.Get(req.CaseID)
	if err != nil {
		return BaselineComparison{}, err
	}
	if manifest.Revision != req.Revision {
		return BaselineComparison{}, fmt.Errorf("CASE_REVISION_STALE")
	}
	query := req
	query.Cursor = ""
	key, err := canonicalDigest([]any{BaselineVersion, manifest, query})
	if err != nil {
		return BaselineComparison{}, err
	}
	out, hit := s.cachedBaseline(key)
	if !hit {
		out, err = s.computeCaseBaseline(ctx, store, query)
		if err != nil {
			return out, err
		}
	}
	baselineResult := ""
	for _, pin := range manifest.Baselines {
		if pin.ID == req.BaselineID {
			baselineResult = pin.ResultID
		}
	}
	if err = checkBaselineBytes(ctx, store, req, baselineResult, out); err != nil {
		return BaselineComparison{}, err
	}
	current, err := store.Get(req.CaseID)
	if err != nil {
		return BaselineComparison{}, err
	}
	currentKey, err := canonicalDigest([]any{BaselineVersion, current, query})
	if err != nil || currentKey != key {
		return BaselineComparison{}, fmt.Errorf("BASELINE_BINDING_CHANGED")
	}
	if !hit {
		s.rememberBaseline(key, out)
	}
	offset := 0
	if req.Cursor != "" {
		var cursor struct {
			Query  string
			Offset int
		}
		body, e := base64.RawURLEncoding.DecodeString(req.Cursor)
		if e != nil || json.Unmarshal(body, &cursor) != nil || cursor.Query != out.QuerySHA256 || cursor.Offset < 0 {
			return BaselineComparison{}, fmt.Errorf("BASELINE_CURSOR_STALE")
		}
		offset = cursor.Offset
	}
	if offset > out.Total {
		return BaselineComparison{}, fmt.Errorf("BASELINE_CURSOR_STALE")
	}
	end := min(offset+limit, out.Total)
	out.Difference.Rows = out.Difference.Rows[offset:end]
	if end < out.Total {
		body, _ := json.Marshal(struct {
			Query  string
			Offset int
		}{out.QuerySHA256, end})
		out.Next = base64.RawURLEncoding.EncodeToString(body)
	}
	out.CacheHit = hit
	out.VerificationScope = "original reimport at query start; original/result bytes and complete case binding rechecked on every page; process-local cache expires after 2 minutes"
	body, err := json.Marshal(out)
	if err != nil {
		return BaselineComparison{}, err
	}
	if len(body) > 1<<20 {
		return BaselineComparison{}, fmt.Errorf("BASELINE_RESPONSE_BUDGET: reduce selected profiles or capture metadata")
	}
	return out, nil
}
