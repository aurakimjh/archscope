//go:build !windows

package transparent

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
)

func New(proxy.Config) (capture.Capturer, error) { return nil, capture.ErrModeUnavailable }
