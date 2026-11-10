package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
)

func TestDesktopControlCapturesSavesAndReloadsFixture(t *testing.T) {
	root := t.TempDir()
	service := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "windows")
	path := filepath.Join(root, "control.json")
	controller, err := automation.Start(path, service.executeAutomation)
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Close()
	defer service.shutdownAutomation()
	counter := 0
	call := func(command string, params any) automation.Reply {
		t.Helper()
		counter++
		raw, _ := json.Marshal(params)
		reply, e := automation.Invoke(context.Background(), path, automation.Call{ID: fmt.Sprintf("fixture-%d", counter), Command: command, Params: raw})
		if e != nil || !reply.OK {
			t.Fatalf("command=%s reply=%+v error=%v", command, reply, e)
		}
		return reply
	}
	reply := call("start", capture.Config{ProcessScope: "all_sessions", ListenAddress: "127.0.0.1:0", RetainUnattributedMetadata: true})
	var started capture.Session
	if json.Unmarshal(reply.Result, &started) != nil {
		t.Fatal("invalid started session")
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fixture", "automation")
		io.WriteString(w, "fixture")
	}))
	defer origin.Close()
	proxyURL, _ := url.Parse("http://" + started.ListenAddress)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	response, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	call("pause", map[string]any{})
	call("resume", map[string]any{})
	call("stop", map[string]any{})
	list := call("list", map[string]any{})
	var page struct {
		Total int `json:"total"`
	}
	json.Unmarshal(list.Result, &page)
	if page.Total != 1 {
		t.Fatalf("list=%s", list.Result)
	}
	destination := filepath.Join(root, "fixture.httpsession")
	call("save", CaptureExportRequest{Source: CaptureFileSource{SessionID: string(started.ID)}, Destination: destination, Format: "session", Scope: "all"})
	call("open", map[string]string{"path": destination})
	events := call("events", map[string]any{"after": 0})
	var journal automation.Events
	json.Unmarshal(events.Result, &journal)
	if len(journal.Items) < 2 {
		t.Fatalf("events=%s", events.Result)
	}
	if service.GetDesktopLaunchState().OpenPath != destination {
		t.Fatal("open did not reach desktop launch state")
	}
}
func TestLaunchOnMacFailsWithoutSideEffectsAndRunsOnlyOnce(t *testing.T) {
	root := t.TempDir()
	service := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "darwin")
	service.launchOptions = automation.Options{Capture: true, Scope: "all_sessions"}
	first := service.ApplyDesktopLaunch()
	second := service.ApplyDesktopLaunch()
	if first.Error == "" || first.Error != second.Error || service.GetCurrentCaptureSession().ID != "" {
		t.Fatalf("%+v %+v", first, second)
	}
}

func TestShutdownRejectsLaterCaptureCommands(t *testing.T) {
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	service.shutdownAutomation()
	if _, err := service.StartCapture(capture.Config{ProcessScope: "all_sessions"}); err == nil {
		t.Fatal("capture started after shutdown")
	}
	if service.GetCurrentCaptureSession().ID != "" {
		t.Fatal("shutdown rejection allocated a session")
	}
}
