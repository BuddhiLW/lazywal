package x11

import "github.com/BuddhiLW/lazywal/internal/shell"

// WindowMode decides how the xwinwrap window relates to the window manager.
// Which one is right depends on what else draws on the desktop, so it is a
// strategy chosen per backend rather than a flag baked into Xwinwrap.
type WindowMode interface {
	Apply(b shell.XwinwrapBuilder) shell.XwinwrapBuilder
}

var (
	// Unmanaged is an override-redirect window under everything. Right for
	// bare window managers (xmonad, i3...), where nothing else covers the
	// root window; on desktops with panels it covers the panels.
	Unmanaged WindowMode = unmanaged{}

	// DesktopWindow is a managed window of type desktop, which a desktop
	// environment's window manager keeps under its panels (verified under
	// Xfce, where Unmanaged covers them).
	DesktopWindow WindowMode = desktopWindow{}
)

type unmanaged struct{}

func (unmanaged) Apply(b shell.XwinwrapBuilder) shell.XwinwrapBuilder {
	return b.IgnoreInput().Below().SkipTaskbar().Undecorated().Opacity(1.0).OverrideRedirect()
}

type desktopWindow struct{}

func (desktopWindow) Apply(b shell.XwinwrapBuilder) shell.XwinwrapBuilder {
	return b.IgnoreInput().DesktopType().Below().NoFocus().Sticky().SkipTaskbar().SkipPager().Undecorated().Opacity(1.0)
}
