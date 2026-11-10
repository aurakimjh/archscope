package halog

// Native V7 format 6: a qh.TypedStreams attribute bag followed by an IwiRequest
// list. This is a data reader, never a Delphi object factory. Only the observed
// request/cache/timing/memory-stream classes and primitive tags are accepted.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	MaxEntries   = 100000
	maxValues    = 2000000
	maxString    = 16 << 20
	maxStream    = 16 << 20
	requestClass = "wINetTypes.TwiRequestImpl"
	cacheClass   = "wINetCache.TwiCacheInfoImpl"
	timingClass  = "wStdNotifyParams_dll.TwiTimingInfosImpl"
	streamClass  = "qh.MemStream"
)

var requestIID = []byte{0x2f, 0xc8, 0x85, 0x39, 0x41, 0x7c, 0xc5, 0x47, 0x97, 0xe4, 0x34, 0xf7, 0x2c, 0xa6, 0x13, 0xd5}

type field struct {
	name  string
	value value
}
type value struct {
	tag    byte
	n      uint64
	data   []byte
	object *object
}
type object struct {
	class  string
	fields []field
	extra  []value
	data   []byte
}

// File is a bounded native wire document. Encode preserves primitive tags and
// field order, including unmapped fields. It contains private unredacted bytes;
// user-facing import/export must use Transactions and the session sanitizer.
type File struct {
	attributes []field
	entries    []value
	codepage   int
}
type reader struct {
	data        []byte
	pos, values int
	err         error
}

func (r *reader) fail(message string) {
	if r.err == nil {
		r.err = fmt.Errorf("HALOG_INVALID: %s at byte %d", message, r.pos)
	}
}
func (r *reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.data)-r.pos {
		r.fail("truncated data")
		return nil
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}
func (r *reader) number(n int) uint64 {
	b := r.take(n)
	if len(b) != n {
		return 0
	}
	switch n {
	case 1:
		return uint64(b[0])
	case 2:
		return uint64(binary.LittleEndian.Uint16(b))
	case 4:
		return uint64(binary.LittleEndian.Uint32(b))
	case 8:
		return binary.LittleEndian.Uint64(b)
	}
	r.fail("invalid integer width")
	return 0
}
func (r *reader) count(limit int) int {
	n := r.number(4)
	if n > uint64(limit) {
		r.fail("count limit exceeded")
		return 0
	}
	return int(n)
}
func (r *reader) intN() uint64 {
	t := r.number(1)
	switch {
	case t < 240:
		return t
	case t == 240:
		return 240 + r.number(1)
	case t == 250:
		return r.number(2)
	case t == 251:
		return r.number(4)
	default:
		r.fail("unsupported chunk length")
		return 0
	}
}
func (r *reader) text() string {
	r.values++
	if r.values > maxValues {
		r.fail("value limit exceeded")
		return ""
	}
	t := r.number(1)
	n := uint64(0)
	switch t {
	case 11:
		n = r.number(4)
	case 12:
		n = r.number(1)
	default:
		r.fail("expected byte string")
	}
	if n > maxString {
		r.fail("string limit exceeded")
		return ""
	}
	return string(r.take(int(n)))
}
func (r *reader) bag(depth int) []field {
	n := r.count(256)
	out := make([]field, 0, n)
	seen := map[string]bool{}
	for i := 0; i < n && r.err == nil; i++ {
		name := r.text()
		if name == "" || len(name) > 256 || seen[name] {
			r.fail("invalid or duplicate field")
			break
		}
		seen[name] = true
		out = append(out, field{name, r.read(depth + 1)})
	}
	return out
}
func (r *reader) read(depth int) value {
	if r.err != nil {
		return value{}
	}
	r.values++
	if depth > 16 || r.values > maxValues {
		r.fail("structure limit exceeded")
		return value{}
	}
	v := value{tag: byte(r.number(1))}
	switch v.tag {
	case 0, 1:
	case 4, 8:
		v.n = r.number(4)
	case 5, 14, 17:
		v.n = r.number(8)
		if v.tag == 14 && (math.IsNaN(math.Float64frombits(v.n)) || math.IsInf(math.Float64frombits(v.n), 0)) {
			r.fail("invalid date")
		}
	case 6:
		v.n = r.number(1)
	case 11, 12:
		n := uint64(0)
		if v.tag == 12 {
			n = r.number(1)
		} else {
			n = r.number(4)
		}
		if n > maxString {
			r.fail("string limit exceeded")
		} else {
			v.data = r.take(int(n))
		}
	case 19:
		name := r.text()
		o := &object{class: name}
		v.object = o
		switch name {
		case requestClass, cacheClass, timingClass:
			o.fields = r.bag(depth)
			if name == requestClass {
				if numeric(o.get("Version")) != 7 {
					r.fail("unsupported request schema")
				}
				for i := 0; i < 6 && r.err == nil; i++ {
					o.extra = append(o.extra, r.read(depth+1))
				}
				for i, expected := range []string{cacheClass, cacheClass, timingClass, streamClass, streamClass, streamClass} {
					if i < len(o.extra) && o.extra[i].tag != 0 && (o.extra[i].object == nil || o.extra[i].object.class != expected) {
						r.fail("unexpected request component")
					}
				}
			}
		case streamClass:
			for r.err == nil {
				n := r.intN()
				if n == 0 {
					break
				}
				if n > maxStream || n > uint64(maxStream-len(o.data)) {
					r.fail("stream limit exceeded")
					break
				}
				o.data = append(o.data, r.take(int(n))...)
			}
		default:
			r.fail("unsupported native class")
		}
	default:
		r.fail(fmt.Sprintf("unsupported primitive tag %d", v.tag))
	}
	return v
}
func (o *object) get(name string) value {
	if o != nil {
		for _, f := range o.fields {
			if f.name == name {
				return f.value
			}
		}
	}
	return value{}
}
func numeric(v value) int64 {
	switch v.tag {
	case 1:
		return 1
	case 4:
		return int64(int32(v.n))
	case 5:
		return int64(v.n)
	case 6, 8:
		return int64(v.n)
	}
	return 0
}

