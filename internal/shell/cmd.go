// Package shell runs external programs. Commands are built as argv values by
// typed builders (never as shell strings), so paths with spaces or quotes are
// passed through untouched and nothing is interpreted by a shell.
package shell

import (
	"strconv"
	"strings"
)

// Cmd is an immutable program invocation: a program name and its arguments.
type Cmd struct {
	Name string
	Args []string
}

// Command builds a Cmd from a program name and arguments.
func Command(name string, args ...string) Cmd {
	return Cmd{Name: name, Args: append([]string(nil), args...)}
}

// Argv returns the name followed by the arguments.
func (c Cmd) Argv() []string {
	return append([]string{c.Name}, c.Args...)
}

// String renders the command for logs, quoting arguments a shell would split.
// It is for humans only: commands are never executed through a shell.
func (c Cmd) String() string {
	parts := make([]string, 0, len(c.Args)+1)
	for _, a := range c.Argv() {
		if a == "" || strings.ContainsAny(a, " \t\n'\"\\$`*?[]{}()<>|&;#~") {
			a = strconv.Quote(a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}
