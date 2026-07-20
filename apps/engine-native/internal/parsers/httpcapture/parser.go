// Package httpcapture parses untrusted HAR exports into a redacted,
// dialect-labelled HTTP evidence model. It is deliberately file-only: live
// proxy capture is a later capability and must feed the same model.
package httpcapture

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	coreprofiler "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

const (
	FormatAuto           = "auto"
	FormatHAR            = "har"
	ReasonInvalidHAR     = "INVALID_HAR"
	ReasonMissingEntries = "HAR_MISSING_ENTRIES"
)

type Options struct {
	Format     string
	MaxEntries int
}
type ParseResult struct {
	Format, Dialect string
	Entries         []Entry
	Diagnostics     *diagnostics.ParserDiagnostics
}
type Entry struct {
	StartedAt                                                          time.Time
	Method, URL, Host, Path, StatusText, MIMEType, Process             string
	Status                                                             int
	DurationMS, BlockedMS, DNSMS, ConnectMS, SendMS, WaitMS, ReceiveMS float64
	RequestBytes, ResponseBytes                                        int64
	Error                                                              bool
	Fidelity                                                           string
}

type har struct {
	Log struct {
		Creator struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"creator"`
		Browser struct {
			Name string `json:"name"`
		} `json:"browser"`
		Entries []harEntry `json:"entries"`
	} `json:"log"`
}
type harEntry struct {
	StartedDateTime string  `json:"startedDateTime"`
	Time            float64 `json:"time"`
	Request         struct {
		Method      string `json:"method"`
		URL         string `json:"url"`
		HeadersSize int64  `json:"headersSize"`
		BodySize    int64  `json:"bodySize"`
	} `json:"request"`
	Response struct {
		Status      int    `json:"status"`
		StatusText  string `json:"statusText"`
		HeadersSize int64  `json:"headersSize"`
		BodySize    int64  `json:"bodySize"`
		Content     struct {
			MIMEType string `json:"mimeType"`
			Size     int64  `json:"size"`
		} `json:"content"`
	} `json:"response"`
	Timings struct {
		Blocked, DNS, Connect, Send, Wait, Receive float64 `json:"-"`
	} `json:"timings"`
}

func (t *harEntry) UnmarshalJSON(data []byte) error {
	type alias harEntry
	var raw struct {
		*alias
		Timings map[string]json.RawMessage `json:"timings"`
	}
	raw.alias = (*alias)(t)
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for key, target := range map[string]*float64{"blocked": &t.Timings.Blocked, "dns": &t.Timings.DNS, "connect": &t.Timings.Connect, "send": &t.Timings.Send, "wait": &t.Timings.Wait, "receive": &t.Timings.Receive} {
		if value, ok := raw.Timings[key]; ok {
			_ = json.Unmarshal(value, target)
		}
	}
	return nil
}

func ParseFile(path string, opts Options) (ParseResult, error) {
	diags := diagnostics.New(FormatHAR)
	diags.SetSourceFile(path)
	f, err := os.Open(path)
	if err != nil {
		return ParseResult{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.UseNumber()
	var payload har
	if err := dec.Decode(&payload); err != nil {
		diags.AddError(0, ReasonInvalidHAR, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags}, err
	}
	if payload.Log.Entries == nil {
		err := fmt.Errorf("HAR log.entries is required")
		diags.AddError(0, ReasonMissingEntries, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags}, err
	}
	limit := opts.MaxEntries
	if limit <= 0 {
		limit = 100_000
	}
	entries := make([]Entry, 0, min(limit, len(payload.Log.Entries)))
	for i, raw := range payload.Log.Entries {
		if len(entries) >= limit {
			diags.AddWarning(i+1, "HAR_ENTRY_CAP", fmt.Sprintf("entry cap %d reached", limit), "", false)
			break
		}
		entry, err := normalizeEntry(raw)
		if err != nil {
			diags.AddSkipped(i+1, ReasonInvalidHAR, err.Error(), "")
			continue
		}
		entries = append(entries, entry)
		diags.ParsedRecords++
	}
	diags.TotalLines = len(payload.Log.Entries)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].StartedAt.Before(entries[j].StartedAt) })
	return ParseResult{Format: FormatHAR, Dialect: detectDialect(payload.Log.Creator.Name, payload.Log.Browser.Name), Entries: entries, Diagnostics: diags}, nil
}

func normalizeEntry(raw harEntry) (Entry, error) {
	if strings.TrimSpace(raw.Request.URL) == "" {
		return Entry{}, fmt.Errorf("entry request.url is required")
	}
	url := coreprofiler.RedactText(raw.Request.URL).Text
	host, path := hostAndPath(url)
	started, _ := time.Parse(time.RFC3339Nano, raw.StartedDateTime)
	responseBytes := raw.Response.BodySize
	if responseBytes < 0 {
		responseBytes = raw.Response.Content.Size
	}
	if responseBytes < 0 {
		responseBytes = 0
	}
	requestBytes := raw.Request.BodySize
	if requestBytes < 0 {
		requestBytes = 0
	}
	return Entry{StartedAt: started, Method: strings.ToUpper(strings.TrimSpace(raw.Request.Method)), URL: url, Host: host, Path: path, Status: raw.Response.Status, StatusText: raw.Response.StatusText, MIMEType: raw.Response.Content.MIMEType, DurationMS: max0(raw.Time), BlockedMS: known(raw.Timings.Blocked), DNSMS: known(raw.Timings.DNS), ConnectMS: known(raw.Timings.Connect), SendMS: known(raw.Timings.Send), WaitMS: known(raw.Timings.Wait), ReceiveMS: known(raw.Timings.Receive), RequestBytes: requestBytes, ResponseBytes: responseBytes, Error: raw.Response.Status >= 400, Fidelity: "har_import"}, nil
}
func detectDialect(creator, browser string) string {
	value := strings.ToLower(creator + " " + browser)
	for _, pair := range []struct{ term, id string }{{"chrome", "chrome"}, {"firefox", "firefox"}, {"safari", "safari"}, {"charles", "charles"}, {"fiddler", "fiddler"}, {"proxyman", "proxyman"}, {"insomnia", "insomnia"}} {
		if strings.Contains(value, pair.term) {
			return pair.id
		}
	}
	return "generic"
}
func hostAndPath(raw string) (string, string) {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	parts := strings.SplitN(trimmed, "/", 2)
	host := parts[0]
	path := "/"
	if len(parts) > 1 {
		path += "/" + parts[1]
	}
	return host, path
}
func known(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
func max0(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
