package config_test

import (
	"strings"
	"testing"

	"github.com/mewisme/vutils/internal/config"
)

func TestShareCodeRoundTrip(t *testing.T) {
	c := config.Default()
	c.MapX = 12
	c.MapY = 34
	c.MapW = 200
	c.MapH = 180
	c.Step = 3
	c.Thickness = 2
	c.Opacity = 0.75
	c.Color = "#AABBCC"
	c.Enabled = true
	c.Calibration = true
	c.CircleMode = true
	c.ShowHorizontal = false
	c.ShowVertical = true

	code, err := config.Encode("ranked", c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(code, "vutils;1;") {
		t.Fatalf("prefix: %s", code)
	}

	name, got, err := config.Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if name != "ranked" {
		t.Fatalf("name=%q", name)
	}
	if got.MapX != 12 || got.MapY != 34 || got.MapW != 200 || got.MapH != 180 {
		t.Fatalf("rect %+v", got)
	}
	if got.Step != 3 || got.Thickness != 2 || got.Opacity != 0.75 || got.Color != "#AABBCC" {
		t.Fatalf("style %+v", got)
	}
	if !got.Enabled || !got.Calibration || !got.CircleMode || got.ShowHorizontal || !got.ShowVertical {
		t.Fatalf("flags %+v", got)
	}
}

func TestShareCodeBadMagic(t *testing.T) {
	if _, _, err := config.Decode("nope;1;x;1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestShareCodeTruncated(t *testing.T) {
	if _, _, err := config.Decode("vutils;1;x;1;y;2"); err == nil {
		t.Fatal("expected missing tags")
	}
}

func TestShareCodeOpacityColor(t *testing.T) {
	code := "vutils;1;x;1;y;2;w;10;h;10;s;5;t;1;o;60;c;00FFFF;e;0;k;0;r;0;H;1;V;1"
	_, got, err := config.Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Opacity != 0.6 || got.Color != "#00FFFF" {
		t.Fatalf("%+v", got)
	}
	if _, _, err := config.Decode("vutils;1;x;1;y;2;w;10;h;10;s;5;t;1;o;999;c;00FFFF"); err == nil {
		t.Fatal("expected bad opacity")
	}
	if _, _, err := config.Decode("vutils;1;x;1;y;2;w;10;h;10;s;5;t;1;o;50;c;GGGGGG"); err == nil {
		t.Fatal("expected bad color")
	}
}
