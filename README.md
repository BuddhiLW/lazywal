# Lazywal - Multi-monitor Video Wallpaper Manager

[![GoDoc](https://godoc.org/github.com/BuddhiLW/lazywal?status.svg)](https://godoc.org/github.com/BuddhiLW/lazywal)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

Lazywal is a terminal client for setting up animated wallpapers on Linux: X11 window managers, GNOME, KDE Plasma, MATE, Cinnamon, Xfce, LXQt/LXDE and Wayland compositors such as Hyprland and Sway. Built with [Bonzai](https://github.com/rwxrob/bonzai) for native MCP (Model Context Protocol) support.

![showcase](./output.gif)

## Features

- Works across X11 and Wayland: detects your session and picks the right backend
- Multi-monitor support with automatic positioning
- Video files and animated GIFs
- Pywal integration for system-wide color schemes
- **MCP-native**: Control wallpapers via AI assistants (Claude, etc.)

## Installation

### Using Go
```bash
go install github.com/BuddhiLW/lazywal@latest
go install github.com/BuddhiLW/lazywal/cmd/lazywal-mcp@latest
```

### From Source
```bash
git clone https://github.com/BuddhiLW/lazywal
cd lazywal
make install PREFIX=$HOME/.local
```

### Supported desktops

Lazywal detects the session from `XDG_SESSION_TYPE` / `XDG_CURRENT_DESKTOP` and picks the best ready backend. Run `lazywal backends` to see which one your session uses and everything each backend is missing.

| Backend | Desktops | Needs |
|---------|----------|-------|
| `xwinwrap` | X11 window managers: xmonad, i3, bspwm, awesome, openbox, dwm… | [xwinwrap](https://github.com/ujjwal96/xwinwrap), [mpv](https://mpv.io), xrandr |
| `mate` | MATE (X11): hides Caja's desktop icons while playing | same as `xwinwrap` |
| `cinnamon` | Cinnamon (X11): hides Nemo's desktop icons while playing | same as `xwinwrap` |
| `xfce` | Xfce (X11): stops `xfdesktop` while playing | same as `xwinwrap` |
| `lxqt`, `lxde` | LXQt / LXDE (X11): turns off the pcmanfm desktop while playing | same as `xwinwrap` |
| `x11-desktop` | Other X11 desktops with panels (Budgie, GNOME/KDE without their plugin…) | same as `xwinwrap` |
| `gnome` | GNOME, Wayland or X11 (incl. Ubuntu) | [Hanabi extension](https://github.com/jeffshee/gnome-ext-hanabi) |
| `plasma` | KDE Plasma 6, Wayland or X11 | [Smart Video Wallpaper Reborn](https://github.com/luisbocanegra/plasma-smart-video-wallpaper-reborn) plugin, `dbus-send` |
| `mpvpaper` | Wayland layer-shell compositors: Hyprland, Sway, river, niri, Wayfire, labwc, COSMIC, KDE… | [mpvpaper](https://github.com/GhostNaN/mpvpaper) |

`lazywal clear` undoes what the backend changed (restores desktop icons, disables Hanabi, switches Plasma back to the image wallpaper).

Force a backend with `LAZYWAL_BACKEND`, e.g. `LAZYWAL_BACKEND=xwinwrap lazywal set video.mp4`.

Scheduled jobs (cron, systemd timers) usually only export `DISPLAY` or `WAYLAND_DISPLAY`. Lazywal then keeps the backend that set the current wallpaper instead of guessing; to choose explicitly, export `XDG_SESSION_TYPE` and `XDG_CURRENT_DESKTOP` in the job, or set `LAZYWAL_BACKEND`.

Optional, for `colors`/`pywal`: [pywal](https://github.com/dylanaraps/pywal), ffmpeg and ffprobe.

### Your own backends

Desktops lazywal doesn't know yet, or a different way to play video on one it does, can be added in `~/.config/lazywal/backends.json` (or `$XDG_CONFIG_HOME/lazywal/backends.json`), without changing lazywal:

```json
{
  "backends": [
    {
      "name": "hyprland-dp1",
      "description": "Hyprland: mpvpaper with sound, on DP-1 only",
      "rules": [{ "session": "wayland", "desktops": ["hyprland"], "priority": 200 }],
      "spawn": [["mpvpaper", "-o", "loop-file=inf panscan=1", "DP-1", "{{path}}"]]
    },
    {
      "name": "xwinwrap-hwdec",
      "description": "X11 window managers: xwinwrap + mpv with hardware decoding",
      "requires": ["mpv"],
      "spawn": [["xwinwrap", "-fs", "-ni", "-b", "-nf", "-un", "-ov", "--",
                 "mpv", "-wid", "WID", "--hwdec=auto", "--no-audio", "--loop-file=inf", "--", "{{path}}"]]
    }
  ]
}
```

- Commands are argument lists, never shell strings. `{{path}}` and `{{uri}}` expand inside a single argument, so paths with spaces or quotes need no quoting.
- `set` commands run to completion, in order, on every set (so they must be safe to repeat). `spawn` commands are long-running players that lazywal tracks and kills on the next `set` or `clear`. `clear` commands undo what `set` changed and must succeed when nothing is set.
- `requires` lists programs needed besides each command's own program (e.g. `mpv` when `xwinwrap` runs it).
- `rules` decide when the backend is picked: `session` (`x11`/`wayland`), `desktops` (any of, from `XDG_CURRENT_DESKTOP`, lower-case), `exclude`, and `priority` (built-ins use 0 to 100, higher wins). A backend without rules is only used through `LAZYWAL_BACKEND=<name>`.
- Reusing a built-in's name replaces that built-in.

## CLI Usage

```bash
# Set wallpaper (auto-detects monitors)
lazywal set /path/to/video.mp4

# Set and apply pywal colors
lazywal set /path/to/video.mp4 pywal

# Stop the wallpaper and restore the desktop
lazywal clear

# Show detected session, backends and missing dependencies
lazywal backends
lazywal test

# Show help
lazywal help
```

## MCP Server (AI Integration)

Lazywal includes an MCP server that allows AI assistants to control your wallpaper.

### Quick Setup for Claude Code

```bash
# Add to Claude Code
claude mcp add --scope user --transport stdio lazywal -- lazywal-mcp
```

### Manual Configuration

Add to `~/.claude.json` or Claude Desktop config:

```json
{
  "mcpServers": {
    "lazywal": {
      "command": "lazywal-mcp",
      "transport": "stdio"
    }
  }
}
```

### Available MCP Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `set` | Set video/GIF as wallpaper | `path` (required): absolute path to file |
| `clear` | Stop the wallpaper and restore the desktop | none |
| `backends` | List backends and which one the session uses | none |
| `pywal` | Apply pywal colors from wallpaper frame | none |

### Example Prompts

Once configured, you can ask Claude:
- *"Set my wallpaper to ~/Videos/ocean.mp4"*
- *"Update my terminal colors to match the wallpaper"*
- *"Clear my animated wallpaper"*

### State Persistence

The MCP server uses bonzai's `injson` persister to track wallpaper PIDs and the active backend across calls. State is stored in `~/.local/state/lazywal/state.json`, ensuring previous wallpapers are properly killed when setting new ones.

## Autocompletion

```bash
# Add to ~/.bashrc
complete -C lazywal lazywal
```

Or run:
```bash
./auto-completion.bash
```

## Installing Dependencies

### xwinwrap
```bash
# With curl
curl -sSL https://gist.githubusercontent.com/BuddhiLW/5f43e75c81a56106d04cea6bbce0a238/raw/xwinwrap | bash

# With wget
wget -qO- https://gist.githubusercontent.com/BuddhiLW/5f43e75c81a56106d04cea6bbce0a238/raw/xwinwrap | bash
```

### GNOME: Hanabi
Hanabi is not on extensions.gnome.org; build it from [its repository](https://github.com/jeffshee/gnome-ext-hanabi#installation) (needs `meson`, plus `node` and `npm` for GNOME 50):
```bash
git clone https://github.com/jeffshee/gnome-ext-hanabi.git             # GNOME 50 and later
# git clone https://github.com/jeffshee/gnome-ext-hanabi.git -b javascript  # GNOME 45 to 49
cd gnome-ext-hanabi && make install
```
Then log out and back in (Wayland needs a new session to load extensions). `lazywal set` enables it; `lazywal clear` puts back the video, slideshow setting and enabled state Hanabi had before.

### KDE Plasma: Smart Video Wallpaper Reborn
Desktop right-click → **Desktop and Wallpaper** → **Get New Plugins** → search "Smart Video Wallpaper Reborn".

### Hyprland, Sway & other wlroots compositors: mpvpaper
```bash
yay -S mpvpaper   # Arch (AUR); other distros: build from https://github.com/GhostNaN/mpvpaper
```

### ffmpeg, mpv
```bash
# macOS
brew install ffmpeg mpv

# Debian/Ubuntu
sudo apt-get install ffmpeg mpv

# Arch
sudo pacman -S ffmpeg mpv
```

### pywal (optional)
```bash
pip install pywal
```

## Contributing

Pull requests welcome! For major changes, please open an issue first.

## License

[MIT License](LICENSE)

## Credits

- Original lazywal-cli by [Zayac-The-Engineer](https://github.com/Zayac-The-Engineer)
- Go rewrite by [Pedro G. Branquinho](https://github.com/BuddhiLW)
- Built with [Bonzai](https://github.com/rwxrob/bonzai) CLI framework
