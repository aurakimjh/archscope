package metrichistory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
)

// The manually produced, allowlisted export contains no prompt/response bodies.
// Its version pins field meaning; importing never runs tools or contacts Ollama.
type appleCapture struct {
	SelectionSHA string          `json:"model_selection_sha256,omitempty"`
	Contract     string          `json:"contract"`
	Collector    string          `json:"collector_version"`
	Host         string          `json:"host_id"`
	Boot         string          `json:"boot_id"`
	Device       string          `json:"device_local_id"`
	OS           string          `json:"macos_version"`
	Build        string          `json:"macos_build"`
	Hardware     string          `json:"hardware_model"`
	Chip         string          `json:"chip"`
	Engine       string          `json:"ollama_version"`
	Snapshots    []appleSnapshot `json:"snapshots"`
}
type appleValue struct {
	Value  *string `json:"value"`
	Reason string  `json:"unavailable_reason,omitempty"`
}
type appleSnapshot struct {
	Excluded   *string      `json:"excluded_model_count,omitempty"`
	Matched    *string      `json:"selected_model_match_count,omitempty"`
	Start      *string      `json:"capture_start_ns"`
	End        *string      `json:"capture_end_ns"`
	Memory     string       `json:"memory_capacity_bytes"`
	Free       string       `json:"vm_free_bytes"`
	Active     string       `json:"vm_active_bytes"`
	Inactive   string       `json:"vm_inactive_bytes"`
	Wired      string       `json:"vm_wired_bytes"`
	Compressor string       `json:"vm_compressor_bytes"`
	SwapTotal  string       `json:"swap_total_bytes"`
	SwapUsed   string       `json:"swap_used_bytes"`
	Pressure   appleValue   `json:"pressure_level"`
	GPU        appleValue   `json:"gpu_utilization"`
	Power      appleValue   `json:"power_watts"`
	Thermal    appleValue   `json:"thermal_warning_level"`
	Models     []appleModel `json:"models"`
}
type appleModel struct {
	Name         string      `json:"name"`
	Digest       string      `json:"digest"`
	Size         json.Number `json:"size"`
	GPUSize      json.Number `json:"size_vram"`
	Context      json.Number `json:"context_length"`
	Quantization string      `json:"quantization_level"`
}

func appleText(s string) bool {
	return s != "" && s != "unknown" && len(s) <= 256 && !strings.ContainsFunc(s, unicode.IsControl)
}
func appleJSON(v any) json.RawMessage { raw, _ := json.Marshal(v); return raw }

