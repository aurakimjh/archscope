//go:build !windows

package comautomation

import "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/admission"

func setAdmissionHub(*Server, *admission.Hub) {}
