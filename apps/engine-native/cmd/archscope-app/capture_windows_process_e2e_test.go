//go:build windows

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/procmap"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type windowsCaptureClientRequest struct {
	Proxy string
	URL   string
}

// Each child is a real Windows process with its own PID/start time and TCP
// connection. No resolver mocks or system proxy/CA changes are involved.
func TestWindowsCaptureProcessSelectionE2E(t *testing.T) {
	first, firstKey := startWindowsCaptureClient(t)
	second, secondKey := startWindowsCaptureClient(t)
	if firstKey == secondKey || firstKey.StartTime == "" || secondKey.StartTime == "" {
		t.Fatalf("distinct process instances required: %+v, %+v", firstKey, secondKey)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-ArchScope-Fixture", "windows-two-client")
		_, _ = io.WriteString(w, r.URL.Path)
	}))
	defer origin.Close()

	for _, both := range []bool{false, true} {
		t.Run(fmt.Sprintf("both_selected_%t", both), func(t *testing.T) {
			service := newCaptureService(t.TempDir(), certstore.New(memoryTrustBackend{}, nil))
			keys := []models.ProcessKey{firstKey}
			if both {
				keys = append(keys, secondKey)
			}
			started, err := service.StartCapture(capture.Config{
				ListenAddress: "127.0.0.1:0", AllowedProcesses: keys,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _, _ = service.StopCapture(string(started.ID)) })
			first(t, windowsCaptureClientRequest{"http://" + started.ListenAddress, origin.URL + "/client-a"})
			second(t, windowsCaptureClientRequest{"http://" + started.ListenAddress, origin.URL + "/client-b"})
			if _, err := service.StopCapture(string(started.ID)); err != nil {
				t.Fatal(err)
			}
			page, err := service.FetchCaptureTransactions(CaptureFetchRequest{SessionID: string(started.ID), Limit: 10})
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if both {
				want = 2
			}
			if len(page.Items) != want {
				t.Fatalf("stored %d rows, want %d", len(page.Items), want)
			}
			seen := map[models.ProcessKey]bool{}
			for _, item := range page.Items {
				tx := item.Transaction
				if tx.Process == nil || tx.Process.Attribution != "confirmed" {
					t.Fatalf("unconfirmed Windows attribution: %+v", tx.Process)
				}
				seen[tx.Process.Key] = true
				wantPath := "/client-a"
				if tx.Process.Key == secondKey {
					wantPath = "/client-b"
				}
				if tx.Path != wantPath || tx.StatusCode != http.StatusOK {
					t.Fatalf("unexpected exchange: %+v", tx)
				}
			}
			if !seen[firstKey] || seen[secondKey] != both {
				t.Fatalf("selected instances=%+v, observed=%+v", keys, seen)
			}
			stats, err := service.GetCaptureStats(string(started.ID))
			if err != nil || stats.Observed != 2 || stats.Persisted != uint64(want) || stats.Dropped != uint64(2-want) {
				t.Fatalf("stats=%+v err=%v", stats, err)
			}
		})
	}
}

func startWindowsCaptureClient(t *testing.T) (func(*testing.T, windowsCaptureClientRequest), models.ProcessKey) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestWindowsCaptureClientHelper$")
	cmd.Env = append(os.Environ(), "ARCHSCOPE_WINDOWS_CAPTURE_CLIENT=1")
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("client exit: %v", err)
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Error("client did not exit after stdin closed")
		}
	})
	reader := bufio.NewReader(output)
	readReply := func(t *testing.T) string {
		t.Helper()
		reply := make(chan string, 1)
		go func() {
			line, err := reader.ReadString('\n')
			if err != nil {
				line = fmt.Sprint(err)
			}
			reply <- line
		}()
		select {
		case line := <-reply:
			return line
		case <-time.After(10 * time.Second):
			t.Fatal("client response timed out")
			return ""
		}
	}
	if line := readReply(t); line != "ready\n" {
		t.Fatalf("client readiness: %q", line)
	}
	processes, err := procmap.List()
	if err != nil {
		t.Fatal(err)
	}
	var key models.ProcessKey
	for _, process := range processes {
		if process.Key.PID == int32(cmd.Process.Pid) {
			key = process.Key
		}
	}
	if key.StartTime == "" {
		t.Fatal("running child absent from native process snapshot")
	}
	return func(t *testing.T, request windowsCaptureClientRequest) {
		t.Helper()
		if err := json.NewEncoder(input).Encode(request); err != nil {
			t.Fatal(err)
		}
		if line := readReply(t); line != "ok\n" {
			t.Fatalf("client request: %q", line)
		}
	}, key
}

func TestWindowsCaptureClientHelper(t *testing.T) {
	if os.Getenv("ARCHSCOPE_WINDOWS_CAPTURE_CLIENT") != "1" {
		t.Skip("subprocess helper")
	}
	fmt.Println("ready")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request windowsCaptureClientRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			t.Fatal(err)
		}
		proxy, err := url.Parse(request.Proxy)
		if err != nil {
			t.Fatal(err)
		}
		transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		response, err := client.Get(request.URL)
		if err != nil {
			t.Fatal(err)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		transport.CloseIdleConnections()
		if readErr != nil || closeErr != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("status=%d read=%v close=%v", response.StatusCode, readErr, closeErr)
		}
		fmt.Println("ok")
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}
