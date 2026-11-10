// Package admission connects a trusted desktop controller to the redacted
// pre-storage decision. It cannot bypass process scope, redaction or body gates.
package admission

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Operational timeout: bound a desktop callback; this is not a diagnostic rule.
const DefaultTimeout = 500 * time.Millisecond

type Decision func(context.Context, string, models.CaptureTransaction) (bool, error)
type source struct {
	active     func() bool
	owner      string
	generation uint64
	ctx        context.Context
	cancel     context.CancelFunc
	closed     bool
}
type Hub struct {
	mu      sync.Mutex
	owners  map[string]Decision
	active  map[string]func() bool
	sources map[string]*source
	Timeout time.Duration
}

func New() *Hub {
	return &Hub{owners: map[string]Decision{}, active: map[string]func() bool{}, sources: map[string]*source{}, Timeout: DefaultTimeout}
}
func (h *Hub) Register(fn Decision) (string, func(), error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", nil, e
	}
	key := hex.EncodeToString(b[:])
	h.mu.Lock()
	h.owners[key] = fn
	h.mu.Unlock()
	return key, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.owners, key)
		delete(h.active, key)
		for _, s := range h.sources {
			if s.owner == key {
				s.closed = true
				s.cancel()
			}
		}
	}, nil
}
func (h *Hub) Validate(key string) error {
	if key == "" {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.owners[key] == nil {
		return fmt.Errorf("unknown admission owner")
	}
	return nil
}
func (h *Hub) Begin(id, key string, generation uint64) error {
	if key == "" {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.owners[key] == nil {
		return fmt.Errorf("unknown admission owner")
	}
	if s := h.sources[id]; s != nil {
		s.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.sources[id] = &source{owner: key, generation: generation, ctx: ctx, cancel: cancel, active: h.active[key]}
	return nil
}
func (h *Hub) Decide(parent context.Context, id string, tx models.CaptureTransaction) (bool, error) {
	h.mu.Lock()
	s := h.sources[id]
	if s == nil {
		h.mu.Unlock()
		return true, nil
	}
	fn := h.owners[s.owner]
	if s.generation != tx.Generation {
		h.mu.Unlock()
		return false, context.Canceled
	}
	if s.active != nil && !s.active() {
		h.mu.Unlock()
		return true, nil
	}
	if s.closed || fn == nil {
		h.mu.Unlock()
		return false, context.Canceled
	}
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(s.ctx, timeout)
	h.mu.Unlock()
	defer cancel()
	stop := context.AfterFunc(parent, cancel)
	defer stop()
	// Registered callbacks must respect ctx. The native adapter only enqueues
	// an STA request and waits on ctx, even while its client RPC is blocked.
	allowed, e := fn(ctx, id, tx)
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	return allowed, e
}
func (h *Hub) CancelPending(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.sources[id]; s != nil && !s.closed {
		s.cancel()
		s.ctx, s.cancel = context.WithCancel(context.Background())
	}
}
func (h *Hub) Advance(id string, generation uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.sources[id]; s != nil {
		s.cancel()
		s.ctx, s.cancel = context.WithCancel(context.Background())
		s.generation = generation
	}
}
func (h *Hub) End(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.sources[id]; s != nil {
		s.closed = true
		s.cancel()
	}
}
func (h *Hub) Forget(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if s := h.sources[id]; s != nil {
		s.cancel()
		delete(h.sources, id)
	}
}

func (h *Hub) RegisterActive(fn Decision, active func() bool) (string, func(), error) {
	key, release, err := h.Register(fn)
	if err == nil {
		h.mu.Lock()
		h.active[key] = active
		h.mu.Unlock()
	}
	return key, release, err
}
func (h *Hub) CancelOwner(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, s := range h.sources {
		if s.owner == key && !s.closed {
			s.cancel()
			s.ctx, s.cancel = context.WithCancel(context.Background())
		}
	}
}
