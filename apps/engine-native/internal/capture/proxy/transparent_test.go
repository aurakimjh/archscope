package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTransparentStreamsUseOriginalRouteAndAttribution(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "HTTP", true: "TLS"}[secure], func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Fixture", "transparent")
				io.WriteString(w, "fixture")
			})
			var origin *httptest.Server
			if secure {
				origin = httptest.NewTLSServer(handler)
			} else {
				origin = httptest.NewServer(handler)
			}
			defer origin.Close()
			roots := x509.NewCertPool()
			if secure {
				roots.AddCert(origin.Certificate())
			}
			authority, err := NewAuthority(time.Now())
			if err != nil {
				t.Fatal(err)
			}
			defer authority.Close()
			server, err := New(Config{Authority: authority, UpstreamRoots: roots})
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var transactions []models.CaptureTransaction
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_, err = server.Start(ctx, capture.Config{}, func(tx models.CaptureTransaction) error {
				mu.Lock()
				transactions = append(transactions, tx)
				mu.Unlock()
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			defer server.Stop(context.Background())
			relay, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer relay.Close()
			process := &models.ProcessInstance{Key: models.ProcessKey{PID: 42, StartTime: "fixture"}, Name: "direct-client", Attribution: "confirmed"}
			served := make(chan struct{})
			go func() {
				defer close(served)
				conn, err := relay.Accept()
				if err == nil {
					server.ServeTransparent(ctx, conn, origin.Listener.Addr().String(), process)
				}
			}()
			clientRoots := x509.NewCertPool()
			cert, err := x509.ParseCertificate(authority.CertificateDER())
			if err != nil {
				t.Fatal(err)
			}
			clientRoots.AddCert(cert)
			transport := &http.Transport{Proxy: nil, ForceAttemptHTTP2: false, TLSClientConfig: &tls.Config{RootCAs: clientRoots, MinVersion: tls.VersionTLS12}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "tcp", relay.Addr().String())
			}}
			defer transport.CloseIdleConnections()
			scheme, host := "http", "unresolvable.fixture.invalid"
			if secure {
				scheme = "https"
				host = origin.Certificate().DNSNames[0]
			}
			client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
			response, err := client.Post(scheme+"://"+host+"/direct", "text/plain", strings.NewReader("fixture request"))
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			transport.CloseIdleConnections()
			select {
			case <-served:
			case <-time.After(5 * time.Second):
				t.Fatal("transparent stream did not close")
			}
			mu.Lock()
			defer mu.Unlock()
			if len(transactions) != 1 || transactions[0].Process.Key != process.Key || transactions[0].StatusCode != 200 || !strings.Contains(transactions[0].URL, "/direct") {
				t.Fatalf("unexpected capture: %+v", transactions)
			}
		})
	}
}
