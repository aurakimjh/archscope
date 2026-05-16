package serverlog

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/textio"
)

var errStopIteration = errors.New("stop server-log iteration")

type Record struct {
	Timestamp   time.Time
	Severity    string
	Product     string
	Component   string
	Thread      string
	Host        string
	ServiceName string
	EventType   string
	Message     string
	TraceID     string
	RequestID   string
	RawLine     string
}

type Options struct {
	MaxLines int
	Strict   bool
}

const (
	ReasonNoFormatMatch    = "NO_FORMAT_MATCH"
	ReasonInvalidTimestamp = "INVALID_TIMESTAMP"
)

var SupportedFormats = map[string]struct{}{
	"auto":         {},
	"apache-error": {},
	"glassfish":    {},
	"jboss":        {},
	"jetty":        {},
	"nginx-error":  {},
	"payara":       {},
	"tomcat":       {},
	"weblogic":     {},
	"websphere":    {},
	"wildfly":      {},
}

type parseError struct {
	Reason  string
	Message string
}

func ParseLineForFormat(line, format string) (*Record, *parseError) {
	format = canonicalFormat(format)
	if format == "auto" {
		if detected := DetectLineFormat(line); detected != "" {
			return ParseLineForFormat(line, detected)
		}
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "line does not match supported server log formats"}
	}
	switch format {
	case "tomcat":
		return parseTomcatLine(line)
	case "jetty":
		return parseJettyLine(line)
	case "jboss", "wildfly":
		return parseJBossLine(line, format)
	case "weblogic":
		return parseWebLogicLine(line)
	case "websphere":
		return parseWebSphereLine(line)
	case "glassfish", "payara":
		return parseGlassFishLine(line, format)
	case "nginx-error":
		return parseNginxErrorLine(line)
	case "apache-error":
		return parseApacheErrorLine(line)
	default:
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "unsupported server log format selector"}
	}
}

func DetectLineFormat(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "<") && strings.Contains(trimmed, "<BEA-"):
		return "weblogic"
	case strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "SystemOut"):
		return "websphere"
	case strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "javax.enterprise.system"):
		return "glassfish"
	case strings.Contains(trimmed, " org.jboss.") || strings.Contains(trimmed, " [org.jboss."):
		return "jboss"
	case strings.Contains(trimmed, " org.wildfly.") || strings.Contains(trimmed, " [org.wildfly."):
		return "wildfly"
	case strings.Contains(trimmed, ":oej") || strings.Contains(trimmed, "org.eclipse.jetty"):
		return "jetty"
	case strings.Contains(trimmed, " [error] ") && strings.Contains(trimmed, "#"):
		return "nginx-error"
	case strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, ":error]"):
		return "apache-error"
	case strings.Contains(trimmed, " org.apache.catalina.") || strings.Contains(trimmed, " catalina"):
		return "tomcat"
	default:
		return ""
	}
}

var (
	tomcatRE    = regexp.MustCompile(`^(?P<time>\d{2}-[A-Za-z]{3}-\d{4} \d{2}:\d{2}:\d{2}(?:\.\d+)?) (?P<severity>\S+) \[(?P<thread>[^\]]+)\] (?P<component>\S+) (?P<message>.*)$`)
	jettyRE     = regexp.MustCompile(`^(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?):(?P<severity>[A-Z]+):(?P<component>[^:]+):(?P<thread>[^:]*): (?P<message>.*)$`)
	jbossRE     = regexp.MustCompile(`^(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d+) (?P<severity>[A-Z]+) \[(?P<component>[^\]]+)\] \((?P<thread>[^\)]+)\) (?P<message>.*)$`)
	weblogicRE  = regexp.MustCompile(`^<(?P<time>[^>]+)> <(?P<severity>[^>]+)> <(?P<component>[^>]+)> <(?P<host>[^>]*)> <(?P<service>[^>]*)> <(?P<thread>[^>]*)>.*?<(?P<message>[^>]*)>$`)
	websphereRE = regexp.MustCompile(`^\[(?P<time>[^\]]+)\]\s+\S+\s+(?P<component>\S+)\s+(?P<severity>[A-Z])\s+(?P<message>.*)$`)
	glassfishRE = regexp.MustCompile(`^\[(?P<time>[^\]]+)\] \[(?P<product>[^\]]+)\] \[(?P<severity>[^\]]+)\] \[[^\]]*\] \[(?P<component>[^\]]+)\].*?\[\[(?P<message>.*)\]\]$`)
	nginxErrRE  = regexp.MustCompile(`^(?P<time>\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[(?P<severity>[^\]]+)\] \S+: (?P<message>.*)$`)
	apacheErrRE = regexp.MustCompile(`^\[(?P<time>[^\]]+)\] \[(?P<component>[^\]]+)\](?: \[[^\]]+\])* (?P<message>.*)$`)
)

