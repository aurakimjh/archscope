package main

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"testing"
)

func TestLegacyBridgeReturnsInertTreeAndErrors(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	decoded := service.InspectLegacyContent("viewstate", "/wFn")
	if decoded == nil || decoded.State != "available" || decoded.Root.Kind != "boolean" || decoded.Root.Value != "true" {
		t.Fatalf("%+v", decoded)
	}
	invalid := service.InspectLegacyContent("amf0", "not base64!")
	if invalid.State != "invalid" {
		t.Fatal(invalid)
	}
	unsupported := service.InspectLegacyContent("viewstate", "/wEy")
	if unsupported.State != "unsupported" {
		t.Fatal(unsupported)
	}
}
