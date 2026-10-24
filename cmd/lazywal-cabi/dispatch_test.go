// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

type decoded struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value"`
	Error string          `json:"error"`
}

func call(t *testing.T, op, request string) decoded {
	t.Helper()
	var d decoded
	if err := json.Unmarshal(dispatch(op, []byte(request)), &d); err != nil {
		t.Fatalf("%s: envelope is not JSON: %v", op, err)
	}
	return d
}

func TestUnknownOperationIsAnErrorEnvelope(t *testing.T) {
	d := call(t, "nope", "{}")
	if d.OK || !strings.Contains(d.Error, `unknown operation "nope"`) {
		t.Fatalf("got %+v", d)
	}
}

func TestOpsListsTheSameCatalogInEveryBuild(t *testing.T) {
	d := call(t, "ops", "")
	var ops []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(d.Value, &ops); err != nil || !d.OK {
		t.Fatalf("got %+v, %v", d, err)
	}
	var names []string
	for _, op := range ops {
		names = append(names, op.Name)
	}
	if got := strings.Join(names, ","); got != "backends,candidates,detect_session,ops" {
		t.Fatalf("names = %s", got)
	}
}

func TestDetectSessionReadsTheSuppliedEnvironment(t *testing.T) {
	d := call(t, "detect_session", `{"env":{"XDG_SESSION_TYPE":"wayland","XDG_CURRENT_DESKTOP":"ubuntu:GNOME"}}`)
	var s struct {
		Type     string   `json:"type"`
		Desktops []string `json:"desktops"`
		Inferred bool     `json:"inferred"`
		Summary  string   `json:"summary"`
	}
	if err := json.Unmarshal(d.Value, &s); err != nil || !d.OK {
		t.Fatalf("got %+v, %v", d, err)
	}
	if s.Type != "wayland" || strings.Join(s.Desktops, ":") != "ubuntu:gnome" || s.Inferred {
		t.Fatalf("got %+v", s)
	}
}

func TestDetectSessionWithAnEmptyEnvironmentIsInferredAndUnknown(t *testing.T) {
	d := call(t, "detect_session", `{"env":{}}`)
	if !d.OK || !strings.Contains(string(d.Value), `"summary":"session=unknown desktop=none"`) {
		t.Fatalf("got %+v", d)
	}
}

func TestCandidatesRanksBackendsForAGnomeWaylandSession(t *testing.T) {
	d := call(t, "candidates", `{"env":{"XDG_SESSION_TYPE":"wayland","XDG_CURRENT_DESKTOP":"GNOME"}}`)
	var c struct {
		Candidates []struct {
			Name string `json:"name"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(d.Value, &c); err != nil || !d.OK {
		t.Fatalf("got %+v, %v", d, err)
	}
	if len(c.Candidates) == 0 {
		t.Fatalf("no candidates for gnome on wayland: %s", d.Value)
	}
}