func (b *builder) apple(body []byte, options Options) error {
	var c appleCapture
	if len(body) > 4<<20 {
		return fmt.Errorf("APPLE_CAPTURE_BUDGET: maximum 4 MiB")
	}
	if e := decodeSource(b.ctx, body, &c, true); e != nil {
		return e
	}
	if c.Contract != "apple-ollama-capture.v1" || (c.Collector != "1.0.0" && c.Collector != "1.1.0") || c.Engine != "0.40.16" {
		return fmt.Errorf("APPLE_CAPTURE_VERSION: qualified collector 1.0.0 or 1.1.0 / Ollama 0.40.16 required")
	}
	for _, s := range []string{c.Host, c.Boot, c.Device, c.OS, c.Build, c.Hardware, c.Chip} {
		if !appleText(s) {
			return fmt.Errorf("APPLE_CAPTURE_IDENTITY: local host/boot/device and platform versions required")
		}
	}
	for _, tag := range []string{c.Host, c.Boot, c.Device} {
		if !captureTag.MatchString(tag) {
			return fmt.Errorf("APPLE_CAPTURE_TAG: sanitized local tags required")
		}
	}
	if !strings.HasPrefix(c.Chip, "Apple ") || len(c.Snapshots) == 0 || len(c.Snapshots) > 256 {
		return fmt.Errorf("APPLE_CAPTURE_SCOPE: Apple Silicon; 1..256 snapshots required")
	}
	if (options.Producer != "" && options.Producer != "ollama") || (options.ProducerVersion != "" && options.ProducerVersion != c.Engine) || options.Namespace != "" || options.ExportID != "" || string(b.set.Sources[0].Query) != "{}" {
		return fmt.Errorf("APPLE_CAPTURE_OPTIONS: capture owns producer/platform identity; no overrides")
	}
	b.set.Sources[0].Producer = "ollama"
	b.set.Sources[0].ProducerVersion = c.Engine
	b.set.Sources[0].Query = appleJSON(map[string]string{"contract": c.Contract, "collector_version": c.Collector, "system_sources": "hw.memsize,vm_stat,vm.swapusage,kern.memorystatus_vm_pressure_level,pmset -g therm", "engine_source": "GET /api/ps"})
	b.origin = "apple-ollama-capture.v1:collector:" + c.Collector + ":ollama:" + c.Engine
	b.set.Warnings = append(b.set.Warnings,
		"Shared system memory categories overlap; never sum them or subtract model estimates to calculate free GPU memory. Model size_vram is a producer-reported residency estimate, not dedicated VRAM or GPU utilization.",
		"Capture start/end bounds describe sequential read skew, not simultaneous samples or a metric aggregation interval. No workload causation, GPU efficiency, OOM threshold, temperature or energy inference.",
		"Private macOS pressure code is preserved without enum interpretation. Missing telemetry is no_recorded_value with an explicit reason; no automatic privileged collection.")
	resource := appleJSON(map[string]string{"host_id": c.Host, "boot_id": c.Boot, "device_local_id": c.Device, "vendor": "apple", "memory_model": "unified_shared", "macos_version": c.OS, "macos_build": c.Build, "hardware_model": c.Hardware, "chip": c.Chip, "engine": "ollama", "engine_version": c.Engine, "collector_version": c.Collector})
	if c.Collector == "1.1.0" {
		decoded, err := hex.DecodeString(c.SelectionSHA)
		if err != nil || len(decoded) != 32 || strings.ToLower(c.SelectionSHA) != c.SelectionSHA {
			return fmt.Errorf("APPLE_CAPTURE_SELECTION_HASH_REQUIRED")
		}
		var identity map[string]string
		_ = json.Unmarshal(resource, &identity)
		identity["model_selection_sha256"] = c.SelectionSHA
		resource = appleJSON(identity)
		b.set.Warnings = append(b.set.Warnings, "Capture selects one exact model-name hash. excluded_model_count reports other loaded models without names/digests; selected_model_match_count=0 means the selector is not loaded, not that the host has no loaded models. Never treat the retained model list as the complete loaded inventory.")
	} else if c.SelectionSHA != "" {
		return fmt.Errorf("APPLE_CAPTURE_SELECTION_VERSION")
	}
	var previous uint64
	for i, snapshot := range c.Snapshots {
		start, ok := m.Time(snapshot.Start)
		end, endOK := m.Time(snapshot.End)
		if !ok || !endOK || start > end || i > 0 && start <= previous {
			return fmt.Errorf("APPLE_CAPTURE_TIME: snapshot %d must have increasing, nonoverlapping canonical bounds", i)
		}
		previous = end
		attrs := map[string]string{"scope": "system", "capture_semantics": "sequential_read_window"}
		add := func(name, unit, description string, value *string, reason string, attributes map[string]string) error {
			if value != nil {
				if _, e := m.Uint(*value); e != nil {
					return fmt.Errorf("APPLE_CAPTURE_VALUE: snapshots/%d/%s", i, name)
				}
			}
			a := map[string]string{}
			for k, v := range attributes {
				a[k] = v
			}
			if reason != "" {
				a["unavailable_reason"] = reason
			}
			id, e := b.addSeries(m.Series{Name: name, Description: description, Unit: unit, Kind: "gauge", Temporality: "unspecified", Resource: resource, Attributes: appleJSON(a), Scope: m.Scope{Name: c.Contract, Version: c.Collector}})
			if e != nil {
				return e
			}
			point := m.Point{SeriesID: id, Start: snapshot.Start, End: snapshot.End, Value: value}
			if value == nil {
				point.Special = "no_recorded_value"
				point.Flags = 1
			}
			return b.addPoint(point, fmt.Sprintf("snapshots/%d/%s/%s", i, name, m.Hash(a)))
		}
		if c.Collector == "1.1.0" {
			if snapshot.Excluded == nil || snapshot.Matched == nil || (*snapshot.Matched != "0" && *snapshot.Matched != "1") || len(snapshot.Models) != int((*snapshot.Matched)[0]-'0') {
				return fmt.Errorf("APPLE_CAPTURE_SELECTION_COVERAGE_REQUIRED")
			}
			if _, err := m.Uint(*snapshot.Excluded); err != nil {
				return fmt.Errorf("APPLE_CAPTURE_EXCLUDED_COUNT_INVALID")
			}
			for _, model := range snapshot.Models {
				hash := sha256.Sum256([]byte(model.Name))
				if hex.EncodeToString(hash[:]) != c.SelectionSHA {
					return fmt.Errorf("APPLE_CAPTURE_SELECTION_MISMATCH")
				}
			}
			selectionAttrs := map[string]string{"scope": "model_selection", "capture_semantics": "sequential_read_window"}
			if err := add("ollama.selection.excluded_model_count", "{model}", "Other loaded model count; identities intentionally excluded. Not selected-model memory.", snapshot.Excluded, "", selectionAttrs); err != nil {
				return err
			}
			if err := add("ollama.selection.selected_model_match_count", "{model}", "Exact selected-model load state (0 or 1); zero is not an empty host inventory.", snapshot.Matched, "", selectionAttrs); err != nil {
				return err
			}
		} else if snapshot.Excluded != nil || snapshot.Matched != nil {
			return fmt.Errorf("APPLE_CAPTURE_SELECTION_VERSION")
		}
		for _, v := range []struct{ name, value string }{
			{"apple.system.memory_capacity", snapshot.Memory}, {"apple.system.vm_free", snapshot.Free}, {"apple.system.vm_active", snapshot.Active}, {"apple.system.vm_inactive", snapshot.Inactive}, {"apple.system.vm_wired", snapshot.Wired}, {"apple.system.vm_compressor", snapshot.Compressor}, {"apple.system.swap_total", snapshot.SwapTotal}, {"apple.system.swap_used", snapshot.SwapUsed},
		} {
			if e := add(v.name, "By", "System-wide bytes; overlapping VM categories are not a partition or a GPU allocation.", &v.value, "", attrs); e != nil {
				return e
			}
		}
		capacity, e := m.Uint(snapshot.Memory)
		if e != nil || capacity.Sign() == 0 {
			return fmt.Errorf("APPLE_CAPTURE_CAPACITY")
		}
		for name, value := range map[string]string{"vm_free": snapshot.Free, "vm_active": snapshot.Active, "vm_inactive": snapshot.Inactive, "vm_wired": snapshot.Wired, "vm_compressor": snapshot.Compressor} {
			v, err := m.Uint(value)
			if err != nil || v.Cmp(capacity) > 0 {
				return fmt.Errorf("APPLE_CAPTURE_PHYSICAL_BOUND: snapshots/%d/%s exceeds capacity", i, name)
			}
		}
		swapTotal, _ := m.Uint(snapshot.SwapTotal)
		swapUsed, _ := m.Uint(snapshot.SwapUsed)
		if swapUsed.Cmp(swapTotal) > 0 {
			return fmt.Errorf("APPLE_CAPTURE_SWAP: used exceeds total")
		}
		for _, v := range []struct {
			name, unit, description string
			value                   appleValue
			supported               bool
		}{
			{"apple.system.pressure_level", "{code}", "Raw private sysctl code; categorical, no percent, ratio or stable enum mapping.", snapshot.Pressure, true},
			{"apple.device.gpu_utilization", "1", "Unavailable: residency estimates do not measure GPU activity.", snapshot.GPU, false},
			{"apple.device.power", "W", "Unavailable: no privileged power collection.", snapshot.Power, false},
			{"apple.system.thermal_warning_level", "{code}", "pmset warning level; not a temperature or measured normal when no level was recorded.", snapshot.Thermal, true},
		} {
			if v.value.Value == nil {
				switch v.value.Reason {
				case "not_recorded", "not_collected", "permission_required", "unsupported":
				default:
					return fmt.Errorf("APPLE_CAPTURE_AVAILABILITY: %s", v.name)
				}
			} else if !v.supported || v.value.Reason != "" {
				return fmt.Errorf("APPLE_CAPTURE_UNSUPPORTED_MEASUREMENT: %s", v.name)
			}
			scope := attrs
			if strings.HasPrefix(v.name, "apple.device.") {
				scope = map[string]string{"scope": "device", "capture_semantics": "sequential_read_window"}
			}
			if e := add(v.name, v.unit, v.description, v.value.Value, v.value.Reason, scope); e != nil {
				return e
			}
		}
		if snapshot.Models == nil || len(snapshot.Models) > 64 {
			return fmt.Errorf("APPLE_CAPTURE_MODELS: explicit 0..64 models required")
		}
		seen := map[string]bool{}
		for _, model := range snapshot.Models {
			_, e := hex.DecodeString(model.Digest)
			if !appleText(model.Name) || !appleText(model.Quantization) || len(model.Digest) != 64 || strings.ToLower(model.Digest) != model.Digest || e != nil || seen[model.Digest] {
				return fmt.Errorf("APPLE_CAPTURE_MODEL_IDENTITY")
			}
			seen[model.Digest] = true
			size, sizeErr := m.Uint(string(model.Size))
			gpuSize, gpuErr := m.Uint(string(model.GPUSize))
			if sizeErr != nil || gpuErr != nil || gpuSize.Cmp(size) > 0 {
				return fmt.Errorf("APPLE_CAPTURE_MODEL_BOUND: size_vram must not exceed size")
			}
			modelAttrs := map[string]string{"scope": "model_estimate", "model": model.Name, "model_digest": model.Digest, "quantization": model.Quantization, "context_length": string(model.Context)}
			contextLength, e := m.Uint(string(model.Context))
			if e != nil || contextLength.Sign() == 0 {
				return fmt.Errorf("APPLE_CAPTURE_MODEL_CONTEXT")
			}
			for _, v := range []struct{ name, value string }{{"ollama.model.loaded_size_estimate", string(model.Size)}, {"ollama.model.gpu_residency_estimate", string(model.GPUSize)}} {
				if e := add(v.name, "By", "Producer-reported per-model estimate; shared memory, no dedicated VRAM, utilization, capacity or cross-model sum.", &v.value, "", modelAttrs); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
