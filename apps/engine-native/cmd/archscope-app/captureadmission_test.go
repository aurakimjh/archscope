package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestCaptureAdmissionCancellationDuringLifecycle(t *testing.T) {
	for _, action := range []string{"stop", "clear", "pause"} {
		t.Run(action, func(t *testing.T) {
			s := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
			defer s.shutdownAutomation()
			entered := make(chan struct{}, 1)
			key, release, e := s.manager.Admissions().Register(func(ctx context.Context, _ string, _ models.CaptureTransaction) (bool, error) {
				entered <- struct{}{}
				<-ctx.Done()
				return false, ctx.Err()
			})
			if e != nil {
				t.Fatal(e)
			}
			defer release()
			started, e := s.StartCapture(capture.Config{AdmissionOwner: key, ProcessScope: "all_sessions", ListenAddress: "127.0.0.1:0", RetainUnattributedMetadata: true})
			if e != nil {
				t.Fatal(e)
			}
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "synthetic") }))
			defer origin.Close()
			proxy, _ := url.Parse("http://" + started.ListenAddress)
			transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
			requested := make(chan struct{})
			go func() {
				defer close(requested)
				r, e := client.Get(origin.URL)
				if e == nil {
					io.Copy(io.Discard, r.Body)
					r.Body.Close()
				}
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("admission not called")
			}
			done := make(chan error, 1)
			go func() {
				var e error
				switch action {
				case "clear":
					_, e = s.manager.Clear(started.ID)
				case "pause":
					_, e = s.manager.SetPaused(started.ID, true)
				default:
					_, e = s.StopCapture(string(started.ID))
				}
				done <- e
			}()
			select {
			case e = <-done:
				if e != nil {
					t.Fatal(e)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("reentrant lifecycle deadlock")
			}
			<-requested
			if action != "stop" {
				if _, e = s.StopCapture(string(started.ID)); e != nil {
					t.Fatal(e)
				}
			}
			page, e := s.manager.Fetch(started.ID, store.Filter{}, "", 10)
			if e != nil || len(page.Items) != 0 {
				t.Fatal("denied exchange persisted", page, e)
			}
		})
	}
}
