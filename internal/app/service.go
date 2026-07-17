package app

import (
	"fmt"
	"strings"
	"sync"

	"github.com/mewisme/vutils/internal/config"
	"github.com/mewisme/vutils/internal/overlay"
)

// Service wires config persistence to the standalone overlay.
type Service struct {
	mu         sync.Mutex
	path       string
	store      config.Store
	overlayMgr *overlay.Manager
}

// NewService loads config from path (or defaults). Call Start to show overlay.
func NewService(path string) (*Service, error) {
	store, err := config.LoadOrDefault(path)
	if err != nil {
		return nil, err
	}
	return &Service{
		path:       path,
		store:      store,
		overlayMgr: overlay.NewManager(),
	}, nil
}

// Start applies the current config to the overlay (show/hide).
func (s *Service) Start() error {
	s.mu.Lock()
	cfg := config.Active(s.store)
	s.mu.Unlock()
	if err := s.overlayMgr.Apply(cfg); err != nil {
		return fmt.Errorf("apply overlay: %w", err)
	}
	return nil
}

// GetConfig returns the active profile's overlay config.
func (s *Service) GetConfig() config.OverlayConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return config.Active(s.store)
}

func (s *Service) putActive(cfg config.OverlayConfig) {
	if s.store.Profiles == nil {
		s.store.Profiles = map[string]config.OverlayConfig{}
	}
	name := s.store.ActiveProfile
	if name == "" {
		name = config.DefaultProfileName
		s.store.ActiveProfile = name
	}
	s.store.Profiles[name] = cfg
}

func (s *Service) persistLocked() error {
	return config.Save(s.path, s.store)
}

// SaveConfig validates, writes into the active profile, persists, and applies.
func (s *Service) SaveConfig(cfg config.OverlayConfig) error {
	cfg, err := config.Validate(cfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.putActive(cfg)
	err = s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.overlayMgr.Apply(cfg)
}

// SetOverlayEnabled toggles overlay visibility and persists.
func (s *Service) SetOverlayEnabled(enabled bool) error {
	s.mu.Lock()
	cfg := config.Active(s.store)
	cfg.Enabled = enabled
	s.mu.Unlock()
	return s.SaveConfig(cfg)
}

// UpdateOverlayConfig live-updates the active profile in memory and applies overlay (no disk write).
func (s *Service) UpdateOverlayConfig(cfg config.OverlayConfig) error {
	cfg, err := config.Validate(cfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.putActive(cfg)
	s.mu.Unlock()
	return s.overlayMgr.Apply(cfg)
}

// SetCalibrationMode toggles calibration border and live-applies.
func (s *Service) SetCalibrationMode(enabled bool) error {
	s.mu.Lock()
	cfg := config.Active(s.store)
	cfg.Calibration = enabled
	s.mu.Unlock()
	return s.UpdateOverlayConfig(cfg)
}

// ResetConfig restores defaults for the active profile only, persists, and applies.
func (s *Service) ResetConfig() (config.OverlayConfig, error) {
	cfg := config.Default()
	if err := s.SaveConfig(cfg); err != nil {
		return config.OverlayConfig{}, err
	}
	return cfg, nil
}

// ListProfiles returns profile names (default first, then sorted).
func (s *Service) ListProfiles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return config.ProfileNames(s.store)
}

// ActiveProfile returns the current profile name.
func (s *Service) ActiveProfile() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store.ActiveProfile == "" {
		return config.DefaultProfileName
	}
	return s.store.ActiveProfile
}

// SetActiveProfile switches active profile, persists, and applies overlay.
// Callers should flush the leaving profile via UpdateOverlayConfig/SaveConfig first.
func (s *Service) SetActiveProfile(name string) error {
	s.mu.Lock()
	if _, ok := s.store.Profiles[name]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("profile %q not found", name)
	}
	s.store.ActiveProfile = name
	cfg := config.Active(s.store)
	err := s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.overlayMgr.Apply(cfg)
}

// CreateProfile clones the active overlay into a new profile and selects it.
func (s *Service) CreateProfile(name string) error {
	name = strings.TrimSpace(name)
	if err := config.ValidateProfileName(name); err != nil {
		return err
	}
	s.mu.Lock()
	if _, exists := s.store.Profiles[name]; exists {
		s.mu.Unlock()
		return fmt.Errorf("profile %q already exists", name)
	}
	clone := config.Active(s.store)
	s.store.Profiles[name] = clone
	s.store.ActiveProfile = name
	err := s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.overlayMgr.Apply(clone)
}

// RenameProfile renames a profile. Refuses renaming "default".
func (s *Service) RenameProfile(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == config.DefaultProfileName {
		return fmt.Errorf("cannot rename the default profile")
	}
	if err := config.ValidateProfileName(newName); err != nil {
		return err
	}
	if newName == oldName {
		return nil
	}
	s.mu.Lock()
	cfg, ok := s.store.Profiles[oldName]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("profile %q not found", oldName)
	}
	if _, exists := s.store.Profiles[newName]; exists {
		s.mu.Unlock()
		return fmt.Errorf("profile %q already exists", newName)
	}
	s.store.Profiles[newName] = cfg
	delete(s.store.Profiles, oldName)
	if s.store.ActiveProfile == oldName {
		s.store.ActiveProfile = newName
	}
	err := s.persistLocked()
	s.mu.Unlock()
	return err
}

// DeleteProfile removes a profile. Refuses default and the last remaining profile.
func (s *Service) DeleteProfile(name string) error {
	if name == config.DefaultProfileName {
		return fmt.Errorf("cannot delete the default profile")
	}
	s.mu.Lock()
	if _, ok := s.store.Profiles[name]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("profile %q not found", name)
	}
	if len(s.store.Profiles) <= 1 {
		s.mu.Unlock()
		return fmt.Errorf("cannot delete the last profile")
	}
	delete(s.store.Profiles, name)
	if s.store.ActiveProfile == name {
		s.store.ActiveProfile = config.DefaultProfileName
	}
	cfg := config.Active(s.store)
	err := s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.overlayMgr.Apply(cfg)
}

// Path returns the config file path.
func (s *Service) Path() string {
	return s.path
}

// Shutdown hides and destroys the overlay window.
func (s *Service) Shutdown() {
	if s.overlayMgr != nil {
		s.overlayMgr.Stop()
	}
}
