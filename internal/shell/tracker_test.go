package shell_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
)

const trackerKey = "lazywal_test_pids"

func newTracker() (shell.Tracker, *shelltest.Fake, *shelltest.Store) {
	f, s := shelltest.New(), shelltest.NewStore()
	return shell.Tracker{Runner: f, Store: s, Key: trackerKey}, f, s
}

func TestTrackerSpawnTracksPIDs(t *testing.T) {
	tr, f, s := newTracker()
	first := shell.Mpv().File("/a.mp4").Build()
	second := shell.Mpvpaper().File("/b.mp4").Build()

	pid1, err := tr.Spawn(first)
	if err != nil {
		t.Fatal(err)
	}
	pid2, err := tr.Spawn(second)
	if err != nil {
		t.Fatal(err)
	}

	if pid1 <= 0 || pid2 <= 0 || pid1 == pid2 {
		t.Fatalf("Spawn() pids = %d, %d", pid1, pid2)
	}
	if got := tr.PIDs(); !slices.Equal(got, []int{pid1, pid2}) {
		t.Errorf("PIDs() = %v, want [%d %d]", got, pid1, pid2)
	}
	if got, want := s.Get(trackerKey), fmt.Sprintf("%d/fake-%d,%d/fake-%d", pid1, pid1, pid2, pid2); got != want {
		t.Errorf("stored %q, want %q", got, want)
	}
	if len(f.Started) != 2 || f.Started[0].String() != first.String() || f.Started[1].String() != second.String() {
		t.Errorf("started %q", f.Commands())
	}
}

func TestTrackerSpawnFailureTracksNothing(t *testing.T) {
	tr, f, s := newTracker()
	boom := errors.New("exec: mpv: not found")
	f.Errors["mpv"] = boom

	pid, err := tr.Spawn(shell.Mpv().File("/a.mp4").Build())
	if !errors.Is(err, boom) || pid != 0 {
		t.Fatalf("Spawn() = %d, %v; want 0, %v", pid, err, boom)
	}
	if got := tr.PIDs(); len(got) != 0 {
		t.Errorf("PIDs() = %v, want none", got)
	}
	if got := s.Get(trackerKey); got != "" {
		t.Errorf("stored %q, want nothing", got)
	}
}

