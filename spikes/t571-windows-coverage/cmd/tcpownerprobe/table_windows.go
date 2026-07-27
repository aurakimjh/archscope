//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	addressFamilyIPv4       = 2
	addressFamilyIPv6       = 23
	tcpTableOwnerPIDAll     = 5
	windowsSuccess          = 0
	windowsInsufficientData = 122
	windowsNoData           = 232
)

var getExtendedTCPTable = syscall.NewLazyDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")

func pollTCP() ([]netTCPRow, error) {
	ipv4Data, err := readOwnerPIDTable(addressFamilyIPv4)
	if err != nil {
		return nil, fmt.Errorf("GetExtendedTcpTable IPv4: %w", err)
	}
	ipv4Rows, err := decodeIPv4TCPTable(ipv4Data)
	if err != nil {
		return nil, fmt.Errorf("decode IPv4 TCP table: %w", err)
	}

	ipv6Data, err := readOwnerPIDTable(addressFamilyIPv6)
	if err != nil {
		return nil, fmt.Errorf("GetExtendedTcpTable IPv6: %w", err)
	}
	ipv6Rows, err := decodeIPv6TCPTable(ipv6Data)
	if err != nil {
		return nil, fmt.Errorf("decode IPv6 TCP table: %w", err)
	}
	return append(ipv4Rows, ipv6Rows...), nil
}

func readOwnerPIDTable(addressFamily uint32) ([]byte, error) {
	var size uint32
	code, _, _ := getExtendedTCPTable.Call(
		0,
		uintptr(unsafe.Pointer(&size)),
		0,
		uintptr(addressFamily),
		tcpTableOwnerPIDAll,
		0,
	)
	if code == windowsNoData {
		return emptyTCPTable(), nil
	}
	if code != windowsSuccess && code != windowsInsufficientData {
		return nil, syscall.Errno(code)
	}

	for attempt := 0; attempt < 3; attempt++ {
		if size < 4 {
			size = 4
		}
		buffer := make([]byte, size)
		code, _, _ = getExtendedTCPTable.Call(
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&size)),
			0,
			uintptr(addressFamily),
			tcpTableOwnerPIDAll,
			0,
		)
		switch code {
		case windowsSuccess:
			if size > uint32(len(buffer)) {
				return nil, fmt.Errorf("API returned size %d for %d-byte buffer", size, len(buffer))
			}
			return buffer[:size], nil
		case windowsNoData:
			return emptyTCPTable(), nil
		case windowsInsufficientData:
			continue
		default:
			return nil, syscall.Errno(code)
		}
	}
	return nil, fmt.Errorf("TCP table grew during three consecutive reads")
}

func emptyTCPTable() []byte {
	return []byte{0, 0, 0, 0}
}
