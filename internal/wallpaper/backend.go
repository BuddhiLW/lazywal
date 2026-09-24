package wallpaper

import (
	"errors"
	"strings"
)

// Backend puts a video on the desktop background of some kind of session.
// It is the port every adapter under internal/backend implements.
//
// Contract (enforced for every implementation by backendtest.Contract, so
// any Backend can stand in for any other):
//
//   - Name is stable, unique, lower-case and contains no spaces.
//   - Describe is a non-empty one-line summary.
//   - Check has no side effects on the desktop. It returns nil when the
//     backend can run, or a *MissingDepsError naming every missing
//     dependency, not just the first one found.
//   - Set plays v, replacing whatever this backend showed before. It never
//     requires a Clear first, and calling it repeatedly is fine.
//   - Clear stops the wallpaper and undoes every desktop change Set made.
//     It succeeds when nothing is set and is safe to call repeatedly.
type Backend interface {
	Name() string
	Describe() string
	Check() error
	Set(v Video) error
	Clear() error
}

// MissingDepsError lists everything a backend still needs.
type MissingDepsError struct{ Deps []string }

func (e *MissingDepsError) Error() string {
	return "missing dependencies: " + strings.Join(e.Deps, ", ")
}

// Requirements accumulates missing dependencies for a Check.
type Requirements struct {
	lookPath func(program string) error
	missing  []string
}

// Require starts a dependency check that finds programs with lookPath.
func Require(lookPath func(program string) error) *Requirements {
	return &Requirements{lookPath: lookPath}
}

// Programs records each program not installed; it reports whether all were found.
func (r *Requirements) Programs(names ...string) bool {
	ok := true
	for _, name := range names {
		if r.lookPath(name) != nil {
			r.missing = append(r.missing, name)
			ok = false
		}
	}
	return ok
}

// That records what as missing unless ok; it returns ok.
func (r *Requirements) That(ok bool, what string) bool {
	if !ok {
		r.missing = append(r.missing, what)
	}
	return ok
}

// Include merges the missing dependencies from another Check result. A nil
// err adds nothing; an error that is not a *MissingDepsError is added as text.
func (r *Requirements) Include(err error) {
	if err == nil {
		return
	}
	var m *MissingDepsError
	if errors.As(err, &m) {
		r.missing = append(r.missing, m.Deps...)
		return
	}
	r.missing = append(r.missing, err.Error())
}

// Err returns a *MissingDepsError listing everything missing, or nil.
func (r *Requirements) Err() error {
	if len(r.missing) == 0 {
		return nil
	}
	return &MissingDepsError{Deps: append([]string(nil), r.missing...)}
}