func parseTomcatLine(line string) (*Record, *parseError) {
	g := captureGroups(tomcatRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not a Tomcat catalina log line"}
	}
	ts, err := parseTime(g["time"], "02-Jan-2006 15:04:05.000", "02-Jan-2006 15:04:05")
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: err.Error()}
	}
	return record(ts, "tomcat", g["severity"], g["component"], g["thread"], "", "", g["message"], line), nil
}

func parseJettyLine(line string) (*Record, *parseError) {
	g := captureGroups(jettyRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not a Jetty request/server log line"}
	}
	ts, err := parseTime(g["time"], "2006-01-02 15:04:05.000", "2006-01-02 15:04:05")
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: err.Error()}
	}
	return record(ts, "jetty", g["severity"], g["component"], g["thread"], "", "", g["message"], line), nil
}

func parseJBossLine(line, product string) (*Record, *parseError) {
	g := captureGroups(jbossRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not a JBoss/WildFly server log line"}
	}
	ts, err := parseTime(g["time"], "2006-01-02 15:04:05,000")
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: err.Error()}
	}
	return record(ts, product, g["severity"], g["component"], g["thread"], "", "", g["message"], line), nil
}

func parseWebLogicLine(line string) (*Record, *parseError) {
	g := captureGroups(weblogicRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not a WebLogic server log line"}
	}
	ts, err := parseTime(strings.TrimSuffix(g["time"], " KST"), "Jan 2, 2006 3:04:05 PM", "Jan 02, 2006 3:04:05 PM")
	if err != nil {
		ts = time.Time{}
	}
	return record(ts, "weblogic", g["severity"], g["component"], g["thread"], g["host"], g["service"], g["message"], line), nil
}

func parseWebSphereLine(line string) (*Record, *parseError) {
	g := captureGroups(websphereRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not a WebSphere SystemOut/SystemErr line"}
	}
	ts, _ := parseTime(strings.TrimSuffix(g["time"], " KST"), "1/2/06 15:04:05:000")
	severity := map[string]string{"E": "ERROR", "W": "WARN", "O": "INFO"}[g["severity"]]
	if severity == "" {
		severity = g["severity"]
	}
	return record(ts, "websphere", severity, g["component"], "", "", "", g["message"], line), nil
}

func parseGlassFishLine(line, product string) (*Record, *parseError) {
	g := captureGroups(glassfishRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not a GlassFish/Payara server log line"}
	}
	ts, _ := parseTime(g["time"], "2006-01-02T15:04:05.000-0700", time.RFC3339)
	return record(ts, firstNonEmpty(product, strings.ToLower(g["product"])), g["severity"], g["component"], "", "", "", g["message"], line), nil
}

func parseNginxErrorLine(line string) (*Record, *parseError) {
	g := captureGroups(nginxErrRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not an nginx error log line"}
	}
	ts, err := parseTime(g["time"], "2006/01/02 15:04:05")
	if err != nil {
		return nil, &parseError{Reason: ReasonInvalidTimestamp, Message: err.Error()}
	}
	return record(ts, "nginx", g["severity"], "nginx.worker", "", "", "", g["message"], line), nil
}

func parseApacheErrorLine(line string) (*Record, *parseError) {
	g := captureGroups(apacheErrRE, line)
	if g == nil {
		return nil, &parseError{Reason: ReasonNoFormatMatch, Message: "not an Apache error log line"}
	}
	ts, _ := parseTime(g["time"], "Mon Jan 2 15:04:05.000000 2006", "Mon Jan 02 15:04:05.000000 2006")
	severity := "ERROR"
	component := g["component"]
	if parts := strings.Split(component, ":"); len(parts) > 1 {
		severity = strings.ToUpper(parts[len(parts)-1])
	}
	return record(ts, "apache", severity, component, "", "", "", g["message"], line), nil
}

