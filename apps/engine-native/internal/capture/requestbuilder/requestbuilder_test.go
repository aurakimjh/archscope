package requestbuilder

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestExplicitRawFormMultipartAndRedirect(t *testing.T) {
	var requests int
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/form":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if strings.Join(r.PostForm["x"], ",") != "one,two" {
				t.Errorf("form=%v", r.PostForm)
			}
		case "/multipart":
			if err := r.ParseMultipartForm(MaxRequestBytes); err != nil {
				t.Error(err)
			}
			defer r.MultipartForm.RemoveAll()
			if r.FormValue("x") != "한글" {
				t.Error("missing text")
			}
			file, h, e := r.FormFile("upload")
			if e != nil {
				t.Error(e)
				return
			}
			defer file.Close()
			data, _ := io.ReadAll(file)
			if string(data) != "file contents" || h.Filename != "renamed.txt" {
				t.Errorf("%s %s", data, h.Filename)
			}
		case "/raw":
			data, _ := io.ReadAll(r.Body)
			if string(data) != `{"value":1}` || r.Header.Get("X-Test") != "edited" {
				t.Errorf("raw=%s headers=%v", data, r.Header)
			}
		case "/redirect":
			w.Header().Set("Location", "/must-not-follow")
			w.WriteHeader(302)
			return
		case "/must-not-follow":
			t.Error("followed redirect without an explicit send")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=private")
		w.WriteHeader(201)
		io.WriteString(w, `{"value":"ok","password":"private"}`)
	}))
	defer origin.Close()
	client := NewClient()
	file := filepath.Join(t.TempDir(), "source.txt")
	os.WriteFile(file, []byte("file contents"), 0600)
	cases := []Request{{ID: "raw", URL: origin.URL + "/raw", Method: "PATCH", BodyMode: "raw", Body: `{"value":1}`, Headers: []models.HeaderField{{Name: "Content-Type", Value: "application/json"}, {Name: "X-Test", Value: "edited"}}}, {ID: "form", URL: origin.URL + "/form", Method: "POST", BodyMode: "form", Parts: []Part{{Name: "x", Value: "one"}, {Name: "x", Value: "two"}}}, {ID: "multipart", URL: origin.URL + "/multipart", Method: "POST", BodyMode: "multipart", Parts: []Part{{Name: "x", Value: "한글"}, {Name: "upload", Path: file, Filename: "renamed.txt"}}}, {ID: "redirect", URL: origin.URL + "/redirect", Method: "GET"}}
	for _, input := range cases {
		tx, err := client.Send(input)
		if err != nil {
			t.Fatal(err)
		}
		if input.ID == "redirect" {
			if tx.StatusCode != 302 || tx.RedirectURL != "/must-not-follow" {
				t.Fatalf("%+v", tx)
			}
			continue
		}
		if tx.StatusCode != 201 || tx.State != models.TxComplete || strings.Contains(tx.Response.BodyPreview, "private") || !strings.Contains(tx.Response.BodyPreview, "ok") {
			t.Fatalf("%+v", tx)
		}
		for _, h := range tx.Response.Headers {
			if h.Name == "Set-Cookie" && strings.Contains(h.Value, "private") {
				t.Fatal("response secret leaked")
			}
		}
	}
	if requests != 4 {
		t.Fatalf("sent %d requests", requests)
	}
}
func TestCancelTimeoutAndResponseLimit(t *testing.T) {
	entered := make(chan struct{}, 2)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			entered <- struct{}{}
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		gzipWriter := gzip.NewWriter(w)
		w.Header().Set("Content-Encoding", "gzip")
		gzipWriter.Write([]byte(strings.Repeat("a", MaxResponseBytes+512)))
		gzipWriter.Close()
	}))
	defer origin.Close()
	client := NewClient()
	result := make(chan models.CaptureTransaction, 1)
	go func() {
		tx, err := client.Send(Request{ID: "cancel", URL: origin.URL + "/slow"})
		if err != nil {
			t.Error(err)
		}
		result <- tx
	}()
	<-entered
	if !client.Cancel("cancel") {
		t.Fatal("active request not canceled")
	}
	select {
	case tx := <-result:
		if tx.State != models.TxFailed {
			t.Fatalf("%+v", tx)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not stop send")
	}
	tx, err := client.Send(Request{ID: "limit", URL: origin.URL + "/large"})
	if err != nil || tx.Response.BodyStorage != "truncated" || len(tx.Response.BodyPreview) != MaxResponseBytes {
		t.Fatalf("storage=%s bytes=%d %v", tx.Response.BodyStorage, len(tx.Response.BodyPreview), err)
	}
	if _, err = execute(context.Background(), Request{URL: origin.URL, Headers: []models.HeaderField{{Name: "X-Test", Value: "bad\r\nInjected: yes"}}}); err == nil {
		t.Fatal("header injection accepted")
	}
	if _, err = client.Send(Request{ID: "invalid", URL: "file:///tmp/private"}); err == nil {
		t.Fatal("non-HTTP target accepted")
	}
}

func TestTimeoutAndTLSVerification(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer origin.Close()
	started := time.Now()
	tx, err := NewClient().Send(Request{ID: "timeout", URL: origin.URL, TimeoutSeconds: 1})
	if err != nil || tx.State != models.TxFailed || time.Since(started) > 3*time.Second {
		t.Fatalf("timeout state=%s error=%v", tx.State, err)
	}
	tls := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("untrusted certificate was accepted") }))
	defer tls.Close()
	tx, err = NewClient().Send(Request{ID: "tls", URL: tls.URL})
	if err != nil || tx.State != models.TxFailed {
		t.Fatalf("TLS state=%s error=%v", tx.State, err)
	}
}
