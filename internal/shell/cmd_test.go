package shell_test

import (
	"slices"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
)

func assertArgv(t *testing.T, got shell.Cmd, want ...string) {
	t.Helper()
	if !slices.Equal(got.Argv(), want) {
		t.Errorf("argv = %q, want %q", got.Argv(), want)
	}
}

func TestCmdString(t *testing.T) {
	tests := []struct {
		name string
		cmd  shell.Cmd
		want string
	}{
		{"no args", shell.Command("xrandr"), "xrandr"},
		{"plain args", shell.Command("pgrep", "-x", "mpv"), "pgrep -x mpv"},
		{"option with equals", shell.Command("mpv", "--loop-file=inf"), "mpv --loop-file=inf"},
		{"space", shell.Command("mpv", "/tmp/my video.mp4"), `mpv "/tmp/my video.mp4"`},
		{"tab", shell.Command("echo", "a\tb"), `echo "a\tb"`},
		{"newline", shell.Command("echo", "a\nb"), `echo "a\nb"`},
		{"single quote", shell.Command("mpv", "it's.mp4"), `mpv "it's.mp4"`},
		{"double quote", shell.Command("mpv", `a"b.mp4`), `mpv "a\"b.mp4"`},
		{"backslash", shell.Command("echo", `a\b`), `echo "a\\b"`},
		{"empty arg", shell.Command("gsettings", "set", "", "x"), `gsettings set "" x`},
		{"dollar", shell.Command("echo", "$HOME"), `echo "$HOME"`},
		{"semicolon", shell.Command("echo", "a;rm -rf /"), `echo "a;rm -rf /"`},
		{"pipe and ampersand", shell.Command("echo", "a|b&c"), `echo "a|b&c"`},
		{"glob", shell.Command("ls", "*.mp4"), `ls "*.mp4"`},
		{"comment", shell.Command("echo", "#x"), `echo "#x"`},
		{"backtick", shell.Command("echo", "`id`"), "echo \"`id`\""},
		{"program name with space", shell.Command("/opt/my apps/mpv", "-v"), `"/opt/my apps/mpv" -v`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cmd.String(); got != tt.want {
				t.Errorf("String() = %s, want %s", got, tt.want)
			}
		})
	}
}

// shelltest.Fake keys its responses on String, so different argv must never
// render the same.
func TestCmdStringIsUnambiguous(t *testing.T) {
	pairs := [][2]shell.Cmd{
		{shell.Command("a b"), shell.Command("a", "b")},
		{shell.Command("x", "a b"), shell.Command("x", "a", "b")},
		{shell.Command("x", ""), shell.Command("x")},
		{shell.Command("x", `"y"`), shell.Command("x", "y")},
		{shell.Command("x", `"a b"`), shell.Command("x", "a b")},
	}
	for _, p := range pairs {
		if p[0].String() == p[1].String() {
			t.Errorf("%q and %q both render as %s", p[0].Argv(), p[1].Argv(), p[0].String())
		}
	}
}

func TestCommandCopiesArgs(t *testing.T) {
	args := []string{"-x", "mpv"}
	c := shell.Command("pgrep", args...)
	args[0], args[1] = "MUTATED", "MUTATED"
	assertArgv(t, c, "pgrep", "-x", "mpv")

	// Spare capacity in the caller's slice must not be shared either.
	buf := make([]string, 1, 8)
	buf[0] = "a"
	c = shell.Command("echo", buf...)
	_ = append(buf, "leak")
	buf[0] = "MUTATED"
	assertArgv(t, c, "echo", "a")
}

func TestArgvDoesNotAlias(t *testing.T) {
	c := shell.Command("echo", "a", "b")
	argv := c.Argv()
	argv[0], argv[1] = "MUTATED", "MUTATED"
	assertArgv(t, c, "echo", "a", "b")
}

func TestCommandWithoutArgs(t *testing.T) {
	c := shell.Command("xrandr")
	if c.Name != "xrandr" || len(c.Args) != 0 {
		t.Errorf("Command(xrandr) = %+v", c)
	}
	assertArgv(t, c, "xrandr")
}
