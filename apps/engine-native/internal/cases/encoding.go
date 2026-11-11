package cases

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
)

type DecodeOptions struct {
	Encoding         string   `json:"encoding"`
	Sniff            []string `json:"sniff"`
	AllowReplacement bool     `json:"allow_replacement"`
}
type Line struct {
	Number    string `json:"number"`
	RawStart  string `json:"raw_start"`
	RawEnd    string `json:"raw_end"`
	TextStart string `json:"text_start"`
	TextEnd   string `json:"text_end"`
}
type Decoded struct {
	Text           string   `json:"text"`
	Encoding       Encoding `json:"encoding"`
	Lines          []Line   `json:"lines"`
	SourceSHA256   string   `json:"source_sha256"`
	TotalLines     string   `json:"total_lines"`
	IndexTruncated bool     `json:"index_truncated"`
}

func encodingName(s string) string {
	switch strings.ToLower(s) {
	case "utf8", "utf-8":
		return "utf-8"
	case "utf16le", "utf-16le":
		return "utf-16le"
	case "utf16be", "utf-16be":
		return "utf-16be"
	case "cp949", "windows-949":
		return "cp949"
	case "euc-kr":
		return "euc-kr"
	}
	return ""
}
func bom(b []byte) (string, int) {
	if bytes.HasPrefix(b, []byte{0xef, 0xbb, 0xbf}) {
		return "utf-8", 3
	}
	if bytes.HasPrefix(b, []byte{0xff, 0xfe}) {
		return "utf-16le", 2
	}
	if bytes.HasPrefix(b, []byte{0xfe, 0xff}) {
		return "utf-16be", 2
	}
	return "", 0
}

// DecodeText is bounded and keeps original byte coordinates. CP949 extensions
// are deliberately not relabelled EUC-KR. Ambiguous legacy encodings need choice.
func DecodeText(raw []byte, opts DecodeOptions) (Decoded, error) {
	name, basis, skip, e := selectTextEncoding(context.Background(), raw, opts)
	if e != nil {
		return Decoded{}, e
	}
	result, e := decodeNamed(raw, name, skip, opts.AllowReplacement)
	if e != nil {
		return Decoded{}, e
	}
	result.Encoding = finishEncoding(raw, name, basis, opts.Encoding != "", result.Encoding.Replacements)
	result.SourceSHA256 = hash(raw)
	return result, nil
}
func selectTextEncoding(ctx context.Context, raw []byte, opts DecodeOptions) (string, string, int, error) {
	if len(raw) > MaxObjectBytes {
		return "", "", 0, problem("CASE_BUDGET_EXCEEDED", "source byte limit")
	}
	if len(opts.Sniff) > 2 {
		return "", "", 0, problem("ENCODING_UNSUPPORTED", "at most two permitted legacy sniff candidates")
	}
	for _, candidate := range opts.Sniff {
		n := encodingName(candidate)
		if n != "cp949" && n != "euc-kr" {
			return "", "", 0, problem("ENCODING_UNSUPPORTED", "sniff only accepts permitted legacy candidates")
		}
	}
	name, basis := encodingName(opts.Encoding), "explicit"
	detected, skip := bom(raw)
	if opts.Encoding != "" && name == "" {
		return "", "", 0, problem("ENCODING_UNSUPPORTED", "unsupported explicit encoding")
	}
	if name != "" && detected != "" && name != detected {
		return "", "", 0, problem("ENCODING_BOM_CONFLICT", "explicit encoding conflicts with BOM")
	}
	if name == "" {
		if detected != "" {
			name, basis = detected, "bom"
		} else if utf8.Valid(raw) {
			name, basis = "utf-8", "validated-utf8"
		} else {
			candidates := map[string]bool{}
			for _, v := range opts.Sniff {
				n := encodingName(v)
				if n != "cp949" && n != "euc-kr" {
					return "", "", 0, problem("ENCODING_UNSUPPORTED", "sniff only accepts permitted legacy candidates")
				}
				if _, _, _, e := scanNamed(ctx, raw, n, 0, false, nil); e == nil {
					candidates[n] = true
				}
			}
			if len(candidates) != 1 {
				return "", "", 0, problem("ENCODING_SELECTION_REQUIRED", "legacy encoding is unknown or ambiguous; select explicitly")
			}
			for n := range candidates {
				name = n
			}
			basis = "permitted-sniff"
		}
	}

	return name, basis, skip, nil
}
func finishEncoding(raw []byte, name, basis string, override bool, replacements string) Encoding {
	digest := hash(raw)
	index, _ := json.Marshal(struct{ Digest, Encoding, Replacements, DecoderRevision string }{digest, name, replacements, "text-index-v1"})
	return Encoding{Name: name, Basis: basis, Override: override, Replacements: replacements, IndexRevision: hash(index)}
}

const MaxIndexedLines = 1_000_000