func record(ts time.Time, product, severity, component, thread, host, service, message, raw string) *Record {
	return &Record{
		Timestamp:   ts,
		Severity:    normalizeSeverity(severity),
		Product:     strings.ToLower(strings.TrimSpace(product)),
		Component:   strings.TrimSpace(component),
		Thread:      strings.TrimSpace(thread),
		Host:        strings.TrimSpace(host),
		ServiceName: strings.TrimSpace(service),
		EventType:   classifyEventType(severity, component, message),
		Message:     strings.TrimSpace(message),
		TraceID:     extractKV(message, "trace_id", "traceId"),
		RequestID:   extractKV(message, "request_id", "requestId", "request"),
		RawLine:     raw,
	}
}

func classifyEventType(severity, component, message string) string {
	text := strings.ToLower(component + " " + message)
	switch {
	case strings.Contains(text, "stuck thread"):
		return "stuck_thread"
	case strings.Contains(text, "hung thread"):
		return "hung_thread"
	case strings.Contains(text, "thread pool") || strings.Contains(text, "executor") || strings.Contains(text, "rejectedexecution"):
		return "thread_pool_pressure"
	case strings.Contains(text, "datasource") || strings.Contains(text, "jdbc") || strings.Contains(text, "connection pool"):
		return "datasource_pool"
	case strings.Contains(text, "deploy") || strings.Contains(text, "deployment") || strings.Contains(text, "application failed"):
		return "deployment"
	case strings.Contains(text, "started") || strings.Contains(text, "startup") || strings.Contains(text, "server is ready"):
		return "startup"
	case strings.Contains(text, "managed server") || strings.Contains(text, "server health"):
		return "managed_server_health"
	case strings.Contains(text, "connect() failed") || strings.Contains(text, "upstream") || strings.Contains(text, "proxy") || strings.Contains(text, "worker"):
		return "worker_error"
	case isErrorSeverity(severity):
		return "severe_error"
	default:
		return "server_event"
	}
}

func ForEachRecord(path, format string, opts Options, fn func(Record) error) (*diagnostics.ParserDiagnostics, error) {
	format = canonicalFormat(format)
	if _, ok := SupportedFormats[format]; !ok {
		return nil, fmt.Errorf("unsupported server log format: %s", format)
	}
	if opts.MaxLines < 0 {
		return nil, fmt.Errorf("max_lines must be a positive integer")
	}
	diags := diagnostics.New(format)
	diags.SetSourceFile(path)
	err := textio.ForEachTextLine(path, "", func(lineNumber int, line string) error {
		if opts.MaxLines > 0 && lineNumber > opts.MaxLines {
			return errStopIteration
		}
		diags.TotalLines++
		if strings.TrimSpace(line) == "" {
			return nil
		}
		rec, perr := ParseLineForFormat(line, format)
		if rec != nil {
			diags.ParsedRecords++
			return fn(*rec)
		}
		diags.AddSkipped(lineNumber, perr.Reason, perr.Message, line)
		if opts.Strict {
			return fmt.Errorf("%s:%d: %s: %s", path, lineNumber, perr.Reason, perr.Message)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStopIteration) {
		return diags, err
	}
	if diags.TotalLines == 0 {
		diags.AddWarning(0, "EMPTY_FILE", "Server log file is empty.", "", false)
	}
	return diags, nil
}

func ParseFile(path, format string, opts Options) ([]Record, *diagnostics.ParserDiagnostics, error) {
	records := []Record{}
	diags, err := ForEachRecord(path, format, opts, func(record Record) error {
		records = append(records, record)
		return nil
	})
	return records, diags, err
}

func captureGroups(re *regexp.Regexp, line string) map[string]string {
	m := re.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	out := map[string]string{}
	names := re.SubexpNames()
	for i := 1; i < len(m); i++ {
		if names[i] != "" {
			out[names[i]] = m[i]
		}
	}
	return out
}

func parseTime(value string, layouts ...string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

func canonicalFormat(format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		return "auto"
	}
	if format == "apache" {
		return "apache-error"
	}
	if format == "nginx" {
		return "nginx-error"
	}
	return format
}

func normalizeSeverity(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "SEVERE", "FATAL", "CRITICAL", "ERROR", "ERR":
		return "ERROR"
	case "WARNING":
		return "WARN"
	case "":
		return "INFO"
	default:
		return value
	}
}

func isErrorSeverity(value string) bool {
	switch normalizeSeverity(value) {
	case "ERROR", "FATAL", "CRITICAL", "SEVERE":
		return true
	}
	return false
}

func extractKV(text string, keys ...string) string {
	for _, key := range keys {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(key) + `=([A-Za-z0-9_.:-]+)`)
		if m := re.FindStringSubmatch(text); m != nil {
			return m[1]
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
