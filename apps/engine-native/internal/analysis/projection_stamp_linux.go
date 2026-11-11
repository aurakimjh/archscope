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
	return changeStamp{int64(stat.Ctim.Sec), int64(stat.Ctim.Nsec)}, true
}
