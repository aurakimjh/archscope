//go:build windows

package procmap

import (
	"sort"
	"unsafe"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"golang.org/x/sys/windows"
)

// List returns selectable process instances. A process whose start time cannot
// be read cannot be selected safely because Windows may reuse its PID.
func List() ([]models.ProcessInstance, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	err = windows.Process32First(snapshot, &entry)
	result := []models.ProcessInstance{}
	for err == nil {
		if entry.ProcessID > 0 {
			process := processInstance(int32(entry.ProcessID))
			if process.Key.StartTime != "" {
				result = append(result, *process)
			}
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	if err != windows.ERROR_NO_MORE_FILES {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Key.PID < result[j].Key.PID
	})
	return result, nil
}
