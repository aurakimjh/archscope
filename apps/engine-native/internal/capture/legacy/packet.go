package legacy

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func (d *decoder) packet() (models.ContentNode, error) {
	root := models.ContentNode{Kind: "amf-packet"}
	version, e := d.u16()
	if e != nil {
		return root, e
	}
	if version != 0 && version != 3 {
		return root, issue{"unsupported", "unsupported AMF packet version"}
	}
	root.Value = fmt.Sprint(version)
	for _, header := range []bool{true, false} {
		length, e := d.u16()
		if e != nil {
			return root, e
		}
		if e = count(length); e != nil {
			return root, e
		}
		for i := 0; i < length; i++ {
			name, e := d.utf()
			if e != nil {
				return root, e
			}
			target := name
			if header {
				_, e = d.byte()
			} else {
				var response string
				response, e = d.utf()
				target += " → " + response
			}
			if e != nil {
				return root, e
			}
			size, e := d.u32()
			if e != nil {
				return root, e
			}
			start := d.pos
			d.refs0 = 0
			d.refs3 = 0
			d.strings = nil
			d.traits = nil
			// A packet's declared message length is an enforced boundary, not a hint.
			end := len(d.data)
			if size != 0xffffffff {
				if uint64(size) > uint64(len(d.data)-start) {
					return root, issue{"invalid", "packet body length exceeds input"}
				}
				end = start + int(size)
			}
			all := d.data
			d.data = all[:end]
			child, e := d.amf0(1)
			d.data = all
			if e != nil {
				return root, e
			}
			if size != 0xffffffff && d.pos != end {
				return root, issue{"invalid", "packet body length mismatch"}
			}
			child.Name = target
			root.Children = append(root.Children, child)
		}
	}
	return root, nil
}
