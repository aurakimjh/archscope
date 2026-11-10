package proxy

import (
	"bytes"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"image"
	"image/png"
	"io"
	"net/http"
	"testing"
)

func TestRasterPreviewRequiresEnabledCompleteBody(t *testing.T) {
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	for _, capture := range []bool{false, true} {
		recorder := newBodyRecorder(io.NopCloser(bytes.NewReader(data.Bytes())), capture, DefaultMaxBodyBytes)
		io.Copy(io.Discard, recorder)
		message := models.HTTPMessage{ContentType: "image/png"}
		recorder.apply(&message, "")
		if (message.ImagePreview != nil) != capture {
			t.Fatalf("capture=%v preview=%v", capture, message.ImagePreview)
		}
	}
}

func TestCookieAttributesPreserveDuplicateNames(t *testing.T) {
	response := &http.Response{Header: http.Header{"Set-Cookie": []string{"id=one; Path=/a; Secure; HttpOnly; SameSite=Lax", "id=two; Path=/b; Max-Age=60"}}}
	cookies := responseCookies(response)
	if len(cookies) != 2 || cookies[0].Name != cookies[1].Name || len(cookies[0].Attributes) < 3 || cookies[1].Attributes[0].Value != "/b" {
		t.Fatalf("%+v", cookies)
	}
}

func TestLegacyPreviewRequiresBodyCapture(t *testing.T) {
	raw := []byte{2, 0, 2, 'h', 'i'}
	for _, enabled := range []bool{false, true} {
		recorder := newBodyRecorder(io.NopCloser(bytes.NewReader(raw)), enabled, DefaultMaxBodyBytes)
		io.Copy(io.Discard, recorder)
		message := models.HTTPMessage{ContentType: "application/amf0"}
		recorder.apply(&message, "")
		if (message.LegacyPreview != nil) != enabled {
			t.Fatalf("enabled=%v", enabled)
		}
		if enabled && message.LegacyPreview.Root.Children[0].Value != "hi" {
			t.Fatalf("%+v", message.LegacyPreview)
		}
	}
}
