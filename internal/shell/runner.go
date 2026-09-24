package shell

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Runner is the port through which lazywal touches the operating system.
// Backends depend on this interface, never on os/exec, so they can be tested
// with shelltest.Fake and never touch the real desktop in tests.
type Runner interface {
	// Run runs c to completion. A non-zero exit returns an *ExitError that
	// carries the combined output.
	Run(c Cmd) error
	// Output runs c and returns its trimmed stdout.
	Output(c Cmd) (string, error)
	// Start launches c detached, in its own process group, with no stdio,
	// and returns its PID without waiting for it.
	Start(c Cmd) (int, error)
	// KillGroup SIGKILLs the process group led by pid. It does nothing when
	// pid no longer leads its own group: Start always creates a group, so
	// such a PID was reused by an unrelated process.
	KillGroup(pid int)
	// ProcessName returns the kernel's name for pid (truncated to 15 bytes,
	// as in /proc/<pid>/comm), or "" when it is unknown or pid is gone.
	ProcessName(pid int) string
	// ProcessIdentity returns a token that tells pid's process apart from
	// any other that has had or will have the same PID, even across a
	// reboot. Unlike the name it survives exec, so it still matches after a
	// wrapper (env, nice, prime-run...) replaces itself with the player. It
	// returns "" when the identity is unknown or pid is gone.
	ProcessIdentity(pid int) string
	// LookPath returns an error when program is not installed.
	LookPath(program string) error
}

// ExitError reports a command that exited with a non-zero status.
type ExitError struct {
	Cmd    Cmd
	Code   int
	Output string
}

func (e *ExitError) Error() string {
	msg := fmt.Sprintf("%s: exit status %d", e.Cmd, e.Code)
	if out := strings.TrimSpace(e.Output); out != "" {
		msg += ": " + out
	}
	return msg
}

// ExitCode returns 0 for a nil error, the exit status for an *ExitError, and
// -1 for any other error (e.g. the program could not be started).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return -1
}

// Exec is the real Runner, backed by os/exec.
type Exec struct {
	// Log, when set, receives every long-running command started. Short
	// commands (checks, gsettings...) are not logged; their failures are
	// returned as errors.
	Log *log.Logger
}

func (e Exec) logf(format string, args ...any) {
	if e.Log != nil {
		e.Log.Printf(format, args...)
	}
}

func (e Exec) command(c Cmd) *exec.Cmd { return exec.Command(c.Name, c.Args...) }

func (e Exec) Run(c Cmd) error {
	out, err := e.command(c).CombinedOutput()
	return wrapErr(c, err, out)
}

func (e Exec) Output(c Cmd) (string, error) {
	cmd := e.command(c)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", wrapErr(c, err, stderr.Bytes())
	}
	return strings.TrimSpace(string(out)), nil
}

func (e Exec) Start(c Cmd) (int, error) {
	e.logf("Running command: %s", c)
	cmd := e.command(c)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("starting %s: %w", c, err)
	}
	// Reap the child if this process outlives it (e.g. the MCP server).
	go cmd.Wait()
	return cmd.Process.Pid, nil
}

func (Exec) KillGroup(pid int) {
	if pid <= 1 {
		return
	}
	if pgid, err := syscall.Getpgid(pid); err == nil && pgid == pid {
		syscall.Kill(-pgid, syscall.SIGKILL)
	}
}

func (Exec) ProcessName(pid int) string {
	comm, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(comm))
}

// ProcessIdentity is "<boot_id>:<starttime>": the kernel's random ID for
// this boot and the process's start time in clock ticks since boot, which
// exec does not change.
func (Exec) ProcessIdentity(pid int) string {
	if pid <= 0 {
		return ""
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	start := statStartTime(string(stat))
	if start == "" {
		return ""
	}
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(boot)) + ":" + start
}

// statStartTime returns field 22 (starttime) of a /proc/<pid>/stat line, or
// "". Field 2 is the name in parentheses and may itself contain spaces and
// ')', so fields are counted from after the last ')'.
func statStartTime(stat string) string {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return ""
	}
	fields := strings.Fields(stat[i+1:])
	const startTime = 22 - 3 // fields[0] is field 3 (state)
	if len(fields) <= startTime {
		return ""
	}
	return fields[startTime]
}

func (Exec) LookPath(program string) error {
	_, err := exec.LookPath(program)
	return err
}

func wrapErr(c Cmd, err error, out []byte) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &ExitError{Cmd: c, Code: exitErr.ExitCode(), Output: string(out)}
	}
	return fmt.Errorf("%s: %w", c, err)
}
