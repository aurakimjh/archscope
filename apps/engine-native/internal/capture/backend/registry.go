// Package backend separates capture transport selection from session lifecycle.
// Built-in transports include the explicit proxy and an opt-in exact-hash
// Windows driver adapter; unsupported configurations fail before allocation.
package backend

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/proxy"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/transparent"
)

const ExplicitProxy = "explicit_proxy"
const TransparentWindows = "windows_transparent"

type Capability struct {
	Name           string   `json:"name"`
	Available      bool     `json:"available"`
	Transparent    bool     `json:"transparent"`
	RequiresDriver bool     `json:"requiresDriver"`
	Reason         string   `json:"reason,omitempty"`
	Scopes         []string `json:"scopes"`
	Protocols      []string `json:"protocols"`
}
type Factory interface {
	Capability(platform string) Capability
	New(proxy.Config) (capture.Capturer, error)
}
type Registry struct{ factories []Factory }

func Default() *Registry {
	return &Registry{factories: []Factory{explicitFactory{}, transparentFactory{}}}
}
func (r *Registry) Capabilities(platform string) []Capability {
	out := []Capability{}
	for _, factory := range r.factories {
		out = append(out, factory.Capability(platform))
	}
	return out
}
func (r *Registry) Validate(platform, name string) (string, error) {
	if name == "" {
		name = ExplicitProxy
	}
	for _, factory := range r.factories {
		capability := factory.Capability(platform)
		if capability.Name != name {
			continue
		}
		if !capability.Available {
			return "", fmt.Errorf("%w: backend %s: %s", capture.ErrModeUnavailable, name, capability.Reason)
		}
		return name, nil
	}
	return "", fmt.Errorf("%w: unknown capture backend", capture.ErrModeUnavailable)
}
func (r *Registry) Create(platform, name string, config proxy.Config) (capture.Capturer, error) {
	name, err := r.Validate(platform, name)
	if err != nil {
		return nil, err
	}
	for _, factory := range r.factories {
		if factory.Capability(platform).Name == name {
			return factory.New(config)
		}
	}
	return nil, capture.ErrModeUnavailable
}

type explicitFactory struct{}

func (explicitFactory) Capability(platform string) Capability {
	available := platform == "windows"
	reason := ""
	if !available {
		reason = "Windows TCP-owner process attribution is required"
	}
	return Capability{Name: ExplicitProxy, Available: available, Reason: reason, Scopes: []string{"all_sessions", "current_session", "selected", "launch"}, Protocols: []string{"http/1.0", "http/1.1", "explicit opaque CONNECT fallback"}}
}
func (explicitFactory) New(config proxy.Config) (capture.Capturer, error) { return proxy.New(config) }

type transparentFactory struct{}

func (transparentFactory) Capability(platform string) Capability {
	_, err := transparent.DriverStatus()
	available := platform == "windows" && err == nil
	reason := "Experimental WinDivert 2.2.2-A IPv4 TCP ports 80/443; administrator rights required. New verified process flows only; IPv6/QUIC/existing connections are not redirected. Native Windows acceptance remains unverified."
	if err != nil {
		reason = err.Error()
	}
	return Capability{Name: TransparentWindows, Available: available, Transparent: true, RequiresDriver: true, Reason: reason, Scopes: []string{"all_sessions", "current_session", "selected", "launch"}, Protocols: []string{"IPv4 TCP HTTP/1.x", "TLS with trusted capture CA; opaque h2-only fallback"}}
}
func (transparentFactory) New(config proxy.Config) (capture.Capturer, error) {
	return transparent.New(config)
}
