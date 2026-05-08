// Package textio ports archscope_engine.common.file_utils — the
// encoding-safe text iterator that every parser uses to walk lines
// from disk regardless of whether the source is utf-8, utf-8-sig
// (BOM-prefixed), cp949 (Korean Windows tools), or utf-16 (some JVM
// thread dump exporters on Windows).
//
// We mirror Python's detection heuristics:
//
//   1. Byte order marks (utf-16 BE/LE, utf-8-sig).
//   2. Even/odd null-byte ratio — utf-16 emits NUL on every other
//      byte, so a 30%+ ratio at the right parity wins.
//   3. Try-decode fallback in order utf-8, utf-8-sig, cp949,
//      latin-1 — latin-1 always succeeds so the chain never raises.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] textio 패키지 — 인코딩 안전 텍스트 라인 iterator.
//
// 왜 필요한가?
//   ArchScope 의 입력은 다양한 환경에서 옴:
//     • Linux / macOS 의 UTF-8 (대부분).
//     • UTF-8-sig (BOM 포함) — Windows 에서 "ANSI" 로 저장한 파일이
//       그대로 BOM 가지고 들어옴.
//     • CP949 — 한국 Windows 의 일부 도구가 출력하는 텍스트.
//     • UTF-16 (BE/LE) — 일부 JVM thread dump exporter (Windows 환경)
//       가 UTF-16 으로 출력. NUL 바이트가 양 옆마다 등장.
//   파서가 매번 인코딩을 따지지 않도록 textio 가 흡수.
//
// 인코딩 감지 알고리즘 (Python 동치)
//   1) BOM 검사: 처음 2~3 byte 가 BOM 이면 즉시 결정.
//   2) NUL 비율: 짝수/홀수 위치별 NUL 비율 → 30% 이상이면 UTF-16
//      (LE/BE 구별).
//   3) Try-decode 체인: utf-8 → utf-8-sig → cp949 → latin-1.
//      latin-1 은 항상 성공하므로 체인이 절대 실패하지 않음.
//
// 라인 iterator
//   bufio.Scanner 기반. 큰 파일에 메모리 안전한 라인 스트림 제공.
//   파서는 ParseFile / ParseString / ReadHead 헬퍼만 호출.
//
// 회귀 안전망
//   T-031 fix: mid-file decode 실패 시 fallback 재시도가 라인을 중복
//   emit 하지 않도록 보장 (옛날 버그 회귀 방지).
package textio

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
)

// DefaultEncodings is the fallback chain used when no explicit
// encoding is supplied. Matches the Python tuple.
var DefaultEncodings = []string{"utf-8", "utf-8-sig", "cp949", "latin-1"}

// DefaultProbeBytes mirrors the Python 1 MiB probe window.
const DefaultProbeBytes = 1 << 20

// TextLineContext carries a window of lines around the offending
// row so a parser-debug log can preserve the surrounding context
// without exploding.
type TextLineContext struct {
	LineNumber int
	Before     *string
	Target     string
	After      *string
}

