# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build the binary
go build -o lazywal ./cmd/lazywal/main.go

# Install locally (requires dependencies: mpv, xwinwrap, ffmpeg)
make install PREFIX=$HOME/.local

# Run tests
go test ./...

# Run a single test
go test ./loop -run TestGetMonitors -v
```

## Architecture Overview

Lazywal is a CLI tool for setting animated video wallpapers on Linux X11 systems with multi-monitor support.

### Core Components

**Entry Point** (`cmd/lazywal/main.go`): Minimal entry that calls `loop.Cmd.Run()`.

**CLI Framework**: Uses [rwxrob/bonzai](https://github.com/rwxrob/bonzai) - a composable CLI framework. Commands are defined as `*Z.Cmd` structs with `Call` functions. The root command is `loop.Cmd` in `loop/cmd.go`.

**Command Structure** (`loop/cmd.go`):
- `Cmd` - Root command with subcommands: `set`, `clear/kill`, `pywal`, `test/dependencies`
- Uses `rwxrob/conf` for config and `rwxrob/vars` for persistent variable storage

**Wallpaper Management** (`loop/loop.go`):
- `Wallpaper` struct manages running processes per monitor
- `Config` holds path and dimensions
- PIDs are tracked in bonzai vars (`lazywal_pids`, `lazywal_monitor_pids_<name>`)
- Uses `xwinwrap` + `mpv` for rendering videos on X11 root window

**Monitor Detection** (`loop/helpers.go`):
- `GetMonitors()` parses `xrandr --current` output to detect connected displays
- Returns `[]Monitor` with name, dimensions, position, and primary status

**Pywal Integration** (`loop/pywal.go`):
- Extracts random frame from video using `ffmpeg`
- Passes frame to `wal -n -i` for color scheme generation

### Key Types

```go
type Monitor struct {
    Name       string
    Dimensions Size      // Width, Height
    Position   Position  // X, Y offset
    Primary    bool
}

type Wallpaper struct {
    Config   *Config
    Running  map[string]*exec.Cmd  // monitor name -> process
    Monitors []Monitor
}
```

### External Dependencies

Required binaries (checked by `internal/check/dependencies.go`):
- `xwinwrap` - Creates window on desktop background
- `mpv` - Video player
- `ffmpeg` - Frame extraction for pywal

Optional:
- `wal` (pywal) - Color scheme generation

### Testing

Tests use `execCommand` variable injection to mock shell commands. See `loop/test_helpers.go` for `testMockCommand()`.
