//go:build !windows

package procmap

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func List() ([]models.ProcessInstance, error) {
	return nil, fmt.Errorf("process selection requires Windows")
}

func CurrentSession() (uint32, error) { return 0, fmt.Errorf("session selection requires Windows") }
