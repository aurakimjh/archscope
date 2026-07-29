//go:build windows

package procmap

import (
	"fmt"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"golang.org/x/sys/windows"
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

func ownerPIDRows() ([]tcpRow, error) {
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
		return []byte{0, 0, 0, 0}, nil
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
			return []byte{0, 0, 0, 0}, nil
		case windowsInsufficientData:
			continue
		default:
			return nil, syscall.Errno(code)
		}
	}
	return nil, fmt.Errorf("TCP table grew during three consecutive reads")
}

func processInstance(pid int32) *models.ProcessInstance {
	process := &models.ProcessInstance{
		Key:         models.ProcessKey{PID: pid},
		Name:        "pid-" + strconv.FormatInt(int64(pid), 10),
		Attribution: "inferred",
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return process
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size) == nil {
		process.ExecPath = windows.UTF16ToString(buffer[:size])
		process.Name = filepath.Base(process.ExecPath)
	}
	process.Key.StartTime = processStartTimeFromHandle(handle)
	return process
}

func processStartTime(pid int32) string {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)
	return processStartTimeFromHandle(handle)
}

func processStartTimeFromHandle(handle windows.Handle) string {
	var created, exited, kernel, user windows.Filetime
	if windows.GetProcessTimes(handle, &created, &exited, &kernel, &user) == nil {
		return time.Unix(0, created.Nanoseconds()).UTC().Format(time.RFC3339Nano)
	}
	return ""
}
