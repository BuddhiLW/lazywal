// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

//go:build js || wasip1

package main

import (
	"context"
	"encoding/json"
	"errors"
)

// errNeedsProcesses answers the backend operations on wasm: wiring a backend
// needs internal/shell, which uses process groups that wasm does not have.
var errNeedsProcesses = errors.New("needs the C ABI build: lazywal's backends start and inspect processes")

func unavailable(context.Context, json.RawMessage) (any, error) { return nil, errNeedsProcesses }

func init() {
	operations["candidates"] = operation{Doc: candidatesDoc, Pure: true, handle: unavailable}
	operations["backends"] = operation{Doc: backendsDoc, Pure: false, handle: unavailable}
}
