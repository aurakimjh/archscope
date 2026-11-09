package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/dumppolicy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type ProcessResolver interface {
	Resolve(client, proxy net.Addr) (*models.ProcessInstance, error)
}

// Exchange is everything one intercepted request/response pair produced. It
// replaces the former positional argument list so the timing recorder and the
// optional body recorders can travel with the exchange instead of widening the
// interface every time a new observation is added.
type Exchange struct {
	Client   net.Addr
	Request  *http.Request
	Response *http.Response
	Started  time.Time
	Ended    time.Time
	// RequestBody/ResponseBody always carry the transferred byte counts; they
	// carry retained bytes only when the session enabled body capture.
	RequestBody  *bodyRecorder
	ResponseBody *bodyRecorder
	Trace        *upstreamTrace
	Process      *models.ProcessInstance
}

type Interceptor interface {
	Transaction(exchange Exchange) models.CaptureTransaction
	Passthrough(client net.Addr, host string, protocols []string, started, ended time.Time, requestBytes, responseBytes int64, process *models.ProcessInstance, reason string) models.CaptureTransaction
}

const (
	CaptureModeMITM        = "proxy_mitm"
	CaptureModePassthrough = "proxy_passthrough"
	CaptureModeNotCaptured = "proxy_not_captured"
	FidelityPending        = "pending"
	FidelityDecodedWire    = "decoded_wire"
	FidelityUnsupported    = "unsupported"
	BodyStorageOmitted     = "omitted"
)

type Config struct {
	ListenAddress    string
	Authority        *Authority
	UpstreamRoots    *x509.CertPool
	Resolver         ProcessResolver
	Interceptor      Interceptor
	AllowPassthrough map[string]time.Time
	Progress         func(models.CaptureTransaction)
	// Stamp binds a request to the current log before attribution or I/O.
	// Its generation is carried unchanged into the terminal transaction.
	Stamp       func(*models.CaptureTransaction)
	Now         func() time.Time
	DialContext func(context.Context, string, string) (net.Conn, error)
	// CaptureBodies requests the body tier. New rejects it unless the SEC-10
	// crash-dump exclusion preflight verifies independently at construction
	// time; the flag is a request, never a grant.
	CaptureBodies bool
	// MaxBodyBytes caps a single retained message body. Zero selects
	// DefaultMaxBodyBytes.
	MaxBodyBytes int
}

type Server struct {
	cfg         Config
	listener    net.Listener
	server      *http.Server
	onTx        func(models.CaptureTransaction) error
	addr        string
	closed      atomic.Bool
	stopDone    chan struct{}
	stopErr     error
	wg          sync.WaitGroup
	handlerWG   sync.WaitGroup
	connMu      sync.Mutex
	connections map[net.Conn]struct{}
}

type resolvedProcess struct {
	process *models.ProcessInstance
}

type resolvedProcessContextKey struct{}

// dumpPolicyCurrent is this package's own handle on the SEC-10 preflight.
// New deliberately re-runs the check instead of accepting a result from its
// caller: the session manager's rejection and this one must not share a single
// point that a stale or custom caller could satisfy once.
var dumpPolicyCurrent = dumppolicy.Current

func New(cfg Config) (*Server, error) {
	if cfg.CaptureBodies && !dumpPolicyCurrent().Verified() {
		return nil, capture.ErrBodyCapturePreflightPending
	}
	if cfg.Authority == nil {
		return nil, errors.New("proxy authority is required")
	}
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = "127.0.0.1:0"
	}
	if err := validateListenAddress(cfg.ListenAddress); err != nil {
		return nil, err
	}
	if cfg.Interceptor == nil {
		cfg.Interceptor = SemanticInterceptor{}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.DialContext == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		cfg.DialContext = dialer.DialContext
	}
	return &Server{
		cfg: cfg, stopDone: make(chan struct{}),
		connections: map[net.Conn]struct{}{},
	}, nil
}

func validateListenAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid proxy listen address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("capture proxy may listen only on a loopback address")
	}
	return nil
}

func (s *Server) Name() string                         { return "proxy_h1_mitm" }
func (s *Server) Available() (bool, string)            { return true, "" }
func (s *Server) RequiredPrivilege() capture.Privilege { return capture.PrivilegeNone }

