package automation

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLaunchOptionsRejectAmbiguityAndFakeNative(t *testing.T) {
	root := t.TempDir()
	exe := filepath.Join(root, "fixture.exe")
	out, err := ParseOptions([]string{"--capture", "--process", exe, "--duration", "2s", "--destination", filepath.Join(root, "capture.har"), "--format", "har"})
	if err != nil || out.Process != exe || out.Duration.Seconds() != 2 {
		t.Fatalf("%+v %v", out, err)
	}
	legacy, err := ParseOptions([]string{"/p" + exe, "/t=2", "/fFULL"})
	if err != nil || legacy.Format != "xml" || legacy.Duration.Minutes() != 2 {
		t.Fatalf("%+v %v", legacy, err)
	}
	for _, args := range [][]string{{"--capture", "/fNATIVE"}, {"--capture", "--pid", "-1"}, {"--capture", "--process", exe, "--pid", "1"}, {"--capture", "--duration", "-1s"}, {"--capture", "--destination", filepath.Join(root, "fake.halog"), "--duration", "2s"}, {"--capture", "--destination", filepath.Join(root, "none.har")}, {"--open", exe, "--capture"}, {"--capture", "--pid", "1", "--pid", "1"}} {
		if _, err := ParseOptions(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestAuthenticatedControlReplayAndOriginRejection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.json")
	var executed atomic.Int32
	s, err := Start(path, func(ctx context.Context, c Call) (any, error) {
		executed.Add(1)
		return map[string]bool{"started": true}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	call := Call{ID: "unique", Command: "start", Params: json.RawMessage(`{}`)}
	for i := 0; i < 2; i++ {
		reply, err := Invoke(context.Background(), path, call)
		if err != nil || !reply.OK {
			t.Fatalf("%+v %v", reply, err)
		}
	}
	if executed.Load() != 1 {
		t.Fatal("retry repeated mutation")
	}
	call.Params = json.RawMessage(`{"different":true}`)
	reply, err := Invoke(context.Background(), path, call)
	if err != nil || reply.OK {
		t.Fatalf("%+v %v", reply, err)
	}
	req, _ := http.NewRequest("POST", s.descriptor.URL, strings.NewReader(`{"id":"browser","command":"status"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.descriptor.Token)
	req.Header.Set("Origin", "http://attacker.invalid")
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("browser origin accepted")
	}
	req.Header.Del("Origin")
	req.Header.Del("Authorization")
	req.Body, _ = req.GetBody()
	response, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("unauthenticated controller accepted")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("descriptor not removed")
	}
}
func TestDescriptorCannotRedirectCredentials(t *testing.T) {
	for _, url := range []string{"http://127.0.0.1:1@remote.invalid/v1/control", "http://127.0.0.1:1/other", "http://127.0.0.1:1/v1/control?x=y"} {
		path := filepath.Join(t.TempDir(), "bad.json")
		raw, _ := json.Marshal(Descriptor{Version: 1, URL: url, Token: strings.Repeat("a", 64)})
		os.WriteFile(path, raw, 0600)
		if _, err := Invoke(context.Background(), path, Call{}); err == nil {
			t.Fatal("invalid descriptor accepted")
		}
	}
}
func TestJournalIsBoundedAndReportsGaps(t *testing.T) {
	j := Journal{}
	for i := 0; i < 600; i++ {
		j.Append("updated", "fixture", map[string]int{"index": i})
	}
	result := j.Read(0, 100)
	if !result.ResyncRequired || len(result.Items) != 100 || result.Latest != 600 {
		t.Fatalf("%+v", result)
	}
	raw := strings.Repeat("x", 300<<10)
	j.Append("new", "fixture", raw)
	result = j.Read(600, 1)
	if !strings.Contains(string(result.Items[0].Data), "resyncRequired") {
		t.Fatal("oversized event retained")
	}
}

func TestLaunchOptionsAcceptExecutableShortcuts(t *testing.T) {
	link := filepath.Join(t.TempDir(), "fixture.LNK")
	for _, args := range [][]string{{"--process", link}, {"/p", link}} {
		got, err := ParseOptions(args)
		if err != nil || got.Process != link || !got.Capture {
			t.Fatalf("%+v %v", got, err)
		}
	}
}

func TestCOMEmbeddingIsExplicitAndSCMCompatible(t *testing.T) {
	for _, embedding := range []string{"-Embedding", "/Embedding", "/embedding"} {
		got, err := ParseOptions([]string{"--com-server", embedding})
		if err != nil || !got.COMServer {
			t.Fatal(got, err)
		}
		if _, err := ParseOptions([]string{embedding}); err == nil {
			t.Fatal("implicit COM server activation accepted")
		}
	}
}
