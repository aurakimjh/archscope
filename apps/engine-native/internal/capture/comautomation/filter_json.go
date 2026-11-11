package comautomation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// This is an explicit ArchScope format, never an inferred vendor .flt codec.
type filterDocument struct {
	Format  string         `json:"format"`
	Version int            `json:"version"`
	Active  *bool          `json:"active"`
	Where   *predicateSpec `json:"where"`
}
type predicateSpec struct {
	All        []predicateSpec `json:"all,omitempty"`
	Any        []predicateSpec `json:"any,omitempty"`
	Not        *predicateSpec  `json:"not,omitempty"`
	Field      string          `json:"field,omitempty"`
	Op         string          `json:"op,omitempty"`
	Value      json.RawMessage `json:"value,omitempty"`
	IgnoreCase bool            `json:"ignoreCase,omitempty"`
}
type predicate struct {
	group               string
	children            []*predicate
	field, op, text     string
	number              int64
	numeric, ignoreCase bool
	pattern             *regexp.Regexp
}
type matchState uint8

const (
	matchUnknown matchState = iota
	matchFalse
	matchTrue
)

func decodeNormalizedFilter(b []byte) (*logFilter, error) {
	if len(b) > 256<<10 {
		return nil, fmt.Errorf("filter exceeds 256 KiB")
	}
	if !utf8.Valid(b) {
		return nil, fmt.Errorf("filter must be UTF-8")
	}
	if e := validateJSONKeys(json.NewDecoder(bytes.NewReader(b)), 0); e != nil {
		return nil, e
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var doc filterDocument
	if e := decoder.Decode(&doc); e != nil {
		return nil, e
	}
	if e := decoder.Decode(new(any)); e != io.EOF {
		return nil, fmt.Errorf("trailing filter data")
	}
	if doc.Format != "archscope.http-filter" || doc.Version != 1 || doc.Active == nil || doc.Where == nil {
		return nil, fmt.Errorf("requires archscope.http-filter version 1, active and where")
	}
	count := 0
	root, e := compilePredicate(*doc.Where, 1, &count)
	if e != nil {
		return nil, e
	}
	return &logFilter{active: *doc.Active, root: root}, nil
}

// encoding/json otherwise silently accepts duplicate object members, making a
// human-reviewed condition differ from the parsed condition.
func validateJSONKeys(d *json.Decoder, depth int) error {
	if depth > 40 {
		return fmt.Errorf("filter JSON nesting exceeds limit")
	}
	token, e := d.Token()
	if e != nil {
		return e
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delimiter == '{' {
		seen := map[string]bool{}
		for d.More() {
			key, e := d.Token()
			if e != nil {
				return e
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate or invalid filter key")
			}
			seen[name] = true
			if e = validateJSONKeys(d, depth+1); e != nil {
				return e
			}
		}
	} else if delimiter == '[' {
		for d.More() {
			if e = validateJSONKeys(d, depth+1); e != nil {
				return e
			}
		}
	} else {
		return fmt.Errorf("invalid JSON delimiter")
	}
	_, e = d.Token()
	return e
}
func compilePredicate(s predicateSpec, depth int, count *int) (*predicate, error) {
	*count = *count + 1
	if depth > 16 || *count > 256 {
		return nil, fmt.Errorf("filter exceeds depth 16 or 256 nodes")
	}
	forms := 0
	if s.All != nil {
		forms++
	}
	if s.Any != nil {
		forms++
	}
	if s.Not != nil {
		forms++
	}
	if s.Field != "" || s.Op != "" || s.Value != nil {
		forms++
	}
	if forms != 1 {
		return nil, fmt.Errorf("each condition requires exactly one group or predicate")
	}
	p := &predicate{}
	if s.All != nil || s.Any != nil || s.Not != nil {
		if s.IgnoreCase {
			return nil, fmt.Errorf("ignoreCase applies only to a text predicate")
		}
		items := s.All
		p.group = "all"
		if s.Any != nil {
			items = s.Any
			p.group = "any"
		}
		if s.Not != nil {
			items = []predicateSpec{*s.Not}
			p.group = "not"
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("empty filter group")
		}
		for _, item := range items {
			child, e := compilePredicate(item, depth+1, count)
			if e != nil {
				return nil, e
			}
			p.children = append(p.children, child)
		}
		return p, nil
	}
	p.field = s.Field
	p.op = s.Op
	p.ignoreCase = s.IgnoreCase
	switch s.Field {
	case "status", "requestSize", "responseSize", "processPID":
		p.numeric = true
	case "url", "host", "path", "method", "mime", "processName", "scheme", "httpVersion", "coverage", "fidelity":
	default:
		return nil, fmt.Errorf("unsupported filter field %q", s.Field)
	}
	if p.numeric {
		if s.IgnoreCase {
			return nil, fmt.Errorf("numeric predicate cannot ignore case")
		}
		switch s.Op {
		case "eq", "neq", "lt", "lte", "gt", "gte":
		default:
			return nil, fmt.Errorf("invalid numeric operator")
		}
		n, e := strconv.ParseInt(string(s.Value), 10, 64)
		if e != nil || n < 0 {
			return nil, fmt.Errorf("numeric value must be a nonnegative int64")
		}
		p.number = n
	} else {
		if e := json.Unmarshal(s.Value, &p.text); e != nil || bytes.Equal(s.Value, []byte("null")) || len(p.text) > 4096 {
			return nil, fmt.Errorf("text value must be a string of at most 4096 bytes")
		}
		switch s.Op {
		case "eq", "neq", "contains", "startsWith", "endsWith":
		case "regex", "like":
			pattern := p.text
			if s.Op == "regex" && len(pattern) > 1024 {
				return nil, fmt.Errorf("regex exceeds 1024 bytes")
			}
			if s.Op == "like" {
				var b strings.Builder
				b.WriteString("(?s)^")
				for _, r := range pattern {
					switch r {
					case '%':
						b.WriteString(".*")
					case '_':
						b.WriteByte('.')
					default:
						b.WriteString(regexp.QuoteMeta(string(r)))
					}
				}
				b.WriteByte('$')
				pattern = b.String()
			}
			if p.ignoreCase {
				pattern = "(?i)" + pattern
			}
			var e error
			p.pattern, e = regexp.Compile(pattern)
			if e != nil {
				return nil, fmt.Errorf("invalid filter regex: %w", e)
			}
		default:
			return nil, fmt.Errorf("invalid text operator")
		}
	}
	return p, nil
}
func (p *predicate) evaluate(tx models.CaptureTransaction) matchState {
	if p.group != "" {
		unknown := false
		for _, c := range p.children {
			value := c.evaluate(tx)
			if p.group == "not" {
				if value == matchTrue {
					return matchFalse
				}
				if value == matchFalse {
					return matchTrue
				}
				return matchUnknown
			}
			if p.group == "all" && value == matchFalse {
				return matchFalse
			}
			if p.group == "any" && value == matchTrue {
				return matchTrue
			}
			unknown = unknown || value == matchUnknown
		}
		if unknown {
			return matchUnknown
		}
		if p.group == "all" {
			return matchTrue
		}
		return matchFalse
	}
	text := ""
	number := int64(0)
	known := true
	switch p.field {
	case "url":
		text = tx.URL
	case "host":
		text = tx.Host
	case "path":
		text = tx.Path
	case "method":
		text = tx.Method
	case "mime":
		text = strings.TrimSpace(strings.SplitN(tx.Response.ContentType, ";", 2)[0])
	case "scheme":
		text = tx.Scheme
	case "httpVersion":
		text = tx.HTTPVersion
	case "coverage":
		text = tx.Coverage
	case "fidelity":
		text = tx.Fidelity
	case "status":
		number = int64(tx.StatusCode)
		known = number > 0
	case "requestSize":
		number = tx.Request.BodySize
		known = number >= 0
	case "responseSize":
		number = tx.Response.BodySize
		known = number >= 0
	case "processPID":
		if tx.Process == nil {
			known = false
		} else {
			number = int64(tx.Process.Key.PID)
			known = number > 0
		}
	case "processName":
		if tx.Process != nil {
			text = tx.Process.Name
		}
	}
	if (!p.numeric && text == "") || !known {
		return matchUnknown
	}
	if p.pattern != nil {
		if p.pattern.MatchString(text) {
			return matchTrue
		}
		return matchFalse
	}
	matched := false
	if p.numeric {
		switch p.op {
		case "eq":
			matched = number == p.number
		case "neq":
			matched = number != p.number
		case "lt":
			matched = number < p.number
		case "lte":
			matched = number <= p.number
		case "gt":
			matched = number > p.number
		case "gte":
			matched = number >= p.number
		}
	} else {
		wanted := p.text
		if p.ignoreCase {
			text = strings.ToLower(text)
			wanted = strings.ToLower(wanted)
		}
		switch p.op {
		case "eq":
			matched = text == wanted
		case "neq":
			matched = text != wanted
		case "contains":
			matched = strings.Contains(text, wanted)
		case "startsWith":
			matched = strings.HasPrefix(text, wanted)
		case "endsWith":
			matched = strings.HasSuffix(text, wanted)
		case "regex", "like":
			matched = p.pattern.MatchString(text)
		}
	}
	if matched {
		return matchTrue
	}
	return matchFalse
}
