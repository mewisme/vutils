# Valorant Utils (vutils)

Standalone Windows overlay for aligning a minimap guide — crosshair lines over a region you place yourself.

Draws only on your screen at coordinates you set. **No** Valorant / Riot / Vanguard process access, memory reads, or injection.

## Features

- **Minimap crosshair** — horizontal and vertical guides through the region center; toggle each arm independently
- **Live overlay** — always-on-top, click-through; opacity and thickness you control
- **Calibration mode** — rectangle or circle border to match the in-game minimap box
- **Position & size** — D-pad nudge with configurable step; W/H resize keeps the crosshair center fixed; hold buttons to repeat
- **Style** — hex color, color picker, thickness, opacity slider; changes apply live
- **Profiles** — multiple named setups in one config; switch from the dropdown; Profile menu for New / Duplicate / Rename / Delete / Export / Import (`default` is protected)
- **Share codes** — export/import a profile as a compact `vutils;…` string
- **Persisted config** — save/reset; auto-created at `~/.vutils/config.json` on first run; old flat configs migrate automatically
- **Updates** — Help → Check for Updates against GitHub releases

## Install

**Scoop** (adds a Start Menu shortcut **Valorant Utils**):

```powershell
scoop bucket add mew https://github.com/mewisme/scoop-mew
scoop install mew/vutils
```

Or from the latest release:

```powershell
scoop install https://github.com/mewisme/vutils/releases/latest/download/vutils.json
```

**From source** (Windows):

```bat
build.bat
```

Or manually:

```bash
go run github.com/akavel/rsrc@v0.10.2 -ico icon.ico -arch amd64 -o rsrc_windows_amd64.syso
go build -ldflags="-H windowsgui -X github.com/mewisme/vutils/internal/version.Version=dev" -o vutils.exe .
```

Requires Go 1.26+. Output: `build\bin\vutils.exe` when using `build.bat`.

## Quick start

1. Run Valorant **Windowed Borderless** with minimap **Keep Player Centered**.
2. Open vutils → enable **Overlay** + **Calibration**.
3. Nudge and resize until the border matches your minimap.
4. Set color / opacity / thickness; turn off calibration when aligned.
5. **Save**. Optional: create more profiles (e.g. per resolution) via **Profile → New / Duplicate**.

## Config

Path: `~/.vutils/config.json`

```json
{
  "activeProfile": "default",
  "profiles": {
    "default": {
      "mapX": 30,
      "mapY": 30,
      "mapW": 300,
      "mapH": 300,
      "step": 5,
      "thickness": 1,
      "opacity": 0.6,
      "color": "#00FFFF",
      "enabled": false,
      "calibration": false,
      "circleMode": false,
      "showHorizontal": true,
      "showVertical": true
    }
  }
}
```

A pre-profiles flat file is rewritten into this shape on first launch after upgrade.

## Share codes

**Profile → Export Code…** copies a shareable string for the active profile. **Profile → Import Code…** pastes one and creates a new profile.

Example:

```
vutils;1;n;ranked;x;30;y;30;w;300;h;300;s;5;t;1;o;60;c;00FFFF;e;0;k;0;r;0;H;1;V;1
```

Tags: `n` name hint, `x/y/w/h` rect, `s` step, `t` thickness, `o` opacity 0–100, `c` RRGGBB, `e/k/r` enabled/calibration/circle, `H/V` crosshair arms.

## License

MIT © 2026 Mew — use at your own risk.
