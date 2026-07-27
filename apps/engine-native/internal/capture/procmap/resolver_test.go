package procmap

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
	copy(row[12:16], net.IPv4(127, 0, 0, 1).To4())
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
		!got.RemoteAddress.Equal(net.IPv4(127, 0, 0, 1)) || got.OwningPID != 12345 || got.State != 5 {
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
