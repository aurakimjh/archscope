//go:build windows

package transparent

import (
	"context"
	"errors"
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/procmap"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"golang.org/x/sys/windows"
	"net"
	"net/netip"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

type nativeDriver struct {
	library                           windows.Handle
	open, recv, send, checksum, close uintptr
	handle                            uintptr
}

// Explicit fields preserve the 80-byte WINDIVERT_ADDRESS ABI on x86 and x64.
type nativeAddress struct {
	Timestamp       int64
	Flags, Reserved uint32
	Data            [64]byte
}
type adapter struct {
	proxy     *proxy.Server
	driver    *nativeDriver
	relay     net.Listener
	mapper    *mapper
	config    capture.Config
	once      sync.Once
	done      chan struct{}
	workers   sync.WaitGroup
	errors    chan error
	closed    atomic.Bool
	failureMu sync.Mutex
	failure   error
	stopErr   error
}

func New(config proxy.Config) (capture.Capturer, error) {
	path, err := DriverStatus()
	if err != nil {
		return nil, err
	}
	server, err := proxy.New(config)
	if err != nil {
		return nil, err
	}
	library, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return nil, fmt.Errorf("load verified WinDivert: %w", err)
	}
	driver := &nativeDriver{library: library}
	for name, target := range map[string]*uintptr{"WinDivertOpen": &driver.open, "WinDivertRecv": &driver.recv, "WinDivertSend": &driver.send, "WinDivertHelperCalcChecksums": &driver.checksum, "WinDivertClose": &driver.close} {
		*target, err = windows.GetProcAddress(library, name)
		if err != nil {
			windows.FreeLibrary(library)
			return nil, fmt.Errorf("WinDivert API unavailable")
		}
	}
	return &adapter{proxy: server, driver: driver, done: make(chan struct{}), errors: make(chan error, 1)}, nil
}
func (*adapter) Name() string { return "windows_transparent" }
func (*adapter) Available() (bool, string) {
	_, err := DriverStatus()
	if err != nil {
		return false, err.Error()
	}
	return true, ""
}
func (*adapter) RequiredPrivilege() capture.Privilege { return capture.Privilege("administrator") }
func (a *adapter) Errors() <-chan error               { return a.errors }
func (a *adapter) report(err error) {
	a.failureMu.Lock()
	a.failure = errors.Join(a.failure, err)
	a.failureMu.Unlock()
	select {
	case a.errors <- err:
	default:
	}
}
func (a *adapter) Start(ctx context.Context, config capture.Config, onTx func(models.CaptureTransaction) error) (string, error) {
	a.config = config
	address, err := a.proxy.Start(ctx, config, onTx)
	if err != nil {
		return "", err
	}
	relay, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		return "", err
	}
	a.relay = relay
	port := uint16(relay.Addr().(*net.TCPAddr).Port)
	a.mapper = newMapper(port)
	a.mapper.selectedProcesses(config.AllowedProcesses)
	filter := fmt.Sprintf("outbound and ip and tcp and !loopback and (tcp.DstPort == 80 or tcp.DstPort == 443 or tcp.SrcPort == %d)", port)
	bytes, err := windows.BytePtrFromString(filter)
	if err != nil {
		return "", err
	}
	args := []uintptr{uintptr(unsafe.Pointer(bytes)), 0, 123, 0}
	if runtime.GOARCH == "386" {
		args = append(args, 0)
	}
	handle, _, callErr := syscall.SyscallN(a.driver.open, args...)
	runtime.KeepAlive(bytes)
	if handle == ^uintptr(0) || handle == 0 {
		return "", fmt.Errorf("open WinDivert (administrator rights and compatible driver required): %w", callErr)
	}
	a.driver.handle = handle
	a.workers.Add(2)
	go a.receive()
	go a.accept(ctx)
	return address, nil
}
func (a *adapter) SetSelectedProcesses(keys []models.ProcessKey) {
	if a.mapper != nil {
		a.mapper.selectedProcesses(keys)
	}
}
func (a *adapter) admit(local, remote netip.AddrPort) (*models.ProcessInstance, bool) {
	process, err := (procmap.Resolver{}).Resolve(net.TCPAddrFromAddrPort(local), net.TCPAddrFromAddrPort(remote))
	if err != nil || !allowsProcess(process, a.config, a.mapper.selected, int32(os.Getpid())) {
		return nil, false
	}
	return process, true
}

func (a *adapter) receive() {
	defer a.workers.Done()
	packet := make([]byte, 65535)
	for {
		var length uint32
		var address nativeAddress
		result, _, err := syscall.SyscallN(a.driver.recv, a.driver.handle, uintptr(unsafe.Pointer(&packet[0])), uintptr(len(packet)), uintptr(unsafe.Pointer(&length)), uintptr(unsafe.Pointer(&address)))
		if result == 0 {
			if !a.closed.Load() {
				a.report(fmt.Errorf("WinDivert receive: %w", err))
			}
			return
		}
		if length == 0 || length > uint32(len(packet)) {
			a.report(fmt.Errorf("invalid WinDivert packet length"))
			return
		}
		if a.mapper.reflect(packet[:length], address.Flags&(1<<17) != 0, a.admit) {
			address.Flags &^= 1 << 17
			args := []uintptr{uintptr(unsafe.Pointer(&packet[0])), uintptr(length), uintptr(unsafe.Pointer(&address)), 0}
			if runtime.GOARCH == "386" {
				args = append(args, 0)
			}
			valid, _, err := syscall.SyscallN(a.driver.checksum, args...)
			if valid == 0 {
				a.report(fmt.Errorf("WinDivert checksum: %w", err))
				return
			}
		}
		result, _, err = syscall.SyscallN(a.driver.send, a.driver.handle, uintptr(unsafe.Pointer(&packet[0])), uintptr(length), 0, uintptr(unsafe.Pointer(&address)))
		runtime.KeepAlive(packet)
		runtime.KeepAlive(&address)
		if result == 0 {
			if !a.closed.Load() {
				a.report(fmt.Errorf("WinDivert reinjection: %w", err))
			}
			return
		}
	}
}
func (a *adapter) accept(ctx context.Context) {
	defer a.workers.Done()
	for {
		conn, err := a.relay.Accept()
		if err != nil {
			return
		}
		remote, ok := addrPort(conn.RemoteAddr())
		local, valid := addrPort(conn.LocalAddr())
		if !ok || !valid {
			conn.Close()
			continue
		}
		f, ok := a.mapper.lookup(remote, local)
		if !ok {
			conn.Close()
			continue
		}
		a.workers.Add(1)
		go func() {
			defer a.workers.Done()
			defer a.mapper.finish(f)
			a.proxy.ServeTransparent(ctx, conn, f.original.remote.String(), f.process)
		}()
	}
}
func (a *adapter) Stop(ctx context.Context) error {
	a.once.Do(func() {
		if a.mapper != nil {
			a.mapper.stopAdmission()
		}
		if a.relay != nil {
			a.relay.Close()
		}
		a.stopErr = a.proxy.Stop(ctx)
		a.closed.Store(true)
		if a.driver.handle != 0 {
			result, _, err := syscall.SyscallN(a.driver.close, a.driver.handle)
			if result == 0 {
				a.stopErr = errors.Join(a.stopErr, err)
			}
		}
		a.workers.Wait()
		a.failureMu.Lock()
		a.stopErr = errors.Join(a.stopErr, a.failure)
		a.failureMu.Unlock()
		windows.FreeLibrary(a.driver.library)
		close(a.done)
	})
	return a.stopErr
}
