//go:build windows

package main

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTransparentNativeClient(t *testing.T) {
	marker := os.Getenv("ARCHSCOPE_TRANSPARENT_CLIENT_MARKER")
	if marker == "" {
		return
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, Timeout: 10 * time.Second}
	response, err := client.Get(os.Getenv("ARCHSCOPE_TRANSPARENT_FIXTURE_URL"))
	if err != nil {
		os.WriteFile(marker, []byte("FAILED"), 0600)
		os.Exit(2)
	}
	_, err = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if err != nil {
		os.Exit(3)
	}
	os.WriteFile(marker, []byte("DONE"), 0600)
	os.Exit(0)
}
func TestWindowsTransparentDirectClientE2E(t *testing.T) {
	if os.Getenv("ARCHSCOPE_RUN_TRANSPARENT_E2E") != "1" {
		t.Skip("opt-in elevated Windows driver test with a peer HTTP fixture")
	}
	fixture, err := url.Parse(os.Getenv("ARCHSCOPE_TRANSPARENT_FIXTURE_URL"))
	if err != nil || fixture.Scheme != "http" || fixture.Hostname() == "" || fixture.User != nil || (fixture.Port() != "" && fixture.Port() != "80") {
		t.Fatal("provide an unauthenticated peer HTTP fixture on port 80")
	}
	if strings.EqualFold(fixture.Hostname(), "localhost") {
		t.Fatal("loopback traffic is outside the transparent filter")
	}
	marker := filepath.Join(t.TempDir(), "client-marker")
	t.Setenv("ARCHSCOPE_TRANSPARENT_CLIENT_MARKER", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	defer service.shutdownAutomation()
	started, err := service.StartCapture(capture.Config{Backend: "windows_transparent", ProcessScope: "launch", LaunchPath: executable, LaunchArgs: []string{"-test.run=^TestTransparentNativeClient$"}, ListenAddress: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(started.AllowedProcesses) != 1 {
		t.Fatal("launched process identity missing")
	}
	deadline := time.Now().Add(15 * time.Second)
	complete := false
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(marker); err == nil {
			complete = string(data) == "DONE"
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err = service.StopCapture(string(started.ID)); err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatal("direct client fixture did not complete")
	}
	page, err := service.FetchCaptureTransactions(CaptureFetchRequest{SessionID: string(started.ID), Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	confirmed := 0
	for _, row := range page.Items {
		tx := row.Transaction
		if tx.Process != nil && tx.Process.Key == started.AllowedProcesses[0] && tx.Process.Attribution == "confirmed" {
			confirmed++
		}
	}
	if confirmed == 0 {
		t.Fatal("proxy-ignoring child produced no attributed transparent capture")
	}
	fmt.Printf("native transparent direct-client proof: %d attributed exchanges\n", confirmed)
}
