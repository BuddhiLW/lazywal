package wallpaper

import (
	"fmt"
	"strings"
)

// SessionType is the display server protocol of a graphical session.
type SessionType string

const (
	AnySession SessionType = ""
	X11        SessionType = "x11"
	Wayland    SessionType = "wayland"
)

// Session describes the graphical session lazywal runs in.
type Session struct {
	Type     SessionType
	Desktops []string // lower-cased XDG_CURRENT_DESKTOP entries, e.g. ["ubuntu", "gnome"]
	// Inferred is set when nothing declared the session: its type was guessed
	// from DISPLAY / WAYLAND_DISPLAY and no desktop is named. That is what a
	// cron job or systemd unit sees, even inside a desktop environment.
	Inferred bool
}

// Is reports whether any desktop entry equals one of names (lower-case).
func (s Session) Is(names ...string) bool {
	for _, d := range s.Desktops {
		for _, n := range names {
			if d == n {
				return true
			}
		}
	}
	return false
}

func (s Session) String() string {
	t := string(s.Type)
	if t == "" {
		t = "unknown"
	}
	d := strings.Join(s.Desktops, ":")
	if d == "" {
		d = "none"
	}
	return fmt.Sprintf("session=%s desktop=%s", t, d)
}

// DetectSession reads the session type and desktop from the environment.
func DetectSession(getenv func(string) string) Session {
	var s Session

	switch t := SessionType(strings.ToLower(getenv("XDG_SESSION_TYPE"))); t {
	case X11, Wayland:
		s.Type = t
	default: // unset, "tty", "unspecified"...
		if getenv("WAYLAND_DISPLAY") != "" {
			s.Type = Wayland
		} else if getenv("DISPLAY") != "" {
			s.Type = X11
		}
		s.Inferred = true
	}

	desktop := getenv("XDG_CURRENT_DESKTOP")
	if desktop == "" {
		desktop = getenv("DESKTOP_SESSION")
	}
	for _, d := range strings.Split(desktop, ":") {
		if d = strings.ToLower(strings.TrimSpace(d)); d != "" {
			s.Desktops = append(s.Desktops, d)
		}
	}
	// Compositors that don't always set XDG_CURRENT_DESKTOP.
	if getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" && !s.Is("hyprland") {
		s.Desktops = append(s.Desktops, "hyprland")
	}
	if getenv("SWAYSOCK") != "" && !s.Is("sway") {
		s.Desktops = append(s.Desktops, "sway")
	}
	s.Inferred = s.Inferred && len(s.Desktops) == 0
	return s
}
