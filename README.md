# Valorant utils

Standalone Windows overlay for aligning a minimap guide — crosshair lines over a region you place yourself.

## Features

- **Minimap crosshair** — horizontal and vertical guides through the region center; toggle each arm independently
- **Live overlay** — always-on-top, click-through, opacity and thickness you control
- **Calibration mode** — rectangle or circle border so you can match the in-game minimap box
- **Position & size** — D-pad nudge with configurable step; W/H resize keeps the crosshair center fixed
- **Style** — hex color, color picker, thickness, opacity slider; changes apply live
- **Persisted config** — save/reset; auto-created at `~/.vutils/config.json` on first run
- **Updates** — Help → Check for Updates against GitHub releases

Standalone only: draws on your screen at coordinates you set. No game process access.

## Install

**Scoop:**

```powershell
scoop bucket add mew https://github.com/mewisme/scoop-mew
scoop install mew/vutils
```

Or from the latest release:

```powershell
scoop install https://github.com/mewisme/vutils/releases/latest/download/vutils.json
```

**From source:**

```bash
# Windows: build.bat embeds icon.ico then builds
build.bat

# or manually:
go run github.com/akavel/rsrc@v0.10.2 -ico icon.ico -arch amd64 -o rsrc_windows_amd64.syso
go build -ldflags="-H windowsgui -X github.com/mewisme/vutils/internal/version.Version=dev" -o vutils.exe .
```

## Quick start

1. Run Valorant **Windowed Borderless** with minimap **Keep Player Centered**.
2. Open vutils → enable **Overlay** + **Calibration**.
3. Nudge and resize until the border matches your minimap.
4. Set color / opacity / thickness; turn off calibration when aligned.
5. **Save**.

## License

MIT — use at your own risk.