// DetectFromBytes returns the best-guess encoding for a byte sample.
// Falls back to Python's order; latin-1 always succeeds so the
// function is total under non-empty input.
func DetectFromBytes(data []byte, encodings []string) (string, error) {
	if len(encodings) == 0 {
		encodings = DefaultEncodings
	}
	if len(data) >= 2 {
		switch {
		case data[0] == 0xff && data[1] == 0xfe:
			return "utf-16-le", nil
		case data[0] == 0xfe && data[1] == 0xff:
			return "utf-16-be", nil
		}
	}
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return "utf-8-sig", nil
	}

	if hint := utf16ByOddEvenNull(data); hint != "" {
		return hint, nil
	}

	var lastErr error
	for _, enc := range encodings {
		if err := tryDecode(enc, data); err == nil {
			return enc, nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("no encodings configured")
}

func utf16ByOddEvenNull(data []byte) string {
	const probe = 4096
	if len(data) == 0 {
		return ""
	}
	sample := data
	if len(sample) > probe {
		sample = sample[:probe]
	}
	var evenNulls, oddNulls int
	for i, b := range sample {
		if b == 0 {
			if i%2 == 0 {
				evenNulls++
			} else {
				oddNulls++
			}
		}
	}
	evenLen := (len(sample) + 1) / 2
	oddLen := len(sample) / 2
	if oddLen == 0 || evenLen == 0 {
		return ""
	}
	oddRatio := float64(oddNulls) / float64(oddLen)
	evenRatio := float64(evenNulls) / float64(evenLen)
	if oddRatio > 0.30 && evenRatio < 0.05 {
		return "utf-16-le"
	}
	if evenRatio > 0.30 && oddRatio < 0.05 {
		return "utf-16-be"
	}
	return ""
}

// DetectEncoding probes up to `probeBytes` of `path` and returns the
// best-guess encoding. Mirrors Python's `detect_text_encoding`.
func DetectEncoding(path string, probeBytes int) (string, error) {
	if probeBytes <= 0 {
		probeBytes = DefaultProbeBytes
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	buf := make([]byte, probeBytes)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return DetectFromBytes(buf[:n], DefaultEncodings)
}

// IterTextLines yields lines from `path` honoring the detected (or
// caller-supplied) encoding. Trailing `\n` is stripped — same shape
// as Python's `iter_text_lines`.
func IterTextLines(path, encoding string) ([]string, error) {
	if encoding == "" {
		detected, err := DetectEncoding(path, DefaultProbeBytes)
		if err != nil {
			return nil, err
		}
		encoding = detected
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	body, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	decoded, err := decodeBytes(body, encoding)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(decoded))
	scanner.Buffer(make([]byte, 0, 1<<20), 1<<26)
	out := make([]string, 0, 256)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// IterTextLinesWithContext attaches a one-line before/after window to
// each line so parser-debug logs can preserve the surrounding
// context without re-reading the file.
func IterTextLinesWithContext(path, encoding string) ([]TextLineContext, error) {
	lines, err := IterTextLines(path, encoding)
	if err != nil {
		return nil, err
	}
	out := make([]TextLineContext, 0, len(lines))
	for i, line := range lines {
		entry := TextLineContext{LineNumber: i + 1, Target: line}
		if i > 0 {
			prev := lines[i-1]
			entry.Before = &prev
		}
		if i+1 < len(lines) {
			next := lines[i+1]
			entry.After = &next
		}
		out = append(out, entry)
	}
	return out, nil
}

// DecodeBytes decodes raw bytes into a UTF-8 Go string using the given
// encoding label (utf-8 / utf-8-sig / cp949 / utf-16-le / utf-16-be /
// latin-1). Public wrapper around the internal decoder so consumers
// like the thread-dump registry can decode a 4KB head sample without
// re-reading the file.
func DecodeBytes(data []byte, encoding string) (string, error) {
	return decodeBytes(data, encoding)
}

func tryDecode(encoding string, data []byte) error {
	_, err := decodeBytes(data, encoding)
	return err
}

func decodeBytes(data []byte, encoding string) (string, error) {
	switch normalize(encoding) {
	case "utf-8":
		if !utf8.Valid(data) {
			return "", errors.New("invalid utf-8 input")
		}
		return string(data), nil
	case "utf-8-sig":
		bom := []byte{0xef, 0xbb, 0xbf}
		stripped := data
		if bytes.HasPrefix(stripped, bom) {
			stripped = stripped[len(bom):]
		}
		if !utf8.Valid(stripped) {
			return "", errors.New("invalid utf-8 input under utf-8-sig")
		}
		return string(stripped), nil
	case "cp949", "euc-kr":
		decoder := korean.EUCKR.NewDecoder()
		out, err := decoder.Bytes(data)
		if err != nil {
			return "", err
		}
		return string(out), nil
	case "latin-1", "iso-8859-1":
		runes := make([]rune, len(data))
		for i, b := range data {
			runes[i] = rune(b)
		}
		return string(runes), nil
	case "utf-16", "utf-16-le":
		return decodeUTF16(data, binary.LittleEndian)
	case "utf-16-be":
		return decodeUTF16(data, binary.BigEndian)
	}
	return "", errors.New("unsupported encoding: " + encoding)
}

func decodeUTF16(data []byte, order binary.ByteOrder) (string, error) {
	// Strip BOM if present.
	if len(data) >= 2 {
		if (order == binary.LittleEndian && data[0] == 0xff && data[1] == 0xfe) ||
			(order == binary.BigEndian && data[0] == 0xfe && data[1] == 0xff) {
			data = data[2:]
		}
	}
	if len(data)%2 != 0 {
		// Drop the trailing odd byte rather than failing — parsers
		// only need the bulk content for format detection.
		data = data[:len(data)-1]
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[i*2 : i*2+2])
	}
	return string(utf16.Decode(units)), nil
}

func normalize(encoding string) string {
	return strings.ToLower(strings.TrimSpace(encoding))
}
