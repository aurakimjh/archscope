package halog

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"crypto/sha256"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const bodyLimit = 10 << 20

func timestamp(v value) string {
	if v.tag != 14 {
		return ""
	}
	days := math.Float64frombits(v.n)
	if days <= 0 || days > 2958465 {
		return ""
	}
	seconds := (days - 25569) * 86400
	whole, frac := math.Modf(seconds)
	wall := time.Unix(int64(whole), int64(math.Round(frac*1000))*int64(time.Millisecond)).UTC()
	// OLE DATE has no timezone. Match the native application's interpretation
	// in the current host timezone; the parser discloses this assumption.
	return time.Date(wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), time.Local).Format(time.RFC3339Nano)
}
func oleDate(raw string) value {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return date(0)
	}
	t = t.In(time.Local)
	wall := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	return date(float64(wall.Unix())/86400 + float64(wall.Nanosecond())/86400e9 + 25569)
}
func headerFields(raw string) (string, []models.HeaderField) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	fields := []models.HeaderField{}
	if len(lines) == 0 {
		return "", fields
	}
	for _, line := range lines[1:] {
		name, v, ok := strings.Cut(line, ":")
		if ok && len(fields) < 10000 {
			fields = append(fields, models.HeaderField{Name: strings.TrimSpace(name), Value: strings.TrimSpace(v)})
		}
	}
	return lines[0], fields
}
func header(fields []models.HeaderField, name string) string {
	for _, h := range fields {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}
func stream(v value) ([]byte, bool) {
	if v.object != nil && v.object.class == streamClass {
		return v.object.data, true
	}
	return nil, false
}
func body(m *models.HTTPMessage, raw []byte, known bool, encoding string) (decodedWork int64) {
	m.BodyStorage = "omitted"
	m.BodySize = -1
	m.BodyDecoded = -1
	if !known {
		return
	}
	m.BodySize = int64(len(raw))
	m.BodyEncoding = encoding
	var source io.Reader = bytes.NewReader(raw)
	var closer io.Closer
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "identity":
	case "gzip", "x-gzip":
		r, err := gzip.NewReader(source)
		if err != nil {
			m.BodyStorage = "encoded"
			return
		}
		source = r
		closer = r
	case "deflate":
		r, err := zlib.NewReader(source)
		if err != nil {
			m.BodyStorage = "encoded"
			return
		}
		source = r
		closer = r
	default:
		m.BodyStorage = "encoded"
		return
	}
	if closer != nil {
		defer closer.Close()
	}
	data, err := io.ReadAll(io.LimitReader(source, bodyLimit+1))
	decodedWork = int64(len(data))
	if err != nil || len(data) > bodyLimit {
		m.BodyStorage = "truncated"
		return
	}
	m.BodyDecoded = int64(len(data))
	m.BodyStorage = "inline"
	if utf8.Valid(data) {
		m.BodyPreview = string(data)
	} else {
		m.BodyStorage = "binary"
	}
	return
}
func cookies(m *models.HTTPMessage, request bool) {
	h := http.Header{}
	for _, f := range m.Headers {
		h.Add(f.Name, f.Value)
	}
	var list []*http.Cookie
	if request {
		list = (&http.Request{Header: h}).Cookies()
	} else {
		list = (&http.Response{Header: h}).Cookies()
	}
	for _, c := range list {
		f := models.HeaderField{Name: c.Name, Value: c.Value}
		if c.Path != "" {
			f.Attributes = append(f.Attributes, models.CookieAttribute{Name: "Path", Value: c.Path})
		}
		if c.Domain != "" {
			f.Attributes = append(f.Attributes, models.CookieAttribute{Name: "Domain", Value: c.Domain})
		}
		if c.HttpOnly {
			f.Attributes = append(f.Attributes, models.CookieAttribute{Name: "HttpOnly", Value: "true"})
		}
		if c.Secure {
			f.Attributes = append(f.Attributes, models.CookieAttribute{Name: "Secure", Value: "true"})
		}
		if c.SameSite != 0 {
			f.Attributes = append(f.Attributes, models.CookieAttribute{Name: "SameSite", Value: map[http.SameSite]string{http.SameSiteDefaultMode: "Default", http.SameSiteLaxMode: "Lax", http.SameSiteStrictMode: "Strict", http.SameSiteNoneMode: "None"}[c.SameSite]})
		}
		m.Cookies = append(m.Cookies, f)
	}
}

