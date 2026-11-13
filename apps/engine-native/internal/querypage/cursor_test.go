package querypage

import (
	"encoding/base64"
	"testing"
)

func TestLegacyWireAndBinding(t *testing.T) {
	want := base64.RawURLEncoding.EncodeToString([]byte(`{"Result":"result","Query":"query","Offset":2}`))
	if Encode("result", "query", 2) != want {
		t.Fatal("wire changed")
	}
	for _, tc := range []struct {
		token, result, query string
		bad                  bool
	}{
		{want, "result", "query", false}, {want, "other", "query", true}, {want, "result", "other", true}, {"!", "result", "query", true},
		{Encode("result", "query", -1), "result", "query", true},
	} {
		n, e := Decode(tc.token, tc.result, tc.query)
		if (e != nil) != tc.bad || !tc.bad && n != 2 {
			t.Fatal(n, e)
		}
	}
	if _, e := Bounds(int(^uint(0)>>1), 200, 3); e == nil {
		t.Fatal("accepted huge offset")
	}
	if end, e := Bounds(2, 200, 3); e != nil || end != 3 {
		t.Fatal(end, e)
	}
}
