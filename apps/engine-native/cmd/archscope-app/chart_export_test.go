package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveChartArtifactFormatsAndErrors(t *testing.T) {
	s := &EngineService{}
	dir := t.TempDir()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 3))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ format, data string }{
		{"png", "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())},
		{"svg", "data:image/svg+xml;charset=UTF-8,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%3E%3Ctext%3EA+B%3C/text%3E%3C/svg%3E"},
		{"csv", "time,value\n1,2\n"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			path := filepath.Join(dir, "chart."+tc.format)
			if err := s.SaveChartArtifact(ChartArtifactRequest{path, tc.format, tc.data}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil || len(data) == 0 {
				t.Fatal(err)
			}
			if tc.format == "svg" && !bytes.Contains(data, []byte("A+B")) {
				t.Fatal("plus sign corrupted")
			}
		})
	}
	for _, req := range []ChartArtifactRequest{{"", "csv", "x"}, {filepath.Join(dir, "x.png"), "png", "data:image/png;base64,AAAA"}, {filepath.Join(dir, "x.svg"), "svg", "data:image/svg+xml,%3Chtml/%3E"}, {filepath.Join(dir, "x.png"), "csv", "x"}, {filepath.Join(dir, "x.exe"), "exe", "x"}, {filepath.Join(dir, "missing", "x.csv"), "csv", "x"}} {
		if err := s.SaveChartArtifact(req); err == nil {
			t.Fatalf("invalid output accepted: %s/%s", req.Path, req.Format)
		}
	}
}
