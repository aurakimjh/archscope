package halog

import (
	"fmt"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/korean"
	textunicode "golang.org/x/text/encoding/unicode"
	"unicode/utf8"
)

// V7 persists ANSI variants, not wide-string variants. The native application
// interprets these using its Windows ANSI codepage. ArchScope exports also carry
// an advisory codepage attribute so its own imports remain portable.
func charset(codepage int) (encoding.Encoding, error) {
	switch codepage {
	case 1252:
		return charmap.Windows1252, nil
	case 949:
		return korean.EUCKR, nil
	case 65001:
		return textunicode.UTF8, nil
	default:
		return nil, fmt.Errorf("HALOG_ENCODING: unsupported ANSI codepage %d; choose 1252, 949 or 65001", codepage)
	}
}
func legacyEncode(s string, codepage int) ([]byte, error) {
	if !utf8.ValidString(s) {
		return nil, fmt.Errorf("HALOG_ENCODING: invalid UTF-8 metadata")
	}
	e, err := charset(codepage)
	if err != nil {
		return nil, err
	}
	b, err := e.NewEncoder().Bytes([]byte(s))
	if err != nil {
		return nil, fmt.Errorf("HALOG_ENCODING: metadata is not representable in codepage %d; use HAR/session or another codepage", codepage)
	}
	return b, nil
}
