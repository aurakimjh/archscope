package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
)

// countReader counts bytes read through a body and forwards Close.
type countReader struct {
	r io.ReadCloser
	n int64
}

func (c *countReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.n += int64(n)
	return n, err
}

func (c *countReader) Close() error {
	if c.r == nil {
		return nil
	}
	return c.r.Close()
}

// countConn counts bytes read from a connection (used on the passthrough path).
type countConn struct {
	net.Conn
	rd int64
}

func (c *countConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.rd += int64(n)
	return n, err
}

func newReader(r io.Reader) *bufio.Reader { return bufio.NewReader(r) }

func readRequest(br *bufio.Reader) (*http.Request, error) { return http.ReadRequest(br) }
