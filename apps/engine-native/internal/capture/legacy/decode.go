// Package legacy reads serialization tokens into inert data nodes. It never
// instantiates classes, invokes externalizers, converters or BinaryFormatter.
package legacy

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"math"
	"strconv"
	"unicode/utf8"
)

const MaxBytes = 1 << 20
const MaxNodes = 4096
const MaxDepth = 32

type issue struct{ state, message string }

func (e issue) Error() string { return e.message }

type trait struct {
	name              string
	fields            []string
	dynamic, external bool
}
type decoder struct {
	vsTypes                  []string
	data                     []byte
	pos, nodes, refs0, refs3 int
	strings                  []string
	traits                   []trait
	vsStrings                []string
	vsIndex                  int
}

func Decode(format string, data []byte) models.LegacyPreview {
	result := models.LegacyPreview{Format: format, State: "available"}
	if len(data) > MaxBytes {
		result.State = "limited"
		result.Error = "input exceeds 1 MiB"
		return result
	}
	d := decoder{data: data, vsTypes: []string{"System.Object", "System.Int32", "System.String", "System.Boolean"}}
	var node models.ContentNode
	var err error
	switch format {
	case "amf0", "amf3":
		node = models.ContentNode{Kind: "sequence"}
		for d.pos < len(data) {
			var child models.ContentNode
			if format == "amf0" {
				child, err = d.amf0(0)
			} else {
				child, err = d.amf3(0)
			}
			if err != nil {
				break
			}
			node.Children = append(node.Children, child)
		}
		if len(data) == 0 {
			err = issue{"invalid", "empty AMF payload"}
		}
	case "amf_packet":
		node, err = d.packet()
	case "viewstate":
		if len(data) < 2 || data[0] != 0xff || data[1] != 1 {
			err = issue{"unsupported", "not an unencrypted ObjectStateFormatter v1 payload"}
		} else {
			d.pos = 2
			node, err = d.viewstate(0)
		}
	default:
		err = issue{"unsupported", "unknown serialization format"}
	}
	result.Consumed = d.pos
	result.TrailingBytes = len(data) - d.pos
	if err != nil {
		result.State = "invalid"
		if e, ok := err.(issue); ok {
			result.State = e.state
		}
		result.Error = err.Error()
	} else {
		if e := ValidateTree(&node); e != nil {
			result.State = "limited"
			result.Error = e.Error()
		} else {
			result.Root = &node
		}
	}
	return result
}
func DecodeBase64(format, value string) models.LegacyPreview {
	if len(value) > base64.StdEncoding.EncodedLen(MaxBytes) {
		return models.LegacyPreview{Format: format, State: "limited", Error: "base64 input exceeds 1 MiB"}
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return models.LegacyPreview{Format: format, State: "invalid", Error: "invalid base64 payload"}
	}
	return Decode(format, data)
}
func (d *decoder) take(n int) ([]byte, error) {
	if n < 0 || n > len(d.data)-d.pos {
		return nil, issue{"invalid", fmt.Sprintf("truncated data at offset %d", d.pos)}
	}
	v := d.data[d.pos : d.pos+n]
	d.pos += n
	return v, nil
}
func (d *decoder) byte() (byte, error) {
	b, e := d.take(1)
	if e != nil {
		return 0, e
	}
	return b[0], nil
}
func (d *decoder) u16() (int, error) {
	b, e := d.take(2)
	if e != nil {
		return 0, e
	}
	return int(binary.BigEndian.Uint16(b)), nil
}
func (d *decoder) u32() (uint32, error) {
	b, e := d.take(4)
	if e != nil {
		return 0, e
	}
	return binary.BigEndian.Uint32(b), nil
}
func (d *decoder) text(n int) (string, error) {
	if n > 65536 {
		return "", issue{"limited", "string exceeds 64 KiB"}
	}
	b, e := d.take(n)
	if e != nil {
		return "", e
	}
	if !utf8.Valid(b) {
		return "", issue{"invalid", "invalid UTF-8 string"}
	}
	return string(b), nil
}
func (d *decoder) utf() (string, error) {
	n, e := d.u16()
	if e != nil {
		return "", e
	}
	return d.text(n)
}
func (d *decoder) number(order binary.ByteOrder) (string, error) {
	b, e := d.take(8)
	if e != nil {
		return "", e
	}
	return strconv.FormatFloat(math.Float64frombits(order.Uint64(b)), 'g', -1, 64), nil
}
func (d *decoder) node(depth int) (models.ContentNode, error) {
	d.nodes++
	if depth > MaxDepth || d.nodes > MaxNodes {
		return models.ContentNode{}, issue{"limited", "node/depth limit exceeded"}
	}
	return models.ContentNode{Offset: d.pos}, nil
}
func count(n int) error {
	if n < 0 || n > MaxNodes {
		return issue{"limited", "collection exceeds 4096 entries"}
	}
	return nil
}
func ref(index, total int) (models.ContentNode, error) {
	if index < 0 || index >= total {
		return models.ContentNode{}, issue{"invalid", "invalid reference index"}
	}
	return models.ContentNode{Kind: "reference", Value: strconv.Itoa(index)}, nil
}
func (d *decoder) amf0(depth int) (models.ContentNode, error) {
	n, e := d.node(depth)
	if e != nil {
		return n, e
	}
	marker, e := d.byte()
	if e != nil {
		return n, e
	}
	switch marker {
	case 0:
		n.Kind = "number"
		n.Value, e = d.number(binary.BigEndian)
	case 1:
		n.Kind = "boolean"
		var b byte
		b, e = d.byte()
		if e == nil && b > 1 {
			return n, issue{"invalid", "invalid boolean"}
		}
		n.Value = strconv.FormatBool(b != 0)
	case 2:
		n.Kind = "string"
		n.Value, e = d.utf()
	case 5:
		n.Kind = "null"
	case 6:
		n.Kind = "undefined"
	case 7:
		var index int
		index, e = d.u16()
		if e == nil {
			return ref(index, d.refs0)
		}
	case 3, 8, 16:
		d.refs0++
		n.Kind = "object"
		if marker == 16 {
			n.Value, e = d.utf()
			if e != nil {
				return n, e
			}
		}
		if marker == 8 {
			_, e = d.u32()
			if e != nil {
				return n, e
			}
			n.Kind = "ecma-array"
		}
		for {
			if len(n.Children) >= MaxNodes {
				return n, issue{"limited", "object exceeds node budget"}
			}
			var name string
			name, e = d.utf()
			if e != nil {
				return n, e
			}
			if name == "" && d.pos < len(d.data) && d.data[d.pos] == 9 {
				d.pos++
				break
			}
			var child models.ContentNode
			child, e = d.amf0(depth + 1)
			if e != nil {
				return n, e
			}
			child.Name = name
			n.Children = append(n.Children, child)
		}
	case 10:
		d.refs0++
		n.Kind = "array"
		var length uint32
		length, e = d.u32()
		if e != nil {
			return n, e
		}
		if e = count(int(length)); e != nil {
			return n, e
		}
		for i := 0; i < int(length); i++ {
			var child models.ContentNode
			child, e = d.amf0(depth + 1)
			if e != nil {
				return n, e
			}
			child.Name = strconv.Itoa(i)
			n.Children = append(n.Children, child)
		}
	case 11:
		d.refs0++
		n.Kind = "date_ms"
		n.Value, e = d.number(binary.BigEndian)
		if e == nil {
			_, e = d.take(2)
		}
	case 12, 15:
		var length uint32
		length, e = d.u32()
		if e == nil {
			n.Kind = "string"
			if marker == 15 {
				n.Kind = "xml"
				d.refs0++
			}
			n.Value, e = d.text(int(length))
		}
	case 17:
		return d.amf3(depth + 1)
	default:
		return n, issue{"unsupported", fmt.Sprintf("unsupported AMF0 marker 0x%02x at %d", marker, n.Offset)}
	}
	return n, e
}
func (d *decoder) u29() (uint32, error) {
	var value uint32
	for i := 0; i < 4; i++ {
		b, e := d.byte()
		if e != nil {
			return 0, e
		}
		if i == 3 {
			return value<<8 | uint32(b), nil
		}
		value = value<<7 | uint32(b&127)
		if b&128 == 0 {
			return value, nil
		}
	}
	return value, nil
}
func (d *decoder) string3() (string, error) {
	h, e := d.u29()
	if e != nil {
		return "", e
	}
	if h&1 == 0 {
		index := int(h >> 1)
		if index >= len(d.strings) {
			return "", issue{"invalid", "invalid string reference"}
		}
		return d.strings[index], nil
	}
	v, e := d.text(int(h >> 1))
	if e == nil && v != "" {
		d.strings = append(d.strings, v)
	}
	return v, e
}
func (d *decoder) amf3(depth int) (models.ContentNode, error) {
	n, e := d.node(depth)
	if e != nil {
		return n, e
	}
	marker, e := d.byte()
	if e != nil {
		return n, e
	}
	switch marker {
	case 0:
		n.Kind = "undefined"
		return n, nil
	case 1:
		n.Kind = "null"
		return n, nil
	case 2, 3:
		n.Kind = "boolean"
		n.Value = strconv.FormatBool(marker == 3)
		return n, nil
	case 4:
		h, e := d.u29()
		n.Kind = "integer"
		signed := int32(h)
		if h&0x10000000 != 0 {
			signed |= ^int32(0x1fffffff)
		}
		n.Value = strconv.FormatInt(int64(signed), 10)
		return n, e
	case 5:
		n.Kind = "number"
		n.Value, e = d.number(binary.BigEndian)
		return n, e
	case 6:
		n.Kind = "string"
		n.Value, e = d.string3()
		return n, e
	}
	if marker < 7 || marker > 17 {
		return n, issue{"unsupported", fmt.Sprintf("unsupported AMF3 marker 0x%02x", marker)}
	}
	h, e := d.u29()
	if e != nil {
		return n, e
	}
	if h&1 == 0 {
		return ref(int(h>>1), d.refs3)
	}
	d.refs3++
	switch marker {
	case 7, 11:
		n.Kind = "xml"
		n.Value, e = d.text(int(h >> 1))
	case 8:
		n.Kind = "date_ms"
		n.Value, e = d.number(binary.BigEndian)
	case 9:
		length := int(h >> 1)
		if e = count(length); e != nil {
			return n, e
		}
		n.Kind = "array"
		for {
			var name string
			name, e = d.string3()
			if e != nil {
				return n, e
			}
			if name == "" {
				break
			}
			var child models.ContentNode
			child, e = d.amf3(depth + 1)
			if e != nil {
				return n, e
			}
			child.Name = name
			n.Children = append(n.Children, child)
		}
		for i := 0; i < length; i++ {
			var child models.ContentNode
			child, e = d.amf3(depth + 1)
			if e != nil {
				return n, e
			}
			child.Name = strconv.Itoa(i)
			n.Children = append(n.Children, child)
		}
	case 10:
		var tr trait
		if h&2 == 0 {
			index := int(h >> 2)
			if index >= len(d.traits) {
				return n, issue{"invalid", "invalid trait reference"}
			}
			tr = d.traits[index]
		} else {
			tr.external = h&4 != 0
			tr.dynamic = h&8 != 0
			length := int(h >> 4)
			if e = count(length); e != nil {
				return n, e
			}
			tr.name, e = d.string3()
			if e != nil {
				return n, e
			}
			for i := 0; i < length; i++ {
				name, err := d.string3()
				if err != nil {
					return n, err
				}
				tr.fields = append(tr.fields, name)
			}
			d.traits = append(d.traits, tr)
		}
		if tr.external {
			return n, issue{"unsupported", "externalizable object requires a class-specific format; no code was executed"}
		}
		n.Kind = "object"
		n.Value = tr.name
		for _, name := range tr.fields {
			child, err := d.amf3(depth + 1)
			if err != nil {
				return n, err
			}
			child.Name = name
			n.Children = append(n.Children, child)
		}
		if tr.dynamic {
			for {
				name, err := d.string3()
				if err != nil {
					return n, err
				}
				if name == "" {
					break
				}
				child, err := d.amf3(depth + 1)
				if err != nil {
					return n, err
				}
				child.Name = name
				n.Children = append(n.Children, child)
			}
		}
	case 12:
		n.Kind = "byte-array"
		length := int(h >> 1)
		if length > 65536 {
			return n, issue{"limited", "byte array exceeds 64 KiB"}
		}
		var value []byte
		value, e = d.take(length)
		n.Value = fmt.Sprintf("%d opaque bytes omitted", len(value))
	case 13, 14, 15, 16:
		length := int(h >> 1)
		if e = count(length); e != nil {
			return n, e
		}
		n.Kind = "vector"
		_, e = d.byte()
		if e != nil {
			return n, e
		}
		if marker == 16 {
			n.Value, e = d.string3()
			if e != nil {
				return n, e
			}
		}
		for i := 0; i < length; i++ {
			child := models.ContentNode{Name: strconv.Itoa(i), Kind: "number", Offset: d.pos}
			if marker == 16 {
				child, e = d.amf3(depth + 1)
				child.Name = strconv.Itoa(i)
			} else if marker == 15 {
				child.Value, e = d.number(binary.BigEndian)
			} else {
				var v uint32
				v, e = d.u32()
				if marker == 13 {
					child.Value = strconv.FormatInt(int64(int32(v)), 10)
				} else {
					child.Value = strconv.FormatUint(uint64(v), 10)
				}
			}
			if e != nil {
				return n, e
			}
			d.nodes++
			if d.nodes > MaxNodes {
				return n, issue{"limited", "vector exceeds node budget"}
			}
			n.Children = append(n.Children, child)
		}
	case 17:
		length := int(h >> 1)
		if e = count(length); e != nil {
			return n, e
		}
		n.Kind = "dictionary"
		_, e = d.byte()
		if e != nil {
			return n, e
		}
		for i := 0; i < length; i++ {
			key, err := d.amf3(depth + 1)
			if err != nil {
				return n, err
			}
			value, err := d.amf3(depth + 1)
			if err != nil {
				return n, err
			}
			if key.Kind == "string" {
				value.Name = key.Value
			}
			n.Children = append(n.Children, models.ContentNode{Name: strconv.Itoa(i), Kind: "entry", Children: []models.ContentNode{key, value}})
		}
	default:
		return n, issue{"unsupported", fmt.Sprintf("unsupported AMF3 marker 0x%02x", marker)}
	}
	return n, e
}

func ValidateTree(root *models.ContentNode) error {
	nodes, total := 0, 0
	var visit func(*models.ContentNode, int) error
	visit = func(node *models.ContentNode, depth int) error {
		if node == nil {
			return nil
		}
		nodes++
		total += len(node.Name) + len(node.Value) + len(node.Kind)
		if depth > MaxDepth || nodes > MaxNodes || total > MaxBytes || node.Offset < 0 || node.Offset > MaxBytes {
			return issue{"limited", "decoded tree exceeds output budget"}
		}
		for i := range node.Children {
			if e := visit(&node.Children[i], depth+1); e != nil {
				return e
			}
		}
		return nil
	}
	return visit(root, 0)
}
