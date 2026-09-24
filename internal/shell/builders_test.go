package shell_test

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/BuddhiLW/lazywal/internal/shell"
)

func TestMpvBuild(t *testing.T) {
	tests := []struct {
		name string
		b    shell.MpvBuilder
		want []string
	}{
		{"bare", shell.Mpv(), []string{"mpv"}},
		{"file only", shell.Mpv().File("/v/my video.mp4"),
			[]string{"mpv", "--", "/v/my video.mp4"}},
		{"options keep insertion order",
			shell.Mpv().NoAudio().Loop().NoResumePlayback().Panscan(1).Option("hwdec=auto").File("/v.mp4"),
			[]string{"mpv", "--no-audio", "--loop-file=inf", "--no-resume-playback", "--panscan=1", "--hwdec=auto", "--", "/v.mp4"}},
		{"panscan fraction", shell.Mpv().Panscan(0.5), []string{"mpv", "--panscan=0.5"}},
		{"embed goes first as two args",
			shell.Mpv().Loop().Embed(shell.XwinwrapWID).File("/v.mp4"),
			[]string{"mpv", "-wid", "WID", "--loop-file=inf", "--", "/v.mp4"}},
		{"embed window id", shell.Mpv().Embed("0x1a00007"), []string{"mpv", "-wid", "0x1a00007"}},
		{"empty embed is no embed", shell.Mpv().Embed("").File("/v.mp4"),
			[]string{"mpv", "--", "/v.mp4"}},
		{"last embed wins", shell.Mpv().Embed("1").Embed("2"), []string{"mpv", "-wid", "2"}},
		{"last file wins", shell.Mpv().File("/a.mp4").File("/b.mp4"), []string{"mpv", "--", "/b.mp4"}},
		{"dash file stays after --", shell.Mpv().Loop().File("-rf"),
			[]string{"mpv", "--loop-file=inf", "--", "-rf"}},
		{"option-looking file stays after --", shell.Mpv().File("--input-ipc-server=/tmp/x"),
			[]string{"mpv", "--", "--input-ipc-server=/tmp/x"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertArgv(t, tt.b.Build(), tt.want...)
		})
	}
}

func TestMpvOptions(t *testing.T) {
	b := shell.Mpv().Loop().NoAudio().Embed(shell.XwinwrapWID).File("/v.mp4")
	opts := b.Options()
	if want := []string{"loop-file=inf", "no-audio"}; !slices.Equal(opts, want) {
		t.Fatalf("Options() = %q, want %q", opts, want)
	}
	opts[0] = "MUTATED"
	assertArgv(t, b.Build(), "mpv", "-wid", "WID", "--loop-file=inf", "--no-audio", "--", "/v.mp4")
	if got := shell.Mpv().Options(); len(got) != 0 {
		t.Errorf("empty Options() = %q", got)
	}
}

func TestBuildResultDoesNotAliasBuilder(t *testing.T) {
	b := shell.Mpv().Loop().File("/v.mp4")
	c := b.Build()
	c.Args[0] = "MUTATED"
	assertArgv(t, b.Build(), "mpv", "--loop-file=inf", "--", "/v.mp4")
}

// Deriving two commands from one base must not let either see the other's
// additions, whatever spare capacity the base's slices happen to have.
func TestMpvBuilderIsImmutable(t *testing.T) {
	for n := range 7 {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			base := shell.Mpv()
			var common []string
			for i := range n {
				opt := fmt.Sprintf("opt%d=%d", i, i)
				base = base.Option(opt)
				common = append(common, "--"+opt)
			}
			a := base.Option("a=1").Embed("WID").File("/a.mp4")
			b := base.Option("b=2").File("/b.mp4")
			_ = base.Option("unused=3").Embed("9")

			assertArgv(t, base.Build(), slices.Concat([]string{"mpv"}, common)...)
			assertArgv(t, a.Build(), slices.Concat([]string{"mpv", "-wid", "WID"}, common, []string{"--a=1", "--", "/a.mp4"})...)
			assertArgv(t, b.Build(), slices.Concat([]string{"mpv"}, common, []string{"--b=2", "--", "/b.mp4"})...)
		})
	}
}

