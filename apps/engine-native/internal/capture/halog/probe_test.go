package halog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestContainerSignaturesDoNotClaimCompatibility(t *testing.T) {
	for _, fixture := range []struct{ data, container string }{{"\x1f\x8babc", "gzip"}, {"PK\x03\x04abc", "zip"}, {"<root private='fixture'/>", "xml_text"}, {`{"type":"archscope-http-session"}`, "json_text"}, {"unknown-native-data", "opaque"}} {
		path := filepath.Join(t.TempDir(), "fixture.halog")
		os.WriteFile(path, []byte(fixture.data), 0600)
		got, err := Probe(path)
		if err != nil || got.Container != fixture.container || got.Compatibility != "unverified" || len(got.SHA256) != 64 {
			t.Fatalf("%+v %v", got, err)
		}
	}
}
func TestPairCannotBecomePassWithoutInterop(t *testing.T) {
	dir := t.TempDir()
	native := filepath.Join(dir, "source.halog")
	xml := filepath.Join(dir, "source.xml")
	os.WriteFile(native, []byte("unverified"), 0600)
	os.WriteFile(xml, []byte("<fixture/>"), 0600)
	pair, err := ProbePair(native, xml)
	if err != nil || pair.Status != "UNVERIFIED" || len(pair.Missing) != 4 {
		t.Fatalf("%+v %v", pair, err)
	}
	if strings.Contains(pair.Native.SHA256, dir) {
		t.Fatal("path escaped into fingerprint")
	}
	if _, err = Probe(dir); err == nil {
		t.Fatal("directory accepted")
	}
}
