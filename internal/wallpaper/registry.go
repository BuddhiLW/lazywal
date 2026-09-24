package wallpaper

import (
	"fmt"
	"sort"
	"strings"
)

// Rule says when a backend suits a session. Rules are data, so new backends
// (built-in or from the user's config) bring their own selection logic and
// the selection code never changes.
type Rule struct {
	// Session limits the rule to one session type; AnySession matches all.
	Session SessionType `json:"session,omitempty"`
	// Desktops matches when the session has any of these desktops
	// (lower-case XDG_CURRENT_DESKTOP entries). Empty matches any desktop,
	// including none.
	Desktops []string `json:"desktops,omitempty"`
	// Exclude rejects sessions having any of these desktops.
	Exclude []string `json:"exclude,omitempty"`
	// Priority orders matching backends; higher is tried first.
	Priority int `json:"priority,omitempty"`
}

// Matches reports whether the rule applies to s.
func (r Rule) Matches(s Session) bool {
	if r.Session != AnySession && r.Session != s.Type {
		return false
	}
	if len(r.Desktops) > 0 && !s.Is(lower(r.Desktops)...) {
		return false
	}
	return !s.Is(lower(r.Exclude)...)
}

func lower(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = strings.ToLower(n)
	}
	return out
}

type registration struct {
	backend Backend
	rules   []Rule
	order   int
}

// Registry holds the available backends and their selection rules.
type Registry struct {
	regs     map[string]*registration
	shadowed map[string][]Backend
	count    int
}

func NewRegistry() *Registry {
	return &Registry{regs: map[string]*registration{}, shadowed: map[string][]Backend{}}
}

// Register adds b with its rules. Registering a name again replaces the
// earlier backend and rules (so user config can override a built-in) while
// keeping its original tie-break position. The replaced backend is kept as
// shadowed: whatever it started before the override must still be stoppable.
func (r *Registry) Register(b Backend, rules ...Rule) {
	name := b.Name()
	if old, ok := r.regs[name]; ok {
		r.shadowed[name] = append(r.shadowed[name], old.backend)
		old.backend, old.rules = b, rules
		return
	}
	r.regs[name] = &registration{backend: b, rules: rules, order: r.count}
	r.count++
}

// Get returns the backend registered under name (case-insensitive).
func (r *Registry) Get(name string) (Backend, error) {
	if reg, ok := r.regs[strings.ToLower(strings.TrimSpace(name))]; ok {
		return reg.backend, nil
	}
	names := make([]string, 0, len(r.regs))
	for _, b := range r.All() {
		names = append(names, b.Name())
	}
	return nil, fmt.Errorf("unknown backend %q (available: %s)", name, strings.Join(names, ", "))
}

// Has reports whether a backend is registered under name.
func (r *Registry) Has(name string) bool {
	_, ok := r.regs[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// SupportsType reports whether the backend registered under name has a rule
// for sessions of type t, ignoring desktop filters.
func (r *Registry) SupportsType(name string, t SessionType) bool {
	reg, ok := r.regs[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return false
	}
	for _, rule := range reg.rules {
		if rule.Session == AnySession || rule.Session == t {
			return true
		}
	}
	return false
}

// Shadowed returns the backends that later registrations replaced under name,
// oldest first.
func (r *Registry) Shadowed(name string) []Backend {
	return append([]Backend(nil), r.shadowed[strings.ToLower(strings.TrimSpace(name))]...)
}

// All returns every backend, sorted by name.
func (r *Registry) All() []Backend {
	list := make([]Backend, 0, len(r.regs))
	for _, reg := range r.regs {
		list = append(list, reg.backend)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list
}

// Candidates returns the backends with a rule matching s, best first:
// highest matching priority, then registration order.
func (r *Registry) Candidates(s Session) []Backend {
	type scored struct {
		reg      *registration
		priority int
	}
	var matches []scored
	for _, reg := range r.regs {
		best, ok := 0, false
		for _, rule := range reg.rules {
			if rule.Matches(s) && (!ok || rule.Priority > best) {
				best, ok = rule.Priority, true
			}
		}
		if ok {
			matches = append(matches, scored{reg, best})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].priority != matches[j].priority {
			return matches[i].priority > matches[j].priority
		}
		return matches[i].reg.order < matches[j].reg.order
	})
	out := make([]Backend, len(matches))
	for i, m := range matches {
		out[i] = m.reg.backend
	}
	return out
}

// NotReadyError reports that no candidate backend can run in a session.
// Backend is the best candidate and Err its Check result, so the user
// learns what to install.
type NotReadyError struct {
	Session Session
	Backend Backend
	Err     error
}

func (e *NotReadyError) Error() string {
	return fmt.Sprintf("backend %s is not ready (%s): %v", e.Backend.Name(), e.Session, e.Err)
}

func (e *NotReadyError) Unwrap() error { return e.Err }

// Select picks the backend for s: the one named by override if given,
// otherwise the first candidate whose Check passes. When no candidate is
// ready it returns the best one along with a *NotReadyError.
func (r *Registry) Select(s Session, override string) (Backend, error) {
	if override != "" {
		return r.Get(override)
	}
	candidates := r.Candidates(s)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no backend supports this session (%s)", s)
	}
	var firstErr error
	for _, b := range candidates {
		err := b.Check()
		if err == nil {
			return b, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return candidates[0], &NotReadyError{Session: s, Backend: candidates[0], Err: firstErr}
}
