<!-- markdown-toc start - Don't edit this section. Run M-x markdown-toc-refresh-toc -->
**Table of Contents**

- [Lazywal - Multi-monitor Video Wallpaper Manager](#lazywal---multi-monitor-video-wallpaper-manager)
    - [Setup](#setup)
    - [Autocompletion](#autocompletion)
    - [Usage](#usage)
        - [Pywal](#pywal)
    - [MCP Server](#mcp-server)
    - [Showcase](#showcase)
- [Installing dependencies (possibilities)](#installing-dependencies-possibilities)
    - [xwinwrap](#xwinwrap)
    - [ffmpeg, mpv (with `brew` or `apt-get`)](#ffmpeg-mpv-with-brew-or-apt-get)

<!-- markdown-toc end -->

[![GoDoc](https://godoc.org/github.com/BuddhiLW/lazywal?status.svg)](https://godoc.org/github.com/BuddhiLW/lazywal)
[![License](https://img.shields.io/badge/license-MIT-brightgreen.svg)](LICENSE)

For more documentation and examples of integration with other systems (Editor, Window Manager etc.) see the wiki: [https://github.com/BuddhiLW/lazywal/wiki](https://github.com/BuddhiLW/lazywal/wiki).

## Showcase

![show-case](./output.gif)


# Lazywal - Multi-monitor Video Wallpaper Manager

Lazywal is a terminal client for setting up animated wallpapers on Linux systems with X11. It supports:
- Multi-monitor setups with correct positioning
- Automatic monitor detection
- Video files and animated GIFs
- Pywal integration for system-wide color schemes

## Installation

### From Releases
Download the appropriate binary for your system from the [releases page](https://github.com/BuddhiLW/lazywal/releases).

### Using Go
```bash
go install github.com/BuddhiLW/lazywal/cmd/lazywal@latest
```

### Dependencies
Required:
* [mpv](https://github.com/mpv-player/mpv) - Video playback
* [xwinwrap](https://github.com/ujjwal96/xwinwrap) - Window creation
* [xrandr](https://www.x.org/wiki/Projects/XRandR/) - Monitor detection

Optional:
* [pywal](https://github.com/dylanaraps/pywal) - Color scheme generation

## Usage

Basic usage:
```bash
# Auto-detect monitors and set wallpaper
lazywal set /path/to/video.mp4

# Set wallpaper with specific display size
lazywal set /path/to/video.mp4 display 1920x1080

# Set wallpaper and update color scheme
lazywal set /path/to/video.mp4 pywal

# Stop all wallpapers
lazywal kill
```

For more documentation and examples, see the [wiki](https://github.com/BuddhiLW/lazywal/wiki).

## Multi-monitor Support
Lazywal automatically detects your monitor configuration and positions wallpapers correctly on each screen. No manual configuration needed!

## MCP Server

Lazywal includes an MCP (Model Context Protocol) server that allows LLMs like Claude to control your wallpaper. This enables AI assistants to change your background based on context, mood, or requests.

### Installation

```bash
# Build and install both lazywal and lazywal-mcp
make install PREFIX=$HOME/.local
```

### Configuration

Add to your Claude Code MCP settings (`~/.claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "lazywal": {
      "command": "lazywal-mcp"
    }
  }
}
```

### Available Tools

| Tool | Description |
|------|-------------|
| `set_wallpaper` | Set a video/GIF as wallpaper. Requires `path` parameter. |
| `clear_wallpaper` | Stop all wallpaper processes |
| `list_monitors` | Get connected monitors with dimensions and positions |
| `get_status` | Get current wallpaper path, PIDs, and running state |
| `apply_pywal` | Extract frame from wallpaper and apply pywal colors |

### Example Usage

Once configured, you can ask Claude things like:
- *"Set my wallpaper to ~/Videos/ocean.mp4"*
- *"What monitors do I have connected?"*
- *"Update my terminal colors to match the wallpaper"*
- *"Clear my wallpaper"*

## Contributing
Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

## License
[MIT License](LICENSE)


# Lazywal (Go rewrite of lazywal-cli)

Lazywal: a terminal client to setup animated (video-loop) wallpapers as the desktop background. It has some extra-features, like pywal native integration, for the flashy - like me.

Compatible with any OS that uses X-server.

<!-- This is a minimalistic animated wallpaper manager for Linux and probably BSD. Was created for me to see if I could do it and to learn how to create AUR packages. -->
## Setup

Dependencies:
* [mpv](https://github.com/mpv-player/mpv)
* [xwinwrap](https://github.com/ujjwal96/xwinwrap) ([aur](https://aur.archlinux.org/packages/xwinwrap-git/))
- [ffmpeg](https://ffmpeg.org/download.html)

(Probably your favorite OS will have these in the package-manager listings)

Optional -- native integration:
* [wal](https://github.com/dylanaraps/pywal/wiki/Installation)

See [Installing dependencies (possibilities)](#installing-dependencies-possibilities) for Linux and MacOS.


Universal install
```bash
make install PREFIX=$HOME/.local
```

Using Go
```bash
go install github.com/BuddhiLW/lazywal/cmd/lazywal@latest 
```


# Installing dependencies (possibilities)
## xwinwrap

This one-liner will install xwinwrap either with `curl` or `wget` (but not both).

Uses this [gist](https://gist.github.com/BuddhiLW/5f43e75c81a56106d04cea6bbce0a238).

With `curl`:
``` bash
curl -sSL https://gist.githubusercontent.com/BuddhiLW/5f43e75c81a56106d04cea6bbce0a238/raw/1aedf2fedbbe89d2d00e56560a950a8af4bca111/xwinwrap | bash
```

With `wget`:
``` bash
wget -qO- https://gist.githubusercontent.com/BuddhiLW/5f43e75c81a56106d04cea6bbce0a238/raw/1aedf2fedbbe89d2d00e56560a950a8af4bca111/xwinwrap | bash
```

## ffmpeg, mpv (with `brew` or `apt-get`)

``` bash
brew install ffmpeg mpv
```

``` bash
sudo apt-get install ffmpeg mpv
```

## Pywal (with pip) -- Optional

``` bash
pip install pywal
```


## Autocompletion

To add auto-completion, you have to add `complete -C lazywal lazywal` at `.bashrc`.

The following script adds it, if it doesn't find the comment "<< lazywal autocompletion <<<<".

```bash
bash ./auto-completion.bash
```

Runs:
```bash
#!/bin/bash

# Define the comment and the lines to add
COMMENT="# << lazywal autocompletion <<<<"
LINE1="complete -C lazywal lazywal"

# Check if the comment already exists in .bashrc
if ! grep -Fxq "$COMMENT" ~/.bashrc; then
    # If the comment does not exist, add the comment and the autocompletion line
    echo "$COMMENT" >> ~/.bashrc
    echo "$LINE1" >> ~/.bashrc
else
    echo "Auto-completion already added in ~/.bashrc"
fi
```

## Usage

For help
```bash
lazywal help
```

``` text
NAME
       lazywal - Lazywal: a terminal client to facilitate setting up video-loops/gifs as background.

SYNOPSIS
       lazywal COMMAND

COMMANDS
       help                      - display help similar to man page format
       conf                      - manage conf in /home/jacobi/.config/lazywal/config.yaml
       var                       - cache variables in /home/jacobi/.cache/lazywal/vars
       set-path|path|set         - Renders the file in <path> as wallpaper with specified display dimension.
       kill|clear                - Kill all process related with 'xwinwrap' that may be hanging.
       update-pywal|colors|pywal - Update pywal scheme to use a random frame from the loop.
       dependencies|test         - Test to see if all dependencies are available in your system.

DESCRIPTION
       Lazywal: a terminal client to help setup video-loops/gifs as background.

       You can use the following commands:

       * lazywal set path            (Tries to get first-display screen-size automatically)
       * lazywal set path display WxH  (Width x Height - e.g. 1440x1080, 2560x1080 etc.)
       * lazywal kill                  (Kills all _xwinwrap_ processes running.)

       Note: path should be the path to the video-loop file.

       See the README.md for more information and examples, or use *_command-tree_ help* to see another man-page about the specific command-tree.

CONTACT
       Site:   buddhilw.com
       Source: git@github.com/BuddhiLW/lazywal.git
       Issues: github.com/BuddhiLW/lazywal/issues

LEGAL
       lazywal (v1.0.3) Copyright 2021-2024 Zayac-The-Engineer, 2024 Pedro G. Branquinho (Go version)
       License MIT License
```

### Pywal

If you add `pywal` at the end of your command, then the `lazywal` binary will use `wal` command to update your X-server color-scheme.

``` bash
lazywal set /path/to/file display 1920x1080 pywal
```



