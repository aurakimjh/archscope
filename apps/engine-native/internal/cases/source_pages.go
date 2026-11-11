package cases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

// One revision-bound decoded source at most: <=64 MiB UTF-8 + <=16 MiB compact
// offsets. Page hits hash the current source through a bounded streaming buffer;
// they never trust only mtime, allocate the full source, or decode it again.
type compactLine struct{ rawStart, rawEnd, textStart, textEnd uint32 }
type sourceIndex struct {
	key, text string
	positions []compactLine
	total     int
	encoding  Encoding
	sha       string
}

func (s *Store) SourcePage(ctx context.Context, caseID, sourceID string, opts DecodeOptions, start, count int) (TextPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if start < 0 || count < 1 || count > 200 {
		return TextPage{}, problem("CASE_PAGE_INVALID", "source page requires start >=0 and count 1..200")
	}
	m, e := s.get(caseID)
	if e != nil {
		return TextPage{}, e
	}
	if m.State == "deleted" {
		return TextPage{}, problem("CASE_DELETED", "restore the case first")
	}
	var v Source
	found := false
	for _, source := range m.Sources {
		if source.ID == sourceID {
			v = source
			found = true
			break
		}
	}
	if !found || v.Storage == "omitted" {
		return TextPage{}, problem("CASE_SOURCE_MISSING", "source missing or omitted")
	}
	useSaved := opts.Encoding == "" && len(opts.Sniff) == 0 && v.Encoding != nil
	if useSaved {
		opts.Encoding = v.Encoding.Name
		opts.AllowReplacement = v.Encoding.Replacements != "0"
	}
	keyBytes, _ := json.Marshal(struct {
		CaseID, Revision, SourceID, Digest string
		Options                            DecodeOptions
	}{caseID, m.Revision, sourceID, v.SHA256, opts})
	key := hash(keyBytes)
	if s.page != nil && s.page.key == key {
		if e = s.verifyPageSource(ctx, caseID, v); e != nil {
			s.page = nil
			return TextPage{}, e
		}
	} else {
		s.page = nil
		var raw []byte
		if v.Storage == "external" {
			raw, e = readLocal(v.ExternalPath, MaxObjectBytes)
		} else {
			raw, e = s.read(path.Join("cases", caseID, "objects", v.SHA256), MaxObjectBytes)
		}
		if e != nil {
			return TextPage{}, e
		}
		if hash(raw) != v.SHA256 || strconv.Itoa(len(raw)) != v.Size {
			return TextPage{}, problem("CASE_HASH_MISMATCH", "source changed; reimport explicitly")
		}
		name, basis, skip, e := selectTextEncoding(ctx, raw, opts)
		if e != nil {
			return TextPage{}, e
		}
		index := &sourceIndex{key: key, sha: v.SHA256, positions: make([]compactLine, 0, min(MaxIndexedLines, len(raw)))}
		text, replaced, total, e := scanNamed(ctx, raw, name, skip, opts.AllowReplacement, func(_ int, rs, re, ts, te int) {
			if len(index.positions) < MaxIndexedLines {
				index.positions = append(index.positions, compactLine{uint32(rs), uint32(re), uint32(ts), uint32(te)})
			}
		})
		if e != nil {
			return TextPage{}, e
		}
		index.text = text
		index.total = total
		index.encoding = finishEncoding(raw, name, basis, opts.Encoding != "", strconv.FormatUint(replaced, 10))
		if useSaved {
			if index.encoding.IndexRevision != v.Encoding.IndexRevision {
				return TextPage{}, problem("CASE_INDEX_STALE", "decoder changed; rebuild encoding/index explicitly")
			}
			index.encoding = *v.Encoding
		}
		s.page = index
	}
	index := s.page
	page := TextPage{Encoding: index.encoding, SourceSHA256: index.sha, Lines: []SourceLine{}, TotalLines: strconv.Itoa(index.total), IndexedLines: strconv.Itoa(len(index.positions)), IndexTruncated: index.total > len(index.positions)}
	budget := 64 << 10
	for i := start; i < len(index.positions) && i < start+count; i++ {
		line := index.positions[i]
		text := index.text[line.textStart:line.textEnd]
		truncated := false
		if len(text) > min(4096, budget) {
			text = text[:min(4096, budget)]
			for !utf8.ValidString(text) {
				text = text[:len(text)-1]
			}
			truncated = true
		}
		page.Lines = append(page.Lines, SourceLine{Line: Line{Number: strconv.Itoa(i + 1), RawStart: strconv.FormatUint(uint64(line.rawStart), 10), RawEnd: strconv.FormatUint(uint64(line.rawEnd), 10), TextStart: strconv.FormatUint(uint64(line.textStart), 10), TextEnd: strconv.FormatUint(uint64(line.textEnd), 10)}, Text: strings.Clone(text), Truncated: truncated})
		budget -= len(text)
		if budget == 0 {
			break
		}
	}
	return page, nil
}
func (s *Store) verifyPageSource(ctx context.Context, caseID string, v Source) error {
	var f *os.File
	var e error
	if v.Storage == "external" {
		info, e := os.Lstat(v.ExternalPath)
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return problem("CASE_SOURCE_INVALID", "source must remain a regular file")
		}
		f, e = openSource(v.ExternalPath)
		if e != nil {
			return e
		}
		current, e := f.Stat()
		if e != nil || !os.SameFile(info, current) {
			f.Close()
			return problem("CASE_SOURCE_CHANGED", "source path changed during open")
		}
	} else {
		p := path.Join("cases", caseID, "objects", v.SHA256)
		if e = s.regular(p); e != nil {
			return e
		}
		f, e = s.root.Open(p)
		if e != nil {
			return e
		}
	}
	defer f.Close()
	before, e := f.Stat()
	if e != nil {
		return e
	}
	if !before.Mode().IsRegular() || before.Size() > MaxObjectBytes {
		return problem("CASE_SOURCE_INVALID", "bounded regular source required")
	}
	digest := sha256.New()
	n, e := io.Copy(digest, &cancelReader{ctx: ctx, r: io.LimitReader(f, MaxObjectBytes+1)})
	if e != nil {
		return e
	}
	after, e := f.Stat()
	if e != nil {
		return e
	}
	if n != before.Size() || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || strconv.FormatInt(n, 10) != v.Size || hex.EncodeToString(digest.Sum(nil)) != v.SHA256 {
		return problem("CASE_HASH_MISMATCH", "source changed; reimport explicitly")
	}
	if v.Storage == "external" {
		now, e := os.Lstat(v.ExternalPath)
		if e != nil || !now.Mode().IsRegular() || !os.SameFile(before, now) {
			return problem("CASE_SOURCE_CHANGED", "source path changed during read")
		}
	}
	return nil
}
