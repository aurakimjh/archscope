package cases

import (
	"context"
	"encoding/base64"
	"encoding/json"
	analyzer "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"math/big"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/common"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

const SearchVersion = "1.1.0"
const MaxSearchRows = 100_000
const MaxSearchBytes = 16 << 20

type SearchQuery struct {
	TimeFrom *string `json:"time_from,omitempty"`
	TimeTo   *string `json:"time_to,omitempty"`
	Clock    string  `json:"clock,omitempty"`
	TimeUnit string  `json:"time_unit,omitempty"`
	CaseID   string  `json:"case_id"`
	Revision string  `json:"revision"`
	Text     string  `json:"text"`
	SourceID string  `json:"source_id,omitempty"`
	ResultID string  `json:"result_id,omitempty"`
	Field    string  `json:"field,omitempty"`
	Cursor   string  `json:"cursor,omitempty"`
	Limit    int     `json:"limit"`
}
type SearchRow struct {
	FrameIDs  []string `json:"frame_ids,omitempty"`
	Timestamp *string  `json:"timestamp,omitempty"`
	Clock     string   `json:"clock,omitempty"`
	TimeUnit  string   `json:"time_unit,omitempty"`
	SourceRef string   `json:"source_ref,omitempty"`
	ProfileID string   `json:"profile_id,omitempty"`
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	SourceID  string   `json:"source_id,omitempty"`
	ResultID  string   `json:"result_id,omitempty"`
	SHA256    string   `json:"sha256"`
	Field     string   `json:"field"`
	Text      string   `json:"text"`
	Line      *Line    `json:"line,omitempty"`
	Truncated bool     `json:"truncated"`
}
type SearchCoverage struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Indexed int    `json:"indexed"`
	Total   int    `json:"total"`
	Reason  string `json:"reason,omitempty"`
}
type SearchPage struct {
	Untimed     int              `json:"untimed_rows"`
	Version     string           `json:"version"`
	CaseID      string           `json:"case_id"`
	Revision    string           `json:"revision"`
	QuerySHA256 string           `json:"query_sha256"`
	IndexSHA256 string           `json:"index_sha256"`
	Rows        []SearchRow      `json:"rows"`
	Coverage    []SearchCoverage `json:"coverage"`
	Matched     int              `json:"matched"`
	Next        string           `json:"next"`
}
type searchIndex struct {
	Version, Key, Checksum string
	Rows                   []SearchRow
	Coverage               []SearchCoverage
}
type searchCursor struct {
	Key, Query, Index string
	Offset            int
}

