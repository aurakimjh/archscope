package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestMITMForwardsH1WithVerifiedUpstream(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Origin", "verified")
		_, _ = io.WriteString(writer, "ok")
	}))
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	authority, err := NewAuthority(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Close()
	progressed := make(chan models.CaptureTransaction, 1)
	server, err := New(Config{
		ListenAddress: "127.0.0.1:0", Authority: authority, UpstreamRoots: roots,
		Progress: func(transaction models.CaptureTransaction) {
			progressed <- transaction
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	captured := make(chan models.CaptureTransaction, 1)
	address, err := server.Start(ctx, capture.Config{}, func(tx models.CaptureTransaction) error {
		captured <- tx
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stopServer(t, server)

	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyURL(proxyURL(address)),
		TLSClientConfig: &tls.Config{
			RootCAs: authority.Pool(), MinVersion: tls.VersionTLS12,
		},
		ForceAttemptHTTP2: false,
	}}
	response, err := client.Get(origin.URL + "/ready?token=secret")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("status=%d body=%q", response.StatusCode, body)
	}
	var progress models.CaptureTransaction
	select {
	case progress = <-progressed:
		if progress.State != models.TxRequestSent || progress.Path != "/ready" ||
			progress.Fidelity != FidelityPending {
			t.Fatalf("progress=%+v", progress)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for progress transaction")
	}
	select {
	case tx := <-captured:
		if tx.CaptureMode != "proxy_mitm" || tx.Path != "/ready" || tx.StatusCode != http.StatusOK {
			t.Fatalf("transaction=%+v", tx)
		}
		if tx.ID != progress.ID {
			t.Fatalf("progress id=%q completion id=%q", progress.ID, tx.ID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for captured transaction")
	}
}

func TestMITMNeverDisablesUpstreamVerification(t *testing.T) {
	origin := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()
	authority, err := NewAuthority(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Close()
	server, err := New(Config{ListenAddress: "127.0.0.1:0", Authority: authority})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	captured := make(chan models.CaptureTransaction, 1)
	address, err := server.Start(ctx, capture.Config{}, func(tx models.CaptureTransaction) error {
		captured <- tx
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stopServer(t, server)
	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyURL(proxyURL(address)),
		TLSClientConfig: &tls.Config{
			RootCAs: authority.Pool(), MinVersion: tls.VersionTLS12,
		},
		ForceAttemptHTTP2: false,
	}}
	response, err := client.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("status=%d, want 502", response.StatusCode)
	}
	select {
	case tx := <-captured:
		if tx.State != models.TxFailed || !strings.Contains(tx.Error, "certificate") {
			t.Fatalf("transaction=%+v", tx)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for upstream verification failure")
	}
}

func TestH2OnlyALPNIsExplicitPassthrough(t *testing.T) {
	origin := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	origin.EnableHTTP2 = true
	origin.StartTLS()
	defer origin.Close()
	hostPort := strings.TrimPrefix(origin.URL, "https://")
	authority, err := NewAuthority(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Close()
	progressed := make(chan models.CaptureTransaction, 1)
	server, err := New(Config{
		ListenAddress: "127.0.0.1:0", Authority: authority,
		Progress: func(tx models.CaptureTransaction) { progressed <- tx },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	captured := make(chan models.CaptureTransaction, 1)
	address, err := server.Start(ctx, capture.Config{}, func(tx models.CaptureTransaction) error {
		captured <- tx
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", hostPort, hostPort); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status=%d", response.StatusCode)
	}
	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true, // fixture origin is self-signed; proxy only tunnels this connection.
		NextProtos:         []string{"h2"}, MinVersion: tls.VersionTLS12,
	})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatal(err)
	}
	if tlsConn.ConnectionState().NegotiatedProtocol != "h2" {
		t.Fatalf("ALPN=%q", tlsConn.ConnectionState().NegotiatedProtocol)
	}
	select {
	case progress := <-progressed:
		if progress.CaptureMode != CaptureModePassthrough ||
			progress.Fidelity != FidelityUnsupported ||
			progress.State != models.TxRequestSent ||
			progress.Path != "" {
			t.Fatalf("passthrough progress=%+v", progress)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for h2 passthrough progress")
	}
	stopServer(t, server)
	select {
	case tx := <-captured:
		if tx.CaptureMode != CaptureModePassthrough ||
			tx.Fidelity != FidelityUnsupported ||
			tx.HTTPVersion != "h2" ||
			tx.Path != "" {
			t.Fatalf("transaction=%+v", tx)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for h2 passthrough record")
	}
}

func TestAuthorityExportsCertificateOnlyAndBoundsExpiry(t *testing.T) {
	now := time.Now().UTC()
	authority, err := NewAuthority(now)
	if err != nil {
		t.Fatal(err)
	}
	der := authority.CertificateDER()
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.NotAfter.After(now.AddDate(1, 0, 1)) {
		t.Fatalf("CA expiry is too long: %s", certificate.NotAfter)
	}
	leaf, err := authority.LeafFor("example.test", now)
	if err != nil {
		t.Fatal(err)
	}
	parsedLeaf, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if parsedLeaf.NotAfter.After(certificate.NotAfter) {
		t.Fatalf("leaf expiry %s exceeds CA %s", parsedLeaf.NotAfter, certificate.NotAfter)
	}
	authority.Close()
	if len(authority.CertificateDER()) != 0 {
		t.Fatal("certificate material remained available after close")
	}
}

func TestProxyRejectsNonLoopbackListener(t *testing.T) {
	authority, err := NewAuthority(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Close()
	if _, err := New(Config{ListenAddress: "0.0.0.0:0", Authority: authority}); err == nil {
		t.Fatal("non-loopback listener was accepted")
	}
}

func stopServer(t *testing.T, server *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := server.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}
