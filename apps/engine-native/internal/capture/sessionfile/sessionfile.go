// Package sessionfile exchanges redacted canonical sessions independently of UI
// and analysis. Portable files never contain application-owned blob references.
package sessionfile

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/content"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/httpcapture"
)

const MaxBytes = 256 << 20
const Kind = "archscope-http-session"

type Document struct {
	XMLName      xml.Name                    `json:"-" xml:"ArchScopeHttpSession"`
	Type         string                      `json:"type"`
	Version      int                         `json:"schemaVersion"`
	Scope        string                      `json:"scope"`
	SelectedID   string                      `json:"selectedId,omitempty"`
	Transactions []models.CaptureTransaction `json:"transactions" xml:"Transactions>Transaction"`
}

func Sanitize(doc *Document) (redact.Summary, error) {
	p := redact.NewPolicy(redact.Options{})
	if doc.Type != Kind || doc.Version != 1 {
		return p.Summary(), fmt.Errorf("unsupported session schema")
	}
	if len(doc.Transactions) > 100000 {
		return p.Summary(), fmt.Errorf("session exceeds 100000 exchanges")
	}
	seen := map[string]bool{}
	for i := range doc.Transactions {
		tx := &doc.Transactions[i]
		if tx.ID == "" || seen[tx.ID] {
			return p.Summary(), fmt.Errorf("empty or duplicate exchange ID")
		}
		seen[tx.ID] = true
		if math.IsNaN(tx.TotalMS) || math.IsInf(tx.TotalMS, 0) || tx.TotalMS < 0 {
			return p.Summary(), fmt.Errorf("invalid duration")
		}
		tx.URL = p.RedactURL(tx.URL)
		tx.RedirectURL = p.RedactURL(tx.RedirectURL)
		if u, e := url.Parse(tx.URL); e == nil {
			tx.Host = u.Host
			tx.Path = u.Path
			tx.Query = u.RawQuery
			tx.Scheme = u.Scheme
		}
		tx.Error, _ = p.RedactBody("text/plain", tx.Error)
		tx.Process = p.RedactProcess(tx.Process)
		for j := range tx.Informational {
			tx.Informational[j].Headers = p.RedactHeaders(tx.Informational[j].Headers)
		}
		for _, m := range []*models.HTTPMessage{&tx.Request, &tx.Response} {
			if len(m.BodyPreview) > 10<<20 {
				return p.Summary(), fmt.Errorf("body exceeds 10 MiB")
			}
			m.ImagePreview = content.Clean(m.ImagePreview)
			m.Headers = p.RedactHeaders(m.Headers)
			for j := range m.Cookies {
				m.Cookies[j].Value, m.Cookies[j].Redacted = p.RedactNamedValue("cookie", m.Cookies[j].Value)
				for a := range m.Cookies[j].Attributes {
					m.Cookies[j].Attributes[a].Value, _ = p.RedactNamedValue(m.Cookies[j].Attributes[a].Name, m.Cookies[j].Attributes[a].Value)
				}
			}
			before := m.BodyPreview
			m.BodyPreview, m.Redacted = p.RedactBody(m.ContentType, before)
			if before != "" && m.BodyPreview == "" {
				m.BodyStorage = "redacted"
			}
			m.BodyRef = ""
		}
	}
	if !seen[doc.SelectedID] {
		doc.SelectedID = ""
	}
	return p.Summary(), nil
}
func Read(path string) (Document, redact.Summary, error) {
	doc := Document{}
	file, err := os.Open(path)
	if err != nil {
		return doc, redact.Summary{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return doc, redact.Summary{}, err
	}
	if len(data) > MaxBytes {
		return doc, redact.Summary{}, fmt.Errorf("session exceeds 256 MiB")
	}
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
		// Inspect depth before allocating the typed XML tree.
		decoder := xml.NewDecoder(bytes.NewReader(data))
		depth := 0
		for {
			token, e := decoder.Token()
			if e == io.EOF {
				break
			}
			if e != nil {
				return doc, redact.Summary{}, e
			}
			switch token.(type) {
			case xml.StartElement:
				depth++
			case xml.EndElement:
				depth--
			}
			if depth > 256 {
				return doc, redact.Summary{}, fmt.Errorf("XML depth exceeds 256")
			}
		}
		err = xml.Unmarshal(data, &doc)
	} else {
		// encoding/json bounds recursion; our tighter scanner bounds containers too.
		quoted, escape, depth := false, false, 0
		for _, b := range data {
			if quoted {
				if escape {
					escape = false
				} else if b == '\\' {
					escape = true
				} else if b == '"' {
					quoted = false
				}
				continue
			}
			if b == '"' {
				quoted = true
			}
			if b == '{' || b == '[' {
				depth++
			}
			if b == '}' || b == ']' {
				depth--
			}
			if depth > 256 {
				return doc, redact.Summary{}, fmt.Errorf("JSON depth exceeds 256")
			}
		}
		err = json.Unmarshal(data, &doc)
		if err == nil && doc.Type == "" {
			var har struct {
				Log struct {
					ArchScope *Document `json:"_archscope"`
				} `json:"log"`
			}
			if e := json.Unmarshal(data, &har); e == nil && har.Log.ArchScope != nil {
				doc = *har.Log.ArchScope
			} else {
				parsed, e := parser.ParseFile(path, parser.Options{})
				if e != nil {
					return doc, redact.Summary{}, e
				}
				doc = Document{Type: Kind, Version: 1, Scope: "all", Transactions: parsed.Entries}
			}
		}
	}
	if err != nil {
		return doc, redact.Summary{}, err
	}
	summary, err := Sanitize(&doc)
	return doc, summary, err
}
func Encode(doc Document, format string) ([]byte, error) {
	// Copy through JSON: body omission/redaction must never mutate the open session.
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	doc = Document{}
	if err = json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if _, err = Sanitize(&doc); err != nil {
		return nil, err
	}
	switch format {
	case "session":
		return json.MarshalIndent(doc, "", "  ")
	case "xml_compact":
		for i := range doc.Transactions {
			for _, m := range []*models.HTTPMessage{&doc.Transactions[i].Request, &doc.Transactions[i].Response} {
				m.BodyPreview = ""
				m.BodyRef = ""
				m.BodyStorage = "export_omitted"
				m.ImagePreview = nil
			}
		}
		fallthrough
	case "xml":
		data, e := xml.MarshalIndent(doc, "", "  ")
		return append([]byte(xml.Header), data...), e
	case "har":
		return encodeHAR(doc)
	case "urls":
		var out strings.Builder
		for _, tx := range doc.Transactions {
			out.WriteString(tx.URL)
			out.WriteByte('\n')
		}
		return []byte(out.String()), nil
	case "text", "html", "xls":
		rows := [][]string{{"ID", "Process", "PID", "Method", "URL", "Status", "Duration (ms)", "Started", "Connection", "Request bytes", "Response bytes", "Request headers", "Response headers", "Request retained data", "Response retained data", "Fidelity", "State"}}
		for _, tx := range doc.Transactions {
			name, pid := "", ""
			if tx.Process != nil {
				name = tx.Process.Name
				pid = strconv.Itoa(int(tx.Process.Key.PID))
			}
			headers := func(h []models.HeaderField) string {
				var b strings.Builder
				for _, v := range h {
					fmt.Fprintf(&b, "%s: %s\n", v.Name, v.Value)
				}
				return b.String()
			}
			rows = append(rows, []string{tx.ID, name, pid, tx.Method, tx.URL, strconv.Itoa(tx.StatusCode), strconv.FormatFloat(tx.TotalMS, 'f', 3, 64), tx.StartedAt, tx.ConnectionID, strconv.FormatInt(tx.Request.BodySize, 10), strconv.FormatInt(tx.Response.BodySize, 10), headers(tx.Request.Headers), headers(tx.Response.Headers), tx.Request.BodyPreview, tx.Response.BodyPreview, tx.Fidelity, string(tx.State)})
		}
		if format == "xls" {
			return encodeXLS(rows)
		}
		var out strings.Builder
		if format == "html" {
			out.WriteString("<!doctype html><meta charset=utf-8><title>HTTP session</title><table border=1>")
			for _, row := range rows {
				out.WriteString("<tr>")
				for _, cell := range row {
					out.WriteString("<td><pre>" + html.EscapeString(cell) + "</pre></td>")
				}
				out.WriteString("</tr>")
			}
			out.WriteString("</table>")
		} else {
			for _, row := range rows {
				for i, cell := range row {
					if i > 0 {
						out.WriteByte('\t')
					}
					out.WriteString(strconv.Quote(cell))
				}
				out.WriteByte('\n')
			}
		}
		return []byte(out.String()), nil
	default:
		return nil, fmt.Errorf("unsupported export format %q", format)
	}
}
func Write(ctx context.Context, path string, doc Document, format string) error {
	if path == "" {
		return context.Canceled
	}
	if format == "responses" {
		raw, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		doc = Document{}
		if err = json.Unmarshal(raw, &doc); err != nil {
			return err
		}
		return writeResponses(ctx, path, doc)
	}
	data, err := Encode(doc, format)
	if err != nil {
		return err
	}
	if len(data) > MaxBytes {
		return fmt.Errorf("export exceeds 256 MiB; narrow the scope")
	}
	return atomicWrite(ctx, path, data)
}
func atomicWrite(ctx context.Context, path string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".archscope-export-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
func writeResponses(ctx context.Context, parent string, doc Document) error {
	if _, err := Sanitize(&doc); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(parent, ".archscope-responses-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	type entry struct {
		ID       string `json:"id"`
		File     string `json:"file,omitempty"`
		Storage  string `json:"storage"`
		Encoding string `json:"encoding,omitempty"`
	}
	manifest := struct {
		Provenance string  `json:"provenance"`
		Entries    []entry `json:"entries"`
	}{Provenance: "Retained semantic previews: text redacted, raster pixels unredacted; not original wire bytes"}
	for i, tx := range doc.Transactions {
		m := tx.Response
		e := entry{ID: tx.ID, Storage: m.BodyStorage, Encoding: m.BodyEncoding}
		if m.BodyPreview != "" {
			e.File = fmt.Sprintf("%06d.response.txt", i+1)
			if err = atomicWrite(ctx, filepath.Join(dir, e.File), []byte(m.BodyPreview)); err != nil {
				return err
			}
		}
		if m.ImagePreview != nil {
			e.File = fmt.Sprintf("%06d.preview.png", i+1)
			imageBytes, decodeErr := base64.StdEncoding.DecodeString(m.ImagePreview.Base64)
			if decodeErr != nil {
				return decodeErr
			}
			if err = atomicWrite(ctx, filepath.Join(dir, e.File), imageBytes); err != nil {
				return err
			}
			e.Encoding = "reencoded_png"
		}
		manifest.Entries = append(manifest.Entries, e)
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err = atomicWrite(ctx, filepath.Join(dir, "manifest.json"), data); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// Always create a new child; never overwrite files in the chosen directory.
	return os.Rename(dir, filepath.Join(parent, "http-responses-"+time.Now().UTC().Format("20060102T150405.000000000")))
}
func encodeHAR(doc Document) ([]byte, error) {
	entries := make([]any, 0, len(doc.Transactions))
	for _, tx := range doc.Transactions {
		fields := func(h []models.HeaderField) []map[string]string {
			v := make([]map[string]string, 0, len(h))
			for _, f := range h {
				v = append(v, map[string]string{"name": f.Name, "value": f.Value})
			}
			return v
		}
		cookies := func(h []models.HeaderField) []map[string]any {
			values := make([]map[string]any, 0, len(h))
			for _, cookie := range h {
				value := map[string]any{"name": cookie.Name, "value": cookie.Value}
				for _, attr := range cookie.Attributes {
					key := strings.ToLower(attr.Name)
					if key == "httponly" {
						key = "httpOnly"
					}
					if key == "samesite" {
						key = "sameSite"
					}
					if key == "httpOnly" || key == "secure" {
						value[key] = attr.Value == "true"
					} else {
						value[key] = attr.Value
					}
				}
				values = append(values, value)
			}
			return values
		}
		query := []map[string]string{}
		if u, e := url.Parse(tx.URL); e == nil {
			for _, part := range strings.Split(u.RawQuery, "&") {
				if part == "" {
					continue
				}
				k, v, _ := strings.Cut(part, "=")
				k, _ = url.QueryUnescape(k)
				v, _ = url.QueryUnescape(v)
				query = append(query, map[string]string{"name": k, "value": v})
			}
		}
		request := map[string]any{"method": tx.Method, "url": tx.URL, "httpVersion": tx.HTTPVersion, "headers": fields(tx.Request.Headers), "cookies": cookies(tx.Request.Cookies), "queryString": query, "headersSize": tx.Request.HeaderSize, "bodySize": tx.Request.BodySize}
		if tx.Request.BodyPreview != "" {
			request["postData"] = map[string]any{"mimeType": tx.Request.ContentType, "text": tx.Request.BodyPreview}
		}
		response := map[string]any{"status": tx.StatusCode, "statusText": tx.StatusText, "httpVersion": tx.HTTPVersion, "headers": fields(tx.Response.Headers), "cookies": cookies(tx.Response.Cookies), "redirectURL": tx.RedirectURL, "headersSize": tx.Response.HeaderSize, "bodySize": tx.Response.BodySize, "content": map[string]any{"size": tx.Response.BodyDecoded, "mimeType": tx.Response.ContentType, "text": tx.Response.BodyPreview}}
		if tx.Response.ImagePreview != nil {
			response["content"] = map[string]any{"size": tx.Response.BodyDecoded, "mimeType": "image/png", "text": tx.Response.ImagePreview.Base64, "encoding": "base64"}
		}
		phases := tx.Timings.ImportedHAR
		if phases == nil {
			phases = tx.Timings.ProxyUpstream
		}
		timings := map[string]float64{"blocked": -1, "dns": -1, "connect": -1, "ssl": -1, "send": 0, "wait": 0, "receive": 0}
		if phases != nil {
			for key, value := range map[string]models.Duration{"blocked": phases.Blocked, "dns": phases.DNS, "connect": phases.Connect, "ssl": phases.TLS, "send": phases.Send, "wait": phases.Wait, "receive": phases.Receive} {
				if value.State == models.TimingKnown {
					timings[key] = value.MS
				}
			}
		}
		started := tx.StartedAt
		if started == "" {
			return nil, fmt.Errorf("HAR requires a start timestamp for exchange %s; use native session", tx.ID)
		}
		entries = append(entries, map[string]any{"startedDateTime": started, "time": tx.TotalMS, "request": request, "response": response, "cache": map[string]any{}, "timings": timings, "connection": tx.ConnectionID})
	}
	return json.MarshalIndent(map[string]any{"log": map[string]any{"version": "1.2", "creator": map[string]string{"name": "ArchScope", "version": "1"}, "entries": entries, "_archscope": doc}}, "", "  ")
}
