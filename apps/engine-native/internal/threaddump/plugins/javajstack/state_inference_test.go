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
