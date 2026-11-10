//go:build !windows

package comautomation

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/automation"
)

type Server struct{}

func Start(automation.Executor, ...Option) (*Server, error) {
	return nil, fmt.Errorf("native COM automation requires Windows")
}
func (*Server) Close() error  { return nil }
func Register(string) error   { return fmt.Errorf("COM registration requires Windows") }
func Unregister(string) error { return fmt.Errorf("COM registration requires Windows") }