func (s *Server) Start(ctx context.Context, _ capture.Config, onTx func(models.CaptureTransaction) error) (string, error) {
	if onTx == nil {
		return "", errors.New("transaction sink is required")
	}
	listener, err := net.Listen("tcp", s.cfg.ListenAddress)
	if err != nil {
		return "", err
	}
	s.listener = listener
	s.addr = listener.Addr().String()
	s.onTx = onTx
	s.server = &http.Server{
		Handler: s, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second,
		ConnContext: func(ctx context.Context, connection net.Conn) context.Context {
			return context.WithValue(ctx, resolvedProcessContextKey{}, resolvedProcess{
				process: s.resolveAddr(connection.RemoteAddr()),
			})
		},
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		_ = s.server.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.Stop(stopCtx)
	}()
	return s.addr, nil
}

func (s *Server) Stop(ctx context.Context) error {
	if !s.closed.CompareAndSwap(false, true) {
		select {
		case <-s.stopDone:
			return s.stopErr
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.server == nil {
		close(s.stopDone)
		return nil
	}
	// Hijacked CONNECT tunnels are outside net/http's graceful-shutdown
	// ownership. Close them before Shutdown so a live H2 passthrough or
	// intercepted keep-alive cannot leave a handler blocked while the capture
	// session is trying to terminalize.
	s.closeHijackedConnections()
	err := s.server.Shutdown(ctx)
	if err != nil {
		_ = s.server.Close()
	}
	s.closeHijackedConnections()
	s.wg.Wait()
	s.handlerWG.Wait()
	s.stopErr = err
	close(s.stopDone)
	return err
}

func (s *Server) Address() string { return s.addr }

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.handlerWG.Add(1)
	defer s.handlerWG.Done()
	if request.Method == http.MethodConnect {
		s.handleConnect(writer, request)
		return
	}
	s.handlePlain(writer, request)
}

func (s *Server) transport() *http.Transport {
	return &http.Transport{
		TLSClientConfig:   &tls.Config{RootCAs: s.cfg.UpstreamRoots, MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2: false,
		DialContext:       s.cfg.DialContext,
	}
}

func (s *Server) handlePlain(writer http.ResponseWriter, request *http.Request) {
	started := s.cfg.Now()
	progress := progressTransaction(request, started, nil, CaptureModeMITM)
	s.stamp(&progress)
	process := s.processForRequest(request)
	progress.Process = process
	progress.Coverage = coverage(process)
	s.progress(progress)
	trace, traceCtx := newUpstreamTrace(request.Context())
	out := request.Clone(traceCtx)
	out.RequestURI = ""
	requestBody := s.newBodyRecorder(request.Body)
	out.Body = requestBody
	transport := s.transport()
	defer transport.CloseIdleConnections()
	response, err := transport.RoundTrip(out)
	if err != nil {
		http.Error(writer, "verified upstream TLS/HTTP failure", http.StatusBadGateway)
		tx := failureTransaction(request, started, s.cfg.Now(), err, CaptureModeMITM, FidelityDecodedWire, process)
		tx.ID = progress.ID
		tx.Generation = progress.Generation
		s.emit(tx)
		return
	}
	defer response.Body.Close()
	responseBody := s.newBodyRecorder(response.Body)
	response.Body = responseBody
	copyHeaders(writer.Header(), response.Header)
	writer.WriteHeader(response.StatusCode)
	_, _ = io.Copy(writer, responseBody)
	tx := s.cfg.Interceptor.Transaction(Exchange{
		Client: parseAddr(request.RemoteAddr), Request: request, Response: response,
		Started: started, Ended: s.cfg.Now(),
		RequestBody: requestBody, ResponseBody: responseBody, Trace: trace, Process: process,
	})
	tx.ID = progress.ID
	tx.Generation = progress.Generation
	s.emit(tx)
}

// newBodyRecorder wraps a message body with the session's body-capture policy.
// A nil body still gets a recorder so byte accounting stays uniform.
func (s *Server) newBodyRecorder(body io.ReadCloser) *bodyRecorder {
	if body == nil {
		body = http.NoBody
	}
	return newBodyRecorder(body, s.cfg.CaptureBodies, s.cfg.MaxBodyBytes)
}

func (s *Server) handleConnect(writer http.ResponseWriter, request *http.Request) {
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		http.Error(writer, "CONNECT hijacking unavailable", http.StatusInternalServerError)
		return
	}
	client, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	s.trackConnection(client)
	defer s.untrackConnection(client)
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		client.Close()
		return
	}
	hostPort := request.Host
	host := hostOnly(hostPort)
	process := s.processForRequest(request)
	protocols, sni, replay, err := peekClientHello(client)
	if err != nil {
		s.tunnel(request.Context(), client, hostPort, nil, process, "unreadable ClientHello")
		return
	}
	if sni != "" {
		host = sni
	}
	if h2Only(protocols) {
		s.tunnel(request.Context(), replay, hostPort, protocols, process, "h2-only ALPN")
		return
	}
	if s.passthroughApproved(host) {
		s.tunnel(request.Context(), replay, hostPort, protocols, process, "explicit scoped passthrough approval")
		return
	}
	s.intercept(replay, host, hostPort, process)
}

