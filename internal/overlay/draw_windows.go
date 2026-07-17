//go:build windows

package overlay

import (
	"strconv"
	"strings"

	"github.com/mewisme/vutils/internal/config"
)

// drawGuides writes premultiplied BGRA pixels into a top-down 32-bit DIB.
func drawGuides(pix []uint32, vw, vh, vx, vy int, cfg config.OverlayConfig) {
	x := cfg.MapX - vx
	y := cfg.MapY - vy
	w := cfg.MapW
	h := cfg.MapH
	if w < 1 || h < 1 {
		return
	}

	r, g, b := parseHexColor(cfg.Color)
	thickness := cfg.Thickness
	if thickness < 1 {
		thickness = 1
	}

	cx := x + w/2
	cy := y + h/2

	if cfg.ShowHorizontal {
		fillRectPixels(pix, vw, vh, x, cy-thickness/2, w, thickness, r, g, b)
	}
	if cfg.ShowVertical {
		fillRectPixels(pix, vw, vh, cx-thickness/2, y, thickness, h, r, g, b)
	}

	if !cfg.Calibration {
		return
	}
	t := thickness
	if t < 1 {
		t = 1
	}
	if cfg.CircleMode {
		strokeEllipse(pix, vw, vh, x, y, w, h, t, r, g, b)
		return
	}
	// Rectangle border: top, bottom, left, right.
	fillRectPixels(pix, vw, vh, x, y, w, t, r, g, b)
	fillRectPixels(pix, vw, vh, x, y+h-t, w, t, r, g, b)
	fillRectPixels(pix, vw, vh, x, y, t, h, r, g, b)
	fillRectPixels(pix, vw, vh, x+w-t, y, t, h, r, g, b)
}

// strokeEllipse draws an elliptical ring inside the minimap rect (calibration).
func strokeEllipse(pix []uint32, vw, vh, x, y, w, h, thickness int, r, g, b uint32) {
	cx := float64(x) + float64(w)/2
	cy := float64(y) + float64(h)/2
	rx := float64(w) / 2
	ry := float64(h) / 2
	if rx < 1 {
		rx = 1
	}
	if ry < 1 {
		ry = 1
	}
	t := float64(thickness)
	inRx := rx - t
	inRy := ry - t

	x0 := x
	y0 := y
	x1 := x + w
	y1 := y + h
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > vw {
		x1 = vw
	}
	if y1 > vh {
		y1 = vh
	}

	pixel := (uint32(0xFF) << 24) | (r << 16) | (g << 8) | b
	for py := y0; py < y1; py++ {
		row := py * vw
		for px := x0; px < x1; px++ {
			dx := (float64(px) + 0.5 - cx) / rx
			dy := (float64(py) + 0.5 - cy) / ry
			if dx*dx+dy*dy > 1 {
				continue // outside outer ellipse
			}
			if inRx > 0 && inRy > 0 {
				idx := (float64(px) + 0.5 - cx) / inRx
				idy := (float64(py) + 0.5 - cy) / inRy
				if idx*idx+idy*idy <= 1 {
					continue // inside inner ellipse
				}
			}
			pix[row+px] = pixel
		}
	}
}

func fillRectPixels(pix []uint32, vw, vh, x, y, w, h int, r, g, b uint32) {
	if w < 1 || h < 1 {
		return
	}
	x2 := x + w
	y2 := y + h
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x2 > vw {
		x2 = vw
	}
	if y2 > vh {
		y2 = vh
	}
	if x >= x2 || y >= y2 {
		return
	}

	// Premultiplied BGRA (little-endian DIB): B | G<<8 | R<<16 | A<<24.
	pixel := (uint32(0xFF) << 24) | (r << 16) | (g << 8) | b
	for py := y; py < y2; py++ {
		row := py * vw
		for px := x; px < x2; px++ {
			pix[row+px] = pixel
		}
	}
}

func parseHexColor(s string) (r, g, b uint32) {
	s = strings.TrimPrefix(strings.ToUpper(s), "#")
	if len(s) != 6 {
		return 0, 255, 255
	}
	rv, err1 := strconv.ParseUint(s[0:2], 16, 8)
	gv, err2 := strconv.ParseUint(s[2:4], 16, 8)
	bv, err3 := strconv.ParseUint(s[4:6], 16, 8)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 255, 255
	}
	return uint32(rv), uint32(gv), uint32(bv)
}