func TestXwinwrapFlags(t *testing.T) {
	inner := shell.Command("true")
	tests := []struct {
		name string
		b    shell.XwinwrapBuilder
		want []string
	}{
		{"none", shell.Xwinwrap(), nil},
		{"IgnoreInput", shell.Xwinwrap().IgnoreInput(), []string{"-ni"}},
		{"Below", shell.Xwinwrap().Below(), []string{"-b"}},
		{"SkipTaskbar", shell.Xwinwrap().SkipTaskbar(), []string{"-st"}},
		{"SkipPager", shell.Xwinwrap().SkipPager(), []string{"-sp"}},
		{"Undecorated", shell.Xwinwrap().Undecorated(), []string{"-un"}},
		{"Sticky", shell.Xwinwrap().Sticky(), []string{"-s"}},
		{"NoFocus", shell.Xwinwrap().NoFocus(), []string{"-nf"}},
		{"Debug", shell.Xwinwrap().Debug(), []string{"-debug"}},
		{"OverrideRedirect", shell.Xwinwrap().OverrideRedirect(), []string{"-ov"}},
		{"DesktopType", shell.Xwinwrap().DesktopType(), []string{"-fdt"}},
		{"Opacity", shell.Xwinwrap().Opacity(1), []string{"-o", "1"}},
		{"Geometry origin", shell.Xwinwrap().Geometry(1920, 1080, 0, 0), []string{"-g", "1920x1080+0+0"}},
		{"Geometry right of primary", shell.Xwinwrap().Geometry(2560, 1440, 1920, 0), []string{"-g", "2560x1440+1920+0"}},
		{"Geometry below primary", shell.Xwinwrap().Geometry(1280, 1024, 0, 1080), []string{"-g", "1280x1024+0+1080"}},
		// XParseGeometry reads a signed offset after '+', so this is x=-1920.
		{"Geometry left of primary", shell.Xwinwrap().Geometry(1920, 1080, -1920, 0), []string{"-g", "1920x1080+-1920+0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertArgv(t, tt.b.Wrap(inner), slices.Concat([]string{"xwinwrap"}, tt.want, []string{"--", "true"})...)
		})
	}
}

func TestXwinwrapWrap(t *testing.T) {
	inner := shell.Mpv().Embed(shell.XwinwrapWID).Loop().NoAudio().File("/v/my video.mp4").Build()
	got := shell.Xwinwrap().Geometry(1920, 1080, 0, 0).IgnoreInput().Below().SkipTaskbar().
		Undecorated().Opacity(1).OverrideRedirect().Debug().Wrap(inner)
	want := []string{
		"xwinwrap", "-g", "1920x1080+0+0", "-ni", "-b", "-st", "-un", "-o", "1", "-ov", "-debug",
		"--", "mpv", "-wid", "WID", "--loop-file=inf", "--no-audio", "--", "/v/my video.mp4",
	}
	assertArgv(t, got, want...)

	inner.Args[0] = "MUTATED"
	assertArgv(t, got, want...)
}

func TestXwinwrapOpacity(t *testing.T) {
	for _, v := range []float64{0, 0.5, 1} {
		assertOpacity(t, v)
	}
}

func TestXwinwrapOpacityKeepsPrecision(t *testing.T) {
	for _, v := range []float64{0.85, 0.25} {
		assertOpacity(t, v)
	}
}

func assertOpacity(t *testing.T, v float64) {
	t.Helper()
	argv := shell.Xwinwrap().Opacity(v).Wrap(shell.Command("true")).Argv()
	if len(argv) != 5 || argv[1] != "-o" {
		t.Fatalf("Opacity(%v) argv = %q", v, argv)
	}
	if got, err := strconv.ParseFloat(argv[2], 64); err != nil || got != v {
		t.Errorf("Opacity(%v) passes %q", v, argv[2])
	}
}