func (s *Server) trackConnection(connection net.Conn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.connections[connection] = struct{}{}
}

func (s *Server) untrackConnection(connection net.Conn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	delete(s.connections, connection)
}

func (s *Server) closeHijackedConnections() {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	for connection := range s.connections {
		_ = connection.Close()
	}
}

func (s *Server) passthroughApproved(host string) bool {
	expires, ok := s.cfg.AllowPassthrough[strings.ToLower(host)]
	return ok && s.cfg.Now().Before(expires)
}

func (s *Server) tunnel(ctx context.Context, client net.Conn, hostPort string, protocols []string, process *models.ProcessInstance, reason string) {
	defer client.Close()
	started := s.cfg.Now()
	progress := progressTransaction(&http.Request{
		Method: http.MethodConnect,
		Host:   hostPort,
		URL:    &url.URL{Scheme: "https", Host: hostPort},
	}, started, process, CaptureModePassthrough)
	s.stamp(&progress)
	s.progress(progress)
	upstream, err := s.cfg.DialContext(ctx, "tcp", hostPort)
	if err != nil {
		tx := failureTransaction(&http.Request{
			Method: http.MethodConnect,
			Host:   hostPort,
			URL:    &url.URL{Scheme: "https", Host: hostPort},
		}, started, s.cfg.Now(), err, CaptureModePassthrough, FidelityUnsupported, process)
		tx.ID = progress.ID
		tx.Generation = progress.Generation
		s.emit(tx)
		return
	}
	defer upstream.Close()
	clientCount := &countingConn{Conn: client}
	upstreamCount := &countingConn{Conn: upstream}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstreamCount, clientCount); done <- struct{}{} }()
	go func() { _, _ = io.Copy(clientCount, upstreamCount); done <- struct{}{} }()
	<-done
	tx := s.cfg.Interceptor.Passthrough(client.RemoteAddr(), hostPort, protocols, started, s.cfg.Now(), clientCount.read, upstreamCount.read, process, reason)
	tx.ID = progress.ID
	tx.Generation = progress.Generation
	s.emit(tx)
}

