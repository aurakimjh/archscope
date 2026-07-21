// Package httpcapture parses untrusted HAR exports through a bounded staged
// pipeline. It is deliberately file-only: live proxy capture is a later group
// and must feed the same models.CaptureTransaction contract.
package httpcapture

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	FormatAuto = "auto"
	FormatHAR  = "har"

	ReasonInvalidHAR      = "INVALID_HAR"
	ReasonNotHAR          = "NOT_HAR"
	ReasonStructural      = "HAR_STRUCTURAL_ERROR"
	ReasonResourceLimit   = "HAR_RESOURCE_LIMIT"
	ReasonSchemaViolation = "HAR_SCHEMA_VIOLATION"
	ReasonURLUnparsable   = "HAR_URL_UNPARSABLE"
)

var (
	ErrNotHAR        = errors.New("input is not a HAR document")
	ErrStructuralHAR = errors.New("HAR structure is invalid")
	ErrResourceLimit = errors.New("HAR resource limit exceeded")
)

type Options struct {
	Format                  string
	MaxEntries              int
	MaxBytes                int64
	MaxStringBytes          int
	MaxBodyBytes            int
	MaxDepth                int
	MaxFields               int
	MaxDecompressionRatio   int
	CustomRedactionPatterns []string
}

type ParseResult struct {
	Format            string
	Dialect           string
	Entries           []models.CaptureTransaction
	Diagnostics       *diagnostics.ParserDiagnostics
	Redaction         redact.Summary
	TimelineAvailable bool
	InputBytes        int64
	DecompressedBytes int64
}

// Entry remains an alias for the common transaction model so the aggregation
// package and external callers cannot drift onto a parser-private shape.
type Entry = models.CaptureTransaction

type harLogEnvelope struct {
	Version string          `json:"version"`
	Creator harCreator      `json:"creator"`
	Browser harCreator      `json:"browser"`
	Entries json.RawMessage `json:"entries"`
}

type harCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type harEntry struct {
	PageRef         string          `json:"pageref"`
	StartedDateTime string          `json:"startedDateTime"`
	Time            *float64        `json:"time"`
	Request         harRequest      `json:"request"`
	Response        harResponse     `json:"response"`
	Cache           json.RawMessage `json:"cache"`
	Timings         harTimings      `json:"timings"`
	ServerIPAddress string          `json:"serverIPAddress"`
	Connection      string          `json:"connection"`
	ArchScope       harArchScope    `json:"_archscope"`
}

type harRequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	HTTPVersion string         `json:"httpVersion"`
	Headers     []harNameValue `json:"headers"`
	QueryString []harNameValue `json:"queryString"`
	Cookies     []harCookie    `json:"cookies"`
	HeadersSize *int64         `json:"headersSize"`
	BodySize    *int64         `json:"bodySize"`
	PostData    *harPostData   `json:"postData"`
}

type harResponse struct {
	Status       int            `json:"status"`
	StatusText   string         `json:"statusText"`
	HTTPVersion  string         `json:"httpVersion"`
	Headers      []harNameValue `json:"headers"`
	Cookies      []harCookie    `json:"cookies"`
	Content      harContent     `json:"content"`
	RedirectURL  string         `json:"redirectURL"`
	HeadersSize  *int64         `json:"headersSize"`
	BodySize     *int64         `json:"bodySize"`
	TransferSize *int64         `json:"_transferSize"`
}

type harNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harCookie struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type harPostData struct {
	MIMEType string         `json:"mimeType"`
	Text     string         `json:"text"`
	Params   []harNameValue `json:"params"`
}

type harContent struct {
	Size        *int64   `json:"size"`
	Compression *float64 `json:"compression"`
	MIMEType    string   `json:"mimeType"`
	Text        *string  `json:"text"`
	Encoding    string   `json:"encoding"`
}

type harTimings struct {
	Blocked *float64 `json:"blocked"`
	DNS     *float64 `json:"dns"`
	Connect *float64 `json:"connect"`
	SSL     *float64 `json:"ssl"`
	Send    *float64 `json:"send"`
	Wait    *float64 `json:"wait"`
	Receive *float64 `json:"receive"`
}

type harArchScope struct {
	Process struct {
		PID         int32  `json:"pid"`
		StartTime   string `json:"startTime"`
		Name        string `json:"name"`
		ExecPath    string `json:"execPath"`
		CommandLine string `json:"commandLine"`
		User        string `json:"user"`
		ParentPID   int32  `json:"parentPid"`
		Attribution string `json:"attribution"`
	} `json:"process"`
}

