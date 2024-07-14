# lazywal-cli

This is a minimalistic animated wallpaper manager for Linux and probably BSD. Was created for me to see if I could do it and to learn how to create AUR packages.
## Setup

Dependencies:
* [mpv](https://github.com/mpv-player/mpv)
* [xwinwrap](https://github.com/ujjwal96/xwinwrap) ([aur](https://aur.archlinux.org/packages/xwinwrap-git/))

Optional -- native integration:
* [wal](https://github.com/dylanaraps/pywal/wiki/Installation)

Universal install
```bash
make install-go
```
Arch Linux [AUR](https://aur.archlinux.org/packages/lazywal-cli/)
```bash
yay -S lazywal-cli
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

If you add `pywal` at the end of your command, then the `lazywal` binary will use `wal` command to update you X-server color-scheme.

``` bash
lazywal set /path/to/file display 1920x1080 pywal
```

## Showcase

![show-case](./output.gif)

## Tested DEs, WMs

#### Works:
* GNOME 3 (mutter)
* Openbox
* XFCE
* ratpoison
* DWM
* xmonad

#### Works, but has weird quirks:
* BSPWM
* i3

#### Doesn't work (for now):
TODO: test more