func (s *Server) intercept(client net.Conn, host, hostPort string, process *models.ProcessInstance) {
	defer client.Close()
	started := s.cfg.Now()
	admission := models.CaptureTransaction{}
	s.stamp(&admission)
	tlsConn := tls.Server(client, &tls.Config{
		MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := hello.ServerName
			if name == "" {
				name = host
			}
			return s.cfg.Authority.LeafFor(name, s.cfg.Now())
		},
	})
	if err := tlsConn.Handshake(); err != nil {
		tx := failureTransaction(&http.Request{
			Method: http.MethodConnect,
			Host:   hostPort,
			URL:    &url.URL{Scheme: "https", Host: hostPort},
		}, started, s.cfg.Now(), fmt.Errorf("client TLS handshake: %w", err), CaptureModeNotCaptured, FidelityUnsupported, process)
		tx.Error = "TLS interception failed; diagnose certificate pinning or trust and require explicit scoped passthrough"
		tx.Generation = admission.Generation
		s.emit(tx)
		return
	}
	transport := s.transport()
	defer transport.CloseIdleConnections()
	reader := bufio.NewReader(tlsConn)
	for {
		request, err := http.ReadRequest(reader)
		if err != nil {
			return
		}
		started := s.cfg.Now()
		progress := progressTransaction(request, started, process, CaptureModeMITM)
		s.stamp(&progress)
		s.progress(progress)
		trace, traceCtx := newUpstreamTrace(request.Context())
		request = request.WithContext(traceCtx)
		requestBody := s.newBodyRecorder(request.Body)
		request.Body = requestBody
		request.URL.Scheme = "https"
		request.URL.Host = hostPort
		request.RequestURI = ""
		request.Header.Del("Proxy-Connection")
		response, err := transport.RoundTrip(request)
		if err != nil {
			writeGatewayError(tlsConn)
			tx := failureTransaction(request, started, s.cfg.Now(), err, CaptureModeMITM, FidelityDecodedWire, process)
			tx.ID = progress.ID
			tx.Generation = progress.Generation
			s.emit(tx)
			return
		}
		responseBody := s.newBodyRecorder(response.Body)
		response.Body = responseBody
		writeErr := response.Write(tlsConn)
		response.Body.Close()
		tx := s.cfg.Interceptor.Transaction(Exchange{
			Client: client.RemoteAddr(), Request: request, Response: response,
			Started: started, Ended: s.cfg.Now(),
			RequestBody: requestBody, ResponseBody: responseBody, Trace: trace, Process: process,
		})
		tx.ID = progress.ID
		tx.Generation = progress.Generation
		s.emit(tx)
		if writeErr != nil || request.Close || response.Close {
			return
		}
	}
}

func (s *Server) resolve(remote string) *models.ProcessInstance {
	return s.resolveAddr(parseAddr(remote))
}

func (s *Server) resolveAddr(remote net.Addr) *models.ProcessInstance {
	if s.cfg.Resolver == nil || s.listener == nil {
		return nil
	}
	process, _ := s.cfg.Resolver.Resolve(remote, s.listener.Addr())
	return process
}

func (s *Server) processForRequest(request *http.Request) *models.ProcessInstance {
	if resolved, ok := request.Context().Value(resolvedProcessContextKey{}).(resolvedProcess); ok {
		return resolved.process
	}
	return s.resolve(request.RemoteAddr)
}

func (s *Server) emit(tx models.CaptureTransaction) {
	if s.onTx != nil {
		_ = s.onTx(tx)
	}
}

func (s *Server) stamp(tx *models.CaptureTransaction) {
	if s.cfg.Stamp != nil {
		s.cfg.Stamp(tx)
	}
}

func (s *Server) progress(tx models.CaptureTransaction) {
	if s.cfg.Progress != nil {
		s.cfg.Progress(tx)
	}
}

func progressTransaction(request *http.Request, started time.Time, process *models.ProcessInstance, mode string) models.CaptureTransaction {
	urlValue := ""
	host := request.Host
	path := ""
	scheme := ""
	version := request.Proto
	if request.URL != nil {
		urlValue = request.URL.String()
		if request.URL.Host != "" {
			host = request.URL.Host
		}
		if request.URL.EscapedPath() != "" {
			path = request.URL.EscapedPath()
		}
		scheme = request.URL.Scheme
	}
	fidelity := FidelityPending
	if mode == CaptureModePassthrough {
		fidelity = FidelityUnsupported
	}
	return models.CaptureTransaction{
		ID: newTransactionID(started), Method: request.Method, URL: urlValue,
		Scheme: scheme, Host: hostOnly(host), Path: path, HTTPVersion: version,
		StartedAt: started.UTC().Format(time.RFC3339Nano), State: models.TxRequestSent,
		CaptureMode: mode, ObservationPoint: "proxy", Coverage: coverage(process),
		Fidelity: fidelity, Process: process,
		Request:  models.HTTPMessage{BodyStorage: BodyStorageOmitted},
		Response: models.HTTPMessage{BodyStorage: BodyStorageOmitted},
	}
}

type SemanticInterceptor struct{}

