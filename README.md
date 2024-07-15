<!-- markdown-toc start - Don't edit this section. Run M-x markdown-toc-refresh-toc -->
**Table of Contents**

- [Lazywal (Go rewrite of lazywal-cli)](#lazywal-go-rewrite-of-lazywal-cli)
    - [Setup](#setup)
    - [Autocompletion](#autocompletion)
    - [Usage](#usage)
        - [Pywal](#pywal)
    - [Showcase](#showcase)

<!-- markdown-toc end -->
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

Universal install
```bash
make install PREFIX=$HOME/.local
```

Using Go
```bash
go install github.com/BuddhiLW/lazywal/cmd/lazywal@latest 
```

Arch Linux [AUR](https://aur.archlinux.org/packages/lazywal-cli/)
```bash
yay -S lazywal-cli
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
COMMANDS
       help              - display help similar to man page format
       set-path|path|set - Renders the file in <path> as wallpaper with specified display dimension.
       kill|clear        - Kill all process related with 'xwinwrap' that may be hanging.

DESCRIPTION
       Lazywal: a terminal client to help setup video-loops/gifs as background.

       You can use the following commands: 
       - lazywal set <path>               (Tries to get first-display screen-size automatically) 
       - lazywal set <path> display <WxH> (Width x Height - e.g. 1440x1080, 2560x1080 etc.) 
       - lazywal kill                     (Kills all _xwinwrap_ processes running.)

       Note: path should be the path to the video-loop file.

       See the README.md for more information and examples, or use *_command-tree_
       help* to see another man-page about the specific command-tree.
```

### Pywal

If you add `pywal` at the end of your command, then the `lazywal` binary will use `wal` command to update your X-server color-scheme.

``` bash
lazywal set /path/to/file display 1920x1080 pywal
```

## Showcase

![show-case](./output.gif)

