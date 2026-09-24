// Package x11 holds the X11 wallpaper adapters: xwinwrap windows running mpv
// on every monitor, and a decorator that hides a desktop environment's icon
// layer while the video plays.
package x11

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/BuddhiLW/lazywal/internal/shell"
)

// Monitor is an active output: its size and position on the X screen.
type Monitor struct {
	Name          string
	Width, Height int
	X, Y          int
	Primary       bool
}

// MonitorSource lists the active monitors, so Xwinwrap can be tested (or fed
// another source) without xrandr.
type MonitorSource interface {
	Monitors() ([]Monitor, error)
}

// Xrandr lists monitors by parsing "xrandr --current", which reads the
// server's current configuration without re-probing outputs.
type Xrandr struct{ Runner shell.Runner }

func (x Xrandr) Monitors() ([]Monitor, error) {
	out, err := x.Runner.Output(shell.XrandrCurrent())
	if err != nil {
		return nil, fmt.Errorf("listing monitors: %w", err)
	}
	return ParseXrandr(out)
}

// ParseXrandr extracts the active monitors from "xrandr --current" output.
// Connected outputs without a mode (turned off) are skipped; it is an error
// when no monitor is active, since there is nowhere to play.
func ParseXrandr(out string) ([]Monitor, error) {
	var monitors []Monitor
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "connected" {
			continue
		}
		m := Monitor{Name: fields[0]}
		found := false
		for _, f := range fields[2:] {
			if f == "primary" {
				m.Primary = true
				continue
			}
			if strings.HasPrefix(f, "(") {
				break // rotation/reflection list; the geometry comes before it
			}
			if m.Width, m.Height, m.X, m.Y, found = parseGeometry(f); found {
				break
			}
		}
		if found {
			monitors = append(monitors, m)
		}
	}
	if len(monitors) == 0 {
		return nil, errors.New("no active monitors found")
	}
	return monitors, nil
}

// parseGeometry parses an X geometry such as "1920x1080+1920+0".
func parseGeometry(s string) (w, h, x, y int, ok bool) {
	size, pos, ok := strings.Cut(s, "+")
	if !ok {
		return 0, 0, 0, 0, false
	}
	ws, hs, ok := strings.Cut(size, "x")
	if !ok {
		return 0, 0, 0, 0, false
	}
	xs, ys, ok := strings.Cut(pos, "+")
	if !ok {
		return 0, 0, 0, 0, false
	}
	var errs [4]error
	w, errs[0] = strconv.Atoi(ws)
	h, errs[1] = strconv.Atoi(hs)
	x, errs[2] = strconv.Atoi(xs)
	y, errs[3] = strconv.Atoi(ys)
	if errors.Join(errs[:]...) != nil || w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	return w, h, x, y, true
}
