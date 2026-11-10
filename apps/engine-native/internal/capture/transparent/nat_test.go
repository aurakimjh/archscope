package transparent

import (
	"encoding/binary"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func frame(src, dst string, flags byte) []byte {
	a := netip.MustParseAddrPort(src)
	b := netip.MustParseAddrPort(dst)
	p := make([]byte, 40)
	p[0] = 0x45
	p[9] = 6
	binary.BigEndian.PutUint16(p[2:], 40)
	aa := a.Addr().As4()
	bb := b.Addr().As4()
	copy(p[12:16], aa[:])
	copy(p[16:20], bb[:])
	binary.BigEndian.PutUint16(p[20:], a.Port())
	binary.BigEndian.PutUint16(p[22:], b.Port())
	p[32] = 0x50
	p[33] = flags
	return p
}
func endpoints(p []byte) (string, string) {
	return netip.AddrPortFrom(netip.AddrFrom4([4]byte(p[12:16])), binary.BigEndian.Uint16(p[20:])).String(), netip.AddrPortFrom(netip.AddrFrom4([4]byte(p[16:20])), binary.BigEndian.Uint16(p[22:])).String()
}
func TestReflectionPreservesOriginalConnectionAndProcess(t *testing.T) {
	m := newMapper(34010)
	process := &models.ProcessInstance{Key: models.ProcessKey{PID: 42, StartTime: "fixture"}, Attribution: "confirmed"}
	calls := 0
	admit := func(local, remote netip.AddrPort) (*models.ProcessInstance, bool) { calls++; return process, true }
	packet := frame("10.0.0.1:50123", "203.0.113.1:443", 2)
	if !m.reflect(packet, true, admit) {
		t.Fatal("SYN not redirected")
	}
	src, dst := endpoints(packet)
	if src != "203.0.113.1:50123" || dst != "10.0.0.1:34010" {
		t.Fatal(src, dst)
	}
	f, ok := m.lookup(netip.MustParseAddrPort(src), netip.MustParseAddrPort(dst))
	if !ok || f.original.remote.String() != "203.0.113.1:443" || f.process.Key != process.Key {
		t.Fatal("original route or process lost")
	}
	response := frame(dst, src, 0x12)
	if !m.reflect(response, true, admit) {
		t.Fatal("response not restored")
	}
	src, dst = endpoints(response)
	if src != "203.0.113.1:443" || dst != "10.0.0.1:50123" || calls != 1 {
		t.Fatal(src, dst, calls)
	}
	m.stopAdmission()
	existing := frame("10.0.0.1:50123", "203.0.113.1:443", 0x10)
	if !m.reflect(existing, true, admit) {
		t.Fatal("stop broke existing flow")
	}
	fresh := frame("10.0.0.1:50124", "203.0.113.1:443", 2)
	if m.reflect(fresh, true, admit) {
		t.Fatal("stopping admitted a new flow")
	}
}
func TestReflectionAvoidsFiveTupleCollisionsAndUnverifiedFlows(t *testing.T) {
	m := newMapper(34010)
	admit := func(netip.AddrPort, netip.AddrPort) (*models.ProcessInstance, bool) {
		return &models.ProcessInstance{}, true
	}
	a := frame("10.0.0.1:50123", "203.0.113.1:80", 2)
	b := frame("10.0.0.1:50123", "203.0.113.1:443", 2)
	if !m.reflect(a, true, admit) || !m.reflect(b, true, admit) {
		t.Fatal("reflection")
	}
	as, _ := endpoints(a)
	bs, _ := endpoints(b)
	if as == bs {
		t.Fatal("virtual connections collided")
	}
	blocked := frame("10.0.0.1:50124", "203.0.113.1:443", 2)
	before := string(blocked)
	if m.reflect(blocked, true, func(netip.AddrPort, netip.AddrPort) (*models.ProcessInstance, bool) { return nil, false }) || string(blocked) != before {
		t.Fatal("unverified flow modified")
	}
	for _, p := range [][]byte{nil, frame("10.0.0.1:1", "203.0.113.1:22", 2), frame("10.0.0.1:1", "203.0.113.1:443", 0x10)} {
		if m.reflect(p, true, admit) {
			t.Fatal("nonadmitted traffic modified")
		}
	}
}
func TestOfficialDriverFingerprints(t *testing.T) {
	root := os.Getenv("ARCHSCOPE_TEST_WINDIVERT_ROOT")
	if root == "" {
		t.Skip("optional official binary fingerprint fixture")
	}
	for _, arch := range []struct{ folder, goarch string }{{"x86", "386"}, {"x64", "amd64"}} {
		if _, err := verifyDriver(filepath.Join(root, arch.folder, "WinDivert.dll"), arch.goarch); err != nil {
			t.Fatal(err)
		}
	}
}
func TestDriverRejectsUnknownBinaries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "WinDivert.dll")
	os.WriteFile(path, []byte("not a driver"), 0600)
	if _, err := verifyDriver(path, "amd64"); err == nil {
		t.Fatal("unknown DLL accepted")
	}
	if _, err := verifyDriver("relative.dll", "amd64"); err == nil {
		t.Fatal("relative driver accepted")
	}
}
func FuzzReflectionDoesNotPanic(f *testing.F) {
	f.Add(frame("10.0.0.1:50123", "203.0.113.1:443", 2))
	f.Fuzz(func(t *testing.T, p []byte) {
		newMapper(34010).reflect(p, true, func(netip.AddrPort, netip.AddrPort) (*models.ProcessInstance, bool) { return nil, false })
	})
}

func TestTransparentScopesRequireExactProcessInstance(t *testing.T) {
	session := uint32(2)
	other := uint32(3)
	p := &models.ProcessInstance{Key: models.ProcessKey{PID: 42, StartTime: "new"}, Attribution: "confirmed", WindowsSessionID: &session}
	for _, scope := range []string{"selected", "launch"} {
		cfg := capture.Config{ProcessScope: scope}
		if allowsProcess(p, cfg, []models.ProcessKey{{PID: 42, StartTime: "old"}}, 9) || !allowsProcess(p, cfg, []models.ProcessKey{p.Key}, 9) {
			t.Fatal("scope ignored start time")
		}
	}
	if allowsProcess(p, capture.Config{ProcessScope: "current_session", ScopeSessionID: &other}, nil, 9) || !allowsProcess(p, capture.Config{ProcessScope: "current_session", ScopeSessionID: &session}, nil, 9) {
		t.Fatal("Windows session scope")
	}
	if allowsProcess(p, capture.Config{}, nil, 42) {
		t.Fatal("capture process admitted")
	}
	p.Attribution = "unknown"
	if allowsProcess(p, capture.Config{}, nil, 9) {
		t.Fatal("unverified process admitted")
	}
}