func (SemanticInterceptor) Transaction(exchange Exchange) models.CaptureTransaction {
	request, response := exchange.Request, exchange.Response
	started, ended := exchange.Started, exchange.Ended
	urlValue := ""
	if request.URL != nil {
		urlValue = request.URL.String()
	}
	pathValue := "/"
	query := ""
	scheme := ""
	host := request.Host
	if request.URL != nil {
		pathValue = request.URL.EscapedPath()
		if pathValue == "" {
			pathValue = "/"
		}
		query = request.URL.RawQuery
		scheme = request.URL.Scheme
		if request.URL.Host != "" {
			host = request.URL.Host
		}
	}

	requestMessage := models.HTTPMessage{
		Headers:     modelHeaders(request.Header),
		Cookies:     requestCookies(request),
		HeaderSize:  headerBytes(request.Header),
		BodyStorage: BodyStorageOmitted,
		ContentType: request.Header.Get("Content-Type"),
	}
	responseMessage := models.HTTPMessage{
		Headers:     modelHeaders(response.Header),
		Cookies:     responseCookies(response),
		HeaderSize:  headerBytes(response.Header),
		BodyStorage: BodyStorageOmitted,
		ContentType: response.Header.Get("Content-Type"),
	}
	applyBody(&requestMessage, exchange.RequestBody, request.Header.Get("Content-Encoding"))
	applyBody(&responseMessage, exchange.ResponseBody, response.Header.Get("Content-Encoding"))

	tx := models.CaptureTransaction{
		ID: newTransactionID(started), Method: request.Method, URL: urlValue, Scheme: scheme,
		Host: hostOnly(host), Path: pathValue, Query: query, HTTPVersion: request.Proto,
		StatusCode: response.StatusCode, StatusText: statusText(response),
		Request:   requestMessage,
		Response:  responseMessage,
		StartedAt: started.UTC().Format(time.RFC3339Nano), EndedAt: ended.UTC().Format(time.RFC3339Nano),
		State: models.TxComplete, TotalMS: float64(ended.Sub(started).Microseconds()) / 1000,
		CaptureMode: CaptureModeMITM, ObservationPoint: "proxy", Coverage: coverage(exchange.Process),
		Fidelity: FidelityDecodedWire, Process: exchange.Process,
	}
	if trace := exchange.Trace; trace != nil && trace.observed() {
		upstream := trace.Phases(ended)
		internal := proxyInternalPhases(started, ended, trace)
		tx.Timings = models.TimingSet{ProxyUpstream: &upstream, ProxyInternal: &internal}
		tx.UsedExistingConnection = trace.Reused()
	}
	return tx
}

// applyBody transfers byte accounting and any retained payload from the
// recorder onto the canonical message. A nil recorder means the path never
// wrapped that body, which is reported as omission rather than as zero bytes.
func applyBody(message *models.HTTPMessage, recorder *bodyRecorder, contentEncoding string) {
	if recorder == nil {
		message.BodySize = -1
		message.BodyStorage = BodyStorageOmitted
		return
	}
	recorder.apply(message, contentEncoding)
}

// statusText normalizes `Status` to the reason phrase alone. net/http reports
// "404 Not Found"; the canonical model keeps the code in StatusCode, so
// repeating it in StatusText would double-render it in every UI status cell.
func statusText(response *http.Response) string {
	text := strings.TrimSpace(response.Status)
	code := strconv.Itoa(response.StatusCode)
	return strings.TrimSpace(strings.TrimPrefix(text, code))
}

func headerBytes(headers http.Header) int64 {
	var total int64
	for name, values := range headers {
		for _, value := range values {
			// name + ": " + value + CRLF
			total += int64(len(name) + len(value) + 4)
		}
	}
	return total
}

func requestCookies(request *http.Request) []models.HeaderField {
	cookies := request.Cookies()
	out := make([]models.HeaderField, 0, len(cookies))
	for _, cookie := range cookies {
		out = append(out, models.HeaderField{Name: cookie.Name, Value: cookie.Value})
	}
	return out
}

func responseCookies(response *http.Response) []models.HeaderField {
	cookies := response.Cookies()
	out := make([]models.HeaderField, 0, len(cookies))
	for _, cookie := range cookies {
		out = append(out, models.HeaderField{Name: cookie.Name, Value: cookie.Value})
	}
	return out
}