// Transactions projects native data into bounded semantic evidence. Serialized
// process labels are retained, but PID and lifetime attribution are unavailable.
// Cache paths, socket handles, GUIDs and raw streams never enter the projection.
func (f *File) Transactions() ([]models.CaptureTransaction, error) {
	return f.transactions(MaxBytes)
}

func (f *File) transactions(decodedLimit int64) ([]models.CaptureTransaction, error) {
	decoder, err := charset(f.codepage)
	if err != nil {
		return nil, err
	}
	text := func(v value) string { data, _ := decoder.NewDecoder().Bytes(v.data); return string(data) }
	out := make([]models.CaptureTransaction, 0, len(f.entries))
	decodedBytes := int64(0)
	for i, v := range f.entries {
		o := v.object
		u, err := url.Parse(text(o.get("URL")))
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("HALOG_INVALID: invalid URL in request %d", i+1)
		}
		id := sha256.Sum256(o.get("ObjGUID").data)
		tx := models.CaptureTransaction{ID: fmt.Sprintf("halog-%x-%d", id[:8], i+1), ConnectionID: fmt.Sprintf("halog-request-%d", i+1), Sequence: i + 1, URL: u.String(), Method: text(o.get("OR_Verb")), Scheme: u.Scheme, Host: u.Host, Path: u.Path, Query: u.RawQuery, HTTPVersion: "HTTP/1.1", StartedAt: timestamp(o.get("TimeStart")), EndedAt: timestamp(o.get("TimeEnd")), Starred: numeric(o.get("Starred")) != 0, Comment: text(o.get("Comment")), RedirectURL: text(o.get("RedirectURL")), CaptureMode: "halog_import", ObservationPoint: "foreign_tool", Coverage: "unavailable", Fidelity: "semantic", StreamProvenance: "reconstructed_semantic", State: models.TxRequestSent}
		_, tx.Request.Headers = headerFields(text(o.get("RequestRawHeaders")))
		status, headers := headerFields(text(o.get("ResponseRawHeaders")))
		tx.Response.Headers = headers
		parts := strings.SplitN(status, " ", 3)
		if len(parts) > 1 {
			tx.HTTPVersion = parts[0]
			tx.StatusCode, _ = strconv.Atoi(parts[1])
			if len(parts) > 2 {
				tx.StatusText = parts[2]
			}
		}
		if tx.StatusCode < 0 || tx.StatusCode > 599 {
			return nil, fmt.Errorf("HALOG_INVALID: invalid response status in request %d", i+1)
		}
		for _, m := range []*models.HTTPMessage{&tx.Request, &tx.Response} {
			m.ContentType = header(m.Headers, "Content-Type")
			m.HeaderSize = -1
			m.TransferSize = -1
		}
		post, known := stream(o.extra[3])
		if requestWire, available := stream(o.extra[4]); available && len(requestWire) > 0 {
			request, parseErr := http.ReadRequest(bufio.NewReader(bytes.NewReader(requestWire)))
			if parseErr == nil {
				requestBody, readErr := io.ReadAll(io.LimitReader(request.Body, maxStream+1))
				request.Body.Close()
				if readErr == nil && len(requestBody) <= maxStream && (len(requestBody) > 0 || !known || len(post) == 0) {
					post, known = requestBody, true
				} else if readErr != nil && len(post) == 0 {
					known = false
				}
			} else if len(post) == 0 {
				known = false
			}
		}
		decodedBytes += body(&tx.Request, post, known, header(tx.Request.Headers, "Content-Encoding"))
		raw, known := stream(o.extra[5])
		responseBody := []byte(nil)
		validBody := false
		if known {
			response, parseErr := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), &http.Request{Method: tx.Method})
			if parseErr == nil {
				responseBody, parseErr = io.ReadAll(io.LimitReader(response.Body, maxStream+1))
				response.Body.Close()
				validBody = parseErr == nil && len(responseBody) <= maxStream
			}
		}
		decodedBytes += body(&tx.Response, responseBody, validBody, header(tx.Response.Headers, "Content-Encoding"))
		if known && len(raw) > 0 && !validBody {
			tx.Response.BodyStorage = "truncated"
		}
		for j, m := range []*models.HTTPMessage{&tx.Request, &tx.Response} {
			key := []string{"RequestRawHeaders", "ResponseRawHeaders"}[j]
			m.HeaderSize = int64(len(o.get(key).data))
			cookies(m, j == 0)
		}
		tx.Request.TransferSize = numeric(o.get("Socket_SendSize"))
		tx.Response.TransferSize = numeric(o.get("Socket_RecvSize"))
		if o.extra[2].object != nil {
			t := o.extra[2].object
			start := uint32(numeric(t.get("StartTick")))
			end := uint32(numeric(t.get("RequestComplete")))
			if start != 0 && end != 0 {
				tx.TotalMS = float64(end - start)
			}
			unknown := models.UnknownDuration()
			ph := models.TimingPhases{Blocked: unknown, DNS: unknown, Connect: unknown, TLS: unknown, Send: unknown, Wait: unknown, Receive: unknown}
			phase := func(a, b string) models.Duration {
				av := uint32(numeric(t.get(a)))
				bv := uint32(numeric(t.get(b)))
				if av == 0 || bv == 0 || uint32(bv-av) > 86400000 {
					return unknown
				}
				return models.KnownDuration(float64(bv - av))
			}
			ph.Send = phase("SendFirstByte", "SendLastByte")
			ph.Wait = phase("SendLastByte", "ReceiveFirstByte")
			ph.Receive = phase("ReceiveFirstByte", "ReceiveLastByte")
			tx.Timings.ImportedHAR = &ph
		}
		completeTick := int64(0)
		if o.extra[2].object != nil {
			completeTick = numeric(o.extra[2].object.get("RequestComplete"))
		}
		if numeric(o.get("Request_Stage")) == 4 || completeTick != 0 {
			tx.State = models.TxComplete
			if tx.StatusCode == 0 || o.get("SR_Result").tag == 0 {
				tx.State = models.TxFailed
				tx.Error = "Legacy request failed or closed without a final HTTP response"
			}
		}
		if name := text(o.get("ProcessName")); name != "" {
			name = strings.ReplaceAll(name, "\\", "/")
			name = name[strings.LastIndex(name, "/")+1:]
			if bracket := strings.LastIndex(name, "["); bracket >= 0 && strings.HasSuffix(name, "]") {
				if _, err := strconv.ParseUint(name[bracket+1:len(name)-1], 10, 32); err == nil {
					name = name[:bracket]
				}
			}
			tx.Process = &models.ProcessInstance{Name: name, Attribution: "foreign_tool"}
		}
		out = append(out, tx)
		// Charge binary and truncated decompression too, even when no preview
		// survives. Otherwise many discarded compressed bodies bypass the budget.
		if decodedBytes > decodedLimit {
			return nil, fmt.Errorf("HALOG_LIMIT: total decoded body work exceeds %d bytes", decodedLimit)
		}
	}
	return out, nil
}

