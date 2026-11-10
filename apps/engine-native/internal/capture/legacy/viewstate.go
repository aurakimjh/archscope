package legacy

import (
	"encoding/binary"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"math"
	"strconv"
	"unicode/utf8"
)

func (d *decoder) varint() (uint32, error) {
	var value uint32
	for i := 0; i < 5; i++ {
		b, e := d.byte()
		if e != nil {
			return 0, e
		}
		if i == 4 && b > 15 {
			return 0, issue{"invalid", "invalid encoded int32"}
		}
		value |= uint32(b&127) << uint(i*7)
		if b&128 == 0 {
			return value, nil
		}
	}
	return 0, issue{"invalid", "invalid encoded int32"}
}
func (d *decoder) vsText() (string, error) {
	length, e := d.varint()
	if e != nil {
		return "", e
	}
	return d.text(int(length))
}
func (d *decoder) vsType() (string, error) {
	token, e := d.byte()
	if e != nil {
		return "", e
	}
	switch token {
	case 41, 42:
		name, err := d.vsText()
		if err == nil {
			d.vsTypes = append(d.vsTypes, name)
		}
		return name, err
	case 43:
		index, e := d.varint()
		if e != nil {
			return "", e
		}
		if uint64(index) >= uint64(len(d.vsTypes)) {
			return "", issue{"invalid", "invalid type reference"}
		}
		return d.vsTypes[index], nil
	default:
		return "", issue{"unsupported", "unsupported ViewState type descriptor"}
	}
}
func (d *decoder) viewstate(depth int) (models.ContentNode, error) {
	n, e := d.node(depth)
	if e != nil {
		return n, e
	}
	token, e := d.byte()
	if e != nil {
		return n, e
	}
	switch token {
	case 100:
		n.Kind = "null"
	case 101:
		n.Kind = "string"
	case 102:
		n.Kind = "integer"
		n.Value = "0"
	case 103, 104:
		n.Kind = "boolean"
		n.Value = strconv.FormatBool(token == 103)
	case 1:
		b, err := d.take(2)
		if err != nil {
			return n, err
		}
		n.Kind = "integer"
		n.Value = strconv.FormatInt(int64(int16(binary.LittleEndian.Uint16(b))), 10)
	case 2:
		v, err := d.varint()
		n.Kind = "integer"
		n.Value = strconv.FormatInt(int64(int32(v)), 10)
		e = err
	case 3:
		v, err := d.byte()
		n.Kind = "byte"
		n.Value = strconv.Itoa(int(v))
		e = err
	case 4:
		if d.pos >= len(d.data) {
			return n, issue{"invalid", "truncated char"}
		}
		r, size := utf8.DecodeRune(d.data[d.pos:])
		if r == utf8.RuneError && size == 1 {
			return n, issue{"invalid", "invalid UTF-8 char"}
		}
		d.pos += size
		n.Kind = "char"
		n.Value = string(r)
	case 5:
		n.Kind = "string"
		n.Value, e = d.vsText()
	case 6:
		b, err := d.take(8)
		if err != nil {
			return n, err
		}
		n.Kind = "dotnet_datetime_bits"
		n.Value = strconv.FormatUint(binary.LittleEndian.Uint64(b), 10)
	case 7:
		n.Kind = "number"
		n.Value, e = d.number(binary.LittleEndian)
	case 8:
		b, err := d.take(4)
		if err != nil {
			return n, err
		}
		n.Kind = "number"
		n.Value = strconv.FormatFloat(float64(math.Float32frombits(binary.LittleEndian.Uint32(b))), 'g', -1, 32)
	case 9:
		b, err := d.take(4)
		if err != nil {
			return n, err
		}
		n.Kind = "argb"
		n.Value = fmt.Sprintf("%08x", binary.LittleEndian.Uint32(b))
	case 10:
		v, err := d.varint()
		n.Kind = "known-color"
		n.Value = strconv.FormatUint(uint64(v), 10)
		e = err
	case 12:
		n.Kind = "empty-color"
	case 15, 16:
		n.Kind = "pair"
		length := 2
		if token == 16 {
			n.Kind = "triplet"
			length = 3
		}
		for i := 0; i < length; i++ {
			child, err := d.viewstate(depth + 1)
			if err != nil {
				return n, err
			}
			child.Name = strconv.Itoa(i)
			n.Children = append(n.Children, child)
		}
	case 20, 21, 22, 23, 24, 60:
		n.Kind = "array"
		if token == 20 || token == 60 {
			n.Value, e = d.vsType()
			if e != nil {
				return n, e
			}
		}
		length, err := d.varint()
		if err != nil {
			return n, err
		}
		if e = count(int(length)); e != nil {
			return n, e
		}
		items := length
		if token == 60 {
			items, e = d.varint()
			if e != nil {
				return n, e
			}
			if items > length {
				return n, issue{"invalid", "invalid sparse array length"}
			}
		}
		if token == 23 || token == 24 {
			n.Kind = "dictionary"
		}
		for i := uint32(0); i < items; i++ {
			index := i
			if token == 60 {
				index, e = d.varint()
				if e != nil {
					return n, e
				}
				if index >= length {
					return n, issue{"invalid", "invalid sparse array index"}
				}
			}
			var child models.ContentNode
			if token == 21 {
				child, e = d.node(depth + 1)
				if e == nil {
					child.Kind = "string"
					child.Value, e = d.vsText()
				}
			} else if token == 23 || token == 24 {
				key, err := d.viewstate(depth + 1)
				if err != nil {
					return n, err
				}
				value, err := d.viewstate(depth + 1)
				if err != nil {
					return n, err
				}
				child, e = d.node(depth + 1)
				child.Kind = "entry"
				if key.Kind == "string" {
					value.Name = key.Value
				}
				child.Children = []models.ContentNode{key, value}
			} else {
				child, e = d.viewstate(depth + 1)
			}
			if e != nil {
				return n, e
			}
			child.Name = strconv.FormatUint(uint64(index), 10)
			n.Children = append(n.Children, child)
		}
	case 25:
		n.Kind = "type-name"
		n.Value, e = d.vsType()
	case 27:
		n.Kind = "unit"
		n.Value, e = d.number(binary.LittleEndian)
		if e == nil {
			var bits []byte
			bits, e = d.take(4)
			if e == nil {
				n.Value += " (unit " + strconv.FormatUint(uint64(binary.LittleEndian.Uint32(bits)), 10) + ")"
			}
		}
	case 28:
		n.Kind = "empty-unit"
	case 30:
		n.Kind = "string"
		n.Value, e = d.vsText()
		if e == nil {
			if len(d.vsStrings) < 255 {
				d.vsStrings = append(d.vsStrings, n.Value)
			} else {
				d.vsStrings[d.vsIndex] = n.Value
			}
			d.vsIndex = (d.vsIndex + 1) % 255
		}
	case 31:
		index, err := d.byte()
		if err != nil {
			return n, err
		}
		if int(index) >= len(d.vsStrings) {
			return n, issue{"invalid", "invalid indexed string reference"}
		}
		n.Kind = "string"
		n.Value = d.vsStrings[index]
	case 11:
		n.Kind = "enum"
		n.Value, e = d.vsType()
		if e == nil {
			var v uint32
			v, e = d.varint()
			n.Value += "=" + strconv.FormatInt(int64(int32(v)), 10)
		}
	case 40, 50:
		return n, issue{"unsupported", "type converters and binary-serialized objects are intentionally not instantiated"}
	default:
		return n, issue{"unsupported", fmt.Sprintf("unsupported ViewState token %d at %d", token, n.Offset)}
	}
	return n, e
}
