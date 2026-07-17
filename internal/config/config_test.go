package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mewisme/vutils/internal/config"
)

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
	c := config.Default()
	c.MapX = 42
	c.Enabled = true
	if err := config.Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.MapX != 42 || !got.Enabled {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadOrDefaultMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	got, err := config.LoadOrDefault(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.MapW != 300 {
		t.Fatalf("expected defaults, got %+v", got)
	}
	_ = os.Remove(path)
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
	if !got.ShowHorizontal || !got.ShowVertical {
		t.Fatalf("expected both lines shown, got %+v", got)
	}
}
