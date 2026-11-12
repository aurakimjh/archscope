//go:build (!windows && !darwin) || (darwin && !cgo)

package visualclipboard

import (
	"context"
	"fmt"
)

func Supported() bool { return false }
func Write(context.Context, uintptr, Payload) error {
	return fmt.Errorf("VISUAL_CLIPBOARD_UNSUPPORTED: save PNG or HTML/TSV instead")
}
