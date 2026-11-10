// Package requestbuilder sends only explicitly composed requests. It shares the
// canonical exchange model but never alters an existing capture or cookie jar.
package requestbuilder

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const MaxRequestBytes = 10 << 20
const MaxResponseBytes = 1 << 20

type Part struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Path        string `json:"path,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}
type Request struct {
	ID             string               `json:"id"`
	Method         string               `json:"method"`
	URL            string               `json:"url"`
	Headers        []models.HeaderField `json:"headers"`
	BodyMode       string               `json:"bodyMode"`
	Body           string               `json:"body"`
	Parts          []Part               `json:"parts"`
	TimeoutSeconds int                  `json:"timeoutSeconds"`
}
type Client struct {
	mu      sync.Mutex
	pending map[string]context.CancelFunc
}

func NewClient() *Client { return &Client{pending: map[string]context.CancelFunc{}} }
func (c *Client) Cancel(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	cancel, ok := c.pending[id]
	if ok {
		cancel()
	}
	return ok
}
func (c *Client) Send(input Request) (models.CaptureTransaction, error) {
	if input.ID == "" || len(input.ID) > 128 {
		return models.CaptureTransaction{}, fmt.Errorf("request ID is required and limited to 128 bytes")
	}
	timeout := input.TimeoutSeconds
	if timeout == 0 {
		timeout = 30
	}
	if timeout < 1 || timeout > 120 {
		return models.CaptureTransaction{}, fmt.Errorf("timeout must be 1–120 seconds")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	c.mu.Lock()
	if _, ok := c.pending[input.ID]; ok {
		c.mu.Unlock()
		return models.CaptureTransaction{}, fmt.Errorf("request is already sending")
	}
	c.pending[input.ID] = cancel
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, input.ID); c.mu.Unlock() }()
	return execute(ctx, input)
}
func execute(ctx context.Context, input Request) (models.CaptureTransaction, error) {
	target, err := url.Parse(input.URL)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil {
		return models.CaptureTransaction{}, fmt.Errorf("enter an absolute HTTP(S) URL without userinfo")
	}
	if len(input.URL) > 16384 || len(input.Headers) > 256 || len(input.Parts) > 128 || len(input.Body) > MaxRequestBytes {
		return models.CaptureTransaction{}, fmt.Errorf("request exceeds editor limits")
	}
	totalPartBytes := 0
	for _, p := range input.Parts {
		totalPartBytes += len(p.Name) + len(p.Value) + len(p.Filename) + len(p.ContentType)
		if totalPartBytes > MaxRequestBytes {
			return models.CaptureTransaction{}, fmt.Errorf("multipart fields exceed 10 MiB")
		}
	}
	body, contentType, err := makeBody(input)
	if err != nil {
		return models.CaptureTransaction{}, err
	}
	req, err := http.NewRequestWithContext(ctx, input.Method, target.String(), bytes.NewReader(body))
	if err != nil {
		return models.CaptureTransaction{}, fmt.Errorf("invalid method or URL")
	}
	headerBytes := 0
	for _, h := range input.Headers {
		headerBytes += len(h.Name) + len(h.Value)
		if headerBytes > 65536 || !validHeaderName(h.Name) || strings.ContainsAny(h.Value, "\r\n\x00") {
			return models.CaptureTransaction{}, fmt.Errorf("invalid or oversized request headers")
		}
		switch strings.ToLower(h.Name) {
		case "content-length", "transfer-encoding", "connection", "proxy-connection", "proxy-authorization", "upgrade", "trailer", "te":
			return models.CaptureTransaction{}, fmt.Errorf("header %s is managed by the HTTP transport", h.Name)
		case "host":
			req.Host = h.Value
		default:
			req.Header.Add(h.Name, h.Value)
		}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	token := make([]byte, 12)
	if _, err = rand.Read(token); err != nil {
		return models.CaptureTransaction{}, err
	}
	started := time.Now()
	tx := models.CaptureTransaction{ID: "builder-" + hex.EncodeToString(token), ConnectionID: "builder-" + input.ID, Sequence: 1, Method: req.Method, URL: target.String(), Scheme: target.Scheme, Host: target.Host, Path: target.Path, HTTPVersion: "HTTP/1.1", StartedAt: started.UTC().Format(time.RFC3339Nano), State: models.TxRequestSent, CaptureMode: "request_builder", ObservationPoint: "request_builder", Coverage: "explicit_request", Fidelity: "semantic", StreamProvenance: "reconstructed_semantic"}
	tx.Request = message(req.Header, body, false)
	tx.Request.BodySize = int64(len(body))
	if req.Host != "" {
		tx.Request.Headers = append(tx.Request.Headers, models.HeaderField{Name: "Host", Value: req.Host})
	}
	// Independent transport: no automatic cookies, redirects, or insecure TLS.
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, MaxResponseHeaderBytes: 1 << 20, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 0}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, sendErr := client.Do(req)
	if sendErr != nil {
		tx.State = models.TxFailed
		tx.Error = sendErr.Error()
	} else {
		defer response.Body.Close()
		tx.HTTPVersion = response.Proto
		tx.StatusCode = response.StatusCode
		tx.StatusText = http.StatusText(response.StatusCode)
		tx.RedirectURL = response.Header.Get("Location")
		var reader io.Reader = response.Body
		encoding := response.Header.Get("Content-Encoding")
		// Go automatically decodes its own negotiated gzip. Explicit gzip also gets
		// a bounded decoded preview; unsupported encodings remain unavailable.
		var gz *gzip.Reader
		if strings.EqualFold(encoding, "gzip") {
			gz, err = gzip.NewReader(response.Body)
			if err == nil {
				defer gz.Close()
				reader = gz
			}
		}
		if err != nil {
			tx.State = models.TxFailed
			tx.Error = "invalid gzip response"
		} else {
			preview, readErr := io.ReadAll(io.LimitReader(reader, MaxResponseBytes+1))
			truncated := len(preview) > MaxResponseBytes
			if truncated {
				preview = preview[:MaxResponseBytes]
			}
			tx.Response = message(response.Header, preview, truncated)
			tx.Response.BodyEncoding = encoding
			tx.Response.BodySize = response.ContentLength
			if encoding != "" && !strings.EqualFold(encoding, "gzip") {
				tx.Response.BodyPreview = ""
				tx.Response.BodyStorage = "encoded"
			}
			if readErr != nil {
				tx.State = models.TxFailed
				tx.Error = readErr.Error()
			} else {
				tx.State = models.TxComplete
			}
		}
	}
	tx.TotalMS = float64(time.Since(started).Microseconds()) / 1000
	tx.EndedAt = time.Now().UTC().Format(time.RFC3339Nano)
	doc := sessionfile.Document{Type: sessionfile.Kind, Version: 1, Transactions: []models.CaptureTransaction{tx}}
	if _, err = sessionfile.Sanitize(&doc); err != nil {
		return models.CaptureTransaction{}, err
	}
	return doc.Transactions[0], nil
}
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}
func message(headers http.Header, body []byte, truncated bool) models.HTTPMessage {
	m := models.HTTPMessage{Headers: []models.HeaderField{}, Cookies: []models.HeaderField{}, HeaderSize: -1, BodySize: int64(len(body)), BodyDecoded: int64(len(body)), TransferSize: -1, ContentType: headers.Get("Content-Type"), BodyStorage: "inline"}
	for name, values := range headers {
		for _, value := range values {
			m.Headers = append(m.Headers, models.HeaderField{Name: name, Value: value})
		}
	}
	if m.ContentType == "" && len(body) > 0 {
		m.ContentType = http.DetectContentType(body)
	}
	if utf8.Valid(body) {
		m.BodyPreview = string(body)
	} else {
		m.BodyStorage = "binary"
	}
	if truncated {
		m.BodyStorage = "truncated"
	}
	return m
}
func makeBody(input Request) ([]byte, string, error) {
	switch input.BodyMode {
	case "", "raw":
		return []byte(input.Body), "", nil
	case "form":
		var b strings.Builder
		for i, p := range input.Parts {
			if i > 0 {
				b.WriteByte('&')
			}
			b.WriteString(url.QueryEscape(p.Name))
			b.WriteByte('=')
			b.WriteString(url.QueryEscape(p.Value))
			if b.Len() > MaxRequestBytes {
				return nil, "", fmt.Errorf("form exceeds 10 MiB")
			}
		}
		return []byte(b.String()), "application/x-www-form-urlencoded", nil
	case "multipart":
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)
		for _, p := range input.Parts {
			if len(p.Name) > 4096 || len(p.Filename) > 4096 {
				return nil, "", fmt.Errorf("multipart name exceeds limit")
			}
			if p.Path == "" {
				if len(p.Value) > MaxRequestBytes-b.Len() {
					return nil, "", fmt.Errorf("multipart exceeds 10 MiB")
				}
				if err := writer.WriteField(p.Name, p.Value); err != nil {
					return nil, "", err
				}
			} else {
				if !filepath.IsAbs(p.Path) {
					return nil, "", fmt.Errorf("choose an absolute upload file")
				}
				file, err := os.Open(p.Path)
				if err != nil {
					return nil, "", fmt.Errorf("upload file is unavailable")
				}
				info, err := file.Stat()
				if err != nil || !info.Mode().IsRegular() || info.Size() > MaxRequestBytes-int64(b.Len()) {
					file.Close()
					return nil, "", fmt.Errorf("upload must be a regular file within the 10 MiB request limit")
				}
				name := filepath.Base(p.Path)
				if p.Filename != "" {
					name = filepath.Base(p.Filename)
				}
				header := textproto.MIMEHeader{}
				header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": p.Name, "filename": name}))
				typ := p.ContentType
				if typ == "" {
					typ = "application/octet-stream"
				}
				if strings.ContainsAny(typ, "\r\n") {
					file.Close()
					return nil, "", fmt.Errorf("invalid part content type")
				}
				header.Set("Content-Type", typ)
				part, e := writer.CreatePart(header)
				if e == nil {
					_, e = io.Copy(part, io.LimitReader(file, MaxRequestBytes-int64(b.Len())+1))
				}
				file.Close()
				if e != nil {
					return nil, "", e
				}
			}
			if b.Len() > MaxRequestBytes {
				return nil, "", fmt.Errorf("multipart exceeds 10 MiB")
			}
		}
		if err := writer.Close(); err != nil {
			return nil, "", err
		}
		if b.Len() > MaxRequestBytes {
			return nil, "", fmt.Errorf("multipart exceeds 10 MiB")
		}
		return b.Bytes(), writer.FormDataContentType(), nil
	default:
		return nil, "", fmt.Errorf("unknown body mode")
	}
}
