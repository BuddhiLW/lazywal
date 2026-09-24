package backendtest

import (
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

func TestStopped(t *testing.T) {
	player := shell.Command("/usr/bin/mpvpaper", "ALL", "/v.mp4")
	tests := []struct {
		name string
		run  func(r *shelltest.Fake, pid int)
		want bool
	}{
		{"left running", func(*shelltest.Fake, int) {}, false},
		{"group killed", func(r *shelltest.Fake, pid int) { r.KillGroup(pid) }, true},
		{"another group killed", func(r *shelltest.Fake, pid int) { r.KillGroup(pid + 1) }, false},
		{"pkill by name", func(r *shelltest.Fake, _ int) { r.Run(shell.Pkill("mpvpaper")) }, true},
		{"pkill without user scope", func(r *shelltest.Fake, _ int) { r.Run(shell.Command("pkill", "-x", "mpvpaper")) }, true},
		{"pkill of another program", func(r *shelltest.Fake, _ int) { r.Run(shell.Pkill("mpv")) }, false},
		{"pkill by pattern", func(r *shelltest.Fake, _ int) { r.Run(shell.Command("pkill", "mpvpaper")) }, false},
		{"pgrep by name", func(r *shelltest.Fake, _ int) { r.Run(shell.Pgrep("mpvpaper")) }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := shelltest.New()
			pid, err := r.Start(player)
			if err != nil {
				t.Fatal(err)
			}
			tt.run(r, pid)
			if got := stopped(r, r.Procs[0]); got != tt.want {
				t.Errorf("stopped() = %v, want %v (calls %q, killed %v)", got, tt.want, r.Commands(), r.Killed)
			}
		})
	}
}

// Only a pkill that ran after the process started can have stopped it.
func TestStoppedIgnoresEarlierPkill(t *testing.T) {
	r := shelltest.New()
	r.Run(shell.Pkill("mpvpaper"))
	if _, err := r.Start(shell.Command("mpvpaper", "ALL", "/v.mp4")); err != nil {
		t.Fatal(err)
	}
	if stopped(r, r.Procs[0]) {
		t.Errorf("pkill before Start counted as stopping it: %q", r.Commands())
	}
}

// trackedPlayer is a well-behaved backend: it tracks its player and kills
// it before starting another and on Clear.
type trackedPlayer struct{ procs shell.Tracker }

func (*trackedPlayer) Name() string     { return "tracked" }
func (*trackedPlayer) Describe() string { return "tracked player" }
func (*trackedPlayer) Check() error     { return nil }

func (p *trackedPlayer) Set(v wallpaper.Video) error {
	p.procs.KillAll()
	_, err := p.procs.Spawn(shell.Command("mpv", "--", v.Path()))
	return err
}

func (p *trackedPlayer) Clear() error {
	p.procs.KillAll()
	return nil
}

// stillBackend starts nothing, so the process checks pass trivially.
type stillBackend struct{}

func (stillBackend) Name() string              { return "still" }
func (stillBackend) Describe() string          { return "starts nothing" }
func (stillBackend) Check() error              { return nil }
func (stillBackend) Set(wallpaper.Video) error { return nil }
func (stillBackend) Clear() error              { return nil }

func TestContractWithRunnerPasses(t *testing.T) {
	t.Run("tracked player", func(t *testing.T) {
		ContractWithRunner(t, func(t *testing.T, r *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
			return &trackedPlayer{procs: shell.Tracker{Runner: r, Store: s, Key: "k"}}
		})
	})
	t.Run("backend that starts nothing", func(t *testing.T) {
		ContractWithRunner(t, func(*testing.T, *shelltest.Fake, *shelltest.Store) wallpaper.Backend {
			return stillBackend{}
		})
	})
}
