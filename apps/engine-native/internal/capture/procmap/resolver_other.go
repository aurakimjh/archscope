//go:build !windows

package procmap

import (
	"fmt"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func ownerPIDRows() ([]tcpRow, error) {
	return nil, fmt.Errorf("direct TCP owner attribution is only available on Windows")
}

func processInstance(pid int32) *models.ProcessInstance {
	return &models.ProcessInstance{
		Key:         models.ProcessKey{PID: pid},
		Attribution: "unknown",
	}
}
