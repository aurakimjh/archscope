// Command t574-mitm-proxy-poc is the build-vs-integrate spike artifact for
// T-574 (docs/ko/SYSTEM_HTTP_CAPTURE.md build-vs-integrate 결정). It is a
// minimal, dependency-free HTTP/1.1 MITM proxy with HTTP/2 passthrough — the
// recommended "option 3" — so the decision rests on a working prototype rather
// than only a paper comparison.
//
//	go run .                 # run the proxy on :8080; set it as your HTTPS proxy,
//	                         # trust the printed CA, and watch captures on stdout
//	go run . -selftest       # offline end-to-end: mint CA, intercept a request
//	                         # to an in-process TLS origin, print the capture
//
// Everything is standard library. There is no third-party proxy dependency and
// nothing to install — the whole point of the recommendation.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "proxy listen address")
	selftest := flag.Bool("selftest", false, "run an offline end-to-end interception test and exit")
	flag.Parse()

	ca, err := newMitmCA(time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "CA:", err)
		os.Exit(1)
	}

	if *selftest {
		if err := runSelftest(ca); err != nil {
			fmt.Fprintln(os.Stderr, "selftest FAILED:", err)
			os.Exit(1)
		}
		fmt.Println("selftest PASSED")
		return
	}

	p := &proxy{
		ca:        ca,
		onCapture: func(c Capture) { b, _ := json.Marshal(c); fmt.Println(string(b)) },
	}
	fmt.Printf("proxy on %s — trust the CA and set it as your HTTPS proxy. Ctrl+C to stop.\n", *listen)
	srv := &http.Server{Addr: *listen, Handler: p}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "serve:", err)
		os.Exit(1)
	}
}

// runSelftest proves the intercept path offline: an in-process TLS origin, a
// client that trusts only the MITM CA, and the proxy in between verifying the
// origin against the origin's own CA. If the capture shows the decoded request
// AND the client got the origin's body, MITM + upstream verification both work.
func runSelftest(ca *mitmCA) error {
	// In-process HTTPS origin.
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Origin", "t574")
		fmt.Fprintf(w, "hello from origin: %s %s", r.Method, r.URL.Path)
	}))
	defer origin.Close()

	// Trust the origin's cert for upstream verification (verification stays ON).
	originPool := x509.NewCertPool()
	originPool.AddCert(origin.Certificate())

	var mu sync.Mutex
	var captures []Capture
	p := &proxy{
		ca:            ca,
		upstreamRoots: originPool,
		onCapture:     func(c Capture) { mu.Lock(); captures = append(captures, c); mu.Unlock() },
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: p}
	go srv.Serve(ln)
	defer srv.Close()
	proxyURL, _ := url.Parse("http://" + ln.Addr().String())

	// Client trusts ONLY the MITM CA — so a successful request proves the proxy
	// presented a validly-minted leaf, i.e. real interception.
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{RootCAs: ca.pool()},
		},
	}
	resp, err := client.Get(origin.URL + "/hello")
	if err != nil {
		return fmt.Errorf("client request: %w", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	fmt.Printf("client saw: status=%d x-origin=%q body=%q\n", resp.StatusCode, resp.Header.Get("X-Origin"), string(body))
	mu.Lock()
	defer mu.Unlock()
	for _, c := range captures {
		b, _ := json.Marshal(c)
		fmt.Println("capture:", string(b))
	}

	// Assertions.
	if resp.StatusCode != 200 {
		return fmt.Errorf("want status 200, got %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Origin") != "t574" {
		return fmt.Errorf("origin header not propagated — interception broke the response")
	}
	var got *Capture
	for i := range captures {
		if captures[i].Intercepted && captures[i].URL != "" {
			got = &captures[i]
			break
		}
	}
	if got == nil {
		return fmt.Errorf("no intercepted transaction captured")
	}
	if got.Status != 200 || got.Fidelity != "decoded_wire" {
		return fmt.Errorf("capture wrong: status=%d fidelity=%s", got.Status, got.Fidelity)
	}
	return nil
}
