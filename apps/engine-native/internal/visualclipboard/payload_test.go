package visualclipboard

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"
)

func TestHTMLFormatUTF8Offsets(t *testing.T) {
	fragment := `<table><tr><td>한글 9007199254740993</td></tr></table>`
	body := HTMLFormat(fragment)
	fields := map[string]int{}
	for _, line := range strings.Split(string(body), "\r\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || key == "Version" {
			continue
		}
		if n, e := strconv.Atoi(value); e == nil {
			fields[key] = n
		}
	}
	start, end := fields["StartFragment"], fields["EndFragment"]
	if string(body[start:end]) != fragment || body[len(body)-1] != 0 || fields["EndHTML"] != len(body)-1 {
		t.Fatal("UTF8 byte offsets invalid", fields)
	}
	if !strings.HasPrefix(string(body[fields["StartHTML"]:]), "<html>") {
		t.Fatal("HTML context offset")
	}
}
func TestDIBV5TopDownStraightAlphaAndBounds(t *testing.T) {
	input := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	input.SetNRGBA(0, 0, color.NRGBA{R: 120, G: 60, B: 30, A: 128})
	input.SetNRGBA(1, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	var pngBody bytes.Buffer
	if e := png.Encode(&pngBody, input); e != nil {
		t.Fatal(e)
	}
	dib, e := DIBV5(pngBody.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if len(dib) != 132 || int32(binary.LittleEndian.Uint32(dib[8:])) != -1 || binary.LittleEndian.Uint32(dib[52:]) != 0xff000000 || !bytes.Equal(dib[124:], []byte{30, 60, 120, 128, 30, 20, 10, 255}) {
		t.Fatal("bitmap masks/orientation/alpha", fmt.Sprintf("%x", dib[124:]))
	}
	if _, e = DIBV5([]byte("not a png")); e == nil {
		t.Fatal("invalid bitmap accepted")
	}
}
