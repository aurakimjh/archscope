package cases

import (
	"bytes"
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
	Text         string   `json:"text"`
	Encoding     Encoding `json:"encoding"`
	Lines        []Line   `json:"lines"`
	SourceSHA256 string   `json:"source_sha256"`
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
	if len(raw) > MaxObjectBytes {
		return Decoded{}, problem("CASE_BUDGET_EXCEEDED", "source byte limit")
	}
	if len(opts.Sniff) > 2 {
		return Decoded{}, problem("ENCODING_UNSUPPORTED", "at most two permitted legacy sniff candidates")
	}
	for _, candidate := range opts.Sniff {
		n := encodingName(candidate)
		if n != "cp949" && n != "euc-kr" {
			return Decoded{}, problem("ENCODING_UNSUPPORTED", "sniff only accepts permitted legacy candidates")
		}
	}
	name, basis := encodingName(opts.Encoding), "explicit"
	detected, skip := bom(raw)
	if opts.Encoding != "" && name == "" {
		return Decoded{}, problem("ENCODING_UNSUPPORTED", "unsupported explicit encoding")
	}
	if name != "" && detected != "" && name != detected {
		return Decoded{}, problem("ENCODING_BOM_CONFLICT", "explicit encoding conflicts with BOM")
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
					return Decoded{}, problem("ENCODING_UNSUPPORTED", "sniff only accepts permitted legacy candidates")
				}
				if _, e := decodeNamed(raw, n, 0, false); e == nil {
					candidates[n] = true
				}
			}
			if len(candidates) != 1 {
				return Decoded{}, problem("ENCODING_SELECTION_REQUIRED", "legacy encoding is unknown or ambiguous; select explicitly")
			}
			for n := range candidates {
				name = n
			}
			basis = "permitted-sniff"
		}
	}
	result, e := decodeNamed(raw, name, skip, opts.AllowReplacement)
	if e != nil {
		return Decoded{}, e
	}
	result.Encoding.Basis = basis
	result.Encoding.Override = opts.Encoding != ""
	result.SourceSHA256 = hash(raw)
	index, _ := json.Marshal(struct{ Digest, Encoding, Replacements, DecoderRevision string }{result.SourceSHA256, name, result.Encoding.Replacements, "text-index-v1"})
	result.Encoding.IndexRevision = hash(index)
	return result, nil
}
func decodeNamed(raw []byte, name string, skip int, allow bool) (Decoded, error) {
	result := Decoded{Encoding: Encoding{Name: name}, Lines: []Line{}}
	var out strings.Builder
	startRaw, startText := skip, 0
	replaced := uint64(0)
	lastCR := false
	flush := func(end int) error {
		if len(result.Lines) >= 1_000_000 {
			return problem("CASE_BUDGET_EXCEEDED", "source line index limit")
		}
		result.Lines = append(result.Lines, Line{strconv.Itoa(len(result.Lines) + 1), strconv.Itoa(startRaw), strconv.Itoa(end), strconv.Itoa(startText), strconv.Itoa(out.Len())})
		startRaw = end
		startText = out.Len()
		return nil
	}
	for pos := skip; pos < len(raw); {
		r, n, bad := runeAt(raw[pos:], name)
		if n < 1 {
			return Decoded{}, problem("ENCODING_INVALID", "decoder made no progress")
		}
		if bad {
			if !allow {
				return Decoded{}, problem("ENCODING_INVALID", "invalid bytes for selected encoding")
			}
			replaced++
		}
		if lastCR && r != '\n' {
			if e := flush(pos); e != nil {
				return Decoded{}, e
			}
			lastCR = false
		}
		out.WriteRune(r)
		pos += n
		if out.Len() > MaxObjectBytes {
			return Decoded{}, problem("CASE_BUDGET_EXCEEDED", "decoded UTF-8 byte limit")
		}
		if r == '\n' {
			if e := flush(pos); e != nil {
				return Decoded{}, e
			}
			lastCR = false
		} else {
			lastCR = r == '\r'
		}
	}
	if startRaw < len(raw) {
		if e := flush(len(raw)); e != nil {
			return Decoded{}, e
		}
	}
	result.Text = out.String()
	result.Encoding.Replacements = strconv.FormatUint(replaced, 10)
	return result, nil
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
	Encoding     Encoding     `json:"encoding"`
	SourceSHA256 string       `json:"source_sha256"`
	Lines        []SourceLine `json:"lines"`
	TotalLines   string       `json:"total_lines"`
}

func SourcePage(d Decoded, start, count int) (TextPage, error) {
	p := TextPage{Encoding: d.Encoding, SourceSHA256: d.SourceSHA256, Lines: []SourceLine{}, TotalLines: strconv.Itoa(len(d.Lines))}
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
		p.Lines = append(p.Lines, SourceLine{line, text, truncated})
		budget -= len(text)
		if budget == 0 {
			break
		}
	}
	return p, nil
}
