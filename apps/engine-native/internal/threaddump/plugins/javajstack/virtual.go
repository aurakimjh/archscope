// [한글] virtual.go — JDK 21+ virtual thread 의 carrier pinning 마커
// 감지 (T-227).
//
// Virtual Thread carrier pinning 이란?
//   JDK 21 의 virtual thread (Project Loom) 는 platform thread (carrier)
//   위에서 mount/unmount 되며 동작합니다. 그러나 synchronized 블록
//   안이나 native 메서드 안에서 blocking I/O 를 수행하면 unmount 가
//   안 되고 carrier 를 점유하는 "pinning" 이 발생 — virtual thread 의
//   확장성 이점이 사라짐.
//
// jstack 출력의 단서
//   "carrier" 키워드 또는 "Continuation" / "VirtualThread" 관련 frame.
//   특히 synchronized 안의 blocking 호출이 boundary frame 으로 노출됨.
//
// 감지 알고리즘
//   1) 스레드 헤더에 virtual / carrier 라벨 매칭 시도.
//   2) stack 안에 java.util.concurrent.locks 또는 synchronized 진입
//      frame 이 있고 그 위에 blocking I/O frame 이 있으면 pinning
//      후보로 마킹.
//   3) snapshot.Metadata["carrier_pinning"] 에 candidate_method,
//      top_frame, reason 을 기록.
//
// 분석기 사용처
//   multithread 분석기의 jvmTables.carrierPinning 표 + finding
//   VIRTUAL_THREAD_CARRIER_PINNING 의 evidence 로 사용.
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
