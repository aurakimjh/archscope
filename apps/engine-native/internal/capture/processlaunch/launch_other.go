//go:build !windows

package processlaunch

import (
	"fmt"
)

func Prepare(Target, string) (*Suspended, error) {
	return nil, fmt.Errorf("process launching for capture requires Windows")
}
