//go:build !darwin && !linux && !windows

package analysis

import "os"

func fileChangeStamp(*os.File, os.FileInfo) (changeStamp, bool) { return changeStamp{}, false }
