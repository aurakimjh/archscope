package cases

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type ArchiveEntry struct {
	Name      string `json:"name"`
	Size      string `json:"size"`
	SHA256    string `json:"sha256"`
	Directory bool   `json:"directory"`
}
type ArchivePreview struct {
	SHA256        string         `json:"sha256"`
	Entries       []ArchiveEntry `json:"entries"`
	ExpandedBytes string         `json:"expanded_bytes"`
}

func nameKey(name string) string {
	return cases.Fold().String(norm.NFC.String(strings.TrimSuffix(name, "/")))
}
func safeName(name string) bool {
	if name == "" || len(name) > 1024 || strings.Count(name, "/") >= 16 || !utf8.ValidString(name) || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || path.Clean(name) != name {
		return false
	}
	for _, p := range strings.Split(name, "/") {
		if p == "." || p == ".." || p == "" || strings.TrimRight(p, ". ") != p {
			return false
		}
		for _, r := range p {
			if r < 32 || strings.ContainsRune(`<>"|?*`, r) {
				return false
			}
		}
		base := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
	}
	return true
}
func nested(name string, b []byte) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".zip", ".tar", ".tgz", ".gz", ".bz2", ".xz", ".7z", ".rar", ".zst", ".jar"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return bytes.HasPrefix(b, []byte{'P', 'K', 3, 4}) || bytes.HasPrefix(b, []byte{'P', 'K', 5, 6}) || bytes.HasPrefix(b, []byte{'P', 'K', 7, 8}) || bytes.HasPrefix(b, []byte{0xfd, '7', 'z', 'X', 'Z', 0}) || bytes.HasPrefix(b, []byte{'B', 'Z', 'h'}) || bytes.HasPrefix(b, []byte{0x28, 0xb5, 0x2f, 0xfd}) || bytes.HasPrefix(b, []byte{0x1f, 0x8b}) || bytes.HasPrefix(b, []byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}) || bytes.HasPrefix(b, []byte("Rar!")) || len(b) > 262 && string(b[257:262]) == "ustar"
}
func readBounded(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, problem("CASE_BUDGET_EXCEEDED", "byte limit exceeded")
	}
	return b, nil
}
func readLocal(file string, limit int64) ([]byte, error) {
	selected, e := os.Lstat(file)
	if e != nil {
		return nil, e
	}
	if !selected.Mode().IsRegular() || selected.Size() > limit {
		return nil, problem("CASE_SOURCE_INVALID", "select a bounded regular local file; resolve links explicitly")
	}
	f, e := openSource(file)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, problem("CASE_SOURCE_INVALID", "regular bounded local file required")
	}
	if !os.SameFile(selected, info) {
		return nil, problem("CASE_SOURCE_CHANGED", "source path replaced during open")
	}
	before := info
	body, e := readBounded(f, limit)
	if e != nil {
		return nil, e
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return nil, e
	}
	digest := sha256.New()
	n, e := io.Copy(digest, io.LimitReader(f, limit+1))
	if e != nil {
		return nil, e
	}
	sum := sha256.Sum256(body)
	if n != int64(len(body)) || !bytes.Equal(digest.Sum(nil), sum[:]) {
		return nil, problem("CASE_SOURCE_CHANGED", "source content changed during read")
	}
	current, e := os.Lstat(file)
	if e != nil || !current.Mode().IsRegular() || !os.SameFile(current, before) {
		return nil, problem("CASE_SOURCE_CHANGED", "source path changed during read")
	}
	after, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || int64(len(body)) != before.Size() {
		return nil, problem("CASE_SOURCE_CHANGED", "source changed during read")
	}
	return body, nil
}
func PreviewArchive(ctx context.Context, file string) (ArchivePreview, error) {
	raw, e := readLocal(file, MaxArchiveBytes)
	if e != nil {
		return ArchivePreview{}, e
	}
	return scanArchive(ctx, raw, nil)
}

