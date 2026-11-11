package cases

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func mustStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "store")
	s, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s, root
}
func mustCreate(t *testing.T, s *Store) Manifest {
	t.Helper()
	m, e := s.Create("synthetic incident")
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func apply(t *testing.T, s *Store, m Manifest, edit Edit) Manifest {
	t.Helper()
	edit.CaseID = m.ID
	edit.ExpectedRevision = m.Revision
	next, e := s.Apply(context.Background(), edit)
	if e != nil {
		t.Fatal(edit.Operation, e)
	}
	return next
}
func file(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
	return p
}
func resultBody(t *testing.T) []byte {
	t.Helper()
	r := models.New("future-evidence", "synthetic")
	r.Summary["unmeasured"] = nil
	r.Summary["zero"] = json.Number("0")
	r.Metadata.Extra["unknown_future"] = json.RawMessage(`{"large":9007199254740993,"precise":0.123456789012345678901234567890}`)
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestT630EncodingFixturesPositionsAndDistinctLegacyLabels(t *testing.T) {
	raw, _ := Contracts.ReadFile("contracts/evidence-text-v1.json")
	var fixtures struct {
		Cases []struct {
			ID, Encoding, Hex, Text, Replacements string
			AllowReplacement                      bool `json:"allow_replacement"`
			Lines                                 [][]string
		}
	}
	if e := json.Unmarshal(raw, &fixtures); e != nil {
		t.Fatal(e)
	}
	for _, f := range fixtures.Cases {
		t.Run(f.ID, func(t *testing.T) {
			b, _ := hex.DecodeString(f.Hex)
			d, e := DecodeText(b, DecodeOptions{Encoding: f.Encoding, AllowReplacement: f.AllowReplacement})
			if e != nil || d.Text != f.Text || len(d.Lines) != len(f.Lines) {
				t.Fatal(d, e)
			}
			for i, line := range d.Lines {
				if line.RawStart != f.Lines[i][0] || line.RawEnd != f.Lines[i][1] {
					t.Fatal(d.Lines, f.Lines)
				}
			}
			if d.Encoding.Replacements != f.Replacements && !(f.Replacements == "" && d.Encoding.Replacements == "0") {
				t.Fatal(d.Encoding)
			}
			page, e := SourcePage(d, 0, 200)
			if e != nil || len(page.Lines) != len(f.Lines) {
				t.Fatal(page, e)
			}
		})
	}
	for _, test := range []struct {
		raw  []byte
		opts DecodeOptions
		code string
	}{
		{[]byte{0x81, 0x41}, DecodeOptions{Encoding: "euc-kr"}, "ENCODING_INVALID"},
		{[]byte{0xb0, 0xa1}, DecodeOptions{Sniff: []string{"cp949", "euc-kr"}}, "ENCODING_SELECTION_REQUIRED"},
		{[]byte{0xff, 0xfe, 0x00, 0x00}, DecodeOptions{Encoding: "utf-8"}, "ENCODING_BOM_CONFLICT"},
		{[]byte{0xff}, DecodeOptions{}, "ENCODING_SELECTION_REQUIRED"},
	} {
		_, e := DecodeText(test.raw, test.opts)
		var pe *Error
		if !errors.As(e, &pe) || pe.Code != test.code {
			t.Fatal(e, test.code)
		}
	}
	d, e := DecodeText([]byte{0x81, 0x41}, DecodeOptions{Sniff: []string{"cp949", "euc-kr"}})
	if e != nil || d.Text != "갂" || d.Encoding.Basis != "permitted-sniff" {
		t.Fatal(d, e)
	}
	a, _ := DecodeText([]byte("a\r\nb\rc\n"), DecodeOptions{})
	if len(a.Lines) != 3 || a.Lines[1].RawEnd != "5" {
		t.Fatal(a.Lines)
	}
	long, _ := DecodeText([]byte(strings.Repeat("가", 3000)), DecodeOptions{})
	page, e := SourcePage(long, 0, 1)
	if e != nil || !page.Lines[0].Truncated || len(page.Lines[0].Text) > 4096 {
		t.Fatal(page, e)
	}
}
func sampleCapture(t *testing.T) Capture {
	key, _ := id("capture")
	v := "0"
	return Capture{SchemaVersion: Version, ID: key, Producer: "synthetic", ProducerVersion: "1", IdentityBasis: "unknown", Clock: Clock{Domain: "relative", Start: &Claim{Value: &v, Basis: "observed"}}, Devices: []Device{{ID: "gpu0", Vendor: "Apple", Kind: "gpu", MemoryModel: "unified", MappingBasis: "observed", Properties: map[string]Claim{}}}, Options: map[string]json.RawMessage{"sample_interval": json.RawMessage(`null`)}}
}
func TestT630CaptureSchemaBasisNeutralDevicesAndConflicts(t *testing.T) {
	capture := sampleCapture(t)
	if e := capture.Validate(); e != nil {
		t.Fatal(e)
	}
	schema, _ := Contracts.ReadFile("contracts/capture-context.schema.json")
	var document any
	json.Unmarshal(schema, &document)
	compiler := jsonschema.NewCompiler()
	if e := compiler.AddResource("urn:test:capture", document); e != nil {
		t.Fatal(e)
	}
	compiled, e := compiler.Compile("urn:test:capture")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(capture)
	var value any
	json.Unmarshal(body, &value)
	if e = compiled.Validate(value); e != nil {
		t.Fatal(e)
	}
	merged, e := MergeCaptureContext(capture, map[string]string{"start": "1"})
	if e != nil || len(merged.Warnings) != 1 || *merged.Clock.Start.Value != "0" || merged.Clock.AnchorUTC != nil {
		t.Fatal(merged, e)
	}
	capture.Options["command"] = json.RawMessage(`"tool --token=private"`)
	if e = capture.Validate(); e == nil {
		t.Fatal("full command accepted")
	}
	delete(capture.Options, "command")
	capture.Clock.Start.Basis = "unknown"
	if e = capture.Validate(); e == nil {
		t.Fatal("known value labelled unknown")
	}
	capture = sampleCapture(t)
	merged, e = MergeCaptureContext(capture, map[string]string{"device.gpu0.vendor": "Other", "clock.domain": "wall"})
	if e != nil || len(merged.Warnings) != 2 || merged.Devices[0].Vendor != "Apple" {
		t.Fatal(merged, e)
	}
	capture = sampleCapture(t)
	capture.Devices[0].ParentID = "gpu0"
	if e = capture.Validate(); e == nil {
		t.Fatal("device cycle")
	}
	redacted, e := RedactJSONStrings(json.RawMessage(`{"secret":"abc","integer":9007199254740993,"decimal":0.1234567890123456789012345,"zero":0,"none":null}`), func(string) string { return "masked" })
	if e != nil || !bytes.Contains(redacted, []byte("9007199254740993")) || !bytes.Contains(redacted, []byte("0.1234567890123456789012345")) || !bytes.Contains(redacted, []byte(`"none":null`)) {
		t.Fatal(string(redacted), e)
	}
}
func TestT630DurableRevisionBaselineStalenessMissingOriginalAndLifecycle(t *testing.T) {
	s, root := mustStore(t)
	m := mustCreate(t, s)
	external := file(t, "original.log", []byte("가\r\n둘\n"))
	m = apply(t, s, m, Edit{Operation: "source", Path: external, Decode: DecodeOptions{Encoding: "utf-8"}})
	sourceID := m.Sources[0].ID
	body := resultBody(t)
	saved, e := s.SaveResult(SaveResult{CaseID: m.ID, ExpectedRevision: m.Revision, SourceIDs: []string{sourceID}, Selection: json.RawMessage(`{"zero":0,"unknown":null}`)}, body)
	if e != nil {
		t.Fatal(e)
	}
	m = saved
	rID := m.Results[0].ID
	m = apply(t, s, m, Edit{Operation: "baseline", ResultID: rID, Conditions: json.RawMessage(`{"warmup":null}`)})
	m = apply(t, s, m, Edit{Operation: "artifact", ResultID: rID, Spec: json.RawMessage(`{"format":"svg"}`)})
	oldRevision := m.Revision
	m = apply(t, s, m, Edit{Operation: "encoding", SourceID: sourceID, Decode: DecodeOptions{Encoding: "cp949", AllowReplacement: true}})
	if !m.Results[0].Stale || !m.Artifacts[0].Stale {
		t.Fatal("dependent references not stale")
	}
	if _, e = s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: oldRevision, Operation: "details"}); e == nil {
		t.Fatal("stale edit accepted")
	}
	if e = os.Remove(external); e != nil {
		t.Fatal(e)
	}
	read, e := s.Get(m.ID)
	if e != nil || read.Baselines[0].OriginalAvailable || read.Results[0].Availability != "snapshot-only" {
		t.Fatal(read, e)
	}
	out, e := s.ExportSnapshot(m.ID, rID)
	if e != nil || !bytes.Equal(body, out) {
		t.Fatal("immutable snapshot changed", e)
	}
	m = apply(t, s, m, Edit{Operation: "archive"})
	if _, e = s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "details"}); e == nil {
		t.Fatal("archived case editable")
	}
	m = apply(t, s, m, Edit{Operation: "restore"})
	if m.State != "restored" {
		t.Fatal(m.State)
	}
	s.Close()
	s2, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	read, e = s2.Get(m.ID)
	if e != nil || read.Revision != m.Revision || read.RetentionDays != nil {
		t.Fatal(read, e)
	}
	if _, e = s2.RebuildIndex(); e != nil {
		t.Fatal(e)
	}
	if e = s2.CleanupCache(); e != nil {
		t.Fatal(e)
	}
	out, e = s2.ExportSnapshot(m.ID, rID)
	if e != nil || !bytes.Equal(out, body) {
		t.Fatal("cache cleanup removed authoritative snapshot", e)
	}
	m = apply(t, s2, m, Edit{Operation: "delete"})
	if _, e = s2.ExportSnapshot(m.ID, rID); e == nil {
		t.Fatal("deleted case readable")
	}
	m = apply(t, s2, m, Edit{Operation: "restore"})
	m = apply(t, s2, m, Edit{Operation: "delete"})
	p, e := s2.DeletionPlan(m.ID)
	if e != nil || p.PhysicalErasureGuaranteed {
		t.Fatal(p, e)
	}
	if e = s2.Purge(m.ID, m.Revision, "wrong"); e == nil {
		t.Fatal("purge without scope")
	}
	if e = s2.Purge(m.ID, m.Revision, p.Token); e != nil {
		t.Fatal(e)
	}
	if _, e = s2.Get(m.ID); e == nil {
		t.Fatal("purged manifest retained")
	}
}
func TestT630AtomicRecoveryTamperAndExternalDeletionBoundary(t *testing.T) {
	s, root := mustStore(t)
	m := mustCreate(t, s)
	external := file(t, "external.log", []byte("original"))
	m = apply(t, s, m, Edit{Operation: "source", Path: external, Copy: true})
	source := m.Sources[0]
	headBytes, _ := os.ReadFile(filepath.Join(root, "cases", m.ID, "HEAD.json"))
	// Crash-left next journal entry is uncommitted; HEAD remains authoritative.
	os.WriteFile(filepath.Join(root, "cases", m.ID, "revisions", "3.json"), []byte("incomplete"), 0600)
	os.WriteFile(filepath.Join(root, "staging", "interrupted"), []byte("temp"), 0600)
	s.Close()
	recovered, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer recovered.Close()
	got, e := recovered.Get(m.ID)
	if e != nil || got.Revision != "2" {
		t.Fatal(got, e)
	}
	got = apply(t, recovered, got, Edit{Operation: "details", Title: "recovered"})
	if got.Revision != "3" {
		t.Fatal(got.Revision)
	}
	if _, e = os.Stat(filepath.Join(root, "staging", "interrupted")); !os.IsNotExist(e) {
		t.Fatal("staging not recovered")
	}
	os.WriteFile(filepath.Join(root, "cases", m.ID, "objects", source.SHA256), []byte("tampered"), 0600)
	if _, e = recovered.Object(m.ID, source.SHA256); e == nil {
		t.Fatal("tampered object accepted")
	}
	// Logical delete can be performed with corrupt evidence: it does not need to
	// rewrite any evidence blobs. Restore/purge still never touch external files.
	os.WriteFile(filepath.Join(root, "cases", m.ID, "objects", source.SHA256), []byte("original"), 0600)
	got = apply(t, recovered, got, Edit{Operation: "delete"})
	p, _ := recovered.DeletionPlan(got.ID)
	if e = recovered.Purge(got.ID, got.Revision, p.Token); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(external)
	if string(raw) != "original" {
		t.Fatal("external original changed")
	}
	if len(headBytes) == 0 {
		t.Fatal("missing commit marker")
	}
}
func makeZIP(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	for k, b := range entries {
		f, e := w.Create(k)
		if e != nil {
			t.Fatal(e)
		}
		f.Write(b)
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return out.Bytes()
}
func TestT630SafeArchiveIntakePartialAndExplicitCaptureProvenance(t *testing.T) {
	raw := makeZIP(t, map[string][]byte{"host-a/server.log": []byte("same"), "host-b/server.log": []byte("same"), "host-c/bad.log": []byte("wrong")})
	p := file(t, "sources.zip", raw)
	preview, e := PreviewArchive(context.Background(), p)
	if e != nil || len(preview.Entries) != 3 {
		t.Fatal(preview, e)
	}
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	m = apply(t, s, m, Edit{Operation: "archive-intake", Path: p, ArchiveSHA256: preview.SHA256, Entries: []string{"host-a/server.log", "host-b/server.log", "host-c/bad.log"}, ExpectedHashes: map[string]string{"host-c/bad.log": hash([]byte("expected"))}})
	if len(m.Sources) != 3 {
		t.Fatal(m.Sources)
	}
	failed := 0
	ids := map[string]bool{}
	for _, v := range m.Sources {
		ids[v.ID] = true
		if v.Status == "failed" {
			failed++
		}
	}
	if len(ids) != 3 || failed != 1 {
		t.Fatal(m.Sources)
	}
	if _, e = s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "archive-intake", Path: p, ArchiveSHA256: hash([]byte("stale")), Entries: []string{"host-a/server.log"}}); e == nil {
		t.Fatal("stale preview accepted")
	}
}
func TestT630ArchiveSecurityFailuresAreAtomic(t *testing.T) {
	for _, entries := range []map[string][]byte{{"../escape": []byte("x")}, {"A.log": []byte("a"), "a.log": []byte("b")}, {"e\u0301.log": []byte("a"), "é.log": []byte("b")}, {"x": []byte("a"), "x/a": []byte("b")}, {"C:/escape": []byte("x")}, {"CON.txt": []byte("x")}, {"inner.zip": []byte("nested")}, {"x.log": []byte{0x1f, 0x8b, 0, 0}}} {
		p := file(t, "bad.zip", makeZIP(t, entries))
		if _, e := PreviewArchive(context.Background(), p); e == nil {
			t.Fatal("unsafe archive accepted", entries)
		}
	}
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeLink, Linkname: "outside"})
	tw.Close()
	if _, e := PreviewArchive(context.Background(), file(t, "link.tar", out.Bytes())); e == nil {
		t.Fatal("hard link accepted")
	}
	out.Reset()
	zw := zip.NewWriter(&out)
	h := &zip.FileHeader{Name: "symlink"}
	h.SetMode(os.ModeSymlink | 0777)
	w, _ := zw.CreateHeader(h)
	w.Write([]byte("outside"))
	zw.Close()
	if _, e := PreviewArchive(context.Background(), file(t, "link.zip", out.Bytes())); e == nil {
		t.Fatal("ZIP symlink accepted")
	}
	many := map[string][]byte{}
	for i := 0; i < MaxEntries+1; i++ {
		many[currentRevision(uint64(i))] = nil
	}
	if _, e := PreviewArchive(context.Background(), file(t, "many.zip", makeZIP(t, many))); e == nil {
		t.Fatal("entry budget ignored")
	}
	bomb := makeZIP(t, map[string][]byte{"large.log": bytes.Repeat([]byte{'x'}, MaxObjectBytes+1)})
	if _, e := PreviewArchive(context.Background(), file(t, "bomb.zip", bomb)); e == nil {
		t.Fatal("expanded entry budget ignored")
	}
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	p := file(t, "atomic.zip", makeZIP(t, map[string][]byte{"safe.log": []byte("x"), "../bad": []byte("x")}))
	_, e := s.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "archive-intake", Path: p, ArchiveSHA256: hash(mustRead(t, p)), Entries: []string{"safe.log"}})
	if e == nil {
		t.Fatal("structural failure accepted")
	}
	got, _ := s.Get(m.ID)
	if len(got.Sources) != 0 || got.Revision != m.Revision {
		t.Fatal("partial structural commit")
	}
}
func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestT630PortableReviewRestoreHashesAndSanitizedCopy(t *testing.T) {
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	original := []byte("Authorization: Bearer synthetic-secret\nmail@example.invalid\n")
	p := file(t, "redact.log", original)
	m = apply(t, s, m, Edit{Operation: "source", Path: p, Copy: true, Sanitize: true, Decode: DecodeOptions{Encoding: "utf-8"}})
	if m.Sources[0].ParentSHA256 != hash(original) || !m.Sources[0].Redaction.Applied {
		t.Fatal(m.Sources)
	}
	body, _ := s.Object(m.ID, m.Sources[0].SHA256)
	if bytes.Contains(body, []byte("synthetic-secret")) || !bytes.Equal(mustRead(t, p), original) {
		t.Fatal("redaction affected original or failed")
	}
	raw := resultBody(t)
	m, e := s.SaveResult(SaveResult{CaseID: m.ID, ExpectedRevision: m.Revision, SourceIDs: []string{m.Sources[0].ID}}, raw)
	if e != nil {
		t.Fatal(e)
	}
	opts := ExportOptions{ManifestReviewed: true, SourceIDs: []string{m.Sources[0].ID}, ResultIDs: []string{m.Results[0].ID}}
	if _, e = s.Portable(context.Background(), m.ID, opts); e == nil {
		t.Fatal("unreviewed sharing allowed")
	}
	m = apply(t, s, m, Edit{Operation: "review-source", SourceID: m.Sources[0].ID, SharingReviewed: true})
	m = apply(t, s, m, Edit{Operation: "review-result", ResultID: m.Results[0].ID, SharingReviewed: true})
	m = apply(t, s, m, Edit{Operation: "baseline", ResultID: m.Results[0].ID})
	pack, e := s.Portable(context.Background(), m.ID, opts)
	if e != nil {
		t.Fatal(e)
	}
	p = file(t, "portable.zip", pack)
	restored, e := s.Restore(context.Background(), p, hash(pack))
	if e != nil || restored.ID == m.ID || restored.State != "restored" || restored.Sources[0].ExternalPath != "" || restored.Results[0].SharingReviewed {
		t.Fatal(restored, e)
	}
	out, e := s.ExportSnapshot(restored.ID, restored.Results[0].ID)
	if e != nil || !bytes.Equal(out, raw) {
		t.Fatal("portable result precision lost", e)
	}
	omitted, e := s.Portable(context.Background(), m.ID, ExportOptions{ManifestReviewed: true, ResultIDs: opts.ResultIDs})
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Restore(context.Background(), file(t, "snapshot.zip", omitted), hash(omitted))
	if e != nil {
		t.Fatal(e)
	}
	got, _ := s.Get(r.ID)
	if got.Sources[0].Storage != "omitted" || got.Baselines[0].OriginalAvailable {
		t.Fatal(got)
	}
	// A valid ZIP CRC cannot conceal a digest mismatch in an object entry.
	zr, _ := zip.NewReader(bytes.NewReader(pack), int64(len(pack)))
	contents := map[string][]byte{}
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := readBounded(r, MaxObjectBytes)
		r.Close()
		if strings.HasPrefix(f.Name, "objects/") {
			b = []byte("changed")
		}
		contents[f.Name] = b
	}
	bad := makeZIP(t, contents)
	if _, e = s.Restore(context.Background(), file(t, "tampered.zip", bad), hash(bad)); e == nil {
		t.Fatal("tampered portable archive restored")
	}
}
func TestT630AssetBundleIdentityAndExclusiveOwnership(t *testing.T) {
	raw, _ := Contracts.ReadFile("contracts/manifest.json")
	var manifest struct{ SHA256 map[string]string }
	json.Unmarshal(raw, &manifest)
	for name, digest := range manifest.SHA256 {
		body, e := Contracts.ReadFile("contracts/" + filepath.Base(name))
		if e != nil || hash(body) != digest {
			t.Fatal(name, e)
		}
		shared := filepath.Join("../../../../../projects-assets", name)
		if body2, e := os.ReadFile(shared); e == nil && !bytes.Equal(body, body2) {
			t.Fatal("shared bundle drift", name)
		}
	}
	s, root := mustStore(t)
	if _, e := Open(root); e == nil {
		t.Fatal("concurrent writer acquired store")
	}
	m := mustCreate(t, s)
	cancel, c := context.WithCancel(context.Background())
	c()
	if _, e := s.Apply(cancel, Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "details"}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestT630CaptureFileHashGroupsAndCorruptObjectDeletionAtRetentionLimit(t *testing.T) {
	s, root := mustStore(t)
	m := mustCreate(t, s)
	capture := sampleCapture(t)
	capture.Files = []CaptureFile{{Name: "host-a/same.log", SHA256: hash([]byte("correct")), Size: "7", EntityIDs: []string{"host-a"}}, {Name: "host-b/same.log", SHA256: hash([]byte("correct")), Size: "7", EntityIDs: []string{"host-b"}}}
	capture.Entities = []Entity{{ID: "host-a"}, {ID: "host-b"}}
	m = apply(t, s, m, Edit{Operation: "capture", Capture: &capture})
	archive := makeZIP(t, map[string][]byte{"host-a/same.log": []byte("correct"), "host-b/same.log": []byte("wrong")})
	p := file(t, "contexts.zip", archive)
	m = apply(t, s, m, Edit{Operation: "archive-intake", CaptureID: capture.ID, Path: p, ArchiveSHA256: hash(archive), Entries: []string{"host-a/same.log", "host-b/same.log"}})
	for _, src := range m.Sources {
		if src.Name == "host-b/same.log" && src.Status != "failed" {
			t.Fatal("capture hash mismatch ignored")
		}
	}
	// Simulate a store at its logical byte/revision ceiling without allocating
	// physical gigabytes. Emergency deletion must remain possible after reopening.
	m.Revision = strconv.Itoa(MaxRevisions)
	if e := s.commit(m, nil); e != nil {
		t.Fatal(e)
	}
	sparse := filepath.Join(root, "cases", m.ID, "objects", strings.Repeat("0", 64))
	f, e := os.Create(sparse)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Truncate(MaxStoreBytes); e != nil {
		t.Fatal(e)
	}
	f.Close()
	s.Close()
	reopened, e := Open(root)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if _, e = reopened.Apply(context.Background(), Edit{CaseID: m.ID, ExpectedRevision: m.Revision, Operation: "details"}); e == nil {
		t.Fatal("ordinary revision exceeded retention budget")
	}
	m = apply(t, reopened, m, Edit{Operation: "delete"})
	plan, _ := reopened.DeletionPlan(m.ID)
	if e = reopened.Purge(m.ID, m.Revision, plan.Token); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(sparse); !os.IsNotExist(e) {
		t.Fatal("purge did not reclaim managed sparse object")
	}
}
func TestT630TarReadCRCAndFutureVersionsRefuseWithoutPublication(t *testing.T) {
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	tw.WriteHeader(&tar.Header{Name: "host/data.log", Typeflag: tar.TypeReg, Size: 4, Mode: 0600})
	tw.Write([]byte("text"))
	tw.Close()
	preview, e := PreviewArchive(context.Background(), file(t, "data.tar", out.Bytes()))
	if e != nil || len(preview.Entries) != 1 || preview.Entries[0].SHA256 != hash([]byte("text")) {
		t.Fatal(preview, e)
	}
	data := makeZIP(t, map[string][]byte{"data.log": []byte("payload")})
	zr, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	offset, _ := zr.File[0].DataOffset()
	data[offset] ^= 0xff
	if _, e = PreviewArchive(context.Background(), file(t, "crc.zip", data)); e == nil {
		t.Fatal("corrupt content/CRC accepted")
	}
	s, _ := mustStore(t)
	m := mustCreate(t, s)
	manifest, _ := json.Marshal(m)
	manifest = bytes.Replace(manifest, []byte(`"schema_version":"1.0.0"`), []byte(`"schema_version":"9.0.0"`), 1)
	pack := makeZIP(t, map[string][]byte{"case.json": manifest})
	p := file(t, "future.zip", pack)
	if _, e = s.Restore(context.Background(), p, hash(pack)); e == nil {
		t.Fatal("future case schema rewritten")
	}
	list, _ := s.List()
	if len(list) != 1 || !bytes.Equal(mustRead(t, p), pack) {
		t.Fatal("failed restore changed source or published case")
	}
}

func TestT630ReadLocalRefusesLinksAndTarTrailingPayload(t *testing.T) {
	original := file(t, "data.log", []byte("data"))
	linked := filepath.Join(t.TempDir(), "link.log")
	if e := os.Symlink(original, linked); e == nil {
		if _, e = readLocal(linked, MaxObjectBytes); e == nil {
			t.Fatal("symbolic source link accepted")
		}
	}
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	tw.WriteHeader(&tar.Header{Name: "data.log", Size: 4, Mode: 0600})
	tw.Write([]byte("data"))
	tw.Close()
	out.Write([]byte("hidden payload"))
	if _, e := PreviewArchive(context.Background(), file(t, "trailing.tar", out.Bytes())); e == nil {
		t.Fatal("TAR trailing payload ignored")
	}
}

func TestT630CancelledAtomicPublicationPreservesHead(t *testing.T) {
	s, root := mustStore(t)
	m := mustCreate(t, s)
	p := filepath.Join("cases", m.ID, "HEAD.json")
	before := mustRead(t, filepath.Join(root, p))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := s.atomicContext(ctx, filepath.ToSlash(p), []byte("not a commit")); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	after := mustRead(t, filepath.Join(root, p))
	if !bytes.Equal(before, after) {
		t.Fatal("cancelled staged write changed HEAD")
	}
	entries, e := os.ReadDir(filepath.Join(root, "cases", m.ID))
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatal("cancelled staging file leaked")
		}
	}
}

func TestT630ExportsCannotOverwriteManagedDataOrExternalOriginal(t *testing.T) {
	s, root := mustStore(t)
	m := mustCreate(t, s)
	original := file(t, "original.log", []byte("immutable input"))
	m = apply(t, s, m, Edit{Operation: "source", Path: original})
	if e := s.CheckExportDestination(m.ID, original); e == nil {
		t.Fatal("export can overwrite original")
	}
	if e := s.CheckExportDestination(m.ID, filepath.Join(root, "cases", m.ID, "HEAD.json")); e == nil {
		t.Fatal("export can overwrite managed HEAD")
	}
	alias := filepath.Join(t.TempDir(), "alias.log")
	if e := os.Link(original, alias); e == nil {
		if e = s.CheckExportDestination(m.ID, alias); e == nil {
			t.Fatal("hard-link alias bypassed original protection")
		}
	}
	if e := s.CheckExportDestination(m.ID, filepath.Join(t.TempDir(), "export.json")); e != nil {
		t.Fatal(e)
	}
	if string(mustRead(t, original)) != "immutable input" {
		t.Fatal("original changed")
	}
}
