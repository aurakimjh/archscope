//go:build windows

package visualclipboard

import (
	"context"
	"encoding/binary"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var user = windows.NewLazySystemDLL("user32.dll")
var kernel = windows.NewLazySystemDLL("kernel32.dll")
var openClip = user.NewProc("OpenClipboard")
var closeClip = user.NewProc("CloseClipboard")
var emptyClip = user.NewProc("EmptyClipboard")
var setClip = user.NewProc("SetClipboardData")
var registerClip = user.NewProc("RegisterClipboardFormatW")
var globalAlloc = kernel.NewProc("GlobalAlloc")
var globalFree = kernel.NewProc("GlobalFree")
var globalLock = kernel.NewProc("GlobalLock")
var globalUnlock = kernel.NewProc("GlobalUnlock")
var moveMemory = kernel.NewProc("RtlMoveMemory")

func Supported() bool { return true }
func Write(ctx context.Context, owner uintptr, p Payload) error {
	if e := p.Validate(); e != nil {
		return e
	}
	if owner == 0 {
		return fmt.Errorf("CLIPBOARD_OWNER_REQUIRED")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	type entry struct {
		format uintptr
		body   []byte
		memory uintptr
	}
	entries := []entry{}
	register := func(name string) (uintptr, error) {
		s, e := windows.UTF16PtrFromString(name)
		if e != nil {
			return 0, e
		}
		id, _, err := registerClip.Call(uintptr(unsafe.Pointer(s)))
		if id == 0 {
			return 0, err
		}
		return id, nil
	}
	if len(p.PNG) > 0 {
		dib, e := DIBV5(p.PNG)
		if e != nil {
			return e
		}
		id, e := register("PNG")
		if e != nil {
			return e
		}
		entries = append(entries, entry{17, dib, 0}, entry{id, p.PNG, 0})
	} else {
		id, e := register("HTML Format")
		if e != nil {
			return e
		}
		utf, e := windows.UTF16FromString(p.Text)
		if e != nil {
			return e
		}
		text := make([]byte, len(utf)*2)
		for i, c := range utf {
			binary.LittleEndian.PutUint16(text[i*2:], c)
		}
		entries = append(entries, entry{id, HTMLFormat(p.HTML), 0}, entry{13, text, 0})
	}
	defer func() {
		for _, entry := range entries {
			if entry.memory != 0 {
				globalFree.Call(entry.memory)
			}
		}
	}()
	for i := range entries {
		if e := ctx.Err(); e != nil {
			return e
		}
		memory, _, e := globalAlloc.Call(0x0002, uintptr(len(entries[i].body)))
		if memory == 0 {
			return e
		}
		entries[i].memory = memory
		pointer, _, e := globalLock.Call(memory)
		if pointer == 0 {
			return e
		}
		moveMemory.Call(pointer, uintptr(unsafe.Pointer(&entries[i].body[0])), uintptr(len(entries[i].body)))
		runtime.KeepAlive(entries[i].body)
		globalUnlock.Call(memory)
	}
	opened := false
	for i := 0; i < 5; i++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		ok, _, _ := openClip.Call(owner)
		if ok != 0 {
			opened = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !opened {
		return fmt.Errorf("CLIPBOARD_BUSY: save PNG/TSV or retry")
	}
	defer closeClip.Call()
	if ok, _, e := emptyClip.Call(); ok == 0 {
		return e
	}
	for i := range entries {
		if ok, _, e := setClip.Call(entries[i].format, entries[i].memory); ok == 0 {
			return fmt.Errorf("CLIPBOARD_WRITE_FAILED: %w", e)
		}
		entries[i].memory = 0
	}
	return nil
}
