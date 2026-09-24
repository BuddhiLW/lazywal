package wallpaper

import (
	"errors"
	"fmt"
)

// State persists what lazywal did across invocations. bonzai's injson
// persister satisfies it.
type State interface {
	Get(key string) string
	Set(key, value string)
}

// State keys.
const (
	StateBackend = "lazywal_backend" // name of the backend that set the wallpaper
	StateVideo   = "lazywal_path"    // path of the current wallpaper
)

// Service is the application's entry point for wallpaper use cases. It
// selects a backend for the session and remembers which one is active, so
// clearing and switching backends always undo the right thing.
type Service struct {
	Registry *Registry
	State    State
	Session  Session
	// Override names a backend to use instead of selecting one.
	Override string
	// Logf, when set, receives progress messages.
	Logf func(format string, args ...any)
}

func (s *Service) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// Backend returns the backend the session would use. When nothing declared
// the session (see Session.Inferred), the backend that set the current
// wallpaper is kept if it is still ready: a cron job inside Xfce must not
// switch to bare xwinwrap and undo the desktop integration.
func (s *Service) Backend() (Backend, error) {
	if name := s.State.Get(StateBackend); s.Override == "" && s.Session.Inferred && name != "" &&
		s.Registry.SupportsType(name, s.Session.Type) {
		if prev, err := s.Registry.Get(name); err == nil && prev.Check() == nil {
			return prev, nil
		}
	}
	return s.Registry.Select(s.Session, s.Override)
}

// clearOthers stops whatever the recorded backend, or a backend an override
// shadowed under that name, may still be showing, except next itself.
func (s *Service) clearOthers(prev string, next Backend) {
	if prev == "" {
		return
	}
	var stale []Backend
	if prev != next.Name() {
		if old, err := s.Registry.Get(prev); err == nil {
			stale = append(stale, old)
		} else {
			s.logf("Previous backend %s is no longer available; anything it started may still be running", prev)
		}
	}
	stale = append(stale, s.Registry.Shadowed(prev)...)
	for _, old := range stale {
		if err := old.Clear(); err != nil {
			s.logf("Clearing previous backend %s: %v", old.Name(), err)
		}
	}
}

// Set plays v and returns the backend used.
func (s *Service) Set(v Video) (Backend, error) {
	if v.IsZero() {
		return nil, errors.New("no video given")
	}
	b, err := s.Backend()
	if err != nil {
		return nil, err
	}
	if err := b.Check(); err != nil {
		return nil, fmt.Errorf("backend %s: %w", b.Name(), err)
	}

	// A different backend may still be showing a video (after an override,
	// a session change, or a backends.json entry replacing a built-in).
	s.clearOthers(s.State.Get(StateBackend), b)

	s.logf("Using backend: %s", b.Name())
	// Record b before Set: if Set fails halfway (e.g. one monitor started),
	// a later Clear must still reach b. Clear is safe when nothing is set.
	s.State.Set(StateBackend, b.Name())
	if err := b.Set(v); err != nil {
		return b, fmt.Errorf("backend %s: %w", b.Name(), err)
	}
	s.State.Set(StateVideo, v.Path())
	return b, nil
}

// Clear stops the wallpaper through the backend that set it (or, if none is
// recorded, the one the session would use) and returns that backend. It
// returns (nil, nil) when nothing is recorded and no backend fits the
// session: there is nothing lazywal could have set.
func (s *Service) Clear() (Backend, error) {
	name := s.State.Get(StateBackend)
	var b Backend
	if name != "" {
		b, _ = s.Registry.Get(name)
	}
	if b == nil {
		// A backend that is not ready can still clear what it may have left.
		b, _ = s.Backend()
		if b == nil {
			if name != "" {
				return nil, fmt.Errorf("backend %s that set the wallpaper is no longer available", name)
			}
			return nil, nil
		}
	}
	var errs []error
	if err := b.Clear(); err != nil {
		errs = append(errs, fmt.Errorf("backend %s: %w", b.Name(), err))
	}
	for _, old := range s.Registry.Shadowed(b.Name()) {
		if err := old.Clear(); err != nil {
			errs = append(errs, fmt.Errorf("shadowed backend %s: %w", old.Name(), err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return b, err
	}
	s.State.Set(StateBackend, "")
	return b, nil
}

// Current returns the video last set.
func (s *Service) Current() (Video, error) {
	path := s.State.Get(StateVideo)
	if path == "" {
		return Video{}, errors.New("no current wallpaper; run lazywal set <path> first")
	}
	return NewVideo(path)
}
