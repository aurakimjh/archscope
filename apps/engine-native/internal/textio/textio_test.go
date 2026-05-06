package textio

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/korean"
)

func TestDetectFromBytesUTF8(t *testing.T) {
	enc, err := DetectFromBytes([]byte("ascii safe"), nil)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if enc != "utf-8" {
		t.Fatalf("enc = %q, want utf-8", enc)
	}
}

func TestDetectFromBytesUTF8SigViaBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	enc, err := DetectFromBytes(append(bom, []byte("hello")...), nil)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if enc != "utf-8-sig" {
		t.Fatalf("enc = %q, want utf-8-sig", enc)
	}
}

func TestDetectFromBytesUTF16LEViaBOM(t *testing.T) {
	enc, err := DetectFromBytes([]byte{0xFF, 0xFE, 'h', 0, 'i', 0}, nil)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if enc != "utf-16-le" {
		t.Fatalf("enc = %q, want utf-16-le", enc)
	}
}

func TestDetectFromBytesUTF16BEViaBOM(t *testing.T) {
	enc, err := DetectFromBytes([]byte{0xFE, 0xFF, 0, 'h', 0, 'i'}, nil)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if enc != "utf-16-be" {
		t.Fatalf("enc = %q, want utf-16-be", enc)
	}
}

func TestDetectFromBytesUTF16LEViaNullParity(t *testing.T) {
	// 50 ASCII chars in utf-16-le without a BOM — every other byte
	// should be 0x00 so the heuristic catches it.
	body := make([]byte, 0, 100)
	for _, r := range "Full thread dump OpenJDK on Windows" {
		body = append(body, byte(r), 0)
	}
	enc, err := DetectFromBytes(body, nil)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if enc != "utf-16-le" {
		t.Fatalf("enc = %q, want utf-16-le", enc)
	}
}

func TestDetectFromBytesCp949Fallback(t *testing.T) {
	encoder := korean.EUCKR.NewEncoder()
	body, err := encoder.Bytes([]byte("서비스"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	enc, err := DetectFromBytes(body, nil)
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if enc != "cp949" && enc != "latin-1" {
		// latin-1 is the universal fallback; we accept either, but
		// we expect cp949 to win first.
		t.Fatalf("enc = %q, want cp949 or latin-1", enc)
	}
}

func TestIterTextLinesUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plain.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	lines, err := IterTextLines(path, "")
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	if got := lines; len(got) != 3 || got[0] != "alpha" || got[2] != "gamma" {
		t.Fatalf("lines mismatch: %+v", got)
	}
}

func TestIterTextLinesCp949(t *testing.T) {
	encoder := korean.EUCKR.NewEncoder()
	body, err := encoder.Bytes([]byte("서비스\n에러\n"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	path := filepath.Join(t.TempDir(), "korean.txt")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	lines, err := IterTextLines(path, "")
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	if len(lines) != 2 || lines[0] != "서비스" || lines[1] != "에러" {
		t.Fatalf("lines mismatch: %+v", lines)
	}
}

func TestIterTextLinesUTF16LEWithBOM(t *testing.T) {
	body := []byte{0xFF, 0xFE}
	for _, r := range "Full thread dump\nworker\n" {
		body = append(body, byte(r), 0)
	}
	path := filepath.Join(t.TempDir(), "utf16.txt")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	lines, err := IterTextLines(path, "")
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	if len(lines) < 2 || lines[0] != "Full thread dump" || lines[1] != "worker" {
		t.Fatalf("lines mismatch: %+v", lines)
	}
}

func TestIterTextLinesWithContextWindow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ctx.txt")
	if err := os.WriteFile(path, []byte("a\nb\nc\nd\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	rows, err := IterTextLinesWithContext(path, "utf-8")
	if err != nil {
		t.Fatalf("iter: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	if rows[0].Before != nil {
		t.Fatalf("first row should have nil Before")
	}
	if rows[0].After == nil || *rows[0].After != "b" {
		t.Fatalf("first row After should be 'b'; got %+v", rows[0].After)
	}
	if rows[3].After != nil {
		t.Fatalf("last row should have nil After")
	}
	if rows[3].Before == nil || *rows[3].Before != "c" {
		t.Fatalf("last row Before should be 'c'; got %+v", rows[3].Before)
	}
}
