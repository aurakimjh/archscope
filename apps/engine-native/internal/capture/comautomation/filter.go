package comautomation

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Legacy filters are data, never Delphi objects. Only the observed wrapper,
// criteria versions and operators are accepted; unknown encodings fail closed.
type logFilter struct {
	active bool
	or     bool
	rules  []filterRule
}
type filterRule struct {
	column   int32
	operator byte
	text     string
	number   *int64
	pattern  *regexp.Regexp
}
type filterReader struct{ *bytes.Reader }

func (r filterReader) take(n int) ([]byte, error) {
	if n < 0 || n > r.Len() {
		return nil, fmt.Errorf("truncated filter")
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}
func (r filterReader) long() (int32, error) {
	b, e := r.take(4)
	if e != nil {
		return 0, e
	}
	return int32(binary.LittleEndian.Uint32(b)), nil
}
func (r filterReader) shortText() (string, error) {
	n, e := r.ReadByte()
	if e != nil {
		return "", e
	}
	b, e := r.take(int(n))
	return string(b), e
}
func (r filterReader) text() (string, error) {
	n, e := r.long()
	if e != nil {
		return "", e
	}
	b, e := r.take(int(n))
	return string(b), e
}

func readLogFilter(path string) (*logFilter, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() || info.Size() > 256<<10 {
		return nil, fmt.Errorf("filter must be a regular file of at most 256 KiB")
	}
	b, e := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if e != nil {
		return nil, e
	}
	return decodeLogFilter(b)
}
func decodeLogFilter(b []byte) (*logFilter, error) {
	if len(b) > 256<<10 {
		return nil, fmt.Errorf("filter exceeds 256 KiB")
	}
	r := filterReader{bytes.NewReader(b)}
	magic, e := r.take(4)
	if e != nil || string(magic) != "TPF0" {
		return nil, fmt.Errorf("unsupported filter container")
	}
	class, e := r.shortText()
	if e != nil || class != "TFilterWrapper" {
		return nil, fmt.Errorf("unsupported filter class")
	}
	name, e := r.shortText()
	if e != nil || name != "" {
		return nil, fmt.Errorf("unsupported filter instance")
	}
	key, e := r.shortText()
	if e != nil || key != "Filter.Active" {
		return nil, fmt.Errorf("missing filter activation")
	}
	flag, e := r.ReadByte()
	if e != nil || (flag != 8 && flag != 9) {
		return nil, fmt.Errorf("invalid filter activation")
	}
	key, e = r.shortText()
	if e != nil || key != "Filter.Criteria" {
		return nil, fmt.Errorf("missing filter criteria")
	}
	tag, e := r.ReadByte()
	if e != nil || tag != 10 {
		return nil, fmt.Errorf("invalid criteria blob")
	}
	n, e := r.long()
	if e != nil {
		return nil, e
	}
	blob, e := r.take(int(n))
	if e != nil {
		return nil, e
	}
	end, e := r.take(2)
	if e != nil || !bytes.Equal(end, []byte{0, 0}) || r.Len() != 0 {
		return nil, fmt.Errorf("unsupported trailing filter data")
	}
	c := filterReader{bytes.NewReader(blob)}
	version, e := c.long()
	if e != nil || (version != -1 && version != -2) {
		return nil, fmt.Errorf("unsupported filter criteria version")
	}
	logical, e := c.ReadByte()
	if e != nil || logical > 1 {
		return nil, fmt.Errorf("unsupported filter logical operator")
	}
	count, e := c.long()
	if e != nil || count < 1 || count > 256 {
		return nil, fmt.Errorf("filter requires 1..256 rules")
	}
	out := &logFilter{active: flag == 9, or: logical == 1}
	for i := int32(0); i < count; i++ {
		kind, e := c.ReadByte()
		if e != nil || kind != 0 {
			return nil, fmt.Errorf("nested filter groups are not supported")
		}
		op, e := c.ReadByte()
		if e != nil || !(op == 0 || op == 1 || op == 4 || op == 6 || op == 7) {
			return nil, fmt.Errorf("unsupported filter comparison")
		}
		_, e = c.text()
		if e != nil {
			return nil, e
		} // Display label is not the comparison value.
		column, e := c.long()
		if e != nil {
			return nil, e
		}
		if version == -2 {
			columnName, e := c.text()
			if e != nil {
				return nil, e
			}
			expected := map[int32]string{3: "RequestMethodColumn", 4: "RequestResultColumn", 5: "RequestSizeColumn", 6: "RequestTypeColumn"}[column]
			if columnName != expected {
				return nil, fmt.Errorf("unsupported filter column name")
			}
		}
		if column < 3 || column > 6 {
			return nil, fmt.Errorf("unsupported filter column %d", column)
		}
		rule := filterRule{column: column, operator: op}
		valueType, e := c.ReadByte()
		if e != nil {
			return nil, e
		}
		switch valueType {
		case 6:
			rule.text, e = c.shortText()
		case 2:
			var v byte
			v, e = c.ReadByte()
			n := int64(int8(v))
			rule.number = &n
		case 3:
			var b []byte
			b, e = c.take(2)
			if e == nil {
				n := int64(int16(binary.LittleEndian.Uint16(b)))
				rule.number = &n
			}
		case 4:
			var v int32
			v, e = c.long()
			n := int64(v)
			rule.number = &n
		default:
			return nil, fmt.Errorf("unsupported filter value type")
		}
		if e != nil {
			return nil, e
		}
		if !utf8.ValidString(rule.text) {
			return nil, fmt.Errorf("unsupported non-UTF8 filter text")
		}
		if column == 4 && strings.EqualFold(rule.text, "(Cache)") {
			return nil, fmt.Errorf("cache-origin filter requires metadata not retained in this capture contract")
		}
		if op == 4 && rule.number == nil {
			return nil, fmt.Errorf("numeric comparison requires numeric value")
		}
		if rule.number != nil && column != 5 {
			return nil, fmt.Errorf("numeric value requires size column")
		}
		if op == 6 || op == 7 {
			var p strings.Builder
			p.WriteString("(?is)^")
			for _, ch := range rule.text {
				switch ch {
				case '%':
					p.WriteString(".*")
				case '_':
					p.WriteByte('.')
				default:
					p.WriteString(regexp.QuoteMeta(string(ch)))
				}
			}
			p.WriteByte('$')
			rule.pattern = regexp.MustCompile(p.String())
		}
		out.rules = append(out.rules, rule)
	}
	if c.Len() != 0 {
		return nil, fmt.Errorf("trailing filter criteria")
	}
	return out, nil
}
func (f *logFilter) matches(tx models.CaptureTransaction) bool {
	if f == nil || !f.active {
		return true
	}
	for _, r := range f.rules {
		value := ""
		switch r.column {
		case 3:
			value = tx.Method
		case 4:
			value = strconv.Itoa(tx.StatusCode)
		case 5:
			value = strconv.FormatInt(tx.Response.BodySize, 10)
		case 6:
			value = strings.SplitN(tx.Response.ContentType, ";", 2)[0]
		}
		match := strings.EqualFold(value, r.text)
		if r.number != nil {
			switch r.operator {
			case 0:
				match = tx.Response.BodySize == *r.number
			case 1:
				match = tx.Response.BodySize != *r.number
			case 4:
				match = tx.Response.BodySize > *r.number
			}
		} else {
			switch r.operator {
			case 1:
				match = !match
			case 6:
				match = r.pattern.MatchString(value)
			case 7:
				match = !r.pattern.MatchString(value)
			}
		}
		if f.or && match {
			return true
		}
		if !f.or && !match {
			return false
		}
	}
	return !f.or
}

// Both collections are immutable snapshots. Count/Item enumerate the retained
// log; FilteredCount/FilteredItem select the matching subset without deleting it.
type filteredLog struct{ all, filtered Collection }

func (f filteredLog) Members() []string {
	return []string{"Count", "Item", "_NewEnum", "FilteredCount", "FilteredItem"}
}
func (f filteredLog) Invoke(ctx context.Context, name string, args []any, put bool) (any, error) {
	switch strings.ToLower(name) {
	case "filteredcount":
		return f.filtered.Invoke(ctx, "Count", args, put)
	case "filtereditem":
		return f.filtered.Invoke(ctx, "Item", args, put)
	}
	return f.all.Invoke(ctx, name, args, put)
}

// MaxLogSize counts rows, not megabytes (reference manual). Evicted IDs stay
// out of this controller's view even if its limit increases. Durable capture
// evidence remains governed by the independent session byte limit.
func (c *Controller) retainRows(input []models.CaptureTransaction) []models.CaptureTransaction {
	rows := make([]models.CaptureTransaction, 0, len(input))
	for _, row := range input {
		if !c.omittedIDs[row.ID] {
			rows = append(rows, row)
		}
	}
	if c.maxLogSize > 0 && len(rows) > c.maxLogSize {
		if c.omittedIDs == nil {
			c.omittedIDs = map[string]bool{}
		}
		for _, row := range rows[:len(rows)-c.maxLogSize] {
			c.omittedIDs[row.ID] = true
		}
		rows = rows[len(rows)-c.maxLogSize:]
	}
	return rows
}
