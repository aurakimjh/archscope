//go:build windows

package distribution

import (
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"unsafe"
)

// Read native version resources without loading or executing the supplied DLL.
func checkRuntimeVersion(path, want string) error {
	size, e := windows.GetFileVersionInfoSize(path, nil)
	if e != nil || size == 0 || size > 1<<20 {
		return fmt.Errorf("OFFLINE_WEBVIEW_VERSION: missing/bounded product version required")
	}
	data := make([]byte, int(size))
	if e = windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&data[0])); e != nil {
		return e
	}
	var info *windows.VS_FIXEDFILEINFO
	var length uint32
	if e = windows.VerQueryValue(unsafe.Pointer(&data[0]), `\`, unsafe.Pointer(&info), &length); e != nil || info == nil || length < uint32(unsafe.Sizeof(*info)) {
		return fmt.Errorf("OFFLINE_WEBVIEW_VERSION: invalid product version")
	}
	actual := fmt.Sprintf("%d.%d.%d.%d", info.ProductVersionMS>>16, info.ProductVersionMS&0xffff, info.ProductVersionLS>>16, info.ProductVersionLS&0xffff)
	runtime.KeepAlive(data)
	if actual != want {
		return fmt.Errorf("OFFLINE_WEBVIEW_VERSION: expected %s, found %s", want, actual)
	}
	return nil
}
