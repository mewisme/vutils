package overlay

import (
	"sync"

	"github.com/mewisme/vutils/internal/config"
)

// State holds the live overlay config shared with the native window.
type State struct {
	mu  sync.RWMutex
	cfg config.OverlayConfig
}

func NewState(cfg config.OverlayConfig) *State {
	return &State{cfg: cfg}
}

func (s *State) Get() config.OverlayConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *State) Set(cfg config.OverlayConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}
