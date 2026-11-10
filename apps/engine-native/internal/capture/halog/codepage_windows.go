//go:build windows

package halog

import "golang.org/x/sys/windows"

func defaultCodePage() int {
	cp, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetACP").Call()
	return int(cp)
}
