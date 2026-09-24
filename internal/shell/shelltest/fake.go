// Package shelltest provides a fake shell.Runner and an in-memory
// shell.Store so backends can be tested without touching the real system.
package shelltest

import (
	"errors"
	"strconv"
	"sync"

	"github.com/BuddhiLW/lazywal/internal/shell"
)

// Fake records every command and answers from scripted responses.
// Responses are looked up by the full command string (Cmd.String()) first,
// then by program name.
type Fake struct {
	mu sync.Mutex

	// Outputs maps a command string or program name to its stdout.
	Outputs map[string]string
	// Errors maps a command string or program name to the error it returns.
	Errors map[string]error
	// Missing lists programs LookPath reports as not installed.
	Missing map[string]bool
	// Names maps a PID to what ProcessName reports; absent PIDs report "".
	Names map[int]string
	// Identities overrides what ProcessIdentity reports for a PID; "" means
	// the process is gone. Without an entry, a PID this fake started
	// reports "fake-<pid>" and any other PID reports "".
	Identities map[int]string
	// Effects maps a program name to a side effect performed by a successful
	// Run, so a fake behaves like the real tool (e.g. ffmpeg writing its
	// output file).
	Effects map[string]func(c shell.Cmd) error

	Calls   []shell.Cmd // every Run, Output and Start, in order
	Started []shell.Cmd // successful Start calls only
	Procs   []Proc      // successful Start calls, with the PID each returned
	Killed  []int       // KillGroup calls

	nextPID int
}

// Proc is a process the fake started.
type Proc struct {
	PID  int
	Cmd  shell.Cmd
	Call int // index of the Start in Calls
}

// firstPID is one below the first PID Start returns.
const firstPID = 100000

// New returns an empty Fake: every command succeeds with no output and every
// program is installed.
func New() *Fake {
	return &Fake{
		Outputs:    map[string]string{},
		Errors:     map[string]error{},
		Missing:    map[string]bool{},
		Names:      map[int]string{},
		Identities: map[int]string{},
		Effects:    map[string]func(shell.Cmd) error{},
	}
}

// Exit returns an error that shell.ExitCode reports as code.
func Exit(c shell.Cmd, code int) error { return &shell.ExitError{Cmd: c, Code: code} }

func (f *Fake) lookup(c shell.Cmd) (string, error) {
	for _, key := range []string{c.String(), c.Name} {
		if err, ok := f.Errors[key]; ok {
			return "", err
		}
	}
	for _, key := range []string{c.String(), c.Name} {
		if out, ok := f.Outputs[key]; ok {
			return out, nil
		}
	}
	return "", nil
}

func (f *Fake) Run(c shell.Cmd) error {
	f.mu.Lock()
	f.Calls = append(f.Calls, c)
	_, err := f.lookup(c)
	effect := f.Effects[c.Name]
	f.mu.Unlock()
	if err != nil || effect == nil {
		return err
	}
	return effect(c)
}

func (f *Fake) Output(c shell.Cmd) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, c)
	return f.lookup(c)
}

func (f *Fake) Start(c shell.Cmd) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, c)
	if _, err := f.lookup(c); err != nil {
		return 0, err
	}
	f.nextPID++
	pid := firstPID + f.nextPID
	f.Started = append(f.Started, c)
	f.Procs = append(f.Procs, Proc{PID: pid, Cmd: c, Call: len(f.Calls) - 1})
	return pid, nil
}

func (f *Fake) KillGroup(pid int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Killed = append(f.Killed, pid)
}

func (f *Fake) ProcessName(pid int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Names[pid]
}

func (f *Fake) ProcessIdentity(pid int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.Identities[pid]; ok {
		return id
	}
	if pid > firstPID && pid <= firstPID+f.nextPID {
		return "fake-" + strconv.Itoa(pid)
	}
	return ""
}

func (f *Fake) LookPath(program string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Missing[program] {
		return errors.New(program + ": not found")
	}
	return nil
}

// Commands returns the string form of every recorded call.
func (f *Fake) Commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.Calls))
	for i, c := range f.Calls {
		out[i] = c.String()
	}
	return out
}

// Count returns how many recorded calls render as cmd.
func (f *Fake) Count(cmd string) int {
	n := 0
	for _, c := range f.Commands() {
		if c == cmd {
			n++
		}
	}
	return n
}

// Ran reports whether cmd was called at least once.
func (f *Fake) Ran(cmd string) bool { return f.Count(cmd) > 0 }

// Reset forgets recorded calls, keeping the scripted responses. Processes
// started before keep their identities, as real ones would keep running.
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls, f.Started, f.Procs, f.Killed = nil, nil, nil, nil
}

// Store is an in-memory shell.Store (and wallpaper.State).
type Store struct {
	mu sync.Mutex
	m  map[string]string
}

func NewStore() *Store { return &Store{m: map[string]string{}} }

func (s *Store) Get(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key]
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}