func TestTrackerPIDsIgnoresGarbage(t *testing.T) {
	tests := []struct {
		stored string
		want   []int
	}{
		{"", nil},
		{"12", []int{12}},
		{"12,34", []int{12, 34}},
		{" 12 , 34 ", []int{12, 34}},
		{"abc", nil},
		{"12,abc,,34,", []int{12, 34}},
		{"-5,0,7", []int{7}},
		{"1.5,2", []int{2}},
		{"12;34", nil},
		{"0x10,99999999999999999999,8", []int{8}},
		{"12/boot:345,34/", []int{12, 34}},
		{"/x,a/b,-1/x,56/x/y", []int{56}},
	}
	for _, tt := range tests {
		t.Run(tt.stored, func(t *testing.T) {
			tr, _, s := newTracker()
			s.Set(trackerKey, tt.stored)
			if got := tr.PIDs(); !slices.Equal(got, tt.want) {
				t.Errorf("PIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrackerSpawnDropsGarbage(t *testing.T) {
	tr, _, s := newTracker()
	s.Set(trackerKey, "junk, 42 ,")

	pid, err := tr.Spawn(shell.Command("sleep", "30"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Get(trackerKey), fmt.Sprintf("42,%d/fake-%d", pid, pid); got != want {
		t.Errorf("stored %q, want %q", got, want)
	}
}

func TestTrackerKillAll(t *testing.T) {
	tr, f, s := newTracker()
	tr.Legacy = []string{"xwinwrap"}
	f.Names[7], f.Names[8] = "xwinwrap", "xwinwrap"
	s.Set(trackerKey, "junk, 7 ,8")
	pid, err := tr.Spawn(shell.Command("sleep", "30"))
	if err != nil {
		t.Fatal(err)
	}

	tr.KillAll()

	if want := []int{7, 8, pid}; !slices.Equal(f.Killed, want) {
		t.Errorf("killed %v, want %v", f.Killed, want)
	}
	if got := s.Get(trackerKey); got != "" {
		t.Errorf("stored %q after KillAll, want empty", got)
	}
	if got := tr.PIDs(); len(got) != 0 {
		t.Errorf("PIDs() after KillAll = %v", got)
	}

	f.Reset()
	tr.KillAll()
	if len(f.Killed) != 0 {
		t.Errorf("second KillAll killed %v", f.Killed)
	}
}

func TestTrackerKeysAreIndependent(t *testing.T) {
	f, s := shelltest.New(), shelltest.NewStore()
	a := shell.Tracker{Runner: f, Store: s, Key: "a"}
	b := shell.Tracker{Runner: f, Store: s, Key: "b"}

	pidA, _ := a.Spawn(shell.Command("sleep", "30"))
	pidB, _ := b.Spawn(shell.Command("sleep", "30"))
	a.KillAll()

	if !slices.Equal(f.Killed, []int{pidA}) {
		t.Errorf("killed %v, want only [%d]", f.Killed, pidA)
	}
	if got := b.PIDs(); !slices.Equal(got, []int{pidB}) {
		t.Errorf("other tracker PIDs() = %v, want [%d]", got, pidB)
	}
}

// A PID stored before a reboot, or reused later in the same boot, now
// belongs to an unrelated process with another identity; it must never be
// killed. Neither is a process that is gone.
func TestTrackerKillsOnlyMatchingIdentities(t *testing.T) {
	tr, f, s := newTracker()
	var pids []int
	for range 3 {
		pid, err := tr.Spawn(shell.Command("mpvpaper", "ALL", "v.mp4"))
		if err != nil {
			t.Fatal(err)
		}
		pids = append(pids, pid)
	}
	ours, reused, gone := pids[0], pids[1], pids[2]
	f.Identities[reused] = "another-process"
	f.Identities[gone] = ""

	tr.KillAll()
	if !slices.Equal(f.Killed, []int{ours}) {
		t.Errorf("killed %v, want only [%d]", f.Killed, ours)
	}
	if s.Get(trackerKey) != "" {
		t.Errorf("tracker not cleared: %q", s.Get(trackerKey))
	}
}

// A player started through a wrapper (env, nice, prime-run...) runs under
// another name once the wrapper execs it; its identity does not change, so
// it is still killed.
func TestTrackerKillsThroughWrappers(t *testing.T) {
	tr, f, _ := newTracker()
	pid, err := tr.Spawn(shell.Command("env", "DRI_PRIME=1", "mpvpaper", "ALL", "v.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	f.Names[pid] = "mpvpaper"

	tr.KillAll()
	if !slices.Equal(f.Killed, []int{pid}) {
		t.Errorf("killed %v, want [%d]", f.Killed, pid)
	}
}

// An identity unknown at Spawn (the process already exited, or no /proc)
// only matches while it is still unknown: once the PID has an identity it
// belongs to someone else.
func TestTrackerUnknownIdentity(t *testing.T) {
	for _, tt := range []struct {
		now  string
		kill bool
	}{
		{"", true},
		{"someone-else", false},
	} {
		t.Run(fmt.Sprintf("now %q", tt.now), func(t *testing.T) {
			tr, f, s := newTracker()
			f.Identities[100001] = ""
			pid, err := tr.Spawn(shell.Command("sleep", "30"))
			if err != nil || pid != 100001 {
				t.Fatalf("Spawn() = %d, %v; want 100001", pid, err)
			}
			if got, want := s.Get(trackerKey), "100001/"; got != want {
				t.Errorf("stored %q, want %q", got, want)
			}
			f.Identities[pid] = tt.now

			tr.KillAll()
			if killed := slices.Contains(f.Killed, pid); killed != tt.kill {
				t.Errorf("killed %v, want kill %v", f.Killed, tt.kill)
			}
		})
	}
}

// State written by lazywal <= v1.4.3 holds bare PIDs with no identity. They
// are killed only when the process has one of the names that version ran.
func TestTrackerLegacyEntries(t *testing.T) {
	tests := []struct {
		name   string
		legacy []string
		want   []int
	}{
		{"names match", []string{"xwinwrap", "bash"}, []int{11, 22}},
		{"no legacy names", nil, nil},
		{"other names only", []string{"mpvpaper"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, f, s := newTracker()
			tr.Legacy = tt.legacy
			s.Set(trackerKey, "11,22,33,44")
			f.Names[11] = "xwinwrap"
			f.Names[22] = "bash"
			f.Names[33] = "firefox" // PID reused since
			// 44 is gone: no name.

			tr.KillAll()
			if !slices.Equal(f.Killed, tt.want) {
				t.Errorf("killed %v, want %v", f.Killed, tt.want)
			}
			if s.Get(trackerKey) != "" {
				t.Errorf("tracker not cleared: %q", s.Get(trackerKey))
			}
		})
	}
}

// Spawning after an upgrade keeps legacy entries bare, so they stay
// subject to the name check.
func TestTrackerSpawnKeepsLegacyEntriesBare(t *testing.T) {
	tr, f, s := newTracker()
	s.Set(trackerKey, "11")
	pid, err := tr.Spawn(shell.Command("sleep", "30"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Get(trackerKey), fmt.Sprintf("11,%d/fake-%d", pid, pid); got != want {
		t.Errorf("stored %q, want %q", got, want)
	}
	f.Names[11] = "firefox"
	tr.KillAll()
	if !slices.Equal(f.Killed, []int{pid}) {
		t.Errorf("killed %v, want only [%d]", f.Killed, pid)
	}
}