func objectValue(class string, fields []field, extra ...value) value {
	return value{tag: 19, object: &object{class: class, fields: fields, extra: extra}}
}
func memory(data []byte) value {
	return value{tag: 19, object: &object{class: streamClass, data: data}}
}
func nativeString(s string, cp int) (value, error) {
	b, err := legacyEncode(s, cp)
	if err != nil {
		return value{}, err
	}
	return str(string(b)), nil
}
func rawHeaders(line string, m models.HTTPMessage, retained []byte) ([]byte, error) {
	if strings.ContainsAny(m.ContentType, "\r\n\x00") {
		return nil, fmt.Errorf("HALOG_EXPORT: invalid content type")
	}
	var b strings.Builder
	b.WriteString(line)
	b.WriteString("\r\n")
	for _, h := range m.Headers {
		if strings.ContainsAny(h.Name+h.Value, "\r\n\x00") || h.Name == "" || strings.ContainsAny(h.Name, ": \t") {
			return nil, fmt.Errorf("HALOG_EXPORT: invalid header")
		}
		if strings.EqualFold(h.Name, "Content-Length") || strings.EqualFold(h.Name, "Content-Encoding") || strings.EqualFold(h.Name, "Transfer-Encoding") {
			continue
		}
		b.WriteString(h.Name)
		b.WriteString(": ")
		b.WriteString(h.Value)
		b.WriteString("\r\n")
	}
	if header(m.Headers, "Content-Type") == "" && m.ContentType != "" {
		b.WriteString("Content-Type: ")
		b.WriteString(m.ContentType)
		b.WriteString("\r\n")
	}
	fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n", len(retained))
	return []byte(b.String()), nil
}

// EncodeTransactions writes sanitized semantic exchanges readable by native V7.
// It reconstructs HTTP framing, retains text previews, and explicitly annotates
// omitted data. Live IDs, actual OS attribution and raw-wire fidelity are not
// representable in this legacy format and are never claimed on reimport.
func EncodeTransactions(entries []models.CaptureTransaction) ([]byte, error) {
	return EncodeTransactionsCodePage(entries, defaultCodePage())
}

