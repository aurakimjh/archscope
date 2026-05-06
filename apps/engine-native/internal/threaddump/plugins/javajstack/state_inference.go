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
