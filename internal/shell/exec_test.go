package shell_test

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/shell/shelltest"
)

// These tests run real, harmless programs (true, false, echo, cat, env,
// sleep) to check the one Runner that touches the OS. -short skips them.

func requirePrograms(t *testing.T, names ...string) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs real processes")
	}
	for _, name := range names {
		if err := (shell.Exec{}).LookPath(name); err != nil {
			t.Skipf("%s not installed: %v", name, err)
		}
	}
}

func TestExecRun(t *testing.T) {
	requirePrograms(t, "true", "false", "cat")
	var logs bytes.Buffer
	e := shell.Exec{Log: log.New(&logs, "", 0)}

	if err := e.Run(shell.Command("true")); err != nil {
		t.Errorf("Run(true) = %v", err)
	}
	if logs.Len() != 0 {
		t.Errorf("log = %q, want short commands unlogged", logs.String())
	}

	err := e.Run(shell.Command("false"))
	var exitErr *shell.ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 1 || exitErr.Cmd.Name != "false" {
		t.Errorf("Run(false) = %#v, want *ExitError code 1", err)
	}

	missing := filepath.Join(t.TempDir(), "no such file")
	err = e.Run(shell.Command("cat", missing))
	if !errors.As(err, &exitErr) || exitErr.Code != 1 {
		t.Fatalf("Run(cat missing) = %v, want *ExitError code 1", err)
	}
	if !strings.Contains(exitErr.Output, missing) || !strings.Contains(err.Error(), missing) {
		t.Errorf("Run(cat missing) output %q, error %q; want stderr naming %q", exitErr.Output, err, missing)
	}
}

func TestExecOutput(t *testing.T) {
	requirePrograms(t, "echo", "cat")
	var e shell.Exec

	if got, err := e.Output(shell.Command("echo", "hi")); err != nil || got != "hi" {
		t.Errorf("Output(echo hi) = %q, %v; want \"hi\"", got, err)
	}

	// Arguments reach the program verbatim: nothing is expanded or split.
	args := []string{"a  b", "$HOME", "; rm -rf /", `it's "quoted"`, "*"}
	got, err := e.Output(shell.Command("echo", args...))
	if want := strings.Join(args, " "); err != nil || got != want {
		t.Errorf("Output(echo ...) = %q, %v; want %q", got, err, want)
	}

	missing := filepath.Join(t.TempDir(), "no such file")
	got, err = e.Output(shell.Command("cat", missing))
	var exitErr *shell.ExitError
	if got != "" || !errors.As(err, &exitErr) || exitErr.Code != 1 || !strings.Contains(exitErr.Output, missing) {
		t.Errorf("Output(cat missing) = %q, %v; want *ExitError code 1 carrying stderr", got, err)
	}
}

func TestExecMissingProgram(t *testing.T) {
	if testing.Short() {
		t.Skip("runs real processes")
	}
	const name = "lazywal-no-such-program-4f9c2e"
	var e shell.Exec

	if err := e.LookPath(name); err == nil {
		t.Errorf("LookPath(%s) = nil, want error", name)
	}
	err := e.Run(shell.Command(name))
	if err == nil || shell.ExitCode(err) != -1 {
		t.Errorf("Run(%s) = %v (code %d), want a non-exit error", name, err, shell.ExitCode(err))
	}
	if _, err := e.Start(shell.Command(name)); err == nil {
		t.Errorf("Start(%s) = nil error", name)
	}
}

func TestExecStartKillGroup(t *testing.T) {
	requirePrograms(t, "sleep")
	var logs bytes.Buffer
	e := shell.Exec{Log: log.New(&logs, "", 0)}

	pid, err := e.Start(shell.Command("sleep", "30"))
	if err != nil {
		t.Fatal(err)
	}
	if got := logs.String(); got != "Running command: sleep 30\n" {
		t.Errorf("log = %q, want the started command", got)
	}
	gone := false
	t.Cleanup(func() {
		if !gone {
			syscall.Kill(pid, syscall.SIGKILL)
		}
	})

	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatalf("Getpgid(%d) = %v", pid, err)
	}
	if pgid != pid {
		t.Errorf("started process is in group %d, want its own group %d", pgid, pid)
	}

	e.KillGroup(pid)
	waitGone(t, pid)
	gone = true
}

func requireProc(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		t.Skip("no /proc:", err)
	}
}

// waitGone fails t unless pid disappears within 2s.
func waitGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("process %d still alive 2s after it was killed", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestExecProcessIdentity(t *testing.T) {
	requireProc(t)
	var e shell.Exec
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		t.Skip("no boot_id:", err)
	}

	self := e.ProcessIdentity(os.Getpid())
	prefix := strings.TrimSpace(string(boot)) + ":"
	start, ok := strings.CutPrefix(self, prefix)
	if _, err := strconv.ParseUint(start, 10, 64); !ok || err != nil {
		t.Errorf("ProcessIdentity(self) = %q, want %q followed by a start time", self, prefix)
	}
	if again := e.ProcessIdentity(os.Getpid()); again != self {
		t.Errorf("ProcessIdentity(self) changed from %q to %q", self, again)
	}
	for _, pid := range []int{0, -1, 1 << 30} {
		if got := e.ProcessIdentity(pid); got != "" {
			t.Errorf("ProcessIdentity(%d) = %q, want \"\"", pid, got)
		}
	}
}

// A player started through a wrapper that execs it keeps its identity, so
// the Tracker still kills it once the wrapper is gone.
func TestExecIdentitySurvivesExec(t *testing.T) {
	requirePrograms(t, "env", "sleep")
	requireProc(t)
	var e shell.Exec
	store := shelltest.NewStore()
	tr := shell.Tracker{Runner: e, Store: store, Key: "k"}

	pid, err := tr.Spawn(shell.Command("env", "sleep", "30"))
	if err != nil {
		t.Fatal(err)
	}
	gone := false
	t.Cleanup(func() {
		if !gone {
			syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	_, before, _ := strings.Cut(store.Get("k"), "/")
	if before == "" {
		t.Fatalf("tracked %q, want an identity", store.Get("k"))
	}

	deadline := time.Now().Add(2 * time.Second)
	for e.ProcessName(pid) != "sleep" {
		if time.Now().After(deadline) {
			t.Fatalf("env did not exec sleep within 2s (name %q)", e.ProcessName(pid))
		}
		time.Sleep(10 * time.Millisecond)
	}
	if after := e.ProcessIdentity(pid); after != before {
		t.Fatalf("identity changed across exec: %q, then %q", before, after)
	}

	tr.KillAll()
	waitGone(t, pid)
	gone = true
	if got := store.Get("k"); got != "" {
		t.Errorf("tracked %q after KillAll", got)
	}
}
