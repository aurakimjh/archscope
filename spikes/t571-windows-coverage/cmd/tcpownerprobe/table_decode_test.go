package main

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

func TestDecodeIPv4TCPTable(t *testing.T) {
	data := make([]byte, 4+ipv4OwnerPIDRowSize)
	binary.LittleEndian.PutUint32(data[:4], 1)
	row := data[4:]
	binary.LittleEndian.PutUint32(row[0:4], 5)
	copy(row[4:8], net.IPv4(192, 168, 0, 10).To4())
	binary.BigEndian.PutUint16(row[8:10], 49152)
	copy(row[12:16], net.IPv4(192, 168, 0, 3).To4())
	binary.BigEndian.PutUint16(row[16:18], 18080)
	binary.LittleEndian.PutUint32(row[20:24], 12345)

	rows, err := decodeIPv4TCPTable(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	got := rows[0]
	if got.LocalPort != 49152 || got.RemotePort != 18080 ||
		got.RemoteAddress != "192.168.0.3" || got.OwningProcess != 12345 || got.State != 5 {
		t.Fatalf("decoded row = %+v", got)
	}
}

func TestDecodeIPv6TCPTable(t *testing.T) {
	data := make([]byte, 4+ipv6OwnerPIDRowSize)
	binary.LittleEndian.PutUint32(data[:4], 1)
	row := data[4:]
	copy(row[0:16], net.ParseIP("2001:db8::10").To16())
	binary.BigEndian.PutUint16(row[20:22], 49153)
	copy(row[24:40], net.ParseIP("2001:db8::3").To16())
	binary.BigEndian.PutUint16(row[44:46], 443)
	binary.LittleEndian.PutUint32(row[48:52], 5)
	binary.LittleEndian.PutUint32(row[52:56], 54321)

	rows, err := decodeIPv6TCPTable(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1", len(rows))
	}
	got := rows[0]
	if got.LocalPort != 49153 || got.RemotePort != 443 ||
		got.RemoteAddress != "2001:db8::3" || got.OwningProcess != 54321 || got.State != 5 {
		t.Fatalf("decoded row = %+v", got)
	}
}

func TestDecodeTCPTableRejectsTruncatedRows(t *testing.T) {
	data := make([]byte, 4+ipv4OwnerPIDRowSize-1)
	binary.LittleEndian.PutUint32(data[:4], 1)

	_, err := decodeIPv4TCPTable(data)
	if err == nil || !strings.Contains(err.Error(), "only") {
		t.Fatalf("err=%v, want truncated-table error", err)
	}
}
