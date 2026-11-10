package sessionfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// Minimal BIFF8/CFB tabular export. All cells are strings, so user content is
// never interpreted as an Excel formula. Explicit limits prevent truncation.
// Structures: MS-XLS 2.4.148, 2.5.294 and MS-CFB v3 (512-byte sectors).
func encodeXLS(rows [][]string) ([]byte, error) {
	if len(rows) > 65536 {
		return nil, fmt.Errorf("XLS supports at most 65535 exchanges; narrow scope or use HAR")
	}
	var book bytes.Buffer
	put16 := func(b []byte, offset int, n uint16) { binary.LittleEndian.PutUint16(b[offset:], n) }
	put32 := func(b []byte, offset int, n uint32) { binary.LittleEndian.PutUint32(b[offset:], n) }
	record := func(buf *bytes.Buffer, id uint16, data []byte) {
		binary.Write(buf, binary.LittleEndian, id)
		binary.Write(buf, binary.LittleEndian, uint16(len(data)))
		buf.Write(data)
	}
	bof := func(buf *bytes.Buffer, kind uint16) {
		b := make([]byte, 16)
		put16(b, 0, 0x600)
		put16(b, 2, kind)
		put16(b, 4, 0x0dbb)
		put16(b, 6, 0x07cc)
		put32(b, 8, 0x41)
		put32(b, 12, 6)
		record(buf, 0x809, b)
	}
	bof(&book, 5)
	record(&book, 0x42, []byte{0xb0, 0x04}) // Unicode codepage 1200
	font := make([]byte, 21)
	put16(font, 0, 200)
	put16(font, 4, 0x7fff)
	put16(font, 6, 400)
	font[12] = 5
	copy(font[16:], "Arial")
	font[14] = 5
	record(&book, 0x31, font)
	xf := make([]byte, 20)
	put16(xf, 4, 0xfff5)
	record(&book, 0xe0, xf)
	xf = make([]byte, 20)
	record(&book, 0xe0, xf)
	record(&book, 0x293, []byte{0, 0x80, 0, 0xff})
	window := make([]byte, 18)
	put16(window, 4, 0x3fcf)
	put16(window, 6, 0x25bc)
	put16(window, 8, 0x38)
	put16(window, 14, 1)
	put16(window, 16, 600)
	record(&book, 0x3d, window)
	bound := make([]byte, 12)
	bound[6] = 4
	copy(bound[8:], "HTTP")
	boundOffset := book.Len() + 4
	record(&book, 0x85, bound)
	record(&book, 0x0a, nil)
	put32(book.Bytes(), boundOffset, uint32(book.Len()))
	bof(&book, 0x10)
	dim := make([]byte, 14)
	put32(dim, 4, uint32(len(rows)))
	put16(dim, 10, 17)
	record(&book, 0x200, dim)
	for r, row := range rows {
		for c, value := range row {
			units := utf16.Encode([]rune(value))
			if len(units) > 4000 {
				return nil, fmt.Errorf("XLS row %d column %d exceeds 4000 UTF-16 units; use full XML or HAR", r+1, c+1)
			}
			cell := make([]byte, 9+len(units)*2)
			put16(cell, 0, uint16(r))
			put16(cell, 2, uint16(c))
			put16(cell, 4, 1)
			put16(cell, 6, uint16(len(units)))
			cell[8] = 1
			for j, v := range units {
				put16(cell, 9+j*2, v)
			}
			record(&book, 0x204, cell)
		}
	}
	w2 := make([]byte, 18)
	put16(w2, 0, 0x6b6)
	record(&book, 0x23e, w2)
	record(&book, 0x0a, nil)
	size := book.Len()
	if size < 4096 {
		size = 4096
	}
	sectors := (size + 511) / 512
	fatCount := 1
	for (sectors+1+fatCount+127)/128 > fatCount {
		fatCount++
	}
	if fatCount > 109 {
		return nil, fmt.Errorf("XLS exceeds 7 MiB container limit; use HAR or XML")
	}
	const free = uint32(0xffffffff)
	const end = uint32(0xfffffffe)
	const fat = uint32(0xfffffffd)
	out := make([]byte, 512+(sectors+1+fatCount)*512)
	header := out[:512]
	copy(header, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1})
	put16(header, 24, 0x3e)
	put16(header, 26, 3)
	put16(header, 28, 0xfffe)
	put16(header, 30, 9)
	put16(header, 32, 6)
	put32(header, 44, uint32(fatCount))
	put32(header, 48, uint32(sectors))
	put32(header, 56, 4096)
	put32(header, 60, end)
	put32(header, 68, end)
	for i := 0; i < 109; i++ {
		v := free
		if i < fatCount {
			v = uint32(sectors + 1 + i)
		}
		put32(header, 76+i*4, v)
	}
	copy(out[512:], book.Bytes())
	directory := out[512+sectors*512:]
	entry := func(index int, name string, kind byte, start uint32, length uint64) {
		b := directory[index*128 : (index+1)*128]
		units := utf16.Encode([]rune(name + "\x00"))
		for j, u := range units {
			put16(b, j*2, u)
		}
		put16(b, 64, uint16(len(units)*2))
		b[66] = kind
		b[67] = 1
		put32(b, 68, free)
		put32(b, 72, free)
		put32(b, 76, free)
		put32(b, 116, start)
		binary.LittleEndian.PutUint64(b[120:], length)
	}
	entry(0, "Root Entry", 5, end, 0)
	put32(directory, 76, 1)
	entry(1, "Workbook", 2, 0, uint64(size))
	table := out[512+(sectors+1)*512:]
	for i := 0; i < fatCount*128; i++ {
		v := free
		switch {
		case i < sectors-1:
			v = uint32(i + 1)
		case i == sectors-1 || i == sectors:
			v = end
		case i > sectors && i < sectors+1+fatCount:
			v = fat
		}
		put32(table, i*4, v)
	}
	return out, nil
}
