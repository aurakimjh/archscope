package main

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
)

func TestStaleStopCannotRemoveNewSessionAuthority(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	first, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StopCapture(string(first.ID)); err != nil {
		t.Fatal(err)
	}
	second, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	defer service.shutdownAutomation()
	before, err := service.InstallCaptureCA()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.StopCapture(string(first.ID)); err == nil {
		t.Fatal("stale stop accepted")
	}
	after := service.GetCaptureCAStatus()
	if after.State != "trusted" || after.Fingerprint != before.Fingerprint || service.GetCurrentCaptureSession().ID != second.ID {
		t.Fatal("stale stop changed the active session's trust")
	}
}

type blockingRemoval struct {
	entered, release chan struct{}
	once             sync.Once
}

func (*blockingRemoval) Install(string, []byte) error { return nil }
func (b *blockingRemoval) Remove(string, []byte) error {
	b.once.Do(func() { close(b.entered); <-b.release })
	return nil
}
func TestRestartWaitsForPreviousTrustCleanup(t *testing.T) {
	backend := &blockingRemoval{entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	unblock := func() { release.Do(func() { close(backend.release) }) }
	defer unblock()
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(backend, nil), "windows")
	defer func() { unblock(); service.shutdownAutomation() }()
	first, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.InstallCaptureCA(); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { _, err := service.StopCapture(string(first.ID)); stopped <- err }()
	select {
	case <-backend.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not start")
	}
	started := make(chan error, 1)
	attempted := make(chan struct{})
	go func() {
		close(attempted)
		_, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0"})
		started <- err
	}()
	<-attempted
	select {
	case err := <-started:
		unblock()
		t.Fatalf("restart crossed unfinished trust cleanup: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	unblock()
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	if err := <-started; err != nil {
		t.Fatal(err)
	}
}
func TestRepeatedDesktopReentryUsesOneCurrentSession(t *testing.T) {
	root := t.TempDir()
	service := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "windows")
	defer service.shutdownAutomation()
	for cycle := 0; cycle < 12; cycle++ {
		current, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0"})
		if err != nil {
			t.Fatal(err)
		}
		var wait sync.WaitGroup
		for i := 0; i < 8; i++ {
			wait.Add(1)
			go func() {
				defer wait.Done()
				if got := service.GetCurrentCaptureSession(); got.ID != current.ID {
					t.Error("reentry changed current session")
				}
				if _, err := service.RecoverCaptureSessions(); err != nil {
					t.Error(err)
				}
				if _, err := service.GetCaptureStats(string(current.ID)); err != nil {
					t.Error(err)
				}
			}()
		}
		wait.Wait()
		if _, err = service.StopCapture(string(current.ID)); err != nil {
			t.Fatal(err)
		}
		if _, err = service.ClearCapture(string(current.ID)); err != nil {
			t.Fatal(err)
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "cap-*", "manifest.json")); len(matches) != 12 {
		t.Fatal("session lifecycle artifacts missing")
	}
	// A fresh process-side manager can inspect the finalized stores without
	// inventing an active desktop session.
	reopened := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "windows")
	if _, err := reopened.RecoverCaptureSessions(); err != nil {
		t.Fatal(err)
	}
	if reopened.GetCurrentCaptureSession().ID != "" {
		t.Fatal("recovery invented a current session")
	}
}

// Keep a canceled control execution from changing the desktop after disconnect.
func TestCanceledDesktopControlDoesNotStartSession(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.executeAutomation(ctx, automation.Call{Command: "start", Params: []byte(`{"processScope":"all_sessions"}`)})
	if err == nil || service.GetCurrentCaptureSession().ID != "" {
		t.Fatal("canceled control allocated capture")
	}
}

func TestShutdownRemovesPreparedAuthorityAndRejectsLateInstall(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	if _, err := service.InstallCaptureCA(); err != nil {
		t.Fatal(err)
	}
	service.shutdownAutomation()
	if service.GetCaptureCAStatus().State != "absent" {
		t.Fatal("prepared trust survived shutdown")
	}
	if _, err := service.InstallCaptureCA(); err == nil {
		t.Fatal("authority installed after shutdown")
	}
}
func TestRemoveAuthorityWhileRunningRequiresStop(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	defer service.shutdownAutomation()
	if _, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallCaptureCA(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RemoveCaptureCA(); err == nil {
		t.Fatal("running authority was removed")
	}
	if service.GetCaptureCAStatus().State != "trusted" {
		t.Fatal("running trust changed")
	}
}
