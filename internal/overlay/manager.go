package overlay

import "github.com/mewisme/vutils/internal/config"

// Manager owns the standalone overlay window lifecycle.
type Manager struct {
	state *State
	host  *nativeHost
}

// NewManager creates an overlay manager. Does not show a window until Apply.
func NewManager() *Manager {
	st := NewState(config.Default())
	return &Manager{
		state: st,
		host:  newNativeHost(st),
	}
}

// Apply updates live config and shows/hides the overlay based on Enabled.
func (m *Manager) Apply(cfg config.OverlayConfig) error {
	m.state.Set(cfg)
	if cfg.Enabled {
		return m.host.show()
	}
	return m.host.hide()
}

// Update pushes config without changing enabled visibility logic beyond Apply.
func (m *Manager) Update(cfg config.OverlayConfig) error {
	return m.Apply(cfg)
}

// Stop hides and destroys the native overlay window.
func (m *Manager) Stop() {
	_ = m.host.destroy()
}

// Config returns the current overlay config.
func (m *Manager) Config() config.OverlayConfig {
	return m.state.Get()
}
