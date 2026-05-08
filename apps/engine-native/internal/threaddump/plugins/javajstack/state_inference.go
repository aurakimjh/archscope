// [한글] state_inference.go — RUNNABLE 인데 사실은 IO/네트워크 대기인
// 스레드 식별 (T-195).
//
// 배경
//   Java 의 RUNNABLE 상태는 "OS 가 스케줄링해도 되는 상태" 일 뿐,
//   실제로는 epoll_wait / socket recv / file read 같은 OS 수준 IO 에서
//   block 되어 있을 수 있음. multi-dump correlator 가 이런 스레드를
//   "일하는 중" 으로 보면 LATENCY_SECTION_DETECTED 같은 finding 을
//   놓침.
//
// 격상 규칙 (frame 시그니처 기반)
//   sun.nio.ch.EPoll.wait                → NETWORK_WAIT
//   sun.nio.ch.SocketChannelImpl.read     → NETWORK_WAIT
//   java.net.SocketInputStream.read       → NETWORK_WAIT
//   sun.nio.ch.FileChannelImpl.read       → IO_WAIT
//   java.io.FileInputStream.read          → IO_WAIT
//   ...
//
// 격상 정책
//   • RUNNABLE 만 격상 — 다른 상태는 절대 변경 안 함 (런타임이 더 정확).
//   • Top frame 만 검사 — wait/read 는 가장 깊은 frame 에 위치.
//   • 격상이 일어났음을 metadata 에 기록 (디버깅 추적용).
package javajstack

import (
	"regexp"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// T-195: Java network/IO state inference.
//
// Promote RUNNABLE threads stuck in epoll/socket frames to NETWORK_WAIT
// and threads stuck in classic file I/O to IO_WAIT. Never touches
// BLOCKED/WAITING/TIMED_WAITING — the runtime knows better than our
// heuristic.

var (
	networkWaitRE = regexp.MustCompile(
		`(?:` +
			`epollWait|EPoll(?:Selector)?\.wait|EPollArrayWrapper\.poll|` +
			`socketAccept|Socket\.\w*accept0|NioSocketImpl\.accept|` +
			`socketRead0|SocketInputStream\.socketRead|` +
			`NioSocketImpl\.read|SocketChannelImpl\.read|` +
			`SocketDispatcher\.read|sun\.nio\.ch\.\w+Selector\.\w*poll|` +
			`netty\.\w+EventLoop\.\w*select|netty\.channel\.nio\.NioEventLoop\.run` +
			`)`,
	)
	ioWaitRE = regexp.MustCompile(
		`(?:` +
			`FileInputStream\.read|FileInputStream\.readBytes|` +
			`FileChannelImpl\.read|FileChannelImpl\.transferFrom|` +
			`RandomAccessFile\.read|RandomAccessFile\.readBytes|` +
			`BufferedReader\.readLine|FileDispatcherImpl\.\w+|` +
			`java\.io\.FileInputStream\.read` +
			`)`,
	)
)

// inferJavaState returns the (possibly promoted) thread state.
//
// Rules (Python parity):
//
//   - non-RUNNABLE threads pass through unchanged.
//   - empty stacks pass through unchanged (we have nothing to match
//     against).
//   - the top frame's "module.function" + "function" form is searched
//     against the network-wait regex first, then the io-wait regex.
func inferJavaState(state models.ThreadState, frames []models.StackFrame) models.ThreadState {
	if state != models.ThreadStateRunnable || len(frames) == 0 {
		return state
	}
	top := frames[0]
	parts := make([]string, 0, 2)
	if top.Module != nil && *top.Module != "" {
		parts = append(parts, *top.Module+"."+top.Function)
	}
	parts = append(parts, top.Function)
	text := strings.Join(parts, " ")
	if networkWaitRE.MatchString(text) {
		return models.ThreadStateNetworkWait
	}
	if ioWaitRE.MatchString(text) {
		return models.ThreadStateIOWait
	}
	return state
}
