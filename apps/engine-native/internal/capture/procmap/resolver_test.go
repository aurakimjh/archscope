package procmap

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
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

func TestMatchingOwnerPIDRequiresExactEndpointTuple(t *testing.T) {
	client := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51000}
	proxy := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 43123}
	rows := []tcpRow{
		{LocalAddress: client.IP, LocalPort: client.Port, RemoteAddress: proxy.IP, RemotePort: 9999, OwningPID: 10},
		{LocalAddress: client.IP, LocalPort: client.Port, RemoteAddress: proxy.IP, RemotePort: proxy.Port, OwningPID: 20},
	}
	pid, ok := matchingOwnerPID(rows, client, proxy)
	if !ok || pid != 20 {
		t.Fatalf("pid=%d ok=%v", pid, ok)
	}
	if _, ok := matchingOwnerPID(rows, client, &net.TCPAddr{IP: proxy.IP, Port: 43124}); ok {
		t.Fatal("mismatched proxy endpoint was attributed")
	}
}

func TestResolverConfirmsStablePIDAndStartTime(t *testing.T) {
	client := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51000}
	proxy := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 43123}
	rowsCalls := 0
	processCalls := 0
	startCalls := 0
	resolver := Resolver{
		ownerRows: func() ([]tcpRow, error) {
			rowsCalls++
			return []tcpRow{{
				LocalAddress: client.IP, LocalPort: client.Port,
				RemoteAddress: proxy.IP, RemotePort: proxy.Port, OwningPID: 42,
			}}, nil
		},
		process: func(pid int32) *models.ProcessInstance {
			processCalls++
			return &models.ProcessInstance{
				Key:         models.ProcessKey{PID: pid, StartTime: "2026-07-29T00:00:00Z"},
				Attribution: "inferred",
			}
		},
		processStart: func(pid int32) string {
			startCalls++
			if pid != 42 {
				t.Fatalf("pid=%d", pid)
			}
			return "2026-07-29T00:00:00Z"
		},
	}
	process, err := resolver.Resolve(client, proxy)
	if err != nil {
		t.Fatal(err)
	}
	if process.Attribution != "confirmed" ||
		rowsCalls != 2 ||
		processCalls != 1 ||
		startCalls != 1 {
		t.Fatalf("process=%+v rows=%d processCalls=%d startCalls=%d", process, rowsCalls, processCalls, startCalls)
	}
}

func TestResolverRejectsPIDReuseAcrossStartTimes(t *testing.T) {
	client := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 51000}
	proxy := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 43123}
	resolver := Resolver{
		ownerRows: func() ([]tcpRow, error) {
			return []tcpRow{{
				LocalAddress: client.IP, LocalPort: client.Port,
				RemoteAddress: proxy.IP, RemotePort: proxy.Port, OwningPID: 42,
			}}, nil
		},
		process: func(pid int32) *models.ProcessInstance {
			return &models.ProcessInstance{
				Key:         models.ProcessKey{PID: pid, StartTime: "2026-07-29T00:00:00Z"},
				Attribution: "inferred",
			}
		},
		processStart: func(int32) string {
			return "2026-07-29T00:00:01Z"
		},
	}
	process, err := resolver.Resolve(client, proxy)
	if err != nil {
		t.Fatal(err)
	}
	if process.Attribution != "inferred" {
		t.Fatalf("PID reuse was confirmed: %+v", process)
	}
}
