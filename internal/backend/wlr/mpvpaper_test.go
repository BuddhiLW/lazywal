package wlr

import (
	"errors"
	"reflect"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
	"github.com/BuddhiLW/lazywal/internal/wallpaper/backendtest"
)

var pkill = shell.Pkill("mpvpaper")

func TestContract(t *testing.T) {
	backendtest.ContractWithRunner(t, func(t *testing.T, r *shelltest.Fake, s *shelltest.Store) wallpaper.Backend {
		return NewMpvpaper(r, s)
	})
}

func TestCheckListsMissingMpvpaper(t *testing.T) {
	r := shelltest.New()
	r.Missing["mpvpaper"] = true
	backendtest.MissingDeps(t, NewMpvpaper(r, shelltest.NewStore()).Check(), "mpvpaper")
}

func TestSetStartsMpvpaperOnAllOutputs(t *testing.T) {
	r := shelltest.New()
	v := backendtest.Video(t)
	if err := NewMpvpaper(r, shelltest.NewStore()).Set(v); err != nil {
		t.Fatal(err)
	}
	if len(r.Started) != 1 {
		t.Fatalf("started %v, want one mpvpaper", r.Started)
	}
	want := []string{"mpvpaper", "-o", "no-audio loop-file=inf panscan=1 no-resume-playback", "ALL", v.Path()}
	if got := r.Started[0].Argv(); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %q\nwant   %q", got, want)
	}
}

func TestSetStopsPreviousPlayersFirst(t *testing.T) {
	r, s := shelltest.New(), shelltest.NewStore()
	b := NewMpvpaper(r, s)
	v := backendtest.Video(t)
	if err := b.Set(v); err != nil {
		t.Fatal(err)
	}
	if got := (shell.Tracker{Store: s, Key: pidsKey}).PIDs(); !reflect.DeepEqual(got, []int{100001}) {
		t.Fatalf("tracked pids = %v, want the first player", got)
	}
	r.Reset()

	if err := b.Set(v); err != nil {
		t.Fatal(err)
	}
	if want := []int{100001}; !reflect.DeepEqual(r.Killed, want) {
		t.Errorf("killed %v, want %v", r.Killed, want)
	}
	if len(r.Calls) != 2 || r.Calls[0].String() != pkill.String() || r.Calls[1].Name != "mpvpaper" {
		t.Errorf("calls = %q, want pkill before mpvpaper", r.Commands())
	}
	if got := (shell.Tracker{Store: s, Key: pidsKey}).PIDs(); !reflect.DeepEqual(got, []int{100002}) {
		t.Errorf("tracked pids = %v, want only the new player", got)
	}
}

func TestSetFailsWhenStalePlayersCannotBeStopped(t *testing.T) {
	r := shelltest.New()
	r.Errors[pkill.String()] = shelltest.Exit(pkill, 3)
	if err := NewMpvpaper(r, shelltest.NewStore()).Set(backendtest.Video(t)); err == nil {
		t.Fatal("Set() = nil, want the pkill error")
	}
	if len(r.Started) != 0 {
		t.Errorf("started %v on top of a stale player", r.Started)
	}
}

func TestClear(t *testing.T) {
	t.Run("kills tracked and untracked players", func(t *testing.T) {
		r, s := shelltest.New(), shelltest.NewStore()
		// 41 and 42 are still the players lazywal started; 43 was reused by
		// an unrelated process and must survive.
		s.Set(pidsKey, "41/id41,42/id42,43/id43")
		r.Identities[41], r.Identities[42], r.Identities[43] = "id41", "id42", "someone-else"
		if err := NewMpvpaper(r, s).Clear(); err != nil {
			t.Fatal(err)
		}
		if want := []int{41, 42}; !reflect.DeepEqual(r.Killed, want) {
			t.Errorf("killed %v, want %v", r.Killed, want)
		}
		if !r.Ran(pkill.String()) {
			t.Errorf("calls = %q, want %q", r.Commands(), pkill)
		}
		if got := s.Get(pidsKey); got != "" {
			t.Errorf("tracked pids = %q after Clear", got)
		}
	})

	t.Run("no mpvpaper running is not an error", func(t *testing.T) {
		r := shelltest.New()
		r.Errors[pkill.String()] = shelltest.Exit(pkill, 1)
		if err := NewMpvpaper(r, shelltest.NewStore()).Clear(); err != nil {
			t.Errorf("Clear() = %v", err)
		}
	})

	t.Run("pkill failure is reported", func(t *testing.T) {
		r := shelltest.New()
		boom := errors.New("pkill: not found")
		r.Errors["pkill"] = boom
		if err := NewMpvpaper(r, shelltest.NewStore()).Clear(); !errors.Is(err, boom) {
			t.Errorf("Clear() = %v, want %v", err, boom)
		}
	})
}
