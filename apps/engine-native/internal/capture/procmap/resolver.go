// Package procmap attributes an accepted proxy connection to its owning
// process. On Windows it uses the direct TCP owner table validated by T-571;
// no shell or polling subprocess is involved.
package procmap

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

const (
	ipv4OwnerPIDRowSize = 24
	ipv6OwnerPIDRowSize = 56
)

type tcpRow struct {
	LocalAddress  net.IP
	LocalPort     int
	RemoteAddress net.IP
	RemotePort    int
	OwningPID     int32
	State         uint32
}

type Resolver struct{}

func (Resolver) Resolve(client, proxy net.Addr) (*models.ProcessInstance, error) {
	clientTCP, ok := client.(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("client address is not TCP: %T", client)
	}
	proxyTCP, ok := proxy.(*net.TCPAddr)
	if !ok {
		return nil, fmt.Errorf("proxy address is not TCP: %T", proxy)
	}
	rows, err := ownerPIDRows()
	if err != nil {
		return nil, err
	}
	pid, ok := matchingOwnerPID(rows, clientTCP, proxyTCP)
	if !ok {
		return nil, fmt.Errorf("TCP owner row disappeared before attribution")
	}
	process := processInstance(pid)
	if process.Key.StartTime == "" {
		return process, nil
	}
	verificationRows, err := ownerPIDRows()
	if err != nil {
		return process, nil
	}
	verifiedPID, ok := matchingOwnerPID(verificationRows, clientTCP, proxyTCP)
	if ok && verifiedPID == pid {
		process.Attribution = "confirmed"
	}
	return process, nil
}

func matchingOwnerPID(rows []tcpRow, clientTCP, proxyTCP *net.TCPAddr) (int32, bool) {
	for _, row := range rows {
		if row.LocalPort != clientTCP.Port || row.RemotePort != proxyTCP.Port {
			continue
		}
		if !sameIP(row.RemoteAddress, proxyTCP.IP) {
			continue
		}
		return row.OwningPID, true
	}
	return 0, false
}

func sameIP(rowIP, addressIP net.IP) bool {
	if addressIP == nil || addressIP.IsUnspecified() {
		return true
	}
	return rowIP.Equal(addressIP)
}

func decodeIPv4TCPTable(data []byte) ([]tcpRow, error) {
	return decodeTCPTable(data, ipv4OwnerPIDRowSize, func(row []byte) tcpRow {
		return tcpRow{
			State:         binary.LittleEndian.Uint32(row[0:4]),
			LocalAddress:  net.IPv4(row[4], row[5], row[6], row[7]),
			LocalPort:     decodeNetworkPort(row[8:12]),
			RemoteAddress: net.IPv4(row[12], row[13], row[14], row[15]),
			RemotePort:    decodeNetworkPort(row[16:20]),
			OwningPID:     int32(binary.LittleEndian.Uint32(row[20:24])),
		}
	})
}

func decodeIPv6TCPTable(data []byte) ([]tcpRow, error) {
	return decodeTCPTable(data, ipv6OwnerPIDRowSize, func(row []byte) tcpRow {
		return tcpRow{
			LocalAddress:  append(net.IP(nil), row[0:16]...),
			LocalPort:     decodeNetworkPort(row[20:24]),
			RemoteAddress: append(net.IP(nil), row[24:40]...),
			RemotePort:    decodeNetworkPort(row[44:48]),
			State:         binary.LittleEndian.Uint32(row[48:52]),
			OwningPID:     int32(binary.LittleEndian.Uint32(row[52:56])),
		}
	})
}

func decodeTCPTable(data []byte, rowSize int, decode func([]byte) tcpRow) ([]tcpRow, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("TCP owner table is shorter than its entry count")
	}
	count := binary.LittleEndian.Uint32(data[:4])
	required := uint64(4) + uint64(count)*uint64(rowSize)
	if required > uint64(len(data)) {
		return nil, fmt.Errorf("TCP owner table declares %d rows (%d bytes), only %d bytes available", count, required, len(data))
	}
	rows := make([]tcpRow, 0, count)
	for offset := 4; offset < int(required); offset += rowSize {
		rows = append(rows, decode(data[offset:offset+rowSize]))
	}
	return rows, nil
}

func decodeNetworkPort(field []byte) int {
	return int(binary.BigEndian.Uint16(field[:2]))
}