func cleanSearch(s string) (string, bool) {
	s = common.RedactText(s).Text
	cut := len(s) > 2048
	if cut {
		s = s[:2048]
		for !utf8.ValidString(s) {
			s = s[:len(s)-1]
		}
	}
	return s, cut
}
func (s *Store) Search(ctx context.Context, q SearchQuery) (SearchPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := SearchPage{Version: SearchVersion, CaseID: q.CaseID, Revision: q.Revision, Rows: []SearchRow{}}
	if q.Limit < 1 || q.Limit > 200 || len(q.Text) > 1024 || len(q.Field) > 128 || len(q.Cursor) > 2048 {
		return out, problem("CASE_QUERY_INVALID", "bounded literal search and limit 1..200 required")
	}
	var from, to *big.Rat
	var parseErr error
	if q.TimeFrom != nil {
		from, parseErr = profilemodel.Number(*q.TimeFrom)
		if parseErr != nil {
			return out, parseErr
		}
	}
	if q.TimeTo != nil {
		to, parseErr = profilemodel.Number(*q.TimeTo)
		if parseErr != nil {
			return out, parseErr
		}
	}
	if from != nil && to != nil && from.Cmp(to) > 0 {
		return out, problem("CASE_QUERY_INVALID", "time start exceeds end")
	}
	if from != nil || to != nil {
		if q.ResultID == "" || q.Clock == "" || q.TimeUnit == "" || len(q.Clock) > 64 || len(q.TimeUnit) > 64 {
			return out, problem("CASE_TIME_SCOPE_REQUIRED", "select one result and explicit clock/unit; relative clocks are never aligned across captures")
		}
	}
	m, e := s.expect(q.CaseID, q.Revision)
	if e != nil {
		return out, e
	}
	if m.State == "deleted" {
		return out, problem("CASE_DELETED", "restore the case first")
	}
	// Even a cache hit must recheck original bytes. Missing/changed sources are
	// coverage gaps and invalidate prior cursors, never stale successful links.
	state := []any{SearchVersion, common.RedactionVersion, m}
	available := map[string]bool{}
	for _, v := range m.Sources {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		available[v.ID] = v.Storage != "omitted" && s.verifyPageSource(ctx, m.ID, v) == nil
		state = append(state, []any{v.ID, available[v.ID]})
	}
	for _, r := range m.Results {
		b, e := s.read(path.Join("cases", m.ID, "objects", r.SHA256), MaxObjectBytes)
		state = append(state, []any{r.ID, e == nil && hash(b) == r.SHA256})
	}
	stateBody, _ := json.Marshal(state)
	key := hash(stateBody)
	cache := path.Join("indexes", "search-"+m.ID+".json")
	var index searchIndex
	body, readErr := s.read(cache, MaxSearchBytes+1<<20)
	if readErr == nil {
		if json.Unmarshal(body, &index) != nil || index.Key != key || index.Version != SearchVersion || len(index.Rows) > MaxSearchRows {
			index = searchIndex{}
		} else {
			check := index.Checksum
			index.Checksum = ""
			encoded, _ := json.Marshal(index)
			index.Checksum = check
			if check != hash(encoded) {
				index = searchIndex{}
			}
		}
	}
	if index.Key != key {
		index = searchIndex{Version: SearchVersion, Key: key, Rows: []SearchRow{}, Coverage: []SearchCoverage{}}
		charge := 0
		full := false
		appendRow := func(row SearchRow) bool {
			row.Text, row.Truncated = cleanSearch(row.Text)
			raw, _ := json.Marshal(row)
			if full || len(index.Rows) >= MaxSearchRows || charge+len(raw) > MaxSearchBytes {
				full = true
				return false
			}
			charge += len(raw)
			index.Rows = append(index.Rows, row)
			return true
		}
		for _, v := range m.Sources {
			coverage := SearchCoverage{ID: v.ID, Kind: "source"}
			if !available[v.ID] {
				coverage.Reason = "original_missing_or_changed"
				index.Coverage = append(index.Coverage, coverage)
				continue
			}
			if full {
				coverage.Reason = "index_budget"
				index.Coverage = append(index.Coverage, coverage)
				continue
			}
			page, e := s.sourcePage(ctx, m.ID, v.ID, DecodeOptions{}, 0, 1)
			if e != nil {
				if ctx.Err() != nil {
					return out, ctx.Err()
				}
				coverage.Reason = "text_decoder_unavailable"
				index.Coverage = append(index.Coverage, coverage)
				continue
			}
			coverage.Total, _ = strconv.Atoi(page.TotalLines)
			for i, line := range s.page.positions {
				if e = ctx.Err(); e != nil {
					return out, e
				}
				coordinate := Line{Number: strconv.Itoa(i + 1), RawStart: strconv.FormatUint(uint64(line.rawStart), 10), RawEnd: strconv.FormatUint(uint64(line.rawEnd), 10), TextStart: strconv.FormatUint(uint64(line.textStart), 10), TextEnd: strconv.FormatUint(uint64(line.textEnd), 10)}
				if !appendRow(SearchRow{ID: v.ID + ":" + coordinate.Number, Kind: "source", SourceID: v.ID, SHA256: v.SHA256, Field: "text", Text: s.page.text[line.textStart:line.textEnd], Line: &coordinate}) {
					break
				}
				coverage.Indexed++
			}
			if coverage.Indexed < coverage.Total {
				coverage.Reason = "indexed_prefix_only"
			}
			index.Coverage = append(index.Coverage, coverage)
		}
		for _, r := range m.Results {
			coverage := SearchCoverage{ID: r.ID, Kind: "result"}
			raw, e := s.read(path.Join("cases", m.ID, "objects", r.SHA256), MaxObjectBytes)
			if e != nil || hash(raw) != r.SHA256 {
				coverage.Reason = "snapshot_missing_or_changed"
				index.Coverage = append(index.Coverage, coverage)
				continue
			}
			result, e := resultcontract.Decode(raw)
			if e != nil {
				coverage.Reason = "snapshot_decode_unavailable"
				index.Coverage = append(index.Coverage, coverage)
				continue
			}
			// Read-only search of saved scalar fields; typed profile details stay in the
			// canonical result query and carry their own selection and source references.
			fields := sortedSearchKeys(result.Summary)
			coverage.Total = len(fields)
			for _, field := range fields {
				value, _ := json.Marshal(result.Summary[field])
				if !appendRow(SearchRow{ID: r.ID + ":" + field, Kind: "result", ResultID: r.ID, SHA256: r.SHA256, Field: "/summary/" + field, Text: string(value)}) {
					break
				}
				coverage.Indexed++
			}

			if core, readErr := analyzer.ReadCore(result); readErr == nil {
				frames := map[string]string{}
				for _, frame := range core.Model.Frames {
					frames[frame.ID] = frame.Display
				}
				visitErr := profilemodel.VisitEvidence(ctx, core.Model, core.Selection, func(profile profilemodel.Profile, sample profilemodel.Sample, path []string, timestamp *string) error {
					coverage.Total++
					if full {
						return nil
					}
					sourceID := ""
					for _, id := range r.SourceIDs {
						for _, sha := range core.Model.SourceHashes {
							if r.SourceHashes[id] == sha {
								if sourceID != "" {
									sourceID = ""
									goto bound
								}
								sourceID = id
							}
						}
					}
				bound:
					names := []string{}
					nameBytes := 0
					for _, id := range path {
						if nameBytes < 4096 {
							name := frames[id]
							if len(name) > 512 {
								name = name[:512]
								for !utf8.ValidString(name) {
									name = name[:len(name)-1]
								}
							}
							names = append(names, name)
							nameBytes += len(name)
						}
					}
					label := []any{profile.ID, profile.Process, profile.Thread, profile.Worker, names, sample.SourceRef, sample.StackID, sample.MetricID, sample.Weight, sample.Labels}
					text, _ := json.Marshal(label)
					if appendRow(SearchRow{ID: r.ID + ":observation:" + strconv.Itoa(coverage.Total), Kind: "profile", SourceID: sourceID, ResultID: r.ID, SHA256: r.SHA256, Field: "profile.observation", Text: string(text), Timestamp: timestamp, Clock: profile.Clock, TimeUnit: profile.TimeUnit, SourceRef: searchIdentity(sample.SourceRef), ProfileID: searchIdentity(profile.ID), FrameIDs: append([]string(nil), path...)}) {
						coverage.Indexed++
					}
					return nil
				})
				if visitErr != nil {
					if ctx.Err() != nil {
						return out, ctx.Err()
					}
					coverage.Reason = "typed_detail_unavailable"
				}
			}
			if coverage.Indexed < coverage.Total {
				coverage.Reason = "index_budget"
			}
			index.Coverage = append(index.Coverage, coverage)
		}
		if e = ctx.Err(); e != nil {
			return out, e
		}
		encoded, _ := json.Marshal(index)
		index.Checksum = hash(encoded)
		encoded, _ = json.Marshal(index)
		if e = s.atomicContext(ctx, cache, encoded); e != nil {
			return out, e
		}
	}
	out.IndexSHA256 = index.Checksum
	out.Coverage = index.Coverage
	query := q
	query.Cursor = ""
	encoded, _ := json.Marshal(query)
	out.QuerySHA256 = hash(encoded)
	offset := 0
	if q.Cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(q.Cursor)
		var cursor searchCursor
		if e != nil || json.Unmarshal(b, &cursor) != nil || cursor.Key != key || cursor.Query != out.QuerySHA256 || cursor.Index != out.IndexSHA256 || cursor.Offset < 0 {
			return out, problem("CASE_CURSOR_STALE", "case, query, source or index changed; restart search")
		}
		offset = cursor.Offset
	}
	text := strings.ToLower(q.Text)
	for _, row := range index.Rows {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		if q.SourceID != "" && q.SourceID != row.SourceID || q.ResultID != "" && q.ResultID != row.ResultID || q.Field != "" && q.Field != row.Field || !strings.Contains(strings.ToLower(row.Text+" "+row.ID+" "+row.SHA256+" "+row.ProfileID+" "+strings.Join(row.FrameIDs, " ")), text) {
			continue
		}
		if from != nil || to != nil {
			if row.Timestamp == nil {
				out.Untimed++
				continue
			}
			if row.Clock != q.Clock || row.TimeUnit != q.TimeUnit {
				continue
			}
			at, err := profilemodel.Number(*row.Timestamp)
			if err != nil {
				return out, problem("CASE_INDEX_INVALID", "rebuild invalid typed timestamp index")
			}
			if from != nil && at.Cmp(from) < 0 || to != nil && at.Cmp(to) >= 0 {
				continue
			}
		}
		if out.Matched >= offset && len(out.Rows) < q.Limit {
			row.Text, _ = cleanSearch(row.Text)
			out.Rows = append(out.Rows, row)
		}
		out.Matched++
	}
	if offset > out.Matched {
		return out, problem("CASE_CURSOR_STALE", "search cursor is past current results")
	}
	next := offset + len(out.Rows)
	if next < out.Matched {
		b, _ := json.Marshal(searchCursor{key, out.QuerySHA256, out.IndexSHA256, next})
		out.Next = base64.RawURLEncoding.EncodeToString(b)
	}
	return out, nil
}
func sortedSearchKeys(m map[string]any) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func DecodeQuery(body []byte, out any) error {
	if len(body) > 64<<10 {
		return problem("CASE_QUERY_INVALID", "query exceeds 64 KiB")
	}
	return decodeStrict(body, out)
}

func (s *Store) OpenEvidence(ctx context.Context, caseID, revision, sourceID, digest, line string) (TextPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, e := s.expect(caseID, revision)
	if e != nil {
		return TextPage{}, e
	}
	found := false
	for _, source := range m.Sources {
		if source.ID == sourceID && source.SHA256 == digest {
			found = true
		}
	}
	if !found {
		return TextPage{}, problem("CASE_LINK_STALE", "source coordinate does not match current case")
	}
	n, e := strconv.Atoi(line)
	if e != nil || n < 1 || strconv.Itoa(n) != line {
		return TextPage{}, problem("CASE_LINK_INVALID", "explicit original line number required")
	}
	page, e := s.sourcePage(ctx, caseID, sourceID, DecodeOptions{}, n-1, 10)
	if e != nil {
		return page, e
	}
	indexed, _ := strconv.Atoi(page.IndexedLines)
	if n > indexed {
		return page, problem("CASE_LINK_OUTSIDE_INDEX", "line is outside indexed prefix; no coordinate is invented")
	}
	for i := range page.Lines {
		page.Lines[i].Text = common.RedactText(page.Lines[i].Text).Text
	}
	return page, nil
}

func searchIdentity(value string) string { text, _ := cleanSearch(value); return text }