func Decode(data []byte) (*File, error) {
	if int64(len(data)) > MaxBytes {
		return nil, fmt.Errorf("HALOG_LIMIT: input exceeds 256 MiB")
	}
	r := reader{data: data}
	count := r.read(0)
	if count.tag != 4 || numeric(count) < 0 || numeric(count) > 32 {
		r.fail("invalid native header")
	}
	f := &File{}
	seen := map[string]bool{}
	for i := int64(0); i < numeric(count) && r.err == nil; i++ {
		name := r.text()
		if seen[name] {
			r.fail("duplicate header")
		}
		seen[name] = true
		f.attributes = append(f.attributes, field{name, r.read(0)})
	}
	h := object{fields: f.attributes}
	f.codepage = defaultCodePage()
	if cp := h.get("ArchScopeCodePage"); cp.tag != 0 {
		f.codepage = int(numeric(cp))
		if _, err := charset(f.codepage); err != nil {
			r.fail("unsupported declared codepage")
		}
	}
	if numeric(h.get("version")) != 6 || string(h.get("TKey").data) != "0A0027000003" {
		r.fail("unsupported native format version")
	}
	if r.number(1) != 19 || r.text() != "qh.IntfList" {
		r.fail("expected native request list")
	}
	iid := r.read(0)
	if !bytes.Equal(iid.data, requestIID) {
		r.fail("unsupported request interface")
	}
	n := r.count(MaxEntries)
	f.entries = make([]value, 0, n)
	for i := 0; i < n && r.err == nil; i++ {
		v := r.read(0)
		if v.object == nil || v.object.class != requestClass {
			r.fail("expected native request")
		}
		f.entries = append(f.entries, v)
	}
	if r.err == nil && r.pos != len(data) {
		r.fail("unexpected trailing data")
	}
	return f, r.err
}

type writer struct {
	bytes.Buffer
	err error
}

func (w *writer) number(v uint64, n int) {
	for i := 0; i < n; i++ {
		w.WriteByte(byte(v))
		v >>= 8
	}
}
func (w *writer) intN(n int) {
	switch {
	case n < 240:
		w.number(uint64(n), 1)
	case n < 496:
		w.number(240, 1)
		w.number(uint64(n-240), 1)
	case n <= 65535:
		w.number(250, 1)
		w.number(uint64(n), 2)
	default:
		w.number(251, 1)
		w.number(uint64(n), 4)
	}
}
func (w *writer) text(s string) { w.write(str(s)) }
func str(s string) value {
	tag := byte(12)
	if len(s) > 255 {
		tag = 11
	}
	return value{tag: tag, data: []byte(s)}
}
func integer(n int64) value  { return value{tag: 4, n: uint64(uint32(n))} }
func cardinal(n int64) value { return value{tag: 8, n: uint64(uint32(n))} }
func boolean(b bool) value {
	if b {
		return value{tag: 1}
	}
	return value{}
}
func date(n float64) value { return value{tag: 14, n: math.Float64bits(n)} }
func (w *writer) bag(fields []field) {
	w.number(uint64(len(fields)), 4)
	for _, f := range fields {
		w.text(f.name)
		w.write(f.value)
	}
}
func (w *writer) write(v value) {
	if w.err != nil {
		return
	}
	if w.Len() > int(MaxBytes) {
		w.err = fmt.Errorf("HALOG_LIMIT: output exceeds 256 MiB")
		return
	}
	w.WriteByte(v.tag)
	switch v.tag {
	case 0, 1:
	case 4, 8:
		w.number(v.n, 4)
	case 5, 14, 17:
		w.number(v.n, 8)
	case 6:
		w.number(v.n, 1)
	case 11, 12:
		if len(v.data) > maxString {
			w.err = fmt.Errorf("HALOG_LIMIT: output string exceeds 16 MiB")
			return
		}
		if v.tag == 12 {
			w.number(uint64(len(v.data)), 1)
		} else {
			w.number(uint64(len(v.data)), 4)
		}
		w.Write(v.data)
	case 19:
		o := v.object
		if o == nil {
			w.err = fmt.Errorf("HALOG_INVALID: missing object")
			return
		}
		w.text(o.class)
		if o.class == streamClass {
			if len(o.data) > maxStream {
				w.err = fmt.Errorf("HALOG_LIMIT: output stream exceeds 16 MiB")
				return
			}
			if len(o.data) > 0 {
				w.intN(len(o.data))
				w.Write(o.data)
			}
			w.intN(0)
		} else {
			w.bag(o.fields)
			for _, v := range o.extra {
				w.write(v)
			}
		}
	default:
		w.err = fmt.Errorf("HALOG_INVALID: unsupported output tag")
	}
}
func (f *File) Encode() ([]byte, error) {
	w := writer{}
	w.write(integer(int64(len(f.attributes))))
	for _, a := range f.attributes {
		w.text(a.name)
		w.write(a.value)
	}
	w.WriteByte(19)
	w.text("qh.IntfList")
	w.write(value{tag: 12, data: requestIID})
	w.number(uint64(len(f.entries)), 4)
	for _, v := range f.entries {
		w.write(v)
	}
	if w.Len() > int(MaxBytes) {
		w.err = fmt.Errorf("HALOG_LIMIT: output exceeds 256 MiB")
	}
	if w.err != nil {
		return nil, w.err
	}
	return w.Bytes(), nil
}
