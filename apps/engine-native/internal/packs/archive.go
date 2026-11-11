package packs

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// EOCD is checked before archive/zip allocates its central-directory index.
func preflight(b []byte) error {
	if len(b) < 22 || len(b) > MaxArchiveBytes {
		return problem("PACK_BUDGET_EXCEEDED", "bounded single-volume ZIP required")
	}
	tail := b[len(b)-22:]
	if binary.LittleEndian.Uint32(tail) != 0x06054b50 || binary.LittleEndian.Uint16(tail[20:]) != 0 || binary.LittleEndian.Uint16(tail[4:]) != 0 || binary.LittleEndian.Uint16(tail[6:]) != 0 {
		return problem("PACK_ARCHIVE_INVALID", "ZIP comments, split archives and trailing payloads are forbidden")
	}
	count := int(binary.LittleEndian.Uint16(tail[10:]))
	if count < 2 || count > MaxFiles || count != int(binary.LittleEndian.Uint16(tail[8:])) {
		return problem("PACK_BUDGET_EXCEEDED", "ZIP entry count")
	}
	size := uint64(binary.LittleEndian.Uint32(tail[12:]))
	offset := uint64(binary.LittleEndian.Uint32(tail[16:]))
	if offset+size != uint64(len(b)-22) {
		return problem("PACK_ARCHIVE_INVALID", "ZIP64/offset mismatch")
	}
	return nil
}
func Validate(ctx context.Context, body []byte, keys map[string]Key, opts InstallOptions) (Revision, error) {
	out := Revision{}
	if e := preflight(body); e != nil {
		return out, e
	}
	zr, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil {
		return out, problem("PACK_ARCHIVE_INVALID", "invalid ZIP directory")
	}
	entries := map[string]*zip.File{}
	folded := map[string]bool{}
	var declared uint64
	for _, f := range zr.File {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		lower := strings.ToLower(f.Name)
		if !safeName(f.Name) || folded[lower] || !f.Mode().IsRegular() || f.Mode().Perm()&0111 != 0 || f.Flags&1 != 0 || f.UncompressedSize64 > MaxFileBytes || len(f.Extra) > 0 {
			return out, problem("PACK_ARCHIVE_INVALID", "unsafe, duplicate, linked, executable, extended or oversized ZIP entry")
		}
		folded[lower] = true
		declared += f.UncompressedSize64
		if declared > MaxExpandedBytes {
			return out, problem("PACK_BUDGET_EXCEEDED", "expanded ZIP limit")
		}
		entries[f.Name] = f
	}
	read := func(name string, limit int) ([]byte, error) {
		f := entries[name]
		if f == nil || f.UncompressedSize64 > uint64(limit) {
			return nil, problem("PACK_INVALID", "missing or oversized required data")
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		b, e := io.ReadAll(io.LimitReader(r, int64(limit)+1))
		if e != nil || len(b) > limit || uint64(len(b)) != f.UncompressedSize64 {
			return nil, problem("PACK_ARCHIVE_INVALID", "size/CRC validation failed")
		}
		return b, nil
	}
	raw, e := read("manifest.json", MaxManifestBytes)
	if e != nil {
		return out, e
	}
	if e = validateSchema("manifest", raw); e != nil {
		return out, problem("PACK_INVALID", "manifest schema failed")
	}
	if e = Strict(raw, &out.Manifest); e != nil {
		return out, problem("PACK_INVALID", "invalid manifest JSON")
	}
	if e = out.Manifest.validate(); e != nil {
		return out, e
	}
	signature, e := read("signature.json", 2048)
	if e != nil {
		return out, e
	}
	if e = Strict(signature, &out.Signature); e != nil {
		return out, problem("PACK_SIGNATURE_INVALID", "invalid detached signature envelope")
	}
	origin, scope := "signed", "application"
	if out.Manifest.Signer == "" {
		if !opts.AllowUnsigned || !scopeOK(opts.Scope) || opts.Scope == "application" || out.Signature.KeyID != "" || out.Signature.Signature != "" {
			return out, problem("PACK_SIGNATURE_REQUIRED", "unsigned local presets need explicit user/case scope")
		}
		origin, scope = "locally_authored", opts.Scope
	} else {
		if opts.Scope != "" && opts.Scope != "application" {
			return out, problem("PACK_SCOPE_INVALID", "signed application packs have application scope")
		}
		key, ok := keys[out.Manifest.Signer]
		if !ok || key.Revoked || out.Signature.KeyID != key.ID {
			return out, problem("PACK_SIGNER_UNTRUSTED", "signer is absent/revoked; register trust separately")
		}
		public, e := key.public()
		if e != nil {
			return out, e
		}
		sig, e := base64.StdEncoding.Strict().DecodeString(out.Signature.Signature)
		if e != nil || !ed25519.Verify(public, raw, sig) {
			return out, problem("PACK_SIGNATURE_INVALID", "signature does not cover exact manifest bytes")
		}
	}
	if len(entries) != len(out.Manifest.Files)+2 {
		return out, problem("PACK_ARCHIVE_INVALID", "undeclared or missing files")
	}
	out.Raw = append([]byte(nil), raw...)
	out.Files = map[string][]byte{}
	for name, info := range out.Manifest.Files {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		b, e := read(name, MaxFileBytes)
		if e != nil {
			return out, e
		}
		if len(b) != info.Size || Hash(b) != info.SHA256 {
			return out, problem("PACK_HASH_MISMATCH", "declared file hash/size differs")
		}
		if !utf8.Valid(b) {
			return out, problem("PACK_EXECUTABLE_FORBIDDEN", "pack data must be UTF-8 text")
		}
		if strings.HasSuffix(name, ".json") {
			if wirejson.Decode(b, new(any)) != nil {
				return out, problem("PACK_INVALID", "invalid JSON data file")
			}
		}
		if strings.HasSuffix(name, ".jsonl") {
			lines := bytes.Split(bytes.TrimSpace(b), []byte{'\n'})
			if len(lines) > 1000 {
				return out, problem("PACK_BUDGET_EXCEEDED", "fixture line limit")
			}
			for _, line := range lines {
				if wirejson.Decode(line, new(any)) != nil {
					return out, problem("PACK_INVALID", "invalid JSONL data file")
				}
			}
		}
		out.Files[name] = b
	}
	out.Ref = Ref{ID: out.Manifest.ID, Version: out.Manifest.Version, SHA256: Hash(raw), ArchiveSHA256: Hash(body), Signer: out.Manifest.Signer, Origin: origin, Scope: scope, Bytes: int(declared)}
	if e = ConformanceContext(ctx, &out); e != nil {
		return out, e
	}
	return out, nil
}

// Archive is a builder for reviewed data. Private keys are supplied in memory by
// a caller; they are never written to a pack, trust store, fixture or application.
func Archive(m Manifest, files map[string][]byte, key ed25519.PrivateKey) ([]byte, error) {
	m.Files = map[string]File{}
	for name, b := range files {
		role := "fixture"
		if strings.HasPrefix(name, "mappings/") {
			role = "mapping"
		}
		if strings.HasPrefix(name, "schemas/") {
			role = "schema"
		}
		if name == m.LicenseFile {
			role = "license"
		}
		if name == m.Changes {
			role = "changes"
		}
		m.Files[name] = File{Hash(b), len(b), role}
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return nil, e
	}
	s := Signature{}
	if len(key) > 0 {
		if len(key) != ed25519.PrivateKeySize {
			return nil, problem("PACK_SIGNATURE_INVALID", "Ed25519 private key size")
		}
		s = Signature{m.Signer, base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw))}
	}
	sig, _ := json.Marshal(s)
	all := map[string][]byte{"manifest.json": raw, "signature.json": sig}
	for p, b := range files {
		all[p] = b
	}
	names := []string{}
	for p := range all {
		names = append(names, p)
	}
	sort.Strings(names)
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for _, p := range names {
		h := &zip.FileHeader{Name: p, Method: zip.Deflate}
		h.SetMode(0600)
		w, e := z.CreateHeader(h)
		if e != nil {
			return nil, e
		}
		if _, e = w.Write(all[p]); e != nil {
			return nil, e
		}
	}
	if e = z.Close(); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
