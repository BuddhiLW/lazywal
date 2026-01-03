# Lazywal - Multi-monitor Video Wallpaper Manager

[![GoDoc](https://godoc.org/github.com/BuddhiLW/lazywal?status.svg)](https://godoc.org/github.com/BuddhiLW/lazywal)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

Lazywal is a terminal client for setting up animated wallpapers on Linux systems with X11. Built with [Bonzai](https://github.com/rwxrob/bonzai) for native MCP (Model Context Protocol) support.

![showcase](./output.gif)

## Features

- Multi-monitor support with automatic positioning
- Video files and animated GIFs
- Pywal integration for system-wide color schemes
- **MCP-native**: Control wallpapers via AI assistants (Claude, etc.)

## Installation

### Using Go
```bash
go install github.com/BuddhiLW/lazywal/cmd/lazywal@latest
go install github.com/BuddhiLW/lazywal/cmd/lazywal-mcp@latest
```

### From Source
```bash
git clone https://github.com/BuddhiLW/lazywal
cd lazywal
make install PREFIX=$HOME/.local
```

### Dependencies

| Required | Optional |
|----------|----------|
| [mpv](https://github.com/mpv-player/mpv) | [pywal](https://github.com/dylanaraps/pywal) |
| [xwinwrap](https://github.com/ujjwal96/xwinwrap) | |
| [xrandr](https://www.x.org/wiki/Projects/XRandR/) | |

## CLI Usage

```bash
# Set wallpaper (auto-detects monitors)
lazywal set /path/to/video.mp4

# Set with specific display size
lazywal set /path/to/video.mp4 display 1920x1080

# Set and apply pywal colors
lazywal set /path/to/video.mp4 pywal

# Stop all wallpapers
lazywal clear

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
| `clear` | Kill all wallpaper processes | none |
| `pywal` | Apply pywal colors from wallpaper frame | none |

### Example Prompts

Once configured, you can ask Claude:
- *"Set my wallpaper to ~/Videos/ocean.mp4"*
- *"Update my terminal colors to match the wallpaper"*
- *"Clear my animated wallpaper"*

### State Persistence

The MCP server uses bonzai's `injson` persister to track wallpaper PIDs across calls. State is stored in `~/.local/state/lazywal/state.json`, ensuring previous wallpapers are properly killed when setting new ones.

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
