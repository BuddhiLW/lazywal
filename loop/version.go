package loop

import "runtime/debug"

// Version is stamped at build time by the Makefile and release workflow:
//
//	go build -ldflags "-X github.com/BuddhiLW/lazywal/loop.Version=v1.5.0"
//
// When empty, the module version recorded by `go install ...@vX.Y.Z` (or the
// VCS-derived version of a local build) is used, so it can't drift from the
// release tag the way a hard-coded string did.
var Version = ""

func version() string {
	if Version != "" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
