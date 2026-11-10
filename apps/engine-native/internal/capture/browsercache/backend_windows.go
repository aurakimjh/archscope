//go:build windows

package browsercache

import (
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"syscall"
	"unsafe"
)

var wininet = windows.NewLazySystemDLL("wininet.dll")
var first = wininet.NewProc("FindFirstUrlCacheEntryW")
var next = wininet.NewProc("FindNextUrlCacheEntryW")
var closeCache = wininet.NewProc("FindCloseUrlCache")
var deleteCache = wininet.NewProc("DeleteUrlCacheEntryW")

type systemBackend struct{}
type cacheInfo struct {
	Size                                       uint32
	Source, Local                              *uint16
	Kind, UseCount, HitRate, SizeLow, SizeHigh uint32
	Modified, Expires, Accessed, Synced        windows.Filetime
	Header                                     *byte
	HeaderSize                                 uint32
	Extension                                  *uint16
	Reserved                                   uint32
}

func (systemBackend) Available() bool { return true }
func (systemBackend) Entries() ([]Entry, error) {
	var handle uintptr
	var size uint32
	var buffer []byte
	for {
		result, _, err := first.Call(0, 0, uintptr(unsafe.Pointer(&size)))
		if result != 0 {
			closeCache.Call(result)
			return nil, fmt.Errorf("unexpected empty WinINet enumeration buffer")
		}
		if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
			return []Entry{}, nil
		}
		if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
			return nil, fmt.Errorf("WinINet enumerate: %w", err)
		}
		if size < uint32(unsafe.Sizeof(cacheInfo{})) || size > 2<<20 {
			return nil, fmt.Errorf("invalid WinINet entry size")
		}
		buffer = make([]byte, size)
		handle, _, err = first.Call(0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
		if handle != 0 {
			break
		}
		if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
			if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				return []Entry{}, nil
			}
			return nil, fmt.Errorf("WinINet enumerate: %w", err)
		}
	}
	defer closeCache.Call(handle)
	entries := []Entry{}
	for {
		info := (*cacheInfo)(unsafe.Pointer(&buffer[0]))
		start := uintptr(unsafe.Pointer(&buffer[0]))
		pointer := uintptr(unsafe.Pointer(info.Source))
		end := start + uintptr(len(buffer))
		if pointer < start || pointer >= end || pointer%2 != 0 {
			return nil, fmt.Errorf("invalid WinINet URL pointer")
		}
		count := int((end - pointer) / 2)
		if count > 32768 {
			count = 32768
		}
		units := unsafe.Slice(info.Source, count)
		terminated := false
		for _, value := range units {
			if value == 0 {
				terminated = true
				break
			}
		}
		if !terminated {
			return nil, fmt.Errorf("unterminated WinINet URL")
		}
		kind := "cache"
		if info.Kind&0x100000 != 0 {
			kind = "cookies"
		}
		entries = append(entries, Entry{windows.UTF16ToString(units), kind, uint64(info.SizeHigh)<<32 | uint64(info.SizeLow), uint64(info.Modified.HighDateTime)<<32 | uint64(info.Modified.LowDateTime)})
		runtime.KeepAlive(buffer)
		if len(entries) > 100000 {
			return nil, fmt.Errorf("too many WinINet entries")
		}
		for {
			size = uint32(len(buffer))
			result, _, err := next.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
			if result != 0 {
				break
			}
			if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				return entries, nil
			}
			if !errors.Is(err, windows.ERROR_INSUFFICIENT_BUFFER) {
				return nil, fmt.Errorf("WinINet next: %w", err)
			}
			if size < uint32(unsafe.Sizeof(cacheInfo{})) || size > 2<<20 {
				return nil, fmt.Errorf("invalid WinINet entry size")
			}
			buffer = make([]byte, size)
		}
	}
}
func (systemBackend) Delete(entry Entry) error {
	key, err := windows.UTF16PtrFromString(entry.Key)
	if err != nil {
		return err
	}
	result, _, err := deleteCache.Call(uintptr(unsafe.Pointer(key)))
	if result == 0 && !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) {
		return fmt.Errorf("WinINet delete failed: %w", err)
	}
	return nil
}
