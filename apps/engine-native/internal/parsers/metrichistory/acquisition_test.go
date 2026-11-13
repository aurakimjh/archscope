package metrichistory

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnedFileMatchesByteImportIncludingCompressedDigest(t *testing.T) {
	raw, e := os.ReadFile("testdata/snapshots.json")
	if e != nil {
		t.Fatal(e)
	}
	var zip bytes.Buffer
	z := gzip.NewWriter(&zip)
	z.Write(raw)
	z.Close()
	for _, body := range [][]byte{raw, zip.Bytes()} {
		p := filepath.Join(t.TempDir(), "snapshots.json")
		if e = os.WriteFile(p, body, 0600); e != nil {
			t.Fatal(e)
		}
		want, e := ParseBytes(context.Background(), body, filepath.Base(p), Options{})
		if e != nil {
			t.Fatal(e)
		}
		got, e := ParseFile(context.Background(), p, Options{})
		if e != nil {
			t.Fatal(e)
		}
		a, _ := json.Marshal(want)
		b, _ := json.Marshal(got)
		if !bytes.Equal(a, b) {
			t.Fatal("source bytes/hash/normalization changed")
		}
	}
}
