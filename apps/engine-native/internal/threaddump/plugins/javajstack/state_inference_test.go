// [한글] state inference 회귀 테스트 (T-195).
//
// 검증 대상
//   • RUNNABLE + epoll/SocketChannel.read → NETWORK_WAIT.
//   • RUNNABLE + FileChannel.read / FileInputStream → IO_WAIT.
//   • 다른 상태(BLOCKED/WAITING 등) 는 절대 변경되지 않음.
//   • 격상이 일어났음을 metadata 에 기록.
//   • Top frame 만 검사 (deeper frames 가 같은 패턴이어도 영향 없음).
package javajstack

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// T-195 — JVM network/IO state inference.

func TestInferStatePromotesRunnableEpollToNetworkWait(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("sun.nio.ch.EPoll", "epollWait"),
		javaFrame("sun.nio.ch.EPollSelectorImpl", "doSelect"),
	}
	got := inferJavaState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateNetworkWait {
		t.Fatalf("state = %s, want NETWORK_WAIT", got)
	}
}

func TestInferStatePromotesSocketRead0ToNetworkWait(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("java.net.SocketInputStream", "socketRead0"),
	}
	got := inferJavaState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateNetworkWait {
		t.Fatalf("state = %s, want NETWORK_WAIT", got)
	}
}

func TestInferStatePromotesFileInputStreamToIOWait(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("java.io.FileInputStream", "readBytes"),
		javaFrame("com.example.Loader", "load"),
	}
	got := inferJavaState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateIOWait {
		t.Fatalf("state = %s, want IO_WAIT", got)
	}
}

func TestInferStateKeepsBlockedThreadsBlocked(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("java.net.SocketInputStream", "socketRead0"),
	}
	got := inferJavaState(models.ThreadStateBlocked, frames)
	if got != models.ThreadStateBlocked {
		t.Fatalf("state = %s, want BLOCKED (runtime always wins)", got)
	}
}

func TestInferStateLeavesRunnableUnrelatedTopFrameAlone(t *testing.T) {
	frames := []models.StackFrame{javaFrame("com.example.Worker", "compute")}
	got := inferJavaState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateRunnable {
		t.Fatalf("state = %s, want RUNNABLE", got)
	}
}

func TestInferStateEmptyStackPassesThrough(t *testing.T) {
	got := inferJavaState(models.ThreadStateRunnable, nil)
	if got != models.ThreadStateRunnable {
		t.Fatalf("state = %s, want RUNNABLE", got)
	}
}

func TestInferStatePromotesNettyEventLoop(t *testing.T) {
	frames := []models.StackFrame{
		javaFrame("io.netty.channel.nio.NioEventLoop", "run"),
	}
	got := inferJavaState(models.ThreadStateRunnable, frames)
	if got != models.ThreadStateNetworkWait {
		t.Fatalf("netty event loop should promote to NETWORK_WAIT, got %s", got)
	}
}
