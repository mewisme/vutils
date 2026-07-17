package app_test

import (
	"path/filepath"
	"testing"

	"github.com/mewisme/vutils/internal/app"
	"github.com/mewisme/vutils/internal/config"
)

func TestProfilesCreateSwitchDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	svc, err := app.NewService(path)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Shutdown()

	if err := svc.CreateProfile("ranked"); err != nil {
		t.Fatal(err)
	}
	if svc.ActiveProfile() != "ranked" {
		t.Fatalf("active=%s", svc.ActiveProfile())
	}
	c := svc.GetConfig()
	c.MapX = 99
	if err := svc.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActiveProfile(config.DefaultProfileName); err != nil {
		t.Fatal(err)
	}
	if svc.GetConfig().MapX == 99 {
		t.Fatal("default should not have ranked MapX")
	}
	if err := svc.SetActiveProfile("ranked"); err != nil {
		t.Fatal(err)
	}
	if svc.GetConfig().MapX != 99 {
		t.Fatalf("ranked MapX=%d", svc.GetConfig().MapX)
	}
	if err := svc.DeleteProfile("ranked"); err != nil {
		t.Fatal(err)
	}
	if svc.ActiveProfile() != config.DefaultProfileName {
		t.Fatalf("after delete active=%s", svc.ActiveProfile())
	}
	if err := svc.DeleteProfile(config.DefaultProfileName); err == nil {
		t.Fatal("expected refuse delete default")
	}
}

func TestProfilesDuplicateRename(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	svc, err := app.NewService(path)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Shutdown()

	c := svc.GetConfig()
	c.MapX = 55
	if err := svc.SaveConfig(c); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateProfile("ranked"); err != nil {
		t.Fatal(err)
	}
	if svc.GetConfig().MapX != 55 {
		t.Fatal("duplicate should clone settings")
	}
	if err := svc.RenameProfile("ranked", "comp"); err != nil {
		t.Fatal(err)
	}
	if svc.ActiveProfile() != "comp" {
		t.Fatalf("active=%s", svc.ActiveProfile())
	}
	if err := svc.RenameProfile(config.DefaultProfileName, "other"); err == nil {
		t.Fatal("expected refuse rename default")
	}
}
