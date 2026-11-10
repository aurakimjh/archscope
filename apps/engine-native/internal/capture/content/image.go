// Package content creates bounded inert previews. Raster previews are re-encoded
// to PNG to remove metadata and active containers; pixel content is not redacted.
package content

import (
	"bytes"
	"encoding/base64"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"strings"
)

const MaxImageBytes = 1 << 20

func Raster(raw []byte, mime string) *models.RasterPreview {
	mime = strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	if len(raw) > MaxImageBytes || len(raw) == 0 || (mime != "image/png" && mime != "image/jpeg" && mime != "image/gif") {
		return nil
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 2048 || config.Height > 2048 || config.Width*config.Height > 1048576 {
		return nil
	}
	if (format == "png" && mime != "image/png") || (format == "jpeg" && mime != "image/jpeg") || (format == "gif" && mime != "image/gif") {
		return nil
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	var out bytes.Buffer
	if png.Encode(&out, decoded) != nil || out.Len() > MaxImageBytes {
		return nil
	}
	return &models.RasterPreview{MIME: "image/png", Base64: base64.StdEncoding.EncodeToString(out.Bytes()), Width: config.Width, Height: config.Height}
}
func Clean(preview *models.RasterPreview) *models.RasterPreview {
	if preview == nil || len(preview.Base64) > base64.StdEncoding.EncodedLen(MaxImageBytes) {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(preview.Base64)
	if err != nil {
		return nil
	}
	return Raster(raw, preview.MIME)
}
