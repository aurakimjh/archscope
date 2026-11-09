//go:build !windows

package processlaunch

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

type Suspended struct{ Key models.ProcessKey }

func Prepare(string, []string, string) (*Suspended, error) {
	return nil, fmt.Errorf("process launching for capture requires Windows")
}
func (*Suspended) Resume() error { return fmt.Errorf("process launching for capture requires Windows") }
func (*Suspended) Abort()        {}
