package main

import (
	"encoding/binary"
	"fmt"
	"net"
)

const (
	ipv4OwnerPIDRowSize = 24
	ipv6OwnerPIDRowSize = 56
)

func decodeIPv4TCPTable(data []byte) ([]netTCPRow, error) {
	return decodeTCPTable(data, ipv4OwnerPIDRowSize, func(row []byte) netTCPRow {
		return netTCPRow{
			State:         binary.LittleEndian.Uint32(row[0:4]),
			LocalPort:     decodeNetworkPort(row[8:12]),
			RemoteAddress: net.IPv4(row[12], row[13], row[14], row[15]).String(),
			RemotePort:    decodeNetworkPort(row[16:20]),
			OwningProcess: int(binary.LittleEndian.Uint32(row[20:24])),
		}
	})
}

func decodeIPv6TCPTable(data []byte) ([]netTCPRow, error) {
	return decodeTCPTable(data, ipv6OwnerPIDRowSize, func(row []byte) netTCPRow {
		return netTCPRow{
			State:         binary.LittleEndian.Uint32(row[48:52]),
			LocalPort:     decodeNetworkPort(row[20:24]),
			RemoteAddress: net.IP(row[24:40]).String(),
			RemotePort:    decodeNetworkPort(row[44:48]),
			OwningProcess: int(binary.LittleEndian.Uint32(row[52:56])),
		}
	})
}

func decodeTCPTable(data []byte, rowSize int, decode func([]byte) netTCPRow) ([]netTCPRow, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("TCP owner table is shorter than its entry count")
	}
	count := binary.LittleEndian.Uint32(data[:4])
	required := uint64(4) + uint64(count)*uint64(rowSize)
	if required > uint64(len(data)) {
		return nil, fmt.Errorf("TCP owner table declares %d rows (%d bytes), only %d bytes available", count, required, len(data))
	}
	rows := make([]netTCPRow, 0, count)
	for offset := 4; offset < int(required); offset += rowSize {
		rows = append(rows, decode(data[offset:offset+rowSize]))
	}
	return rows, nil
}

func decodeNetworkPort(field []byte) int {
	return int(binary.BigEndian.Uint16(field[:2]))
}
