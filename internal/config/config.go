package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed default.json
var defaultJSON []byte

const DefaultProfileName = "default"

// OverlayConfig is one profile's minimap guide overlay settings.
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

// Store is the persisted multi-profile config file.
type Store struct {
	ActiveProfile string                   `json:"activeProfile"`
	Profiles      map[string]OverlayConfig `json:"profiles"`
}

var colorRe = regexp.MustCompile(`(?i)^#[0-9a-f]{6}$`)

// Default returns the embedded default overlay config.
func Default() OverlayConfig {
	var c OverlayConfig
	if err := json.Unmarshal(defaultJSON, &c); err != nil {
		// Embedded JSON is compile-time checked; fall back if somehow broken.
		return OverlayConfig{
			MapX: 30, MapY: 30, MapW: 300, MapH: 300,
			Step: 5, Thickness: 1, Opacity: 0.6, Color: "#00FFFF",
			ShowHorizontal: true, ShowVertical: true,
		}
	}
	c, _ = Validate(c)
	return c
}

// DefaultStore returns a store with a single default profile.
func DefaultStore() Store {
	return Store{
		ActiveProfile: DefaultProfileName,
		Profiles:      map[string]OverlayConfig{DefaultProfileName: Default()},
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

// ValidateProfileName rejects empty names and path separators.
func ValidateProfileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("profile name is empty")
	}
	if strings.ContainsAny(name, `/\:*?|"<>`) {
		return fmt.Errorf("profile name has invalid characters")
	}
	return nil
}

// Normalize ensures default profile exists, validates profiles, and fixes active.
func Normalize(s Store) (Store, error) {
	if s.Profiles == nil {
		s.Profiles = map[string]OverlayConfig{}
	}
	if _, ok := s.Profiles[DefaultProfileName]; !ok {
		s.Profiles[DefaultProfileName] = Default()
	}
	for name, c := range s.Profiles {
		validated, err := Validate(c)
		if err != nil {
			return Store{}, fmt.Errorf("profile %q: %w", name, err)
		}
		s.Profiles[name] = validated
	}
	if s.ActiveProfile == "" {
		s.ActiveProfile = DefaultProfileName
	}
	if _, ok := s.Profiles[s.ActiveProfile]; !ok {
		s.ActiveProfile = DefaultProfileName
	}
	return s, nil
}

// Active returns the active profile's overlay config.
func Active(s Store) OverlayConfig {
	if c, ok := s.Profiles[s.ActiveProfile]; ok {
		return c
	}
	if c, ok := s.Profiles[DefaultProfileName]; ok {
		return c
	}
	return Default()
}

// ProfileNames returns sorted profile names with "default" first.
func ProfileNames(s Store) []string {
	names := make([]string, 0, len(s.Profiles))
	for name := range s.Profiles {
		if name != DefaultProfileName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := make([]string, 0, 1+len(names))
	if _, ok := s.Profiles[DefaultProfileName]; ok {
		out = append(out, DefaultProfileName)
	}
	return append(out, names...)
}

func decodeOverlay(data []byte, raw map[string]json.RawMessage) (OverlayConfig, error) {
	var c OverlayConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return OverlayConfig{}, err
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

// Load reads a Store from JSON. Flat OverlayConfig files migrate into profiles.default
// in memory only — use LoadOrDefault to also rewrite the file.
func Load(path string) (Store, error) {
	s, _, err := load(path)
	return s, err
}

// load returns the store and whether the on-disk file was a legacy flat config.
func load(path string) (Store, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Store{}, false, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Store{}, false, fmt.Errorf("parse config: %w", err)
	}

	if _, hasProfiles := raw["profiles"]; hasProfiles {
		var s Store
		if err := json.Unmarshal(data, &s); err != nil {
			return Store{}, false, fmt.Errorf("parse config: %w", err)
		}
		// Apply omitted show* defaults per profile.
		for name, c := range s.Profiles {
			profRaw := map[string]json.RawMessage{}
			if pr, ok := raw["profiles"]; ok {
				var profiles map[string]json.RawMessage
				if json.Unmarshal(pr, &profiles) == nil {
					if one, ok := profiles[name]; ok {
						_ = json.Unmarshal(one, &profRaw)
					}
				}
			}
			if _, ok := profRaw["showHorizontal"]; !ok {
				c.ShowHorizontal = true
			}
			if _, ok := profRaw["showVertical"]; !ok {
				c.ShowVertical = true
			}
			s.Profiles[name] = c
		}
		s, err := Normalize(s)
		return s, false, err
	}

	// Flat legacy config → wrap as default profile.
	c, err := decodeOverlay(data, raw)
	if err != nil {
		return Store{}, false, fmt.Errorf("parse config: %w", err)
	}
	s, err := Normalize(Store{
		ActiveProfile: DefaultProfileName,
		Profiles:      map[string]OverlayConfig{DefaultProfileName: c},
	})
	return s, true, err
}

// Save writes Store as indented JSON.
func Save(path string, s Store) error {
	s, err := Normalize(s)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// DefaultPath returns ~/.vutils/config.json (user home).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".vutils", "config.json")
	}
	return filepath.Join(home, ".vutils", "config.json")
}

// LoadOrDefault loads path, or writes DefaultStore if missing.
// Legacy flat configs are migrated to the profiles store and rewritten on disk.
func LoadOrDefault(path string) (Store, error) {
	s, migrated, err := load(path)
	if err != nil {
		if os.IsNotExist(err) {
			s = DefaultStore()
			if err := Save(path, s); err != nil {
				return Store{}, fmt.Errorf("write default config: %w", err)
			}
			return s, nil
		}
		return Store{}, err
	}
	if migrated {
		if err := Save(path, s); err != nil {
			return Store{}, fmt.Errorf("migrate config: %w", err)
		}
	}
	return s, nil
}

// ResolvePath returns the config path under the user home (.vutils/config.json).
func ResolvePath() string {
	return DefaultPath()
}
