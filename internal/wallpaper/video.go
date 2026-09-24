// Package wallpaper is lazywal's domain: the video to play, the session it
// plays in, the Backend port that puts it on the desktop, the rules that pick
// a backend, and the Service that ties them together. It knows nothing about
// specific programs; those live in adapter packages under internal/backend.
package wallpaper

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// Video is a validated, absolute path to a playable file.
type Video struct{ path string }

// NewVideo validates path and makes it absolute (backends hand it to other
// processes with other working directories).
func NewVideo(path string) (Video, error) {
	if path == "" {
		return Video{}, errors.New("no video path given")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Video{}, fmt.Errorf("resolving %s: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Video{}, fmt.Errorf("invalid path: %w", err)
	}
	if info.IsDir() {
		return Video{}, fmt.Errorf("invalid path: %s is a directory", abs)
	}
	return Video{path: abs}, nil
}

// Path is the absolute file path.
func (v Video) Path() string { return v.path }

// URI is the file:// URL of the video, percent-encoded.
func (v Video) URI() string { return (&url.URL{Scheme: "file", Path: v.path}).String() }

// IsZero reports whether v is the zero Video (no path).
func (v Video) IsZero() bool { return v.path == "" }

func (v Video) String() string { return v.path }
