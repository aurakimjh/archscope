package httpcapture

import (
	"fmt"
	"net/url"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/halog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const FormatHALOG = "halog"

func parseNative(payload []byte, path string, opts Options, info inputInfo) (ParseResult, error) {
	d := diagnostics.New(FormatHALOG)
	d.SetSourceFile(path)
	result := ParseResult{Format: FormatHALOG, Dialect: "ieinspector-v7-native-v6", Diagnostics: d, InputBytes: info.compressedBytes, DecompressedBytes: int64(len(payload))}
	f, err := halog.Decode(payload)
	if err != nil {
		return result, err
	}
	rows, err := f.Transactions()
	if err != nil {
		return result, err
	}
	if len(rows) > opts.MaxEntries {
		return result, fmt.Errorf("HALOG_LIMIT: request count exceeds configured limit")
	}
	policy := redact.NewPolicy(redact.Options{CustomPatterns: opts.CustomRedactionPatterns})
	times := []string{}
	omitted := false
	for i := range rows {
		x := &rows[i]
		if len(x.URL) > opts.MaxStringBytes || len(x.Method) > opts.MaxStringBytes || len(x.RedirectURL) > opts.MaxStringBytes {
			return result, fmt.Errorf("HALOG_LIMIT: metadata exceeds configured string limit")
		}
		x.URL = policy.RedactURL(x.URL)
		x.RedirectURL = policy.RedactURL(x.RedirectURL)
		if u, err := url.Parse(x.URL); err == nil {
			x.Host = u.Host
			x.Path = u.Path
			x.Query = u.RawQuery
		}
		x.Comment, _ = policy.RedactBody("text/plain", x.Comment)
		x.Error, _ = policy.RedactBody("text/plain", x.Error)
		x.Process = policy.RedactProcess(x.Process)
		if len(x.Comment) > 4096 {
			x.Comment = x.Comment[:4096]
			for !utf8.ValidString(x.Comment) {
				x.Comment = x.Comment[:len(x.Comment)-1]
			}
			d.AddWarning(i+1, "HALOG_COMMENT_TRUNCATED", "Legacy comment exceeds 4096 bytes", "", false)
		}
		for _, m := range []*models.HTTPMessage{&x.Request, &x.Response} {
			if m.BodySize > int64(opts.MaxBodyBytes) || m.BodyDecoded > int64(opts.MaxBodyBytes) {
				return result, fmt.Errorf("HALOG_LIMIT: body exceeds configured limit")
			}
			if m.BodySize > 0 && m.BodyDecoded > m.BodySize*int64(opts.MaxDecompressionRatio) {
				return result, fmt.Errorf("HALOG_LIMIT: body expansion exceeds configured limit")
			}
			for _, h := range m.Headers {
				if len(h.Name) > opts.MaxStringBytes || len(h.Value) > opts.MaxStringBytes {
					return result, fmt.Errorf("HALOG_LIMIT: header exceeds configured string limit")
				}
			}
			m.Headers = policy.RedactHeaders(m.Headers)
			for j := range m.Cookies {
				m.Cookies[j].Value, m.Cookies[j].Redacted = policy.RedactNamedValue("cookie", m.Cookies[j].Value)
				for k := range m.Cookies[j].Attributes {
					a := &m.Cookies[j].Attributes[k]
					a.Value, _ = policy.RedactNamedValue(a.Name, a.Value)
				}
			}
			before := m.BodyPreview
			m.BodyPreview, m.Redacted = policy.RedactBody(m.ContentType, before)
			if before != "" && m.BodyPreview == "" {
				m.BodyStorage = "redacted"
			}
			omitted = omitted || m.BodyStorage != "inline"
		}
		if x.StartedAt != "" {
			times = append(times, x.StartedAt)
		}
	}
	d.TotalLines = len(rows)
	d.ParsedRecords = len(rows)
	d.AddWarning(0, "HALOG_SEMANTIC_IMPORT", "Native format 6/request schema 7; serialized process labels do not verify an OS PID/lifetime. Raw streams/cache paths are not retained in the semantic projection.", "", false)
	d.AddWarning(0, "HALOG_TIMEZONE_ASSUMED", "Native timestamps contain no timezone; interpreted in the current host timezone, as in the reference application.", "", false)
	if omitted {
		d.AddWarning(0, "HALOG_CONTENT_UNAVAILABLE", "Some bodies were absent, incomplete, binary, or used an unsupported encoding; unavailable payload is explicitly labeled.", "", false)
	}
	for _, w := range policy.Warnings() {
		d.AddWarning(0, w.Code, w.Message, "", false)
	}
	result.Entries = rows
	result.Redaction = policy.Summary()
	result.TimelineAvailable = !timestampsDegenerate(times, len(rows))
	return result, nil
}
