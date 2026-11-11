package analysis

import (
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

func fileChangeStamp(f *os.File, _ os.FileInfo) (changeStamp, bool) {
	var basic struct {
		Creation, Access, Write, Change int64
		Attributes, Padding             uint32
	}
	e := windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()), windows.FileBasicInfo, (*byte)(unsafe.Pointer(&basic)), uint32(unsafe.Sizeof(basic)))
	return changeStamp{basic.Change, 0}, e == nil && basic.Change != 0
}
