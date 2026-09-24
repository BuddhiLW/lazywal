package shell

import (
	"slices"
	"strconv"
	"strings"
)

// Store is a persistent string key/value store (bonzai's injson persister
// satisfies it). It lets a later lazywal process find what an earlier one
// started.
type Store interface {
	Get(key string) string
	Set(key, value string)
}

// Tracker starts long-running processes (wallpaper players) and remembers
// them under Key, so a later Set or Clear can kill them.
//
// Entries are stored as "pid/identity", with the Runner's ProcessIdentity
// read right after Start. KillAll only kills a PID whose identity is
// unchanged, so a PID reused by an unrelated process (even after a reboot)
// is never killed, while a player started through a wrapper that execs it
// (env, nice, prime-run...) still is.
type Tracker struct {
	Runner Runner
	Store  Store
	Key    string
	// Legacy lists the kernel process names (as in /proc/<pid>/comm) under
	// which a bare "pid" entry, written by lazywal <= v1.4.3 without an
	// identity, is still killed. Nil drops such entries without killing.
	Legacy []string
}

type tracked struct {
	pid      int
	identity string
	legacy   bool // bare "pid": no identity was recorded
}

func (t Tracker) entries() []tracked {
	var out []tracked
	for _, s := range strings.Split(t.Store.Get(t.Key), ",") {
		pidStr, identity, found := strings.Cut(strings.TrimSpace(s), "/")
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
			out = append(out, tracked{pid: pid, identity: identity, legacy: !found})
		}
	}
	return out
}

func (t Tracker) save(entries []tracked) {
	strs := make([]string, len(entries))
	for i, e := range entries {
		strs[i] = strconv.Itoa(e.pid)
		if !e.legacy {
			strs[i] += "/" + e.identity
		}
	}
	t.Store.Set(t.Key, strings.Join(strs, ","))
}

// PIDs returns the tracked process IDs.
func (t Tracker) PIDs() []int {
	entries := t.entries()
	pids := make([]int, len(entries))
	for i, e := range entries {
		pids[i] = e.pid
	}
	return pids
}

// Spawn starts c detached and tracks its PID and identity. The identity may
// be empty (e.g. the process already exited); KillAll then only kills the
// PID while its identity is still unknown.
func (t Tracker) Spawn(c Cmd) (int, error) {
	pid, err := t.Runner.Start(c)
	if err != nil {
		return 0, err
	}
	t.save(append(t.entries(), tracked{pid: pid, identity: t.Runner.ProcessIdentity(pid)}))
	return pid, nil
}

// KillAll kills every tracked process group that is still ours and forgets
// them all. A process that is gone or whose PID was reused is skipped.
func (t Tracker) KillAll() {
	for _, e := range t.entries() {
		if t.ours(e) {
			t.Runner.KillGroup(e.pid)
		}
	}
	t.Store.Set(t.Key, "")
}

func (t Tracker) ours(e tracked) bool {
	if e.legacy {
		name := t.Runner.ProcessName(e.pid)
		return name != "" && slices.Contains(t.Legacy, name)
	}
	return t.Runner.ProcessIdentity(e.pid) == e.identity
}