func TestXwinwrapBuilderIsImmutable(t *testing.T) {
	flags := []struct {
		apply func(shell.XwinwrapBuilder) shell.XwinwrapBuilder
		arg   string
	}{
		{shell.XwinwrapBuilder.IgnoreInput, "-ni"},
		{shell.XwinwrapBuilder.Below, "-b"},
		{shell.XwinwrapBuilder.SkipTaskbar, "-st"},
		{shell.XwinwrapBuilder.SkipPager, "-sp"},
		{shell.XwinwrapBuilder.Undecorated, "-un"},
		{shell.XwinwrapBuilder.NoFocus, "-nf"},
	}
	inner := shell.Command("true")
	for n := range len(flags) + 1 {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			base := shell.Xwinwrap()
			var common []string
			for _, f := range flags[:n] {
				base = f.apply(base)
				common = append(common, f.arg)
			}
			a := base.Sticky()
			b := base.DesktopType()
			_ = base.Debug()

			assertArgv(t, base.Wrap(inner), slices.Concat([]string{"xwinwrap"}, common, []string{"--", "true"})...)
			assertArgv(t, a.Wrap(inner), slices.Concat([]string{"xwinwrap"}, common, []string{"-s", "--", "true"})...)
			assertArgv(t, b.Wrap(inner), slices.Concat([]string{"xwinwrap"}, common, []string{"-fdt", "--", "true"})...)
		})
	}
}

func TestMpvpaperBuild(t *testing.T) {
	mpvOpts := shell.Mpv().NoAudio().Loop().Panscan(1).NoResumePlayback().Options()
	tests := []struct {
		name string
		b    shell.MpvpaperBuilder
		want []string
	}{
		{"defaults to all outputs", shell.Mpvpaper().File("/v.mp4"),
			[]string{"mpvpaper", "ALL", "/v.mp4"}},
		{"one output", shell.Mpvpaper().Output("DP-1").File("/v.mp4"),
			[]string{"mpvpaper", "DP-1", "/v.mp4"}},
		{"mpv options joined into one -o", shell.Mpvpaper().MpvOptions(mpvOpts...).File("/v.mp4"),
			[]string{"mpvpaper", "-o", "no-audio loop-file=inf panscan=1 no-resume-playback", "ALL", "/v.mp4"}},
		{"options accumulate", shell.Mpvpaper().MpvOptions("a").MpvOptions("b", "c").File("/v.mp4"),
			[]string{"mpvpaper", "-o", "a b c", "ALL", "/v.mp4"}},
		{"no options no -o", shell.Mpvpaper().MpvOptions().File("/v.mp4"),
			[]string{"mpvpaper", "ALL", "/v.mp4"}},
		{"path with spaces is one arg", shell.Mpvpaper().File(`/v/it's a "video".mp4`),
			[]string{"mpvpaper", "ALL", `/v/it's a "video".mp4`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertArgv(t, tt.b.Build(), tt.want...)
		})
	}
}

func TestMpvpaperBuilderIsImmutable(t *testing.T) {
	for n := range 6 {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			base := shell.Mpvpaper()
			var common []string
			for i := range n {
				opt := fmt.Sprintf("o%d", i)
				base = base.MpvOptions(opt)
				common = append(common, opt)
			}
			a := base.MpvOptions("a").Output("DP-1").File("/a.mp4")
			b := base.MpvOptions("b").File("/b.mp4")
			_ = base.MpvOptions("unused").Output("HDMI-A-1")

			joined := func(extra string) string {
				return strings.Join(append(slices.Clone(common), extra), " ")
			}
			assertArgv(t, a.Build(), "mpvpaper", "-o", joined("a"), "DP-1", "/a.mp4")
			assertArgv(t, b.Build(), "mpvpaper", "-o", joined("b"), "ALL", "/b.mp4")
			want := []string{"mpvpaper"}
			if n > 0 {
				want = append(want, "-o", strings.Join(common, " "))
			}
			assertArgv(t, base.File("/c.mp4").Build(), append(want, "ALL", "/c.mp4")...)
		})
	}
}

func TestSimpleBuilders(t *testing.T) {
	assertArgv(t, shell.XrandrCurrent(), "xrandr", "--current")
}

// pgrep and pkill only see this user's processes: another user logged in on
// the same machine may run the same players.
func TestProcessBuildersAreScopedToUser(t *testing.T) {
	uid := strconv.Itoa(os.Getuid())
	assertArgv(t, shell.Pgrep("mpvpaper"), "pgrep", "-u", uid, "-x", "mpvpaper")
	assertArgv(t, shell.PgrepList("pcmanfm"), "pgrep", "-u", uid, "-a", "-x", "pcmanfm")
	assertArgv(t, shell.Pkill("mpvpaper"), "pkill", "-u", uid, "-x", "mpvpaper")
}
