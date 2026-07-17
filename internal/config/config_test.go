package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mewisme/vutils/internal/config"
)

func TestDefaultPath(t *testing.T) {
	p := config.DefaultPath()
	if !strings.Contains(filepath.ToSlash(p), ".vutils/config.json") {
		t.Fatalf("path: %s", p)
	}
}

func TestDefaultAndValidate(t *testing.T) {
	c := config.Default()
	got, err := config.Validate(c)
	if err != nil {
		t.Fatal(err)
	}
	if got.Color != "#00FFFF" {
		t.Fatalf("color: %s", got.Color)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	s := config.DefaultStore()
	c := config.Active(s)
	c.MapX = 42
	c.Enabled = true
	s.Profiles[config.DefaultProfileName] = c
	if err := config.Save(path, s); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	active := config.Active(got)
	if active.MapX != 42 || !active.Enabled {
		t.Fatalf("got %+v", active)
	}
	if got.ActiveProfile != config.DefaultProfileName {
		t.Fatalf("active: %s", got.ActiveProfile)
	}
}

func TestLoadOrDefaultMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	got, err := config.LoadOrDefault(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Active(got).MapW != 300 {
		t.Fatalf("expected defaults, got %+v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config written: %v", err)
	}
	again, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a := config.Active(again)
	if a.MapW != 300 || a.Color != "#00FFFF" {
		t.Fatalf("reloaded %+v", a)
	}
	if _, ok := again.Profiles[config.DefaultProfileName]; !ok {
		t.Fatal("missing default profile")
	}
}

func TestLoadMigratesFlatConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.json")
	if err := os.WriteFile(path, []byte(`{
  "mapX": 1, "mapY": 2, "mapW": 10, "mapH": 10,
  "step": 5, "thickness": 1, "opacity": 0.5, "color": "#00FFFF",
  "enabled": true
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveProfile != config.DefaultProfileName {
		t.Fatalf("active: %s", got.ActiveProfile)
	}
	c := config.Active(got)
	if c.MapX != 1 || !c.Enabled || !c.ShowHorizontal || !c.ShowVertical {
		t.Fatalf("migrated %+v", c)
	}
}

func TestLoadOrDefaultRewritesFlatConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.json")
	if err := os.WriteFile(path, []byte(`{
  "mapX": 7, "mapY": 8, "mapW": 100, "mapH": 100,
  "step": 5, "thickness": 1, "opacity": 0.5, "color": "#ABCDEF",
  "enabled": true
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := config.LoadOrDefault(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Active(got).MapX != 7 {
		t.Fatalf("got %+v", config.Active(got))
	}
	// File on disk must be new store shape (has profiles).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"profiles"`) || !strings.Contains(string(data), `"activeProfile"`) {
		t.Fatalf("disk not migrated:\n%s", data)
	}
	again, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if again.ActiveProfile != config.DefaultProfileName || config.Active(again).MapX != 7 {
		t.Fatalf("reload %+v", again)
	}
}

func TestLoadDefaultsShowLinesWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.json")
	// Old config without showHorizontal/showVertical keys.
	if err := os.WriteFile(path, []byte(`{
  "mapX": 1, "mapY": 2, "mapW": 10, "mapH": 10,
  "step": 5, "thickness": 1, "opacity": 0.5, "color": "#00FFFF"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c := config.Active(got)
	if !c.ShowHorizontal || !c.ShowVertical {
		t.Fatalf("expected both lines shown, got %+v", c)
	}
}

func TestProfileNamesDefaultFirst(t *testing.T) {
	s := config.DefaultStore()
	s.Profiles["zeta"] = config.Default()
	s.Profiles["alpha"] = config.Default()
	names := config.ProfileNames(s)
	if len(names) != 3 || names[0] != config.DefaultProfileName || names[1] != "alpha" || names[2] != "zeta" {
		t.Fatalf("names: %v", names)
	}
}

func TestValidateProfileName(t *testing.T) {
	if err := config.ValidateProfileName("  "); err == nil {
		t.Fatal("expected empty error")
	}
	if err := config.ValidateProfileName("a/b"); err == nil {
		t.Fatal("expected invalid chars")
	}
	if err := config.ValidateProfileName("Ranked"); err != nil {
		t.Fatal(err)
	}
}
