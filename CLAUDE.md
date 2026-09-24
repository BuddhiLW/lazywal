# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build the binaries (stamps the version from VERSION via -ldflags)
make build

# Install locally
make install PREFIX=$HOME/.local

# Run tests
go test ./...

# Run a single package's tests
go test ./internal/backend/x11 -run TestParseXrandr -v
```

## Architecture Overview

Lazywal sets animated video wallpapers on Linux: X11 window managers, X11 desktops, GNOME, KDE Plasma and Wayland compositors. It follows ports & adapters (DDD): the domain knows nothing about specific programs; adapters implement its ports.

```
cmd/lazywal, main.go     entry points: loop.Cmd.Exec()  (root main.go lets `go install github.com/BuddhiLW/lazywal@latest` work)
cmd/lazywal-mcp          MCP server generated from the same bonzai command tree
loop/                    CLI layer (bonzai commands) + composition root (app.go: newApp)
internal/wallpaper/      domain + application service
internal/shell/          infrastructure: Cmd value object, typed command builders, Runner port, Tracker
internal/backend/...     adapters implementing wallpaper.Backend
internal/colors/         pywal colors (ffprobe -> ffmpeg frame -> wal)
```

**CLI** (`loop/`): [bonzai](https://github.com/BuddhiLW/bonzai) commands (`set`, `clear`, `pywal`, `backends`, `test`) with MCP metadata. Commands call `newApp()` (the composition root) instead of using package state, so nothing touches the system until a command runs. `loop.Version` is stamped with `-ldflags`, falling back to Go build info.

### Domain (`internal/wallpaper`)
- `Video` value object (validated absolute path, `URI()`), `Session` + `DetectSession(getenv)`.
- `Backend` port: `Name`, `Describe`, `Check`, `Set(Video)`, `Clear`. Its behavioural contract is documented on the interface and enforced for every implementation by `backendtest.Contract` (Liskov): `Check` is side-effect free and returns a `*MissingDepsError` listing *every* missing dependency (build it with `wallpaper.Require(runner.LookPath)`); `Set` replaces without a prior `Clear`; `Clear` is idempotent and succeeds when nothing is set.
- `Rule` / `Registry`: backends register with data rules (session, desktops, exclude, priority). `Candidates` orders matches by priority; `Select` returns the first ready one, or the best one with a `*NotReadyError`. Selection code never changes when a backend is added (Open-Closed).
- `Service`: application service. Selects the backend, clears a previously active different backend and any backend a same-name override shadowed (`Registry.Shadowed`), records the active backend *before* `Set` (so a partial failure can still be cleared), and remembers the video for pywal. When the session is only inferred from `DISPLAY` (cron), it keeps the recorded backend. `State` port = bonzai's injson persister (`~/.local/state/lazywal/state.json`); `loop/app.go` adopts v1.4.3 state and keeps removed custom backends reachable (`custom.Retired`).

### Infrastructure (`internal/shell`)
- Never build shell strings or use `bash -c`. Every program is a `shell.Cmd` built by a typed builder (`Mpv()`, `Xwinwrap()`, `Mpvpaper()`, ... or small builder funcs inside an adapter package).
- mpv's `-wid` must stay single-dash: xwinwrap only replaces an argument that is exactly `WID`, and mpv rejects `--wid <value>`.
- `Runner` port (`Exec` is the real one) runs, starts detached (own process group), kills groups, and looks up programs. Only `Start` is logged.
- `Tracker` records each spawned player with a process identity that survives exec (boot id + start time from `/proc/<pid>/stat`) and only kills a PID whose identity still matches and that leads its own group. Bare PIDs written by v1.4.3 are killed only when `/proc/<pid>/comm` is in `Tracker.Legacy`.

### Backends (`internal/backend`)
- `x11`: `Xwinwrap` (one xwinwrap+mpv per xrandr monitor) with a `WindowMode` strategy: `Unmanaged` (`-ov`, for bare WMs like xmonad) or `DesktopWindow` (`-fdt`, stays under DE panels). `Desktop` decorator wraps any backend with a `Layer` strategy that hides the DE's desktop-icons window on every `Set` and restores the state captured first on `Clear`. MATE/Cinnamon use a gsettings key (per-user, valid across sessions); Xfce/LXQt/LXDE stop the desktop process and restore its exact captured argv, but only in the session that captured it (state is session-bound, so a re-login re-hides and a cron clear from another session never starts xfdesktop there).
- `gnome`: Hanabi extension via gsettings + gnome-extensions; `Set` captures Hanabi's previous video, slideshow and enabled state once, `Clear` puts them back. `plasma`: Smart Video Wallpaper Reborn via plasmashell `evaluateScript` over dbus-send; `Set` snapshots each desktop's plugin (and an existing playlist) once, `Clear` restores it. `wlr`: mpvpaper.
- `custom`: user backends from `~/.config/lazywal/backends.json` (argv templates with `{{path}}`/`{{uri}}`, rules, priority); a custom backend with a built-in's name replaces it.
- `builtin`: the only place built-ins and their rules are wired.

### Testing
- Tests never touch the real desktop, real processes or `~/.local/state`: use `shelltest.New()` (fake Runner: scripted `Outputs`/`Errors`/`Missing`, `Effects` for side effects, records `Calls`/`Started`/`Killed`) and `shelltest.NewStore()`.
- Every backend runs `backendtest.ContractWithRunner` (the contract plus effect checks on the fake: `Check` starts/kills nothing, a second `Set` and a `Clear` stop every player `Set` started) and a test that `Check` lists all missing dependencies.
- CLI tests replace `newApp` with an in-memory app. `TestCmdTreeValid` catches bonzai developer errors (e.g. `Short` over 49 runes).
- Unit tests assert argv, not how the real tools react to it; changes to player flags should also be checked end to end (xwinwrap under Xvfb, mpvpaper under headless sway).
