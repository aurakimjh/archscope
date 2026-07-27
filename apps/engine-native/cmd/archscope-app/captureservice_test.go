package main

import (
	"context"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
)

type memoryTrustBackend struct{}

func (memoryTrustBackend) Install(string, []byte) error { return nil }
func (memoryTrustBackend) Remove(string, []byte) error  { return nil }

func TestCaptureServiceSessionCanBeAnalyzedAfterStop(t *testing.T) {
	service := newCaptureService(t.TempDir(), certstore.New(memoryTrustBackend{}, nil))
	started, err := service.StartCapture(capture.Config{
		ListenAddress: "127.0.0.1:0", ReserveBytes: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = ctx
	stopped, err := service.StopCapture(string(started.ID))
	if err != nil {
		t.Fatal(err)
	}
	if stopped.State != capture.StateFinalized {
		t.Fatalf("stopped=%+v", stopped)
	}
	result, err := service.AnalyzeCaptureSession(CaptureAnalyzeRequest{SessionID: string(started.ID)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != "http_capture" || result.Summary["total_transactions"] != 0 {
		t.Fatalf("result type=%q summary=%+v", result.Type, result.Summary)
	}
}
