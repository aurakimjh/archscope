package processlaunch

import (
	"fmt"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"sync"
)

// Suspended owns handles only until successful resume or abort. Abort after
// resume is a no-op: a closed numeric Windows handle may already be reused.
type Suspended struct {
	Key     models.ProcessKey
	mu      sync.Mutex
	state   string
	resume  func() error
	abort   func()
	release func()
}

func (s *Suspended) Resume() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != "prepared" {
		return fmt.Errorf("process is not awaiting capture scope")
	}
	if err := s.resume(); err != nil {
		return err
	}
	s.state = "running"
	s.release()
	return nil
}
func (s *Suspended) Abort() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != "prepared" {
		return
	}
	s.state = "aborted"
	s.abort()
	s.release()
}
