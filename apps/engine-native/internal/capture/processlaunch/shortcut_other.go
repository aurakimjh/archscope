//go:build !windows

package processlaunch

import "fmt"

func readShortcut(string) (shortcutProperties, error) {
	return shortcutProperties{}, fmt.Errorf("Windows shortcut resolution requires Windows")
}
