package jenniferprofile

import (
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Acceptance #1 — `Total Transaction : 2` file with two TXID blocks
// must split into two profiles cleanly.
func TestParseFile_TwoTransactionBlocks(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if res.DeclaredTransactionCount != 2 {
		t.Errorf("declared count = %d, want 2", res.DeclaredTransactionCount)
	}
	if res.DetectedTransactionCount != 2 {
		t.Errorf("detected count = %d, want 2", res.DetectedTransactionCount)
	}
	if len(res.FileErrors) != 0 {
		t.Errorf("file errors = %v, want none", res.FileErrors)
	}
	if len(res.Profiles) != 2 {
		t.Fatalf("profiles = %d, want 2", len(res.Profiles))
	}
}

// Acceptance #2 — TXID/GUID/APPLICATION/RESPONSE_TIME/SQL_TIME/
// FETCH_TIME/EXTERNAL_CALL_TIME parse correctly from the 2-column
// header block.
func TestParseFile_HeaderFieldsExtracted(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	p0 := res.Profiles[0]
	if got := p0.Header.TXID; got != "922846047875413046" {
		t.Errorf("TXID = %q", got)
	}
	if got := p0.Header.GUID; got != "2026050908316771PCMONLxtz8x0000001" {
		t.Errorf("GUID = %q", got)
	}
	if got := p0.Header.Application; got != "/prod/common/rule/testrule" {
		t.Errorf("APPLICATION = %q", got)
	}
	if p0.Header.ResponseTimeMs == nil || *p0.Header.ResponseTimeMs != 84 {
		t.Errorf("RESPONSE_TIME = %v", p0.Header.ResponseTimeMs)
	}
	if p0.Header.SQLTimeMs == nil || *p0.Header.SQLTimeMs != 57 {
		t.Errorf("SQL_TIME = %v", p0.Header.SQLTimeMs)
	}
	if p0.Header.FetchTimeMs == nil || *p0.Header.FetchTimeMs != 20 {
		t.Errorf("FETCH_TIME = %v", p0.Header.FetchTimeMs)
	}
	if p0.Header.ExternalCallMs == nil || *p0.Header.ExternalCallMs != 173 {
		t.Errorf("EXTERNAL_CALL_TIME = %v", p0.Header.ExternalCallMs)
	}
	if got := p0.Header.Domain; got != "상품" {
		t.Errorf("DOMAIN label = %q, want '상품'", got)
	}
	if got := p0.Header.DomainID; got != "11181" {
		t.Errorf("DOMAIN ID = %q, want '11181'", got)
	}
}

// Acceptance #3 — body EXTERNAL_CALL events are extracted as a slice
// with elapsed and url filled.
func TestParseFile_ExternalCallExtracted(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	p0 := res.Profiles[0]
	var ext *models.JenniferProfileEvent
	for i := range p0.Body.Events {
		if p0.Body.Events[i].EventType == models.JenniferEventExternalCall {
			ext = &p0.Body.Events[i]
			break
		}
	}
	if ext == nil {
		t.Fatalf("no EXTERNAL_CALL event")
	}
	if ext.ElapsedMs == nil || *ext.ElapsedMs != 173 {
		t.Errorf("elapsed = %v, want 173", ext.ElapsedMs)
	}
	if ext.ExternalURL != "/dev/common/rule/testrule" {
		t.Errorf("url = %q", ext.ExternalURL)
	}
	if ext.ExternalProtocol != "HTTP" {
		t.Errorf("protocol = %q", ext.ExternalProtocol)
	}
	if ext.ExternalClient != "APACHE_HTTP_CLIENT_V5" {
		t.Errorf("client = %q", ext.ExternalClient)
	}
}

// Acceptance #5 — FETCH `[현재/누적] [시간]` parses currentRows /
// cumulativeRows / elapsed.
func TestParseFile_FetchParsing(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	p0 := res.Profiles[0]
	var fetch *models.JenniferProfileEvent
	for i := range p0.Body.Events {
		if p0.Body.Events[i].EventType == models.JenniferEventFetch {
			fetch = &p0.Body.Events[i]
			break
		}
	}
	if fetch == nil {
		t.Fatalf("no FETCH event")
	}
	if fetch.CurrentFetchRows == nil || *fetch.CurrentFetchRows != 1 {
		t.Errorf("currentFetchRows = %v", fetch.CurrentFetchRows)
	}
	if fetch.CumulativeFetchRows == nil || *fetch.CumulativeFetchRows != 1 {
		t.Errorf("cumulativeFetchRows = %v", fetch.CumulativeFetchRows)
	}
	if fetch.ElapsedMs == nil || *fetch.ElapsedMs != 20 {
		t.Errorf("elapsed = %v", fetch.ElapsedMs)
	}
}

// Acceptance #6 — `select 1 from dual` is classified as CHECK_QUERY.
func TestParseFile_CheckQueryClassified(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	p0 := res.Profiles[0]
	hasCheckQuery := false
	for _, ev := range p0.Body.Events {
		if ev.EventType == models.JenniferEventCheckQuery {
			hasCheckQuery = true
			break
		}
	}
	if !hasCheckQuery {
		t.Errorf("no CHECK_QUERY classified")
	}
}

// Acceptance #7 — JAVA_XA.xa_* SQL classifies as TWO_PC_*.
func TestParseFile_TwoPCClassified(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	p0 := res.Profiles[0]
	xaStart, xaEnd, xaPrepare := 0, 0, 0
	for _, ev := range p0.Body.Events {
		switch ev.EventType {
		case models.JenniferEventTwoPCStart:
			xaStart++
		case models.JenniferEventTwoPCEnd:
			xaEnd++
		case models.JenniferEventTwoPCPrepare:
			xaPrepare++
		}
	}
	if xaStart != 1 || xaEnd != 1 || xaPrepare != 1 {
		t.Errorf("xa start/end/prepare = %d/%d/%d (want 1/1/1)", xaStart, xaEnd, xaPrepare)
	}
}

// Acceptance #2 (file-level) — TRANSACTION_COUNT_MISMATCH fires when
// `Total Transaction : 4` is declared but only 1 TXID block exists.
func TestParseFile_CountMismatch(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_count_mismatch.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	found := false
	for _, e := range res.FileErrors {
		if e.Code == "TRANSACTION_COUNT_MISMATCH" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TRANSACTION_COUNT_MISMATCH file error, got %v", res.FileErrors)
	}
}

// FULL Profile validation — sample profile #2 has only START + one
// METHOD + END so it should still pass FULL since it has body
// header / start / end / total. Profile #1 has the full shape too;
// neither should accumulate MISSING_* errors.
func TestParseFile_FullProfile(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for i, p := range res.Profiles {
		if len(p.Errors) != 0 {
			t.Errorf("profile %d errors = %v (want none)", i, p.Errors)
		}
		if !p.Body.HasStart || !p.Body.HasEnd || !p.Body.HasTotal || !p.Body.HasBodyHeader {
			t.Errorf("profile %d body flags = start=%t end=%t total=%t hdr=%t",
				i, p.Body.HasStart, p.Body.HasEnd, p.Body.HasTotal, p.Body.HasBodyHeader)
		}
	}
}

// SOCEKT typo + SOCKET both classify as SOCKET (both ostream and
// istream). Sample includes two SOCEKT lines.
func TestParseFile_SocketTypoSupport(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	p0 := res.Profiles[0]
	socketCount := 0
	for _, ev := range p0.Body.Events {
		if ev.EventType == models.JenniferEventSocket {
			socketCount++
		}
	}
	if socketCount != 2 {
		t.Errorf("socket events = %d, want 2", socketCount)
	}
}

// Body offset calculation — first SQL event at 16:10:54 086 minus
// body START 16:10:52 608 should yield 1478ms.
func TestParseFile_OffsetCalculation(t *testing.T) {
	res, err := ParseFile(filepath.Join("testdata", "sample_two_tx.txt"), Options{})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	p0 := res.Profiles[0]
	var sqlEvent *models.JenniferProfileEvent
	for i := range p0.Body.Events {
		ev := &p0.Body.Events[i]
		if ev.EventType == models.JenniferEventCheckQuery {
			sqlEvent = ev
			break
		}
	}
	if sqlEvent == nil {
		t.Fatalf("no check-query event")
	}
	if sqlEvent.StartOffsetMs == nil {
		t.Fatalf("startOffsetMs is nil")
	}
	if got := *sqlEvent.StartOffsetMs; got != 1478 {
		t.Errorf("startOffsetMs = %d, want 1478", got)
	}
}
