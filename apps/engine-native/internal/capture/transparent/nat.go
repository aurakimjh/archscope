// Package transparent implements the stream adapter over WinDivert's packet
// transport. Packet NAT is separate from HTTP parsing and session persistence.
package transparent

import (
	"encoding/binary"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"net"
	"net/netip"
	"sync"
	"time"
)

type tuple struct{ local, remote netip.AddrPort }
type reflected struct{ remote, local netip.AddrPort }
type flow struct {
	original          tuple
	virtual           reflected
	process           *models.ProcessInstance
	seen              time.Time
	claimed, finished bool
}
type mapper struct {
	mu       sync.Mutex
	port     uint16
	flows    map[tuple]*flow
	reverse  map[reflected]*flow
	stopping bool
	selected []models.ProcessKey
}

func newMapper(port uint16) *mapper {
	return &mapper{port: port, flows: map[tuple]*flow{}, reverse: map[reflected]*flow{}}
}
func (m *mapper) selectedProcesses(keys []models.ProcessKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.selected = append([]models.ProcessKey(nil), keys...)
}
func (m *mapper) stopAdmission() { m.mu.Lock(); defer m.mu.Unlock(); m.stopping = true }
func (m *mapper) lookup(remote, local netip.AddrPort) (*flow, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.reverse[reflected{remote, local}]
	if f == nil || f.claimed {
		return nil, false
	}
	f.claimed = true
	copy := *f
	return &copy, true
}
func (m *mapper) finish(f *flow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if actual := m.flows[f.original]; actual != nil {
		actual.finished = true
		actual.seen = time.Now()
	}
}

// reflect rewrites outbound client/proxy packets into inbound packets. All
// packets outside an admitted 5-tuple remain byte-for-byte unchanged.
func (m *mapper) reflect(packet []byte, outbound bool, admit func(netip.AddrPort, netip.AddrPort) (*models.ProcessInstance, bool)) bool {
	if !outbound || len(packet) < 40 || packet[0]>>4 != 4 || packet[9] != 6 {
		return false
	}
	header := int(packet[0]&15) * 4
	total := int(binary.BigEndian.Uint16(packet[2:4]))
	if header < 20 || total > len(packet) || total < header+20 || binary.BigEndian.Uint16(packet[6:8])&0x3fff != 0 {
		return false
	}
	tcp := packet[header:total]
	if int(tcp[12]>>4)*4 < 20 || int(tcp[12]>>4)*4 > len(tcp) {
		return false
	}
	src := netip.AddrPortFrom(netip.AddrFrom4([4]byte(packet[12:16])), binary.BigEndian.Uint16(tcp[:2]))
	dst := netip.AddrPortFrom(netip.AddrFrom4([4]byte(packet[16:20])), binary.BigEndian.Uint16(tcp[2:4]))
	m.mu.Lock()
	defer m.mu.Unlock()
	var f *flow
	response := src.Port() == m.port
	if response {
		f = m.reverse[reflected{remote: dst, local: src}]
	} else {
		key := tuple{src, dst}
		f = m.flows[key]
		if f == nil {
			if m.stopping || tcp[13]&0x12 != 0x02 || (dst.Port() != 80 && dst.Port() != 443) {
				return false
			}
			now := time.Now()
			for key, old := range m.flows {
				if (old.finished && now.Sub(old.seen) > 2*time.Minute) || (!old.claimed && now.Sub(old.seen) > 30*time.Second) {
					delete(m.reverse, old.virtual)
					delete(m.flows, key)
				}
			}
			if len(m.flows) >= 4096 {
				return false
			}
			process, ok := admit(src, dst)
			if !ok || process == nil {
				return false
			}
			virtual := reflected{remote: netip.AddrPortFrom(dst.Addr(), src.Port()), local: netip.AddrPortFrom(src.Addr(), m.port)}
			for attempts := 0; m.reverse[virtual] != nil; attempts++ {
				if attempts >= 65534 {
					return false
				}
				port := virtual.remote.Port() + 1
				if port == 0 {
					port = 1
				}
				virtual.remote = netip.AddrPortFrom(dst.Addr(), port)
			}
			f = &flow{original: key, virtual: virtual, process: process, seen: now}
			m.flows[key] = f
			m.reverse[virtual] = f
		}
	}
	if f == nil {
		return false
	}
	f.seen = time.Now()
	if response {
		remote := f.original.remote.Addr().As4()
		local := f.original.local.Addr().As4()
		copy(packet[12:16], remote[:])
		copy(packet[16:20], local[:])
		binary.BigEndian.PutUint16(tcp[:2], f.original.remote.Port())
		binary.BigEndian.PutUint16(tcp[2:4], f.original.local.Port())
	} else {
		remote := f.virtual.remote.Addr().As4()
		local := f.virtual.local.Addr().As4()
		copy(packet[12:16], remote[:])
		copy(packet[16:20], local[:])
		binary.BigEndian.PutUint16(tcp[:2], f.virtual.remote.Port())
		binary.BigEndian.PutUint16(tcp[2:4], m.port)
	}
	return true
}
func addrPort(address net.Addr) (netip.AddrPort, bool) {
	tcp, ok := address.(*net.TCPAddr)
	if !ok {
		return netip.AddrPort{}, false
	}
	addr, ok := netip.AddrFromSlice(tcp.IP)
	if !ok {
		return netip.AddrPort{}, false
	}
	return netip.AddrPortFrom(addr.Unmap(), uint16(tcp.Port)), true
}