func (SemanticInterceptor) Passthrough(_ net.Addr, host string, protocols []string, started, ended time.Time, requestBytes, responseBytes int64, process *models.ProcessInstance, reason string) models.CaptureTransaction {
	return models.CaptureTransaction{
		ID: newTransactionID(started), Method: http.MethodConnect, URL: "https://" + host,
		Scheme: "https", Host: hostOnly(host), Path: "", HTTPVersion: strings.Join(protocols, ","),
		Request:   models.HTTPMessage{TransferSize: requestBytes, BodyStorage: BodyStorageOmitted},
		Response:  models.HTTPMessage{TransferSize: responseBytes, BodyStorage: BodyStorageOmitted},
		StartedAt: started.UTC().Format(time.RFC3339Nano), EndedAt: ended.UTC().Format(time.RFC3339Nano),
		State: models.TxComplete, TotalMS: float64(ended.Sub(started).Microseconds()) / 1000,
		CaptureMode: CaptureModePassthrough, ObservationPoint: "proxy", Coverage: coverage(process),
		Fidelity: FidelityUnsupported, Process: process, Error: reason,
	}
}

var transactionSequence atomic.Uint64

func newTransactionID(started time.Time) string {
	return "live-" + strconv.FormatInt(started.UnixNano(), 36) + "-" + strconv.FormatUint(transactionSequence.Add(1), 36)
}

func failureTransaction(request *http.Request, started, ended time.Time, err error, mode, fidelity string, process *models.ProcessInstance) models.CaptureTransaction {
	host := request.Host
	urlValue := ""
	path := ""
	scheme := ""
	if request.URL != nil {
		urlValue = request.URL.String()
		path = request.URL.EscapedPath()
		scheme = request.URL.Scheme
	}
	return models.CaptureTransaction{
		ID: newTransactionID(started), Method: request.Method, URL: urlValue,
		Scheme: scheme, Host: hostOnly(host), Path: path,
		StartedAt: started.UTC().Format(time.RFC3339Nano), EndedAt: ended.UTC().Format(time.RFC3339Nano),
		State: models.TxFailed, TotalMS: float64(ended.Sub(started).Microseconds()) / 1000,
		CaptureMode: mode, ObservationPoint: "proxy", Coverage: coverage(process),
		Fidelity: fidelity, Process: process, Error: err.Error(),
		Request: models.HTTPMessage{BodyStorage: BodyStorageOmitted}, Response: models.HTTPMessage{BodyStorage: BodyStorageOmitted},
	}
}

func modelHeaders(headers http.Header) []models.HeaderField {
	out := make([]models.HeaderField, 0, len(headers))
	for name, values := range headers {
		for _, value := range values {
			out = append(out, models.HeaderField{Name: name, Value: value})
		}
	}
	return out
}

func coverage(process *models.ProcessInstance) string {
	if process == nil || process.Attribution != "confirmed" {
		return "unknown"
	}
	return "confirmed"
}

func hostOnly(value string) string {
	if host, _, err := net.SplitHostPort(value); err == nil {
		return host
	}
	return strings.Trim(value, "[]")
}

func parseAddr(value string) net.Addr {
	addr, err := net.ResolveTCPAddr("tcp", value)
	if err != nil {
		return stringAddr(value)
	}
	return addr
}

type stringAddr string

func (s stringAddr) Network() string { return "tcp" }
func (s stringAddr) String() string  { return string(s) }

type countingConn struct {
	net.Conn
	read int64
}

func (c *countingConn) Read(data []byte) (int, error) {
	n, err := c.Conn.Read(data)
	c.read += int64(n)
	return n, err
}

func copyHeaders(target, source http.Header) {
	for name, values := range source {
		if strings.EqualFold(name, "Proxy-Connection") {
			continue
		}
		for _, value := range values {
			target.Add(name, value)
		}
	}
}

func writeGatewayError(writer io.Writer) {
	const body = "verified upstream TLS/HTTP failure"
	_, _ = fmt.Fprintf(writer, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
}

func proxyURL(address string) *url.URL {
	result, _ := url.Parse("http://" + address)
	return result
}