func ParseFile(path string, opts Options) (result ParseResult, err error) {
	effective := normalizeOptions(opts)
	diags := diagnostics.New(FormatHAR)
	diags.SetSourceFile(path)
	defer recoverParsePanic(&result, &err, diags)
	if effective.Format != FormatAuto && effective.Format != FormatHAR {
		err := fmt.Errorf("unsupported HTTP capture format %q", effective.Format)
		diags.AddError(0, ReasonInvalidHAR, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags}, err
	}

	payload, inputInfo, err := readBoundedInput(path, effective)
	if err != nil {
		diags.AddError(0, ReasonResourceLimit, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags}, err
	}
	if err := preflightJSON(payload, effective); err != nil {
		diags.AddError(0, ReasonResourceLimit, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
	}

	var root map[string]json.RawMessage
	if err := decodeOne(payload, &root); err != nil {
		diags.AddError(0, ReasonInvalidHAR, safeJSONError(err), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
	}
	logRaw, ok := root["log"]
	if !ok {
		err := ErrNotHAR
		if _, netlogEvents := root["events"]; netlogEvents {
			err = fmt.Errorf("%w: Chrome NetLog is not HAR", ErrNotHAR)
		}
		diags.AddError(0, ReasonNotHAR, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
	}

	var logEnvelope harLogEnvelope
	if err := decodeOne(logRaw, &logEnvelope); err != nil {
		err = fmt.Errorf("%w: log must be an object", ErrStructuralHAR)
		diags.AddError(0, ReasonStructural, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
	}
	entriesRaw := bytes.TrimSpace(logEnvelope.Entries)
	if len(entriesRaw) == 0 || entriesRaw[0] != '[' {
		err := fmt.Errorf("%w: log.entries must be an array", ErrStructuralHAR)
		diags.AddError(0, ReasonStructural, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
	}
	var rawEntries []json.RawMessage
	if err := decodeOne(entriesRaw, &rawEntries); err != nil {
		err = fmt.Errorf("%w: log.entries cannot be decoded", ErrStructuralHAR)
		diags.AddError(0, ReasonStructural, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
	}
	if len(rawEntries) > effective.MaxEntries {
		err := fmt.Errorf("%w: entry count %d exceeds limit %d", ErrResourceLimit, len(rawEntries), effective.MaxEntries)
		diags.AddError(0, ReasonResourceLimit, err.Error(), "")
		return ParseResult{Format: FormatHAR, Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
	}

	dialect := DetectDialect(logEnvelope.Creator.Name, logEnvelope.Browser.Name)
	features := FeaturesFor(dialect)
	policy := redact.NewPolicy(redact.Options{CustomPatterns: effective.CustomRedactionPatterns})
	seenDiagnostics := map[string]struct{}{}
	if dialect == DialectGeneric {
		addDiagnostic(diags, seenDiagnostics, "HAR_DIALECT_UNKNOWN", "HAR creator is unknown; generic normalization applied")
	}
	for _, warning := range policy.Warnings() {
		addDiagnostic(diags, seenDiagnostics, warning.Code, warning.Message)
	}

	entries := make([]models.CaptureTransaction, 0, len(rawEntries))
	sequences := map[string]int{}
	validTimes := make([]string, 0, len(rawEntries))
	missingBodies := 0
	unavailableSizes := 0
	missingTimings := false
	schemaViolation := false
	redirectsUnlinkable := false
	negativeTiming := false
	urlUnparsable := false

	for index, raw := range rawEntries {
		if missingRequiredHARFields(raw) {
			schemaViolation = true
		}
		var source harEntry
		if err := decodeOne(raw, &source); err != nil {
			err = fmt.Errorf("%w: entry %d has invalid field types", ErrStructuralHAR, index+1)
			diags.AddError(index+1, ReasonStructural, err.Error(), "")
			return ParseResult{Format: FormatHAR, Dialect: string(dialect), Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
		}
		if bodyInputBytes(source) > effective.MaxBodyBytes {
			err = fmt.Errorf("%w: entry %d body exceeds limit %d", ErrResourceLimit, index+1, effective.MaxBodyBytes)
			diags.AddError(index+1, ReasonResourceLimit, err.Error(), "")
			return ParseResult{Format: FormatHAR, Dialect: string(dialect), Diagnostics: diags, InputBytes: inputInfo.compressedBytes, DecompressedBytes: int64(len(payload))}, err
		}
		connectionID := strings.TrimSpace(source.Connection)
		if connectionID == "" {
			connectionID = "har:" + strconv.Itoa(index+1)
		}
		sequence := sequences[connectionID]
		sequences[connectionID] = sequence + 1
		transaction, signals := mapEntry(source, index, sequence, connectionID, dialect, features, policy)
		entries = append(entries, transaction)
		diags.ParsedRecords++
		if transaction.StartedAt != "" {
			validTimes = append(validTimes, transaction.StartedAt)
		}
		if source.Response.Content.Text == nil {
			missingBodies++
		}
		if sizesUnavailable(source) {
			unavailableSizes++
		}
		missingTimings = missingTimings || signals.missingTimings
		negativeTiming = negativeTiming || signals.negativeTiming
		urlUnparsable = urlUnparsable || signals.urlUnparsable
		redirectsUnlinkable = redirectsUnlinkable || (dialect == DialectInsomnia && source.Response.Status >= 300 && source.Response.Status < 400 && source.Response.RedirectURL == "")
	}
	diags.TotalLines = len(rawEntries)
	for _, warning := range policy.Warnings() {
		addDiagnostic(diags, seenDiagnostics, warning.Code, warning.Message)
	}

	if schemaViolation {
		addDiagnostic(diags, seenDiagnostics, ReasonSchemaViolation, "HAR required fields are missing; conservative defaults applied")
	}
	if missingTimings {
		addDiagnostic(diags, seenDiagnostics, "HAR_TIMINGS_MISSING", "one or more entries have incomplete timings")
	}
	if negativeTiming {
		addDiagnostic(diags, seenDiagnostics, "HAR_TIMING_NEGATIVE", "one or more timing values are invalid negative durations")
	}
	if urlUnparsable {
		addDiagnostic(diags, seenDiagnostics, ReasonURLUnparsable, "one or more request URLs could not be parsed; redacted raw URLs were retained")
	}
	if len(entries) > 0 && unavailableSizes == len(entries) {
		addDiagnostic(diags, seenDiagnostics, "HAR_SIZES_UNAVAILABLE", "HAR transfer sizes are unavailable")
	}
	if len(entries) > 0 && missingBodies == len(entries) {
		addDiagnostic(diags, seenDiagnostics, "HAR_BODIES_ABSENT", "HAR response bodies are absent; size-only analysis is available")
	}
	if dialect == DialectSafari {
		addDiagnostic(diags, seenDiagnostics, "HAR_HEADERS_COLLAPSED", "Safari HAR may collapse duplicate headers")
	}
	if redirectsUnlinkable {
		addDiagnostic(diags, seenDiagnostics, "HAR_REDIRECTS_UNLINKABLE", "redirect chain cannot be reconstructed")
	}
	degenerate := timestampsDegenerate(validTimes, len(entries))
	if degenerate {
		addDiagnostic(diags, seenDiagnostics, "HAR_TIMESTAMPS_DEGENERATE", "HAR timestamps are identical; timeline analysis is disabled")
	} else {
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].StartedAt < entries[j].StartedAt })
	}

	return ParseResult{
		Format:            FormatHAR,
		Dialect:           string(dialect),
		Entries:           entries,
		Diagnostics:       diags,
		Redaction:         policy.Summary(),
		TimelineAvailable: !degenerate,
		InputBytes:        inputInfo.compressedBytes,
		DecompressedBytes: int64(len(payload)),
	}, nil
}

type entrySignals struct {
	missingTimings bool
	negativeTiming bool
	urlUnparsable  bool
}

func mapEntry(source harEntry, index, sequence int, connectionID string, dialect Dialect, features DialectFeatures, policy *redact.Policy) (models.CaptureTransaction, entrySignals) {
	redactedURL := policy.RedactURL(strings.TrimSpace(source.Request.URL))
	parsedURL, urlErr := url.Parse(redactedURL)
	if urlErr != nil {
		parsedURL = &url.URL{}
	}
	method := strings.ToUpper(strings.TrimSpace(source.Request.Method))
	if method == "" {
		method = "GET"
	}
	httpVersion := firstNonEmpty(source.Request.HTTPVersion, source.Response.HTTPVersion, "unknown")
	timings, totalMS, timingSignals := NormalizeTimings(source.Timings, source.Time, dialect, features, sequence > 0)
	started := normalizeTimestamp(source.StartedDateTime)
	ended := ""
	if started != "" && totalMS >= 0 {
		if parsed, err := time.Parse(time.RFC3339Nano, started); err == nil {
			ended = parsed.Add(time.Duration(totalMS * float64(time.Millisecond))).UTC().Format(time.RFC3339Nano)
		}
	}
	request := mapRequest(source.Request, policy)
	response := mapResponse(source.Response, dialect, policy)
	state := models.TxComplete
	errorText := ""
	if source.Response.Status == 0 {
		state = models.TxFailed
		errorText = "response status unavailable"
	}
	process := mapProcess(source.ArchScope)
	process = policy.RedactProcess(process)
	coverage := "full"
	if source.Response.Content.Text == nil {
		coverage = "headers_only"
	}
	return models.CaptureTransaction{
		ID:                     fmt.Sprintf("har-%06d", index+1),
		ConnectionID:           connectionID,
		Sequence:               sequence,
		Method:                 method,
		URL:                    redactedURL,
		Scheme:                 parsedURL.Scheme,
		Host:                   parsedURL.Host,
		Path:                   firstNonEmpty(parsedURL.EscapedPath(), "/"),
		Query:                  parsedURL.RawQuery,
		HTTPVersion:            httpVersion,
		StatusCode:             source.Response.Status,
		StatusText:             source.Response.StatusText,
		Request:                request,
		Response:               response,
		Timings:                models.TimingSet{ImportedHAR: &timings},
		UsedExistingConnection: sequence > 0,
		StartedAt:              started,
		EndedAt:                ended,
		State:                  state,
		TotalMS:                totalMS,
		CaptureMode:            "har_import",
		ObservationPoint:       "foreign_tool",
		Coverage:               coverage,
		Fidelity:               "semantic",
		Process:                process,
		Error:                  errorText,
	}, entrySignals{missingTimings: timingSignals.missing, negativeTiming: timingSignals.negative, urlUnparsable: urlErr != nil}
}

func mapRequest(source harRequest, policy *redact.Policy) models.HTTPMessage {
	bodyText := ""
	mimeType := ""
	if source.PostData != nil {
		bodyText = source.PostData.Text
		mimeType = source.PostData.MIMEType
		if bodyText == "" && len(source.PostData.Params) > 0 {
			values := url.Values{}
			for _, param := range source.PostData.Params {
				value, _ := policy.RedactNamedValue(param.Name, param.Value)
				values.Add(param.Name, value)
			}
			bodyText = values.Encode()
		}
	}
	preview, bodyRedacted := policy.RedactBody(mimeType, bodyText)
	return models.HTTPMessage{
		Headers:      mapHeaders(source.Headers, policy),
		Cookies:      mapCookies(source.Cookies, policy),
		HeaderSize:   sizeValue(source.HeadersSize),
		BodySize:     sizeValue(source.BodySize),
		BodyDecoded:  unknownSize(),
		TransferSize: unknownSize(),
		ContentType:  mimeType,
		BodyStorage:  bodyStorage(preview, bodyRedacted),
		BodyPreview:  inlinePreview(preview),
		Redacted:     bodyRedacted,
	}
}

func mapResponse(source harResponse, dialect Dialect, policy *redact.Policy) models.HTTPMessage {
	bodyText := ""
	if source.Content.Text != nil {
		bodyText = *source.Content.Text
	}
	preview, bodyRedacted := policy.RedactBody(source.Content.MIMEType, bodyText)
	transferSize := sizeValue(source.TransferSize)
	bodySize := sizeValue(source.BodySize)
	if dialect == DialectFirefox && transferSize < 0 {
		transferSize = bodySize
	}
	return models.HTTPMessage{
		Headers:      mapHeaders(source.Headers, policy),
		Cookies:      mapCookies(source.Cookies, policy),
		HeaderSize:   sizeValue(source.HeadersSize),
		BodySize:     bodySize,
		BodyDecoded:  sizeValue(source.Content.Size),
		TransferSize: transferSize,
		BodyEncoding: source.Content.Encoding,
		ContentType:  source.Content.MIMEType,
		BodyStorage:  bodyStorage(preview, bodyRedacted),
		BodyPreview:  inlinePreview(preview),
		Redacted:     bodyRedacted,
	}
}

func mapHeaders(source []harNameValue, policy *redact.Policy) []models.HeaderField {
	headers := make([]models.HeaderField, 0, len(source))
	for _, header := range source {
		if strings.HasPrefix(strings.TrimSpace(header.Name), ":") {
			continue
		}
		headers = append(headers, models.HeaderField{Name: header.Name, Value: header.Value})
	}
	return policy.RedactHeaders(headers)
}

func mapCookies(source []harCookie, policy *redact.Policy) []models.HeaderField {
	cookies := make([]models.HeaderField, 0, len(source))
	for _, cookie := range source {
		value, changed := policy.RedactNamedValue("cookie", cookie.Value)
		cookies = append(cookies, models.HeaderField{Name: cookie.Name, Value: value, Redacted: changed})
	}
	return cookies
}

func mapProcess(source harArchScope) *models.ProcessInstance {
	if source.Process.PID == 0 && source.Process.Name == "" && source.Process.ExecPath == "" {
		return nil
	}
	return &models.ProcessInstance{
		Key:         models.ProcessKey{PID: source.Process.PID, StartTime: source.Process.StartTime},
		Name:        source.Process.Name,
		ExecPath:    source.Process.ExecPath,
		CommandLine: source.Process.CommandLine,
		User:        source.Process.User,
		ParentPID:   source.Process.ParentPID,
		Attribution: firstNonEmpty(source.Process.Attribution, "unknown"),
	}
}

func missingRequiredHARFields(raw json.RawMessage) bool {
	var entry map[string]json.RawMessage
	if json.Unmarshal(raw, &entry) != nil {
		return false
	}
	for _, key := range []string{"startedDateTime", "time", "request", "response", "cache", "timings"} {
		if _, ok := entry[key]; !ok {
			return true
		}
	}
	return false
}

func bodyInputBytes(entry harEntry) int {
	total := 0
	if entry.Request.PostData != nil {
		total += len(entry.Request.PostData.Text)
		for _, param := range entry.Request.PostData.Params {
			total += len(param.Name) + len(param.Value)
		}
	}
	if entry.Response.Content.Text != nil {
		total += len(*entry.Response.Content.Text)
	}
	return total
}

func sizesUnavailable(entry harEntry) bool {
	return sizeValue(entry.Response.HeadersSize) < 0 && sizeValue(entry.Response.BodySize) < 0
}

func timestampsDegenerate(values []string, total int) bool {
	if total < 2 || len(values) != total {
		return false
	}
	first := values[0]
	for _, value := range values[1:] {
		if value != first {
			return false
		}
	}
	return true
}

func normalizeTimestamp(raw string) string {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339Nano)
}

func bodyStorage(preview string, redacted bool) string {
	if preview == "" {
		return "omitted"
	}
	if len(preview) > inlineBodyPreviewBytes {
		return "truncated"
	}
	if redacted {
		return "redacted"
	}
	return "inline"
}

const inlineBodyPreviewBytes = 64 << 10

func inlinePreview(value string) string {
	if len(value) > inlineBodyPreviewBytes {
		end := inlineBodyPreviewBytes
		for end > 0 && !utf8.ValidString(value[:end]) {
			end--
		}
		return value[:end]
	}
	return value
}

func sizeValue(value *int64) int64 {
	if value == nil {
		return unknownSize()
	}
	return *value
}

func unknownSize() int64 { return -1 }

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func addDiagnostic(diags *diagnostics.ParserDiagnostics, seen map[string]struct{}, code, message string) {
	if _, ok := seen[code]; ok {
		return
	}
	seen[code] = struct{}{}
	diags.AddWarning(0, code, message, "", false)
}

func decodeOne(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func safeJSONError(err error) string {
	if syntax, ok := err.(*json.SyntaxError); ok {
		return fmt.Sprintf("invalid JSON at byte %d", syntax.Offset)
	}
	return "invalid JSON structure"
}

func recoverParsePanic(result *ParseResult, err *error, diags *diagnostics.ParserDiagnostics) {
	if recover() == nil {
		return
	}
	panicErr := fmt.Errorf("%w: unexpected parser failure", ErrStructuralHAR)
	diags.AddError(0, ReasonStructural, panicErr.Error(), "")
	*result = ParseResult{Format: FormatHAR, Diagnostics: diags}
	*err = panicErr
}
