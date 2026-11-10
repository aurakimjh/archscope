package content

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestRasterOnlyBoundedPNG(t *testing.T) {
	var data bytes.Buffer
	sample := image.NewRGBA(image.Rect(0, 0, 2, 2))
	sample.Set(0, 0, color.RGBA{R: 255, A: 255})
	png.Encode(&data, sample)
	preview := Raster(data.Bytes(), "image/png")
	if preview == nil || preview.Width != 2 || Clean(preview) == nil {
		t.Fatal(preview)
	}
	if Raster([]byte(`<svg onload="alert(1)"/>`), "image/svg+xml") != nil {
		t.Fatal("active vector accepted")
	}
	if Raster([]byte(strings.Repeat("x", MaxImageBytes+1)), "image/png") != nil {
		t.Fatal("oversized image accepted")
	}
	if Raster(data.Bytes(), "image/jpeg") != nil {
		t.Fatal("MIME mismatch accepted")
	}
}
