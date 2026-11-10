// Package halog reads and writes the observed HTTP Analyzer V7 native format.
// The separate fingerprint probe never establishes compatibility by itself.
package halog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxBytes = int64(256 << 20)

type Fingerprint struct {
	SchemaVersion int    `json:"schemaVersion"`
	Bytes         int64  `json:"bytes"`
	SHA256        string `json:"sha256"`
	Container     string `json:"container"`
	Compatibility string `json:"compatibility"`
}

func IsPath(path string) bool { return strings.EqualFold(filepath.Ext(path), ".halog") }
func Probe(path string) (Fingerprint, error) {
	result := Fingerprint{SchemaVersion: 1, Compatibility: "unverified"}
	file, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return result, fmt.Errorf("legacy probe requires a regular file within 256 MiB")
	}
	hash := sha256.New()
	prefix := make([]byte, 512)
	n, err := io.ReadFull(file, prefix)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return result, err
	}
	prefix = prefix[:n]
	hash.Write(prefix)
	remaining, err := io.Copy(hash, io.LimitReader(file, MaxBytes-int64(n)+1))
	if err != nil {
		return result, err
	}
	result.Bytes = int64(n) + remaining
	if result.Bytes > MaxBytes {
		return result, fmt.Errorf("legacy probe input grew beyond 256 MiB")
	}
	result.SHA256 = hex.EncodeToString(hash.Sum(nil))
	result.Container = container(prefix)
	return result, nil
}
func container(prefix []byte) string {
	switch {
	case bytes.HasPrefix(prefix, []byte{0x1f, 0x8b}):
		return "gzip"
	case bytes.HasPrefix(prefix, []byte("PK\x03\x04")):
		return "zip"
	case bytes.HasPrefix(prefix, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}):
		return "compound"
	case bytes.HasPrefix(prefix, []byte("SQLite format 3\x00")):
		return "sqlite"
	case bytes.HasPrefix(prefix, []byte{0xff, 0xfe, 0x3c, 0}) || bytes.HasPrefix(prefix, []byte{0x3c, 0, 0x3f, 0}):
		return "xml_utf16le"
	case bytes.HasPrefix(prefix, []byte{0xfe, 0xff, 0, 0x3c}):
		return "xml_utf16be"
	case len(prefix) >= 2 && prefix[0]&15 == 8 && (uint16(prefix[0])<<8|uint16(prefix[1]))%31 == 0:
		return "zlib"
	}
	text := bytes.TrimSpace(bytes.TrimPrefix(prefix, []byte{0xef, 0xbb, 0xbf}))
	if bytes.HasPrefix(text, []byte("<")) {
		return "xml_text"
	}
	if bytes.HasPrefix(text, []byte("{")) || bytes.HasPrefix(text, []byte("[")) {
		return "json_text"
	}
	return "opaque"
}

type PairEvidence struct {
	SchemaVersion int         `json:"schemaVersion"`
	Native        Fingerprint `json:"native"`
	FullXML       Fingerprint `json:"fullXml"`
	Status        string      `json:"status"`
	Missing       []string    `json:"missing"`
}

func ProbePair(nativePath, xmlPath string) (PairEvidence, error) {
	out := PairEvidence{SchemaVersion: 1, Status: "UNVERIFIED", Missing: []string{"genuine writer/version provenance", "verified field mapping", "reference-to-ArchScope data comparison", "ArchScope-to-reference reopening"}}
	var err error
	out.Native, err = Probe(nativePath)
	if err != nil {
		return out, err
	}
	out.FullXML, err = Probe(xmlPath)
	if err != nil {
		return out, err
	}
	if !strings.HasPrefix(out.FullXML.Container, "xml_") {
		return out, fmt.Errorf("reference full-XML input does not have an XML signature")
	}
	return out, nil
}
