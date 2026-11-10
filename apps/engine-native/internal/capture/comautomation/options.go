package comautomation

import "github.com/aurakimjh/archscope/apps/engine-native/internal/capture/admission"

// Options only connect trusted in-process desktop services. They are not RPC
// parameters and cannot disable the engine's independent capture/privacy gates.
type Option func(*Server)

func WithAdmission(hub *admission.Hub) Option { return func(s *Server) { setAdmissionHub(s, hub) } }
