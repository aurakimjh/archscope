package transparent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// Fingerprints from the official WinDivert-2.2.2-A.zip, SHA256
// 63cb41763bb4b20f600b6de04e991a9c2be73279e317d4d82f237b150c5f3f15.
// No third-party binary is bundled or downloaded at application runtime.
func DriverStatus() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("transparent capture requires Windows")
	}
	return verifyDriver(os.Getenv("ARCHSCOPE_WINDIVERT_DLL"), runtime.GOARCH)
}
func verifyDriver(path, arch string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("set ARCHSCOPE_WINDIVERT_DLL to the official 2.2.2-A DLL's absolute path")
	}
	hashes := map[string]string{"WinDivert64.sys": "8da085332782708d8767bcace5327a6ec7283c17cfb85e40b03cd2323a90ddc2"}
	switch arch {
	case "amd64":
		hashes[filepath.Base(path)] = "c1e060ee19444a259b2162f8af0f3fe8c4428a1c6f694dce20de194ac8d7d9a2"
	case "386":
		hashes[filepath.Base(path)] = "a321649090c21aaa7529ce5d019d242b1d5f2a2aff04bc3224db409641604a83"
		hashes["WinDivert32.sys"] = "2f43f4251be4d72dd56c91bf6cce475d379eb9ba6c4dda2be3022ea633d5e807"
	default:
		return "", fmt.Errorf("WinDivert adapter supports Windows x86/x64")
	}
	if filepath.Base(path) != "WinDivert.dll" {
		return "", fmt.Errorf("choose the official WinDivert.dll")
	}
	for name, expected := range hashes {
		file, err := os.Open(filepath.Join(filepath.Dir(path), name))
		if err != nil {
			return "", fmt.Errorf("official WinDivert DLL/driver pair is unavailable")
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			file.Close()
			return "", fmt.Errorf("invalid WinDivert binary")
		}
		h := sha256.New()
		_, err = io.Copy(h, io.LimitReader(file, 2<<20+1))
		file.Close()
		if err != nil || hex.EncodeToString(h.Sum(nil)) != expected {
			return "", fmt.Errorf("WinDivert binary fingerprint does not match 2.2.2-A")
		}
	}
	return path, nil
}
