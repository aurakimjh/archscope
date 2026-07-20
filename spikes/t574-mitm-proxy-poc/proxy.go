// The option-3 proxy itself: an HTTP/1.1 semantic MITM with HTTP/2 passthrough,
// built only on the Go standard library. It demonstrates the capabilities the
// T-574 decision hinges on:
//   - CONNECT handling + TLS termination with on-the-fly certs (ca.go)
//   - ALPN-based route: intercept h1, passthrough h2 (clienthello.go)
//   - upstream forwarding WITH certificate verification (never disabled — §11)
//   - a capture record per transaction, shaped like the §6 data model
//
// It is a spike, not the product: bodies are not streamed to a bounded store,
// there is no session cursor API, and PID attribution is only noted (it reuses
// the T-571-validated GetExtendedTcpTable path on Windows).
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Capture is one observed transaction, a spike-sized echo of the §6.2 model.
type Capture struct {
	Time        string   `json:"time"`
	ClientAddr  string   `json:"client_addr"`
	Method      string   `json:"method"`
	Host        string   `json:"host"`
	URL         string   `json:"url"`
	ALPN        []string `json:"alpn,omitempty"`
	Status      int      `json:"status,omitempty"`
	ReqBytes    int64    `json:"req_bytes"`
	RespBytes   int64    `json:"resp_bytes"`
	Intercepted bool     `json:"intercepted"`
	Fidelity    string   `json:"fidelity"` // decoded_wire | semantic | unsupported
	Note        string   `json:"note,omitempty"`
}

type proxy struct {
	ca            *mitmCA
	upstreamRoots *x509.CertPool // nil => system roots; verification is ALWAYS on
	onCapture     func(Capture)
	now           func() time.Time
}

func (p *proxy) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func (p *proxy) emit(c Capture) {
	c.Time = p.clock().Format(time.RFC3339Nano)
	if p.onCapture != nil {
		p.onCapture(c)
	}
}

// upstreamTransport forwards to the real origin with verification intact.
func (p *proxy) upstreamTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig:   &tls.Config{RootCAs: p.upstreamRoots}, // RootCAs nil => system; NEVER InsecureSkipVerify
		ForceAttemptHTTP2: false,                                 // spike terminates upstream as H1 too
		DisableKeepAlives: true,
	}
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	p.handlePlain(w, r)
}

// handlePlain forwards a plaintext (absolute-URI) HTTP request — the trivial
// non-TLS proxy path.
func (p *proxy) handlePlain(w http.ResponseWriter, r *http.Request) {
	tr := p.upstreamTransport()
	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	rc := &countReader{r: r.Body}
	outReq.Body = rc
	resp, err := tr.RoundTrip(outReq)
	if err != nil {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		p.emit(Capture{ClientAddr: r.RemoteAddr, Method: r.Method, Host: r.Host, URL: r.URL.String(),
			Intercepted: false, Fidelity: "semantic", Note: "plain-http upstream error: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	n, _ := io.Copy(w, resp.Body)
	p.emit(Capture{ClientAddr: r.RemoteAddr, Method: r.Method, Host: r.Host, URL: r.URL.String(),
		Status: resp.StatusCode, ReqBytes: rc.n, RespBytes: n, Intercepted: true, Fidelity: "decoded_wire"})
}

func (p *proxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	if _, err := io.WriteString(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		clientConn.Close()
		return
	}

	hostport := r.Host // host:port
	host := hostport
	if h, _, e := net.SplitHostPort(hostport); e == nil {
		host = h
	}

	alpn, sni, replay, err := peekClientHello(clientConn)
	if err != nil {
		// Not TLS (or unreadable) — tunnel raw so we never corrupt the stream.
		p.tunnel(clientConn, hostport, alpn, "peek failed → raw tunnel: "+err.Error())
		return
	}
	if sni != "" {
		host = sni
	}

	if hasH2Only(alpn) {
		// Design decision: do not half-MITM h2. Pass it through untouched.
		p.tunnel(replay, hostport, alpn, "h2-only ALPN → passthrough (fidelity unsupported)")
		return
	}
	p.intercept(replay, host, hostport, r.RemoteAddr, alpn)
}

// tunnel raw-copies bytes both ways without interception (passthrough path).
func (p *proxy) tunnel(clientConn net.Conn, hostport string, alpn []string, note string) {
	defer clientConn.Close()
	up, err := net.DialTimeout("tcp", hostport, 10*time.Second)
	if err != nil {
		p.emit(Capture{Method: "CONNECT", Host: hostport, ALPN: alpn, Intercepted: false,
			Fidelity: "unsupported", Note: note + "; upstream dial failed: " + err.Error()})
		return
	}
	defer up.Close()
	cr := &countConn{Conn: clientConn}
	ur := &countConn{Conn: up}
	done := make(chan struct{}, 2)
	go func() { io.Copy(ur, cr); done <- struct{}{} }()
	go func() { io.Copy(cr, ur); done <- struct{}{} }()
	<-done
	p.emit(Capture{Method: "CONNECT", Host: hostport, ALPN: alpn, Intercepted: false,
		Fidelity: "unsupported", ReqBytes: cr.rd, RespBytes: ur.rd, Note: note})
}

// intercept terminates TLS with a minted cert and forwards decoded H1.
func (p *proxy) intercept(clientConn net.Conn, host, hostport, clientAddr string, alpn []string) {
	defer clientConn.Close()
	now := p.clock()
	cfg := &tls.Config{
		NextProtos: []string{"http/1.1"}, // only offer h1 so we terminate as h1
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := chi.ServerName
			if name == "" {
				name = host
			}
			return p.ca.leafFor(name, now)
		},
	}
	tlsConn := tls.Server(clientConn, cfg)
	if err := tlsConn.Handshake(); err != nil {
		p.emit(Capture{ClientAddr: clientAddr, Method: "CONNECT", Host: hostport, ALPN: alpn,
			Intercepted: false, Fidelity: "unsupported", Note: "TLS handshake failed: " + err.Error()})
		return
	}

	tr := p.upstreamTransport()
	defer tr.CloseIdleConnections()
	br := newReader(tlsConn)
	for {
		req, err := readRequest(br)
		if err != nil {
			return // client closed / EOF
		}
		rc := &countReader{r: req.Body}
		req.Body = rc
		req.URL.Scheme = "https"
		req.URL.Host = hostport
		req.RequestURI = ""
		req.Header.Del("Proxy-Connection")

		resp, err := tr.RoundTrip(req)
		if err != nil {
			writeGatewayError(tlsConn, err)
			p.emit(Capture{ClientAddr: clientAddr, Method: req.Method, Host: host, URL: req.URL.String(),
				ALPN: alpn, Intercepted: true, Fidelity: "decoded_wire",
				ReqBytes: rc.n, Note: "upstream error: " + err.Error()})
			return
		}
		respCounter := &countReader{r: resp.Body}
		resp.Body = respCounter
		writeErr := resp.Write(tlsConn)
		resp.Body.Close()
		p.emit(Capture{ClientAddr: clientAddr, Method: req.Method, Host: host,
			URL: req.URL.String(), ALPN: alpn, Status: resp.StatusCode,
			ReqBytes: rc.n, RespBytes: respCounter.n, Intercepted: true, Fidelity: "decoded_wire"})
		if writeErr != nil || resp.Close || req.Close {
			return
		}
	}
}

func writeGatewayError(w io.Writer, err error) {
	body := "upstream error: " + err.Error()
	fmt.Fprintf(w, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		if strings.EqualFold(k, "Proxy-Connection") {
			continue
		}
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
