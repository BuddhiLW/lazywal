package wallpaper_test

import (
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

func touch(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewVideo(t *testing.T) {
	path := touch(t, "my video.mp4")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || filepath.IsAbs(rel) {
		t.Fatalf("cannot make %s relative to %s: %q, %v", path, cwd, rel, err)
	}
	dir := filepath.Dir(path)

	tests := []struct {
		name string
		in   string
	}{
		{"absolute", path},
		{"relative becomes absolute", rel},
		{"dot segments are cleaned", dir + "/no-such-dir/.././my video.mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := wallpaper.NewVideo(tt.in)
			if err != nil {
				t.Fatalf("NewVideo(%q) = %v", tt.in, err)
			}
			if v.Path() != path || v.String() != path || v.IsZero() {
				t.Errorf("NewVideo(%q) = %q (zero %v), want %q", tt.in, v.Path(), v.IsZero(), path)
			}
		})
	}
}

func TestNewVideoRejects(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		in      string
		wantMsg string
		wantIs  error
	}{
		{"empty", "", "no video path", nil},
		{"missing file", filepath.Join(dir, "nope.mp4"), "invalid path", fs.ErrNotExist},
		{"directory", dir, "is a directory", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := wallpaper.NewVideo(tt.in)
			if err == nil || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Fatalf("NewVideo(%q) = %v, want error containing %q", tt.in, err, tt.wantMsg)
			}
			if tt.wantIs != nil && !errors.Is(err, tt.wantIs) {
				t.Errorf("NewVideo(%q) = %v, want errors.Is %v", tt.in, err, tt.wantIs)
			}
			if !v.IsZero() {
				t.Errorf("NewVideo(%q) returned %q with an error", tt.in, v)
			}
		})
	}
}

func TestVideoZero(t *testing.T) {
	var v wallpaper.Video
	if !v.IsZero() || v.Path() != "" {
		t.Errorf("zero Video = %q, IsZero %v", v.Path(), v.IsZero())
	}
}

func TestVideoURI(t *testing.T) {
	tests := []struct {
		file string
		want string // escaped base name
	}{
		{"plain.mp4", "plain.mp4"},
		{"a b.mp4", "a%20b.mp4"},
		{"a#b.mp4", "a%23b.mp4"},
		{"a?b%c.mp4", "a%3Fb%25c.mp4"},
		{`it's a "video".mp4`, "it%27s%20a%20%22video%22.mp4"},
		{"vídeo.mp4", "v%C3%ADdeo.mp4"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			v, err := wallpaper.NewVideo(touch(t, tt.file))
			if err != nil {
				t.Fatal(err)
			}
			uri := v.URI()
			if !strings.HasPrefix(uri, "file:///") || !strings.HasSuffix(uri, "/"+tt.want) {
				t.Errorf("URI() = %s, want file:///.../%s", uri, tt.want)
			}
			u, err := url.Parse(uri)
			if err != nil {
				t.Fatalf("URI() %s does not parse: %v", uri, err)
			}
			if u.Scheme != "file" || u.Host != "" || u.Path != v.Path() || u.RawQuery != "" || u.Fragment != "" {
				t.Errorf("URI() %s parses to %+v, want path %q", uri, u, v.Path())
			}
		})
	}
}

func TestBackendtestVideo(t *testing.T) {
	v := backendtest.Video(t)
	if !filepath.IsAbs(v.Path()) || !strings.ContainsAny(filepath.Base(v.Path()), `'" `) {
		t.Errorf("backendtest.Video = %q, want an absolute path with spaces and quotes", v.Path())
	}
	if _, err := os.Stat(v.Path()); err != nil {
		t.Errorf("backendtest.Video file: %v", err)
	}
}
