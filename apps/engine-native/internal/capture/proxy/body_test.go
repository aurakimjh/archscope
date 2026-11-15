package proxy

import (
	"bytes"
	"compress/gzip"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/summary"
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

func TestBinaryPreviewCapRemainsVisibleToSummary(t *testing.T) {
	for _, size := range []int{128, 8192} {
		payload := bytes.Repeat([]byte{0xff, 0x00}, size/2)
		for _, encoding := range []string{"", "gzip"} {
			wire := payload
			if encoding == "gzip" {
				var packed bytes.Buffer
				g := gzip.NewWriter(&packed)
				g.Write(payload)
				g.Close()
				wire = packed.Bytes()
			}
			recorder := newBodyRecorder(io.NopCloser(bytes.NewReader(wire)), true, 128)
			if _, err := io.Copy(io.Discard, recorder); err != nil {
				t.Fatal(err)
			}
			message := models.HTTPMessage{ContentType: "application/octet-stream"}
			recorder.apply(&message, encoding)
			expectedStorage := "binary"
			expectedKnown := 1
			if size > 128 {
				expectedStorage = "truncated"
				expectedKnown = 0
			}
			totals := summary.Build([]models.CaptureTransaction{{State: models.TxComplete, Response: message}})
			if message.BodyStorage != expectedStorage || message.BodyDecoded != 128 || message.BodySize != int64(len(wire)) || message.BodyPreview != "" || message.ImagePreview != nil || totals.CompressionKnown != expectedKnown {
				t.Errorf("size=%d encoding=%q: storage=%s decoded=%d wire=%d compressionKnown=%d; want storage=%s known=%d", size, encoding, message.BodyStorage, message.BodyDecoded, message.BodySize, totals.CompressionKnown, expectedStorage, expectedKnown)
			}
		}
	}
}

func TestUnavailableDecodedSizesStayUnknown(t *testing.T) {
	for _, test := range []struct {
		name, encoding   string
		capture, missing bool
		payload          []byte
		decoded          int64
	}{
		{"missing", "", false, true, nil, -1},
		{"omitted", "", false, false, []byte("abc"), -1},
		{"unsupported", "br", true, false, []byte("abc"), -1},
		{"invalid-gzip", "gzip", true, false, []byte("abc"), -1},
		{"observed-empty", "", true, false, nil, 0},
	} {
		var recorder *bodyRecorder
		if !test.missing {
			recorder = newBodyRecorder(io.NopCloser(bytes.NewReader(test.payload)), test.capture, 128)
			io.Copy(io.Discard, recorder)
		}
		message := models.HTTPMessage{}
		applyBody(&message, recorder, test.encoding)
		if message.BodyDecoded != test.decoded {
			t.Errorf("%s decoded=%d want=%d", test.name, message.BodyDecoded, test.decoded)
		}
		if test.missing && (message.BodySize != -1 || message.TransferSize != -1) {
			t.Errorf("unobserved recorder must not invent transferred bytes: %+v", message)
		}
	}
}
