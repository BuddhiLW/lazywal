package wallpaper_test

import (
	"reflect"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

func envMap(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestDetectSession(t *testing.T) {
	x11, wayland := wallpaper.X11, wallpaper.Wayland
	tests := []struct {
		name string
		env  map[string]string
		want wallpaper.Session
	}{
		{"ubuntu gnome wayland",
			map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "ubuntu:GNOME", "WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"},
			wallpaper.Session{Type: wayland, Desktops: []string{"ubuntu", "gnome"}}},
		{"xmonad from startx",
			map[string]string{"DISPLAY": ":0"},
			wallpaper.Session{Type: x11, Inferred: true}},
		{"hyprland without XDG_CURRENT_DESKTOP",
			map[string]string{"WAYLAND_DISPLAY": "wayland-1", "HYPRLAND_INSTANCE_SIGNATURE": "abc"},
			wallpaper.Session{Type: wayland, Desktops: []string{"hyprland"}}},
		{"sway via SWAYSOCK",
			map[string]string{"XDG_SESSION_TYPE": "wayland", "SWAYSOCK": "/run/user/1000/sway.sock"},
			wallpaper.Session{Type: wayland, Desktops: []string{"sway"}}},
		{"mate via DESKTOP_SESSION",
			map[string]string{"XDG_SESSION_TYPE": "x11", "DESKTOP_SESSION": "mate"},
			wallpaper.Session{Type: x11, Desktops: []string{"mate"}}},
		{"tty session type falls back to display vars",
			map[string]string{"XDG_SESSION_TYPE": "tty", "DISPLAY": ":1"},
			wallpaper.Session{Type: x11, Inferred: true}},
		{"budgie",
			map[string]string{"XDG_SESSION_TYPE": "x11", "XDG_CURRENT_DESKTOP": "Budgie:GNOME"},
			wallpaper.Session{Type: x11, Desktops: []string{"budgie", "gnome"}}},
		{"kde",
			map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "KDE"},
			wallpaper.Session{Type: wayland, Desktops: []string{"kde"}}},
		{"cinnamon",
			map[string]string{"XDG_SESSION_TYPE": "x11", "XDG_CURRENT_DESKTOP": "X-Cinnamon"},
			wallpaper.Session{Type: x11, Desktops: []string{"x-cinnamon"}}},
		{"nothing set",
			map[string]string{},
			wallpaper.Session{Inferred: true}},
		{"session type is case-insensitive",
			map[string]string{"XDG_SESSION_TYPE": "Wayland"},
			wallpaper.Session{Type: wayland}},
		{"explicit session type beats display vars",
			map[string]string{"XDG_SESSION_TYPE": "x11", "WAYLAND_DISPLAY": "wayland-0"},
			wallpaper.Session{Type: x11}},
		{"wayland display beats xwayland DISPLAY",
			map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"},
			wallpaper.Session{Type: wayland, Inferred: true}},
		{"unspecified session type falls back to display vars",
			map[string]string{"XDG_SESSION_TYPE": "unspecified", "WAYLAND_DISPLAY": "wayland-0"},
			wallpaper.Session{Type: wayland, Inferred: true}},
		{"XDG_CURRENT_DESKTOP beats DESKTOP_SESSION",
			map[string]string{"XDG_CURRENT_DESKTOP": "XFCE", "DESKTOP_SESSION": "mate"},
			wallpaper.Session{Desktops: []string{"xfce"}}},
		{"desktop entries are trimmed and empty ones dropped",
			map[string]string{"XDG_CURRENT_DESKTOP": " ubuntu :: GNOME :"},
			wallpaper.Session{Desktops: []string{"ubuntu", "gnome"}}},
		{"hyprland is not duplicated",
			map[string]string{"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "Hyprland", "HYPRLAND_INSTANCE_SIGNATURE": "abc"},
			wallpaper.Session{Type: wayland, Desktops: []string{"hyprland"}}},
		{"sway is not duplicated",
			map[string]string{"XDG_CURRENT_DESKTOP": "sway", "SWAYSOCK": "/s.sock"},
			wallpaper.Session{Desktops: []string{"sway"}}},
		{"sway appended after other desktops",
			map[string]string{"XDG_CURRENT_DESKTOP": "wlroots", "SWAYSOCK": "/s.sock"},
			wallpaper.Session{Desktops: []string{"wlroots", "sway"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wallpaper.DetectSession(envMap(tt.env)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DetectSession() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSessionIs(t *testing.T) {
	s := wallpaper.Session{Type: wallpaper.X11, Desktops: []string{"ubuntu", "gnome"}}
	tests := []struct {
		names []string
		want  bool
	}{
		{[]string{"gnome"}, true},
		{[]string{"kde", "ubuntu"}, true},
		{[]string{"kde"}, false},
		{[]string{"GNOME"}, false}, // callers pass lower-case names
		{nil, false},
	}
	for _, tt := range tests {
		if got := s.Is(tt.names...); got != tt.want {
			t.Errorf("Is(%q) = %v, want %v", tt.names, got, tt.want)
		}
	}
	if (wallpaper.Session{}).Is("gnome") {
		t.Error("empty session Is(gnome) = true")
	}
}

func TestSessionString(t *testing.T) {
	tests := []struct {
		s    wallpaper.Session
		want string
	}{
		{wallpaper.Session{Type: wallpaper.X11, Desktops: []string{"ubuntu", "gnome"}}, "session=x11 desktop=ubuntu:gnome"},
		{wallpaper.Session{Type: wallpaper.Wayland}, "session=wayland desktop=none"},
		{wallpaper.Session{}, "session=unknown desktop=none"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}