// scanArchive validates every header, stream size, CRC, path and actual content
// before the caller publishes any case revision. Readers never trust ZIP sizes.
func scanArchive(ctx context.Context, raw []byte, visit func(ArchiveEntry, []byte) error) (ArchivePreview, error) {
	preview := ArchivePreview{SHA256: hash(raw), Entries: []ArchiveEntry{}}
	total := int64(0)
	names := map[string]bool{}
	files := map[string]bool{}
	add := func(name string, dir bool, size int64, r io.Reader) error {
		if e := ctx.Err(); e != nil {
			return e
		}
		name = strings.TrimSuffix(name, "/")
		key := nameKey(name)
		if !safeName(name) || names[key] {
			return problem("ARCHIVE_PATH_UNSAFE", "unsafe or duplicate normalized path")
		}
		if len(preview.Entries) >= MaxEntries {
			return problem("CASE_BUDGET_EXCEEDED", "archive entry limit")
		}
		for p := path.Dir(key); p != "."; p = path.Dir(p) {
			if files[p] {
				return problem("ARCHIVE_PATH_UNSAFE", "file is used as a parent directory")
			}
		}
		if !dir {
			for other := range names {
				if strings.HasPrefix(other, key+"/") {
					return problem("ARCHIVE_PATH_UNSAFE", "file/directory collision")
				}
			}
			files[key] = true
		}
		names[key] = true
		if size < 0 || size > MaxObjectBytes || total+size > MaxArchiveBytes {
			return problem("CASE_BUDGET_EXCEEDED", "expanded archive byte limit")
		}
		total += size
		entry := ArchiveEntry{Name: name, Size: currentRevision(uint64(size)), Directory: dir}
		if dir {
			if size != 0 {
				return problem("ARCHIVE_INVALID", "directory contains payload")
			}
			if _, e := readBounded(&contextReader{ctx: ctx, r: r}, 0); e != nil {
				return e
			}
		} else {
			b, e := readBounded(&contextReader{ctx: ctx, r: r}, size)
			if e != nil {
				return e
			}
			if int64(len(b)) != size {
				return problem("ARCHIVE_INVALID", "truncated entry")
			}
			if nested(name, b) {
				return problem("ARCHIVE_NESTED_FORBIDDEN", "nested archive must be selected and reviewed separately")
			}
			entry.SHA256 = hash(b)
			if visit != nil {
				if e = visit(entry, b); e != nil {
					return e
				}
			}
		}
		preview.Entries = append(preview.Entries, entry)
		return nil
	}
	if bytes.HasPrefix(raw, []byte{'P', 'K'}) {
		// Bound central-directory allocation before archive/zip parses headers. ZIP64
		// is unnecessary at these budgets and deliberately unsupported.
		at := -1
		for i := len(raw) - 22; i >= 0 && i >= len(raw)-65557; i-- {
			if bytes.Equal(raw[i:i+4], []byte{'P', 'K', 5, 6}) && i+22+int(binary.LittleEndian.Uint16(raw[i+20:])) == len(raw) {
				at = i
				break
			}
		}
		if at < 0 {
			return preview, problem("ARCHIVE_INVALID", "missing ZIP end directory")
		}
		tail := raw[at:]
		count := binary.LittleEndian.Uint16(tail[10:])
		if count > MaxEntries || count == 0xffff {
			return preview, problem("CASE_BUDGET_EXCEEDED", "ZIP directory entry limit")
		}
		if binary.LittleEndian.Uint16(tail[4:]) != 0 || binary.LittleEndian.Uint16(tail[6:]) != 0 || binary.LittleEndian.Uint16(tail[8:]) != count || binary.LittleEndian.Uint32(tail[12:]) == 0xffffffff || binary.LittleEndian.Uint32(tail[16:]) == 0xffffffff {
			return preview, problem("ARCHIVE_INVALID", "split/ZIP64 archives unsupported")
		}

		central := int64(binary.LittleEndian.Uint32(tail[16:]))
		centralSize := int64(binary.LittleEndian.Uint32(tail[12:]))
		end := central + centralSize
		if central < 0 || end > int64(at) {
			return preview, problem("ARCHIVE_INVALID", "invalid central directory extent")
		}
		cursor := central
		for i := 0; i < int(count); i++ {
			if cursor+46 > end || !bytes.Equal(raw[cursor:cursor+4], []byte{'P', 'K', 1, 2}) {
				return preview, problem("ARCHIVE_INVALID", "invalid central directory record")
			}
			header := raw[cursor : cursor+46]
			cursor += 46 + int64(binary.LittleEndian.Uint16(header[28:])) + int64(binary.LittleEndian.Uint16(header[30:])) + int64(binary.LittleEndian.Uint16(header[32:]))
			if cursor > end {
				return preview, problem("ARCHIVE_INVALID", "central directory record exceeds extent")
			}
		}
		if cursor != end {
			return preview, problem("ARCHIVE_INVALID", "central directory count/extent mismatch")
		}
		zr, e := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if e != nil {
			return preview, e
		}
		if len(zr.File) != int(count) {
			return preview, problem("ARCHIVE_INVALID", "inconsistent ZIP entry count")
		}
		for _, f := range zr.File {
			mode := f.Mode()
			if !mode.IsRegular() && !mode.IsDir() || f.Flags&1 != 0 || f.Method != zip.Store && f.Method != zip.Deflate {
				return preview, problem("ARCHIVE_LINK_FORBIDDEN", "only unencrypted regular files/directories supported")
			}
			if f.UncompressedSize64 > MaxObjectBytes {
				return preview, problem("CASE_BUDGET_EXCEEDED", "ZIP expanded entry limit")
			}
			r, e := f.Open()
			if e != nil {
				return preview, e
			}
			e = add(f.Name, mode.IsDir(), int64(f.UncompressedSize64), r)
			ce := r.Close()
			if e != nil {
				return preview, e
			}
			if ce != nil {
				return preview, ce
			}
		}
	} else {
		var input io.Reader = bytes.NewReader(raw)
		if bytes.HasPrefix(raw, []byte{0x1f, 0x8b}) {
			g, e := gzip.NewReader(input)
			if e != nil {
				return preview, e
			}
			defer g.Close()
			expanded, e := readBounded(&contextReader{ctx: ctx, r: g}, MaxArchiveBytes)
			if e != nil {
				return preview, e
			}
			input = bytes.NewReader(expanded)
		}
		tr := tar.NewReader(input)
		for {
			h, e := tr.Next()
			if e == io.EOF {
				break
			}
			if e != nil {
				return preview, e
			}
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA && h.Typeflag != tar.TypeDir || len(h.PAXRecords) > 0 || h.Format == tar.FormatGNU {
				return preview, problem("ARCHIVE_LINK_FORBIDDEN", "TAR links, special files and extended/sparse headers unsupported")
			}
			if e = add(h.Name, h.Typeflag == tar.TypeDir, h.Size, tr); e != nil {
				return preview, e
			}
		}
		if remaining, ok := input.(*bytes.Reader); ok {
			tail, e := readBounded(remaining, MaxArchiveBytes)
			if e != nil {
				return preview, e
			}
			if len(bytes.Trim(tail, "\x00")) != 0 {
				return preview, problem("ARCHIVE_INVALID", "unexpected data after TAR end marker")
			}
		}
	}
	if len(preview.Entries) == 0 {
		return preview, problem("ARCHIVE_INVALID", "archive is empty")
	}
	if e := ctx.Err(); e != nil {
		return preview, e
	}
	preview.ExpandedBytes = currentRevision(uint64(total))
	return preview, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if e := r.ctx.Err(); e != nil {
		return 0, e
	}
	return r.r.Read(p)
}