func decodeNamed(raw []byte, name string, skip int, allow bool) (Decoded, error) {
	result := Decoded{Encoding: Encoding{Name: name}, Lines: []Line{}}
	text, replaced, total, e := scanNamed(context.Background(), raw, name, skip, allow, func(number, startRaw, endRaw, startText, endText int) {
		if len(result.Lines) < MaxIndexedLines {
			result.Lines = append(result.Lines, Line{strconv.Itoa(number), strconv.Itoa(startRaw), strconv.Itoa(endRaw), strconv.Itoa(startText), strconv.Itoa(endText)})
		} else {
			result.IndexTruncated = true
		}
	})
	if e != nil {
		return Decoded{}, e
	}
	result.Text = text
	result.Encoding.Replacements = strconv.FormatUint(replaced, 10)
	result.TotalLines = strconv.Itoa(total)
	return result, nil
}
func scanNamed(ctx context.Context, raw []byte, name string, skip int, allow bool, line func(int, int, int, int, int)) (string, uint64, int, error) {
	var out strings.Builder
	out.Grow(len(raw))
	startRaw, startText, total := skip, 0, 0
	replaced := uint64(0)
	lastCR := false
	steps := 0
	flush := func(end int) {
		total++
		if line != nil {
			line(total, startRaw, end, startText, out.Len())
		}
		startRaw = end
		startText = out.Len()
	}
	for pos := skip; pos < len(raw); {
		steps++
		if steps&4095 == 0 {
			if e := ctx.Err(); e != nil {
				return "", 0, 0, e
			}
		}
		r, n, bad := runeAt(raw[pos:], name)
		if n < 1 {
			return "", 0, 0, problem("ENCODING_INVALID", "decoder made no progress")
		}
		if bad {
			if !allow {
				return "", 0, 0, problem("ENCODING_INVALID", "invalid bytes for selected encoding")
			}
			replaced++
		}
		if lastCR && r != '\n' {
			flush(pos)
			lastCR = false
		}
		out.WriteRune(r)
		pos += n
		if out.Len() > MaxObjectBytes {
			return "", 0, 0, problem("CASE_BUDGET_EXCEEDED", "decoded UTF-8 byte limit")
		}
		if r == '\n' {
			flush(pos)
			lastCR = false
		} else {
			lastCR = r == '\r'
		}
	}
	if startRaw < len(raw) {
		flush(len(raw))
	}
	if e := ctx.Err(); e != nil {
		return "", 0, 0, e
	}
	return out.String(), replaced, total, nil
}
func runeAt(b []byte, name string) (rune, int, bool) {
	switch name {
	case "utf-8":
		r, n := utf8.DecodeRune(b)
		return r, n, r == utf8.RuneError && n == 1
	case "utf-16le", "utf-16be":
		if len(b) < 2 {
			return utf8.RuneError, 1, true
		}
		var order binary.ByteOrder = binary.LittleEndian
		if name == "utf-16be" {
			order = binary.BigEndian
		}
		u := order.Uint16(b)
		if u >= 0xd800 && u <= 0xdbff {
			if len(b) >= 4 {
				v := order.Uint16(b[2:])
				if v >= 0xdc00 && v <= 0xdfff {
					return utf16.DecodeRune(rune(u), rune(v)), 4, false
				}
			}
			return utf8.RuneError, 2, true
		}
		if u >= 0xdc00 && u <= 0xdfff {
			return utf8.RuneError, 2, true
		}
		return rune(u), 2, false
	case "cp949", "euc-kr":
		if b[0] < 0x80 {
			return rune(b[0]), 1, false
		}
		if len(b) < 2 {
			return utf8.RuneError, 1, true
		}
		if name == "euc-kr" && (b[0] < 0xa1 || b[0] > 0xfe || b[1] < 0xa1 || b[1] > 0xfe) {
			return utf8.RuneError, 1, true
		}
		v, e := korean.EUCKR.NewDecoder().Bytes(b[:2])
		if e != nil {
			return utf8.RuneError, 1, true
		}
		r, n := utf8.DecodeRune(v)
		if n != len(v) || r == utf8.RuneError {
			return utf8.RuneError, 1, true
		}
		return r, 2, false
	}
	return utf8.RuneError, 1, true
}

type SourceLine struct {
	Line
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}
type TextPage struct {
	Encoding       Encoding     `json:"encoding"`
	SourceSHA256   string       `json:"source_sha256"`
	Lines          []SourceLine `json:"lines"`
	TotalLines     string       `json:"total_lines"`
	IndexedLines   string       `json:"indexed_lines"`
	IndexTruncated bool         `json:"index_truncated"`
}

func SourcePage(d Decoded, start, count int) (TextPage, error) {
	p := TextPage{Encoding: d.Encoding, SourceSHA256: d.SourceSHA256, Lines: []SourceLine{}, TotalLines: strconv.Itoa(len(d.Lines)), IndexedLines: strconv.Itoa(len(d.Lines)), IndexTruncated: d.IndexTruncated}
	if d.TotalLines != "" {
		p.TotalLines = d.TotalLines
	}
	if start < 0 || count < 1 || count > 200 {
		return p, problem("CASE_PAGE_INVALID", "source page requires start >=0 and count 1..200")
	}
	if start >= len(d.Lines) {
		return p, nil
	}
	budget := 64 << 10
	for i := start; i < len(d.Lines) && i < start+count; i++ {
		line := d.Lines[i]
		from, e := strconv.Atoi(line.TextStart)
		if e != nil {
			return p, e
		}
		to, e := strconv.Atoi(line.TextEnd)
		if e != nil {
			return p, e
		}
		if from < 0 || to < from || to > len(d.Text) {
			return p, problem("CASE_INDEX_INVALID", "invalid source positions")
		}
		text := d.Text[from:to]
		truncated := false
		limit := min(4096, budget)
		if len(text) > limit {
			at := limit
			for at > 0 && !utf8.RuneStart(text[at]) {
				at--
			}
			text = text[:at]
			truncated = true
		}
		p.Lines = append(p.Lines, SourceLine{line, strings.Clone(text), truncated})
		budget -= len(text)
		if budget == 0 {
			break
		}
	}
	return p, nil
}
