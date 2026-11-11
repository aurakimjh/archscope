package analysis

import (
	"os"
	"syscall"
)

func fileChangeStamp(_ *os.File, info os.FileInfo) (changeStamp, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return changeStamp{}, false
	}
	return changeStamp{stat.Ctimespec.Sec, stat.Ctimespec.Nsec}, true
}