func EncodeTransactionsCodePage(entries []models.CaptureTransaction, codepage int) ([]byte, error) {
	if _, err := charset(codepage); err != nil {
		return nil, err
	}
	if len(entries) > MaxEntries {
		return nil, fmt.Errorf("HALOG_LIMIT: too many requests")
	}
	f := &File{attributes: []field{{"TKey", str("0A0027000003")}, {"homepage", str("https://github.com/aurakimjh/archscope")}, {"version", value{tag: 6, n: 6}}, {"product", str("ArchScope")}, {"ArchScopeCodePage", integer(int64(codepage))}}}
	for i, tx := range entries {
		if math.IsNaN(tx.TotalMS) || math.IsInf(tx.TotalMS, 0) || tx.TotalMS < 0 || tx.TotalMS > 86400000 {
			return nil, fmt.Errorf("HALOG_EXPORT: invalid duration")
		}
		u, err := url.Parse(tx.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, fmt.Errorf("HALOG_EXPORT: invalid URL")
		}
		if tx.Method == "" || strings.ContainsAny(tx.Method, " \r\n\x00\t") {
			return nil, fmt.Errorf("HALOG_EXPORT: invalid method")
		}
		post := []byte(tx.Request.BodyPreview)
		response := []byte(tx.Response.BodyPreview)
		closed := tx.State == models.TxComplete || tx.State == models.TxFailed || tx.State == models.TxAborted
		if !closed && tx.StatusCode == 0 {
			response = nil
		}
		if len(post) > bodyLimit || len(response) > bodyLimit {
			return nil, fmt.Errorf("HALOG_LIMIT: retained body exceeds 10 MiB")
		}
		version := tx.HTTPVersion
		if version != "HTTP/1.0" && version != "HTTP/1.1" {
			version = "HTTP/1.1"
		}
		req, err := rawHeaders(tx.Method+" "+u.RequestURI()+" "+version, tx.Request, post)
		if err != nil {
			return nil, err
		}
		status := tx.StatusCode
		if status < 0 || status > 599 {
			return nil, fmt.Errorf("HALOG_EXPORT: invalid status")
		}
		statusText := tx.StatusText
		if statusText == "" {
			statusText = http.StatusText(status)
		}
		if strings.ContainsAny(statusText, "\r\n\x00") {
			return nil, fmt.Errorf("HALOG_EXPORT: invalid status text")
		}
		res, err := rawHeaders(fmt.Sprintf("%s %03d %s", version, status, statusText), tx.Response, response)
		if err != nil {
			return nil, err
		}
		if !closed && status == 0 {
			res = nil
		}
		process := ""
		if tx.Process != nil {
			process = tx.Process.Name
		}
		comment := tx.Comment + "\nArchScope semantic export; reconstructed streams, unverified OS attribution."
		if tx.Request.BodyStorage != "inline" || tx.Response.BodyStorage != "inline" {
			comment += " Non-retained/binary data omitted."
		}
		if len(tx.Informational) > 0 {
			comment += " Interim responses omitted."
		}
		reqValue, err := nativeString(string(req), codepage)
		if err != nil {
			return nil, err
		}
		resValue, err := nativeString(string(res), codepage)
		if err != nil {
			return nil, err
		}
		req = reqValue.data
		res = resValue.data
		stringsToWrite := []struct{ name, text string }{{"URL", tx.URL}, {"OR_Verb", tx.Method}, {"OR_ObjectName", u.RequestURI()}, {"ProcessName", process}, {"Comment", comment}, {"RedirectURL", tx.RedirectURL}}
		fields := []field{{"Version", value{tag: 6, n: 7}}, {"Request_Stage", value{tag: 6, n: 4}}, {"SR_Result", boolean(tx.State != models.TxFailed && tx.State != models.TxAborted)}, {"SR_ErrorCode", cardinal(0)}, {"Socket_SendSize", cardinal(int64(len(req) + len(post)))}, {"Socket_RecvSize", cardinal(int64(len(res) + len(response)))}, {"TimeStart", oleDate(tx.StartedAt)}, {"TimeEnd", oleDate(tx.EndedAt)}, {"Starred", value{tag: 6}}, {"ObjGUID", str(fmt.Sprintf("{00000000-0000-4000-8000-%012x}", i+1))}, {"ResponseContent_ReadLength", value{tag: 5, n: uint64(len(response))}}}
		if tx.Starred {
			fields[8].value.n = 1
		}
		if !closed {
			fields[1].value = value{tag: 6, n: 1}
			fields[2].value = boolean(false)
			fields[7].value = date(0)
		}
		for _, s := range stringsToWrite {
			v, err := nativeString(s.text, codepage)
			if err != nil {
				return nil, err
			}
			fields = append(fields, field{s.name, v})
		}
		fields = append(fields, field{"RequestRawHeaders", reqValue}, field{"ResponseRawHeaders", resValue})
		for _, name := range []string{"Socket_Handle", "ConnectHandle", "Chunks", "OriginCallBack", "RequestHandle"} {
			fields = append(fields, field{name, integer(0)})
		}
		for _, name := range []string{"SR_OptionalLength", "IE_PageID"} {
			fields = append(fields, field{name, cardinal(0)})
		}
		for _, name := range []string{"RequestCameFromCache", "ResponseCameFromCache", "IsAsyncConnect"} {
			fields = append(fields, field{name, boolean(false)})
		}
		for _, name := range []string{"URL_SignPart", "Socket_LocalAddr", "Socket_RemoteAddr", "PageTitle", "SR_ErrorMessage"} {
			fields = append(fields, field{name, str("")})
		}
		// V7's property loader requires the native primitive types. Keep the
		// observed property order as well, including the schema marker position.
		byName := object{fields: fields}
		fields = nil
		for _, name := range []string{"Request_Stage", "SR_ErrorCode", "Socket_RecvSize", "SR_Result", "Socket_Handle", "Socket_SendSize", "RequestCameFromCache", "SR_OptionalLength", "Starred", "Version", "TimeStart", "ResponseRawHeaders", "Comment", "ObjGUID", "RequestRawHeaders", "ResponseCameFromCache", "OR_ObjectName", "Socket_RemoteAddr", "TimeEnd", "PageTitle", "OR_Verb", "URL_SignPart", "ResponseContent_ReadLength", "ProcessName", "ConnectHandle", "URL", "Socket_LocalAddr", "Chunks", "RedirectURL", "SR_ErrorMessage", "IE_PageID", "IsAsyncConnect", "OriginCallBack", "RequestHandle"} {
			fields = append(fields, field{name, byName.get(name)})
		}
		timing := []field{}
		ticks := map[string]int64{"StartTick": 1000, "RequestComplete": 1000 + int64(math.Round(tx.TotalMS))}
		if !closed {
			ticks["RequestComplete"] = 0
		}
		phases := tx.Timings.ImportedHAR
		if phases == nil {
			phases = tx.Timings.ProxyUpstream
		}
		if phases != nil {
			valid := func(d models.Duration) bool {
				return d.State == models.TimingKnown && !math.IsNaN(d.MS) && !math.IsInf(d.MS, 0) && d.MS >= 0 && d.MS <= 86400000
			}
			if valid(phases.Send) {
				ticks["SendFirstByte"] = 1000
				ticks["SendLastByte"] = 1000 + int64(math.Round(phases.Send.MS))
				if valid(phases.Wait) {
					ticks["ReceiveFirstByte"] = ticks["SendLastByte"] + int64(math.Round(phases.Wait.MS))
					if valid(phases.Receive) {
						ticks["ReceiveLastByte"] = ticks["ReceiveFirstByte"] + int64(math.Round(phases.Receive.MS))
					}
				}
			}
		}
		for _, name := range []string{"RequestComplete", "ReceiveLastByte", "SendFirstByte", "DNSLookUpStart", "ReceiveFirstByte", "SendLastByte", "StartTick", "ConnectStart"} {
			timing = append(timing, field{name, cardinal(ticks[name])})
		}
		cacheFields := []field{{"LastSyncTime", value{tag: 17}}, {"HeaderInfo", str("")}, {"FileExtension", str("")}, {"LocalFileName", str("")}, {"ExemptDelta", cardinal(0)}, {"EntryType", cardinal(0)}, {"URLInCache", boolean(false)}, {"HitRate", cardinal(0)}, {"ExpireTime", value{tag: 17}}, {"LastModifiedTime", value{tag: 17}}, {"LastAccessTime", value{tag: 17}}, {"FileSize", cardinal(0)}, {"SourceUrlName", str("")}, {"UseCount", cardinal(0)}}
		cache := objectValue(cacheClass, cacheFields)
		f.entries = append(f.entries, objectValue(requestClass, fields, cache, cache, objectValue(timingClass, timing), memory(post), memory(append(req, post...)), memory(append(res, response...))))
	}
	return f.Encode()
}
