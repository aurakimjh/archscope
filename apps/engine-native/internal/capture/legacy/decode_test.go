package legacy

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"strings"
	"testing"
)

func decodeHex(t *testing.T, format, value string) models.LegacyPreview {
	t.Helper()
	raw, e := hex.DecodeString(strings.ReplaceAll(value, " ", ""))
	if e != nil {
		t.Fatal(e)
	}
	return Decode(format, raw)
}
func TestAMF0ObjectsReferencesAndAMF3Switch(t *testing.T) {
	got := decodeHex(t, "amf0", "03 0001 78 02 0002 6869 000009")
	if got.State != "available" || got.Root.Children[0].Children[0].Name != "x" || got.Root.Children[0].Children[0].Value != "hi" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "amf0", "03 0004 73656c66 07 0000 000009")
	if got.State != "available" || got.Root.Children[0].Children[0].Kind != "reference" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "amf0", "11 04 7f")
	if got.State != "available" || got.Root.Children[0].Value != "127" {
		t.Fatalf("%+v", got)
	}
}
func TestAMF3StringsTraitsVectorsAndDictionaries(t *testing.T) {
	got := decodeHex(t, "amf3", "09 05 01 06 05 6869 06 00")
	if got.State != "available" || got.Root.Children[0].Children[1].Value != "hi" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "amf3", "0a 13 01 03 78 04 02 0a 01 04 03")
	if got.State != "available" || got.Root.Children[1].Children[0].Name != "x" || got.Root.Children[1].Children[0].Value != "3" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "amf3", "0d 05 01 00000001 ffffffff")
	if got.State != "available" || got.Root.Children[0].Children[1].Value != "-1" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "amf3", "11 03 00 06 03 6b 04 01")
	if got.State != "available" || got.Root.Children[0].Children[0].Children[1].Name != "k" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "amf3", "04 ffffffff")
	if got.State != "available" || got.Root.Children[0].Value != "-1" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "amf3", "0a 07 01")
	if got.State != "unsupported" {
		t.Fatal(got)
	}
}
func TestPacketLengthsAndViewStateTail(t *testing.T) {
	got := decodeHex(t, "amf_packet", "0000 0000 0001 0001 61 0000 00000004 02 0001 78")
	if got.State != "available" || got.Root.Children[0].Value != "x" {
		t.Fatalf("%+v", got)
	}
	bad := decodeHex(t, "amf_packet", "0000 0000 0001 0001 61 0000 00000003 02 0001 78")
	if bad.State != "invalid" {
		t.Fatal(bad)
	}
	got = decodeHex(t, "viewstate", "ff01 0f 05 02 6869 67 aabb")
	if got.State != "available" || got.TrailingBytes != 2 || got.Root.Children[0].Value != "hi" || got.Root.Children[1].Value != "true" {
		t.Fatalf("%+v", got)
	}
	got = decodeHex(t, "viewstate", "ff01 32")
	if got.State != "unsupported" {
		t.Fatal(got)
	}
}
func TestResourceInvalidAndReferenceBounds(t *testing.T) {
	for _, input := range []struct{ format, hex, state string }{{"amf0", "02000861", "invalid"}, {"amf3", "0600", "invalid"}, {"viewstate", "ff011fff", "invalid"}, {"viewstate", "ff01142b7f00", "invalid"}, {"viewstate", "ff01142bffffffff0f00", "invalid"}, {"amf3", "0cffefffff", "limited"}} {
		got := decodeHex(t, input.format, input.hex)
		if got.State != input.state {
			t.Fatalf("%+v -> %+v", input, got)
		}
	}
	got := Decode("amf0", bytes.Repeat([]byte{5}, MaxNodes+1))
	if got.State != "limited" {
		t.Fatal(got)
	}
	got = Decode("amf0", make([]byte, MaxBytes+1))
	if got.State != "limited" {
		t.Fatal(got)
	}
	if DecodeBase64("viewstate", "not base64!").State != "invalid" {
		t.Fatal("invalid base64 accepted")
	}
	raw, _ := hex.DecodeString("ff0167")
	if DecodeBase64("viewstate", base64.StdEncoding.EncodeToString(raw)).Root.Value != "true" {
		t.Fatal("ViewState base64")
	}
}
func FuzzNeverExecuteOrPanic(f *testing.F) {
	f.Add([]byte{0xff, 1, 100})
	f.Add([]byte{3, 0, 0, 9})
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxBytes {
			return
		}
		for _, format := range []string{"amf0", "amf3", "viewstate", "amf_packet"} {
			got := Decode(format, input)
			if got.Root != nil && ValidateTree(got.Root) != nil {
				t.Fatal("output budget escaped")
			}
		}
	})
}

func TestStringReferenceExpansionBudget(t *testing.T) {
	// One 65535-byte literal followed by aliases must not become a huge JSON tree.
	data := []byte{6, 0x87, 0xff, 0x7f}
	data = append(data, bytes.Repeat([]byte{'a'}, 65535)...)
	for i := 0; i < 40; i++ {
		data = append(data, 6, 0)
	}
	if got := Decode("amf3", data); got.State != "limited" || got.Root != nil {
		t.Fatalf("state=%s", got.State)
	}
}
