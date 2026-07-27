package proxy

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

const maxClientHelloRecord = 64 << 10

type prefixConn struct {
	net.Conn
	prefix []byte
	offset int
}

func (p *prefixConn) Read(data []byte) (int, error) {
	if p.offset < len(p.prefix) {
		n := copy(data, p.prefix[p.offset:])
		p.offset += n
		return n, nil
	}
	return p.Conn.Read(data)
}

func peekClientHello(conn net.Conn) ([]string, string, net.Conn, error) {
	reader := bufio.NewReaderSize(conn, maxClientHelloRecord)
	header, err := reader.Peek(5)
	if err != nil {
		return nil, "", nil, err
	}
	if header[0] != 22 {
		return nil, "", nil, errors.New("not a TLS handshake")
	}
	recordSize := int(binary.BigEndian.Uint16(header[3:5]))
	if recordSize <= 0 || recordSize > maxClientHelloRecord-5 {
		return nil, "", nil, fmt.Errorf("TLS ClientHello record exceeds %d bytes", maxClientHelloRecord)
	}
	_, err = reader.Peek(5 + recordSize)
	if err != nil {
		return nil, "", nil, err
	}
	buffered, err := reader.Peek(reader.Buffered())
	if err != nil {
		return nil, "", nil, err
	}
	copyOfRecord := append([]byte(nil), buffered...)
	alpn, sni := parseClientHello(copyOfRecord[5:])
	return alpn, sni, &prefixConn{Conn: conn, prefix: copyOfRecord}, nil
}

func parseClientHello(data []byte) (alpn []string, sni string) {
	if len(data) < 4 || data[0] != 1 {
		return nil, ""
	}
	data = data[4:]
	if len(data) < 34 {
		return nil, ""
	}
	data = data[34:]
	if len(data) < 1 {
		return nil, ""
	}
	n := int(data[0])
	data = data[1:]
	if len(data) < n+2 {
		return nil, ""
	}
	data = data[n:]
	n = int(binary.BigEndian.Uint16(data[:2]))
	data = data[2:]
	if len(data) < n+1 {
		return nil, ""
	}
	data = data[n:]
	n = int(data[0])
	data = data[1:]
	if len(data) < n+2 {
		return nil, ""
	}
	data = data[n:]
	total := int(binary.BigEndian.Uint16(data[:2]))
	data = data[2:]
	if total < len(data) {
		data = data[:total]
	}
	for len(data) >= 4 {
		kind := binary.BigEndian.Uint16(data[:2])
		size := int(binary.BigEndian.Uint16(data[2:4]))
		data = data[4:]
		if size > len(data) {
			return alpn, sni
		}
		value := data[:size]
		data = data[size:]
		switch kind {
		case 0:
			sni = parseSNI(value)
		case 16:
			alpn = parseALPN(value)
		}
	}
	return alpn, sni
}

func parseALPN(data []byte) []string {
	if len(data) < 2 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(data[:2]))
	data = data[2:]
	if n < len(data) {
		data = data[:n]
	}
	var out []string
	for len(data) > 0 {
		size := int(data[0])
		data = data[1:]
		if size > len(data) {
			break
		}
		out = append(out, string(data[:size]))
		data = data[size:]
	}
	return out
}

func parseSNI(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	n := int(binary.BigEndian.Uint16(data[:2]))
	data = data[2:]
	if n < len(data) {
		data = data[:n]
	}
	for len(data) >= 3 {
		kind := data[0]
		size := int(binary.BigEndian.Uint16(data[1:3]))
		data = data[3:]
		if size > len(data) {
			return ""
		}
		if kind == 0 {
			return string(data[:size])
		}
		data = data[size:]
	}
	return ""
}

func h2Only(protocols []string) bool {
	var h2, h1 bool
	for _, protocol := range protocols {
		switch protocol {
		case "h2":
			h2 = true
		case "http/1.1", "http/1.0":
			h1 = true
		}
	}
	return h2 && !h1
}
