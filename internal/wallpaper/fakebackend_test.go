package wallpaper_test

import (
	"testing"

	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

// fakeBackend records what it is asked to do instead of touching a desktop.
type fakeBackend struct {
	name     string
	checkErr error
	setErr   error
	clearErr error

	checks int
	sets   []wallpaper.Video
	clears int
	// journal, when set, is shared between backends to check call order.
	journal *[]string
}

func (f *fakeBackend) Name() string     { return f.name }
func (f *fakeBackend) Describe() string { return "fake backend " + f.name }

func (f *fakeBackend) Check() error {
	f.checks++
	f.record("check")
	return f.checkErr
}

func (f *fakeBackend) Set(v wallpaper.Video) error {
	f.record("set")
	if f.setErr != nil {
		return f.setErr
	}
	f.sets = append(f.sets, v)
	return nil
}

func (f *fakeBackend) Clear() error {
	f.clears++
	f.record("clear")
	return f.clearErr
}

func (f *fakeBackend) record(op string) {
	if f.journal != nil {
		*f.journal = append(*f.journal, f.name+"."+op)
	}
}

func names(bs []wallpaper.Backend) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name()
	}
	return out
}

func TestFakeBackendHonoursContract(t *testing.T) {
	backendtest.Contract(t, func(t *testing.T) wallpaper.Backend {
		return &fakeBackend{name: "fake"}
	})
}
