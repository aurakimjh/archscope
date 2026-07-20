// ClientHello peeking for the H2-passthrough decision (design option 3).
//
// The recommendation intercepts HTTP/1.1 and *passes through* HTTP/2 rather
// than half-implementing H2 MITM. To decide before terminating TLS, the proxy
// peeks the TLS ClientHello, reads its ALPN list, and routes: if the client
// only offers h2, tunnel the raw bytes to the upstream (fidelity=unsupported,
// honest); otherwise terminate as http/1.1 and intercept. This file proves
// that routing is cheap and needs no third-party library.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
)

// prefixConn re-serves already-read bytes before the underlying conn, so the
// peeked ClientHello can be handed intact to tls.Server or to a raw tunnel.
type prefixConn struct {
	net.Conn
	prefix []byte
	off    int
}

func (p *prefixConn) Read(b []byte) (int, error) {
	if p.off < len(p.prefix) {
		n := copy(b, p.prefix[p.off:])
		p.off += n
		return n, nil
	}
	return p.Conn.Read(b)
}

// peekClientHello reads exactly one TLS handshake record, parses the ALPN
// protocol list, and returns a conn that replays those bytes. It never
// consumes application data. SNI is also returned when present.
func peekClientHello(conn net.Conn) (alpn []string, sni string, replay net.Conn, err error) {
	br := bufio.NewReader(conn)

	// TLS record header: type(1) version(2) length(2).
	hdr, err := br.Peek(5)
	if err != nil {
		return nil, "", nil, fmt.Errorf("peek record header: %w", err)
	}
	if hdr[0] != 0x16 { // 22 = handshake
		return nil, "", nil, fmt.Errorf("not a TLS handshake record (type %d)", hdr[0])
	}
	recLen := int(binary.BigEndian.Uint16(hdr[3:5]))
	full, err := br.Peek(5 + recLen)
	if err != nil {
		return nil, "", nil, fmt.Errorf("peek record body: %w", err)
	}
	// Buffer everything already read so it can be replayed downstream.
	buffered, _ := br.Peek(br.Buffered())
	replay = &prefixConn{Conn: conn, prefix: append([]byte(nil), buffered...)}

	alpn, sni = parseClientHello(full[5:]) // strip record header
	return alpn, sni, replay, nil
}

// parseClientHello walks the handshake body enough to extract ALPN + SNI. It is
// deliberately tolerant: any malformed field just yields empty results and the
// caller falls back to intercept.
func parseClientHello(b []byte) (alpn []string, sni string) {
	defer func() { _ = recover() }() // spike: never crash the proxy on a weird hello

	// handshake header: type(1) length(3)
	if len(b) < 4 || b[0] != 0x01 {
		return nil, ""
	}
	b = b[4:]
	// version(2) random(32)
	if len(b) < 34 {
		return nil, ""
	}
	b = b[34:]
	// session id
	if len(b) < 1 {
		return nil, ""
	}
	sidLen := int(b[0])
	b = b[1:]
	if len(b) < sidLen {
		return nil, ""
	}
	b = b[sidLen:]
	// cipher suites
	if len(b) < 2 {
		return nil, ""
	}
	csLen := int(binary.BigEndian.Uint16(b[:2]))
	b = b[2:]
	if len(b) < csLen {
		return nil, ""
	}
	b = b[csLen:]
	// compression methods
	if len(b) < 1 {
		return nil, ""
	}
	cmLen := int(b[0])
	b = b[1:]
	if len(b) < cmLen {
		return nil, ""
	}
	b = b[cmLen:]
	// extensions
	if len(b) < 2 {
		return nil, ""
	}
	extTotal := int(binary.BigEndian.Uint16(b[:2]))
	b = b[2:]
	if len(b) > extTotal {
		b = b[:extTotal]
	}
	for len(b) >= 4 {
		extType := binary.BigEndian.Uint16(b[:2])
		extLen := int(binary.BigEndian.Uint16(b[2:4]))
		b = b[4:]
		if len(b) < extLen {
			break
		}
		data := b[:extLen]
		b = b[extLen:]
		switch extType {
		case 16: // ALPN
			alpn = append(alpn, parseALPN(data)...)
		case 0: // SNI
			sni = parseSNI(data)
		}
	}
	return alpn, sni
}

func parseALPN(d []byte) []string {
	if len(d) < 2 {
		return nil
	}
	listLen := int(binary.BigEndian.Uint16(d[:2]))
	d = d[2:]
	if len(d) > listLen {
		d = d[:listLen]
	}
	var out []string
	for len(d) >= 1 {
		n := int(d[0])
		d = d[1:]
		if len(d) < n {
			break
		}
		out = append(out, string(d[:n]))
		d = d[n:]
	}
	return out
}

func parseSNI(d []byte) string {
	if len(d) < 2 {
		return ""
	}
	listLen := int(binary.BigEndian.Uint16(d[:2]))
	d = d[2:]
	if len(d) > listLen {
		d = d[:listLen]
	}
	for len(d) >= 3 {
		typ := d[0]
		nameLen := int(binary.BigEndian.Uint16(d[1:3]))
		d = d[3:]
		if len(d) < nameLen {
			break
		}
		if typ == 0 { // host_name
			return string(d[:nameLen])
		}
		d = d[nameLen:]
	}
	return ""
}

// hasH2Only reports whether the ALPN list requests h2 without offering h1.
func hasH2Only(alpn []string) bool {
	h2, h1 := false, false
	for _, p := range alpn {
		switch p {
		case "h2":
			h2 = true
		case "http/1.1", "http/1.0":
			h1 = true
		}
	}
	return h2 && !h1
}
