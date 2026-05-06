package javajstack

import (
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// T-227 — virtual-thread carrier pinning detection.
// T-229 — native-method extraction.

// isVirtualThread mirrors Python's _is_virtual_thread. Looks for either
// the literal `virtualthread` token or `virtual thread` substring in
// thread name + raw block.
func isVirtualThread(record threadDumpRecord) bool {
	text := strings.ToLower(record.ThreadName + "\n" + record.RawBlock)
	return strings.Contains(text, "virtualthread") || strings.Contains(text, "virtual thread")
}

// nativeMethod returns the first frame containing "(Native Method)".
// Mirrors Python's _native_method.
func nativeMethod(record threadDumpRecord) string {
	for _, frame := range record.Stack {
		if strings.Contains(frame, "(Native Method)") {
			return strings.TrimSpace(frame)
		}
	}
	return ""
}

// carrierPinning surfaces a structured payload when the raw block
// mentions virtual-thread carrier or pinning markers. Returns nil
// otherwise. Mirrors Python's _carrier_pinning.
func carrierPinning(rawBlock string, frames []models.StackFrame) map[string]any {
	text := strings.ToLower(rawBlock)
	if !strings.Contains(text, "virtual") {
		return nil
	}
	if !strings.Contains(text, "carrier") && !strings.Contains(text, "pinn") {
		return nil
	}
	var topFrame string
	if len(frames) > 0 {
		topFrame = frames[0].Render()
	}
	candidate := firstNonJDKFrame(frames)
	if candidate == "" {
		candidate = topFrame
	}
	out := map[string]any{
		"reason":           "virtual_thread_carrier_or_pinning_marker",
		"candidate_method": candidate,
	}
	if topFrame != "" {
		out["top_frame"] = topFrame
	} else {
		out["top_frame"] = nil
	}
	return out
}

// firstNonJDKFrame returns the rendered text of the first stack frame
// that doesn't sit under a JDK / sun.* / java.* prefix. Used as the
// pinning candidate method. Mirrors Python's _first_non_jdk_frame.
func firstNonJDKFrame(frames []models.StackFrame) string {
	jdkPrefixes := []string{"java.", "javax.", "jdk.", "sun.", "com.sun.", "java.base."}
	for _, frame := range frames {
		rendered := frame.Render()
		isJDK := false
		for _, prefix := range jdkPrefixes {
			if strings.HasPrefix(rendered, prefix) {
				isJDK = true
				break
			}
		}
		if !isJDK {
			return rendered
		}
	}
	return ""
}
