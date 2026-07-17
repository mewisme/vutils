package config

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	shareMagic   = "vutils"
	shareVersion = "1"
)

func Encode(name string, c OverlayConfig) (string, error) {
	c, err := Validate(c)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, ";", "")
	op := int(c.Opacity*100 + 0.5)
	if op < 0 {
		op = 0
	}
	if op > 100 {
		op = 100
	}
	color := strings.TrimPrefix(strings.ToUpper(c.Color), "#")
	parts := []string{shareMagic, shareVersion}
	if name != "" {
		parts = append(parts, "n", name)
	}
	parts = append(parts,
		"x", strconv.Itoa(c.MapX),
		"y", strconv.Itoa(c.MapY),
		"w", strconv.Itoa(c.MapW),
		"h", strconv.Itoa(c.MapH),
		"s", strconv.Itoa(c.Step),
		"t", strconv.Itoa(c.Thickness),
		"o", strconv.Itoa(op),
		"c", color,
		"e", bool01(c.Enabled),
		"k", bool01(c.Calibration),
		"r", bool01(c.CircleMode),
		"H", bool01(c.ShowHorizontal),
		"V", bool01(c.ShowVertical),
	)
	return strings.Join(parts, ";"), nil
}

// Decode parses a share code into an optional name hint and overlay config.
func Decode(code string) (name string, c OverlayConfig, err error) {
	code = strings.TrimSpace(code)
	parts := strings.Split(code, ";")
	if len(parts) < 2 {
		return "", OverlayConfig{}, fmt.Errorf("invalid share code")
	}
	if parts[0] != shareMagic {
		return "", OverlayConfig{}, fmt.Errorf("not a vutils share code")
	}
	if parts[1] != shareVersion {
		return "", OverlayConfig{}, fmt.Errorf("unsupported share code version %q", parts[1])
	}
	if (len(parts)-2)%2 != 0 {
		return "", OverlayConfig{}, fmt.Errorf("invalid share code tags")
	}

	tags := map[string]string{}
	for i := 2; i+1 < len(parts); i += 2 {
		tags[parts[i]] = parts[i+1]
	}

	required := []string{"x", "y", "w", "h", "s", "t", "o", "c"}
	for _, k := range required {
		if _, ok := tags[k]; !ok {
			return "", OverlayConfig{}, fmt.Errorf("share code missing %q", k)
		}
	}

	c.MapX, err = strconv.Atoi(tags["x"])
	if err != nil {
		return "", OverlayConfig{}, fmt.Errorf("bad x: %w", err)
	}
	c.MapY, err = strconv.Atoi(tags["y"])
	if err != nil {
		return "", OverlayConfig{}, fmt.Errorf("bad y: %w", err)
	}
	c.MapW, err = strconv.Atoi(tags["w"])
	if err != nil {
		return "", OverlayConfig{}, fmt.Errorf("bad w: %w", err)
	}
	c.MapH, err = strconv.Atoi(tags["h"])
	if err != nil {
		return "", OverlayConfig{}, fmt.Errorf("bad h: %w", err)
	}
	c.Step, err = strconv.Atoi(tags["s"])
	if err != nil {
		return "", OverlayConfig{}, fmt.Errorf("bad s: %w", err)
	}
	c.Thickness, err = strconv.Atoi(tags["t"])
	if err != nil {
		return "", OverlayConfig{}, fmt.Errorf("bad t: %w", err)
	}
	op, err := strconv.Atoi(tags["o"])
	if err != nil {
		return "", OverlayConfig{}, fmt.Errorf("bad o: %w", err)
	}
	if op < 0 || op > 100 {
		return "", OverlayConfig{}, fmt.Errorf("opacity must be 0-100")
	}
	c.Opacity = float64(op) / 100.0
	color := strings.ToUpper(tags["c"])
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}
	c.Color = color
	c.Enabled = parse01(tags["e"])
	c.Calibration = parse01(tags["k"])
	c.CircleMode = parse01(tags["r"])
	// Missing H/V → show both (same as old configs).
	if _, ok := tags["H"]; ok {
		c.ShowHorizontal = parse01(tags["H"])
	} else {
		c.ShowHorizontal = true
	}
	if _, ok := tags["V"]; ok {
		c.ShowVertical = parse01(tags["V"])
	} else {
		c.ShowVertical = true
	}
	name = strings.TrimSpace(tags["n"])

	c, err = Validate(c)
	if err != nil {
		return "", OverlayConfig{}, err
	}
	return name, c, nil
}

func bool01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func parse01(s string) bool {
	return s == "1" || strings.EqualFold(s, "true")
}
