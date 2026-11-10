package transparent

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func allowsProcess(process *models.ProcessInstance, config capture.Config, selected []models.ProcessKey, ownPID int32) bool {
	if process == nil || process.Attribution != "confirmed" || process.Key.PID <= 0 || process.Key.StartTime == "" || process.Key.PID == ownPID {
		return false
	}
	switch config.ProcessScope {
	case "selected", "launch":
		for _, key := range selected {
			if process.Key == key {
				return true
			}
		}
		return false
	case "current_session":
		return process.WindowsSessionID != nil && config.ScopeSessionID != nil && *process.WindowsSessionID == *config.ScopeSessionID
	case "", "all_sessions":
		return true
	default:
		return false
	}
}
