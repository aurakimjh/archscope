package content

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"net/url"
	"strings"
	"testing"
)

func TestViewStateChunksAndHTMLSourceMask(t *testing.T) {
	raw, _ := hex.DecodeString("ff010f0502686967")
	encoded := base64.StdEncoding.EncodeToString(raw)
	form := "__VIEWSTATEFIELDCOUNT=2&__VIEWSTATE=" + url.QueryEscape(encoded[:4]) + "&__VIEWSTATE1=" + url.QueryEscape(encoded[4:])
	got := DecodeLegacy([]byte(form), "application/x-www-form-urlencoded")
	if got == nil || got.State != "available" || got.Root.Children[0].Value != "hi" {
		t.Fatalf("%+v", got)
	}
	html := `<input type="hidden" name="__VIEWSTATE" value="` + encoded + `">`
	got = DecodeLegacy([]byte(html), "text/html")
	if got == nil || got.State != "available" {
		t.Fatalf("%+v", got)
	}
	policy := redact.NewPolicy(redact.Options{})
	masked, _ := policy.RedactBody("text/html", html)
	if strings.Contains(masked, encoded) {
		t.Fatal("opaque source leaked")
	}
	masked, _ = policy.RedactBody("application/x-www-form-urlencoded", form)
	if strings.Contains(masked, encoded[:4]) {
		t.Fatal("chunk source leaked")
	}
}
func TestDecodedSecretsAreMaskedAndOriginalTreeUntouched(t *testing.T) {
	raw := append([]byte{3, 0, 8}, []byte("password")...)
	raw = append(raw, 2, 0, 7)
	raw = append(raw, []byte("private")...)
	raw = append(raw, 0, 0, 9)
	source := DecodeLegacy(raw, "application/amf0")
	if source == nil || source.State != "available" {
		t.Fatalf("%+v", source)
	}
	out := CleanLegacy(source, redact.NewPolicy(redact.Options{}))
	encoded, _ := json.Marshal(out)
	if strings.Contains(string(encoded), "private") || !out.Redacted {
		t.Fatalf("%s", encoded)
	}
	original, _ := json.Marshal(source)
	if !strings.Contains(string(original), "private") {
		t.Fatal("source tree mutated")
	}
}
