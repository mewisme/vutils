package app

import (
	"fmt"
	"sync"

	"github.com/mewisme/vutils/internal/config"
	"github.com/mewisme/vutils/internal/overlay"
)

// Service wires config persistence to the standalone overlay.
type Service struct {
	mu         sync.Mutex
	path       string
	cfg        config.OverlayConfig
	overlayMgr *overlay.Manager
}

// NewService loads config from path (or defaults). Call Start to show overlay.
func NewService(path string) (*Service, error) {
	cfg, err := config.LoadOrDefault(path)
	if err != nil {
		return nil, err
	}
	return &Service{
		path:       path,
		cfg:        cfg,
		overlayMgr: overlay.NewManager(),
	}, nil
}

// Start applies the current config to the overlay (show/hide).
func (s *Service) Start() error {
	s.mu.Lock()
	cfg := s.cfg
	s.mu.Unlock()
	if err := s.overlayMgr.Apply(cfg); err != nil {
		return fmt.Errorf("apply overlay: %w", err)
	}
	return nil
}

// GetConfig returns the current overlay config.
func (s *Service) GetConfig() config.OverlayConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// SaveConfig validates, persists, and applies config.
func (s *Service) SaveConfig(cfg config.OverlayConfig) error {
	cfg, err := config.Validate(cfg)
	if err != nil {
		return err
	}
	if err := config.Save(s.path, cfg); err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return s.overlayMgr.Apply(cfg)
}

// SetOverlayEnabled toggles overlay visibility and persists.
func (s *Service) SetOverlayEnabled(enabled bool) error {
	s.mu.Lock()
	cfg := s.cfg
	cfg.Enabled = enabled
	s.mu.Unlock()
	return s.SaveConfig(cfg)
}

// UpdateOverlayConfig live-updates the overlay without writing disk.
func (s *Service) UpdateOverlayConfig(cfg config.OverlayConfig) error {
	cfg, err := config.Validate(cfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return s.overlayMgr.Apply(cfg)
}

// SetCalibrationMode toggles calibration border and live-applies.
func (s *Service) SetCalibrationMode(enabled bool) error {
	s.mu.Lock()
	cfg := s.cfg
	cfg.Calibration = enabled
	s.mu.Unlock()
	return s.UpdateOverlayConfig(cfg)
}

// ResetConfig restores defaults, persists, and applies.
func (s *Service) ResetConfig() (config.OverlayConfig, error) {
	cfg := config.Default()
	if err := s.SaveConfig(cfg); err != nil {
		return config.OverlayConfig{}, err
	}
	return cfg, nil
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
