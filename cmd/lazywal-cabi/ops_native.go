// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

//go:build !js && !wasip1

package main

import (
	"context"
	"encoding/json"
	"io"
	"log"

	"github.com/BuddhiLW/lazywal/internal/backend/builtin"
	"github.com/BuddhiLW/lazywal/internal/shell"
	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

func init() {
	operations["candidates"] = operation{Doc: candidatesDoc, Pure: true, handle: candidates}
	operations["backends"] = operation{Doc: backendsDoc, Pure: false, handle: backends}
}

// memoryStore keeps state for one call only: these operations read the
// registry and never set a wallpaper, so nothing needs to outlive the call.
type memoryStore map[string]string

func (m memoryStore) Get(key string) string { return m[key] }
func (m memoryStore) Set(key, value string) { m[key] = value }

// builtins wires lazywal's built-in backends the way the CLI does, minus the
// user's backends.json: the result depends on the request, not on $HOME.
func builtins() *wallpaper.Registry {
	reg := wallpaper.NewRegistry()
	quiet := log.New(io.Discard, "", 0)
	builtin.Register(reg, builtin.Deps{
		Runner: shell.Exec{Log: quiet},
		Store:  memoryStore{},
		Logf:   quiet.Printf,
	})
	return reg
}

// candidates applies the selection rules only. It runs no dependency check,
// so its answer is a function of the session and the override alone.
func candidates(_ context.Context, raw json.RawMessage) (any, error) {
	var req sessionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	session := req.session()
	reg := builtins()

	ranked := []map[string]any{}
	for _, b := range reg.Candidates(session) {
		ranked = append(ranked, map[string]any{"name": b.Name(), "describe": b.Describe()})
	}
	out := map[string]any{"session": describeSession(session), "candidates": ranked}
	if selected, err := reg.Select(session, req.Override); err != nil {
		out["selection_error"] = err.Error()
	} else {
		out["selected"] = selected.Name()
	}
	return out, nil
}

// backends lists every built-in backend with its readiness. Check looks up
// programs on PATH and never touches the desktop (the Backend contract).
func backends(_ context.Context, raw json.RawMessage) (any, error) {
	var req sessionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	session := req.session()
	reg := builtins()

	selectedName := ""
	if selected, err := reg.Select(session, req.Override); err == nil {
		selectedName = selected.Name()
	}
	rows := []map[string]any{}
	for _, b := range reg.All() {
		row := map[string]any{
			"name":     b.Name(),
			"describe": b.Describe(),
			"selected": b.Name() == selectedName,
			"ready":    true,
		}
		if err := b.Check(); err != nil {
			row["ready"] = false
			row["missing"] = err.Error()
		}
		rows = append(rows, row)
	}
	return map[string]any{"session": describeSession(session), "backends": rows}, nil
}
