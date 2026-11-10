package proxy

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type transparentDestinationKey struct{}

// ServeTransparent consumes a stream already attributed/redirected by a native
// transport. HTTP and TLS reuse the same parser, body policy and exporter path
// as explicit proxy traffic. Upstream dialing stays on the original IP/port;
// SNI/Host remains the TLS/HTTP authority for certificate validation.
func (s *Server) ServeTransparent(ctx context.Context, raw net.Conn, destination string, process *models.ProcessInstance) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn := &observedConnection{Conn: raw, cancel: cancel, done: make(chan struct{})}
	s.connMu.Lock()
	if s.closed.Load() {
		s.connMu.Unlock()
		conn.Close()
		return
	}
	s.handlerWG.Add(1)
	s.connections[conn] = struct{}{}
	s.connMu.Unlock()
	defer s.handlerWG.Done()
	defer s.untrackConnection(conn)
	defer conn.Close()
	ctx = context.WithValue(ctx, transparentDestinationKey{}, destination)
	ctx = context.WithValue(ctx, resolvedProcessContextKey{}, resolvedProcess{process: process, connectionID: newTransactionID(s.cfg.Now())})
	// Header classification is bounded. A fragmented ClientHello is handled by
	// the existing bounded TLS peek, not by interpreting a packet as a message.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	first, err := reader.Peek(1)
	if err != nil {
		return
	}
	conn.SetReadDeadline(time.Time{})
	replay := &bufferedConnection{Conn: conn, reader: reader}
	if first[0] == 0x16 {
		protocols, sni, hello, err := peekClientHello(replay)
		if err != nil {
			s.tunnel(ctx, replay, destination, nil, process, "unreadable transparent ClientHello")
			return
		}
		if h2Only(protocols) || s.passthroughApproved(sni) {
			s.tunnel(ctx, hello, destination, protocols, process, "transparent protocol/scoped passthrough")
			return
		}
		host := hostOnly(destination)
		authority := destination
		if sni != "" {
			host = sni
			_, port, _ := net.SplitHostPort(destination)
			authority = net.JoinHostPort(sni, port)
		}
		s.intercept(ctx, hello, host, authority, process, connectionID(ctx))
		return
	}
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			http.Error(w, "transparent ingress expects origin HTTP", http.StatusBadRequest)
			return
		}
		r.URL.Scheme = "http"
		r.URL.Host = r.Host
		if r.URL.Host == "" {
			r.URL.Host = destination
		}
		r.Header.Del("Proxy-Connection")
		s.ServeHTTP(w, r)
	})}
	_ = server.Serve(&singleConnectionListener{conn: replay, done: conn.done})
}

type observedConnection struct {
	net.Conn
	cancel context.CancelFunc
	once   sync.Once
	done   chan struct{}
}

func (c *observedConnection) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { c.cancel(); close(c.done) })
	return err
}

type bufferedConnection struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConnection) Read(data []byte) (int, error) { return c.reader.Read(data) }

type singleConnectionListener struct {
	conn     net.Conn
	accepted bool
	done     <-chan struct{}
}

func (l *singleConnectionListener) Accept() (net.Conn, error) {
	if !l.accepted {
		l.accepted = true
		return l.conn, nil
	}
	<-l.done
	return nil, net.ErrClosed
}
func (l *singleConnectionListener) Close() error   { return l.conn.Close() }
func (l *singleConnectionListener) Addr() net.Addr { return l.conn.LocalAddr() }
