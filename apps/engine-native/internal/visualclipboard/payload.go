// Package visualclipboard writes already-rendered, bounded report representations.
package visualclipboard

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"image/png"
)

type Payload struct {
	PNG  []byte
	HTML string
	Text string
}

func HTMLFormat(fragment string) []byte {
	const prefix = "<html><head><meta charset=\"utf-8\"></head><body><!--StartFragment-->"
	const suffix = "<!--EndFragment--></body></html>"
	const format = "Version:1.0\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"
	size := len(fmt.Sprintf(format, 0, 0, 0, 0))
	header := fmt.Sprintf(format, size, size+len(prefix)+len(fragment)+len(suffix), size+len(prefix), size+len(prefix)+len(fragment))
	return append([]byte(header+prefix+fragment+suffix), 0)
}
func DIBV5(pngBytes []byte) ([]byte, error) {
	config, e := png.DecodeConfig(bytes.NewReader(pngBytes))
	if e != nil {
		return nil, e
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 3200 || config.Height > 2400 || config.Width*config.Height > 8_000_000 || len(pngBytes) > 16<<20 {
		return nil, fmt.Errorf("CLIPBOARD_IMAGE_BUDGET")
	}
	image, e := png.Decode(bytes.NewReader(pngBytes))
	if e != nil {
		return nil, e
	}
	w, h := config.Width, config.Height
	out := make([]byte, 124+w*h*4)
	put := func(offset int, value uint32) { binary.LittleEndian.PutUint32(out[offset:], value) }
	put(0, 124)
	put(4, uint32(w))
	put(8, uint32(-int32(h)))
	binary.LittleEndian.PutUint16(out[12:], 1)
	binary.LittleEndian.PutUint16(out[14:], 32)
	put(16, 3)
	put(20, uint32(w*h*4))
	put(24, 3780)
	put(28, 3780)
	put(40, 0x00ff0000)
	put(44, 0x0000ff00)
	put(48, 0x000000ff)
	put(52, 0xff000000)
	put(56, 0x73524742)
	put(108, 4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(image.At(image.Bounds().Min.X+x, image.Bounds().Min.Y+y)).(color.NRGBA)
			offset := 124 + (y*w+x)*4
			copy(out[offset:], []byte{c.B, c.G, c.R, c.A})
		}
	}
	return out, nil
}
func (p Payload) Validate() error {
	if len(p.PNG) > 16<<20 || len(p.HTML) > 4<<20 || len(p.Text) > 4<<20 || (len(p.PNG) == 0 && (p.HTML == "" || p.Text == "")) {
		return fmt.Errorf("CLIPBOARD_PAYLOAD_INVALID")
	}
	return nil
}
