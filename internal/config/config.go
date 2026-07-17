package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// OverlayConfig is the persisted minimap guide overlay settings.
type OverlayConfig struct {
	MapX           int     `json:"mapX"`
	MapY           int     `json:"mapY"`
	MapW           int     `json:"mapW"`
	MapH           int     `json:"mapH"`
	Step           int     `json:"step"` // nudge step for D-pad (pixels)
	Thickness      int     `json:"thickness"`
	Opacity        float64 `json:"opacity"`
	Color          string  `json:"color"`
	Enabled        bool    `json:"enabled"`
	Calibration    bool    `json:"calibration"`
	CircleMode     bool    `json:"circleMode"`     // calibration border: circle instead of rectangle
	ShowHorizontal bool    `json:"showHorizontal"` // crosshair horizontal arm
	ShowVertical   bool    `json:"showVertical"`   // crosshair vertical arm
}

var colorRe = regexp.MustCompile(`(?i)^#[0-9a-f]{6}$`)

// Default returns the shipped default overlay config.
func Default() OverlayConfig {
	return OverlayConfig{
		MapX:           30,
		MapY:           30,
		MapW:           300,
		MapH:           300,
		Step:           5,
		Thickness:      1,
		Opacity:        0.6,
		Color:          "#00FFFF",
		Enabled:        false,
		Calibration:    false,
		CircleMode:     false,
		ShowHorizontal: true,
		ShowVertical:   true,
	}
}

// Validate checks config values and returns a sanitized copy or an error.
func Validate(c OverlayConfig) (OverlayConfig, error) {
	if c.MapW < 1 {
		return c, fmt.Errorf("mapW must be >= 1")
	}
	if c.MapH < 1 {
		return c, fmt.Errorf("mapH must be >= 1")
	}
	if c.Step < 1 {
		c.Step = 5 // migrate old configs missing step
	}
	if c.Thickness < 1 {
		return c, fmt.Errorf("thickness must be >= 1")
	}
	if c.Opacity < 0 || c.Opacity > 1 {
		return c, fmt.Errorf("opacity must be between 0 and 1")
	}
	color := strings.ToUpper(c.Color)
	if !colorRe.MatchString(color) {
		return c, fmt.Errorf("color must be #RRGGBB")
	}
	c.Color = color
	return c, nil
}

// Load reads OverlayConfig from a JSON file.
func Load(path string) (OverlayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OverlayConfig{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return OverlayConfig{}, fmt.Errorf("parse config: %w", err)
	}
	var c OverlayConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return OverlayConfig{}, fmt.Errorf("parse config: %w", err)
	}
	// Old configs omit these keys; default show both arms.
	if _, ok := raw["showHorizontal"]; !ok {
		c.ShowHorizontal = true
	}
	if _, ok := raw["showVertical"]; !ok {
		c.ShowVertical = true
	}
	return Validate(c)
}

// Save writes OverlayConfig as indented JSON.
func Save(path string, c OverlayConfig) error {
	c, err := Validate(c)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// PathBesideExecutable returns config.json next to the binary, or cwd fallback.
func PathBesideExecutable() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return filepath.Join(filepath.Dir(exe), "config.json")
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

// LoadOrDefault loads path, or returns Default if the file is missing.
func LoadOrDefault(path string) (OverlayConfig, error) {
	c, err := Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return OverlayConfig{}, err
	}
	return c, nil
}

// ResolvePath returns config.json beside the executable when present,
// otherwise ./config.json for dev (wails dev / go run).
func ResolvePath() string {
	p := PathBesideExecutable()
	if _, err := os.Stat(p); err == nil {
		return p
	}
	if _, err := os.Stat("config.json"); err == nil {
		return "config.json"
	}
	return p
}
