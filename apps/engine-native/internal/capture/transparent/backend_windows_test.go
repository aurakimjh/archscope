//go:build windows

package transparent

import (
	"testing"
	"unsafe"
)

func TestWinDivertAddressABI(t *testing.T) {
	if unsafe.Sizeof(nativeAddress{}) != 80 || unsafe.Offsetof(nativeAddress{}.Data) != 16 {
		t.Fatal("WinDivert address ABI mismatch")
	}
}
