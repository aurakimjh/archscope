package main

import (
	"crypto/tls"
	"net"
	"testing"
	"time"
)

// TestPeekClientHelloALPN drives a real TLS client through net.Pipe and checks
// that peekClientHello recovers its ALPN + SNI and that the h2-only passthrough
// decision fires. This covers the passthrough branch the offline selftest can't
// (Go's http client always also offers http/1.1).
func TestPeekClientHelloALPN(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	go func() {
		cfg := &tls.Config{
			NextProtos:         []string{"h2"},
			ServerName:         "example.com",
			InsecureSkipVerify: true,
		}
		// Handshake will not complete (no server) — we only need the ClientHello.
		_ = tls.Client(c1, cfg).Handshake()
	}()

	_ = c2.SetReadDeadline(time.Now().Add(5 * time.Second))
	alpn, sni, _, err := peekClientHello(c2)
	if err != nil {
		t.Fatalf("peekClientHello: %v", err)
	}
	if sni != "example.com" {
		t.Errorf("sni = %q, want example.com", sni)
	}
	found := false
	for _, p := range alpn {
		if p == "h2" {
			found = true
		}
	}
	if !found {
		t.Errorf("alpn = %v, want it to contain h2", alpn)
	}
	if !hasH2Only(alpn) {
		t.Errorf("hasH2Only(%v) = false, want true (should route to passthrough)", alpn)
	}
}

func TestHasH2Only(t *testing.T) {
	cases := []struct {
		in   []string
		want bool
	}{
		{[]string{"h2"}, true},
		{[]string{"h2", "http/1.1"}, false}, // browsers/Go send both → intercept
		{[]string{"http/1.1"}, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := hasH2Only(c.in); got != c.want {
			t.Errorf("hasH2Only(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
