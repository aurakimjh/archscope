package ingestion

import (
	"bytes"
	"strings"
)

// HTTPCaptureSourceFormat describes the bounded HAR import path. The detector
// only inspects Probe.Head; structural validation remains parser work.
func HTTPCaptureSourceFormat() SourceFormat {
	return SourceFormat{
		ID:            "har",
		SourceKind:    SourceKindHTTPCapture,
		Product:       "HAR",
		ResultType:    "http_capture",
		Parser:        "internal/parsers/httpcapture",
		Extensions:    []string{".har", ".json", ".gz"},
		Detector:      detectHAR,
		DetectorOrder: 20,
	}
}

func detectHAR(probe Probe) Detection {
	head := bytes.TrimPrefix(bytes.TrimSpace(probe.Head), []byte{0xef, 0xbb, 0xbf})
	if len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b {
		if strings.HasSuffix(strings.ToLower(probe.Path), ".har.gz") {
			return Detection{Confidence: 0.65, Reason: "gzip-compressed .har input"}
		}
		return Detection{}
	}
	if len(head) == 0 || head[0] != '{' {
		return Detection{}
	}
	if bytes.Contains(head, []byte(`"events"`)) && !bytes.Contains(head, []byte(`"log"`)) {
		return Detection{}
	}
	if bytes.Contains(head, []byte(`"log"`)) && bytes.Contains(head, []byte(`"entries"`)) {
		return Detection{Confidence: 0.98, Reason: "HAR log.entries signature"}
	}
	if strings.EqualFold(probe.Extension, ".har") {
		return Detection{Confidence: 0.30, Reason: ".har extension"}
	}
	return Detection{}
}
