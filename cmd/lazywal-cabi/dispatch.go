// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

// Command lazywal-cabi exposes lazywal to hosts that load it in-process
// instead of running the lazywal CLI. Every entry point is the same call: an
// operation name and a JSON request in, a JSON envelope out.
//
//	{"ok":true,"value":...}   the operation succeeded
//	{"ok":false,"error":"…"}  it did not; nothing panics across the boundary
//
// One source tree builds three artifacts, one per kind of host:
//
//	go build -buildmode=c-shared -o liblazywal.so ./cmd/lazywal-cabi
//	    C ABI: hive_call(op, json) -> json, hive_free(ptr).
//	GOOS=js GOARCH=wasm go build -o lazywal.js.wasm ./cmd/lazywal-cabi
//	    globalThis.hive_lazywal(op, json) -> json.
//	GOOS=wasip1 GOARCH=wasm go build -o lazywal.wasi.wasm ./cmd/lazywal-cabi
//	    WASI command: argv[1] = op, stdin = request, stdout = envelope.
//
// Only the C ABI build can wire lazywal's backends, which start and inspect
// processes; on wasm those operations answer with an error naming why. The
// catalog is the same in every build, so a host can list before it calls.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/BuddhiLW/lazywal/internal/wallpaper"
)

// handler runs one operation on its decoded request.
type handler func(ctx context.Context, request json.RawMessage) (any, error)

// operation is one entry of the catalog a host can list and call.
type operation struct {
	Doc    string
	Pure   bool
	handle handler
}

// envelope is the only shape that crosses the boundary.
type envelope struct {
	OK    bool   `json:"ok"`
	Value any    `json:"value,omitempty"`
	Error string `json:"error,omitempty"`
}

// defaultTimeout bounds an operation whose request names no timeout_ms.
const defaultTimeout = 30 * time.Second

var operations = map[string]operation{
	"detect_session": {
		Doc:    "Classify a graphical session (type and desktops) from environment variables.",
		Pure:   true,
		handle: detectSession,
	},
}

// "ops" reads the table it belongs to, so it joins after initialization.
func init() {
	operations["ops"] = operation{
		Doc:    "List the operations this library answers.",
		Pure:   true,
		handle: func(context.Context, json.RawMessage) (any, error) { return catalog(), nil },
	}
}

// catalog describes every operation, sorted by name so it reads the same on
// every host.
func catalog() []map[string]any {
	names := make([]string, 0, len(operations))
	for name := range operations {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		op := operations[name]
		out = append(out, map[string]any{"name": name, "doc": op.Doc, "pure": op.Pure})
	}
	return out
}

// dispatch runs op on request and always returns an encoded envelope.
func dispatch(op string, request []byte) []byte {
	return encode(run(op, request))
}

func run(name string, request []byte) (env envelope) {
	defer func() {
		if r := recover(); r != nil {
			env = envelope{Error: fmt.Sprintf("lazywal: %s panicked: %v", name, r)}
		}
	}()

	op, ok := operations[name]
	if !ok {
		return envelope{Error: fmt.Sprintf("lazywal: unknown operation %q", name)}
	}
	if len(request) == 0 {
		request = []byte("{}")
	}
	if !json.Valid(request) {
		return envelope{Error: fmt.Sprintf("lazywal: %s: request is not valid JSON", name)}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeoutOf(request))
	defer cancel()
	value, err := op.handle(ctx, request)
	if err != nil {
		return envelope{Error: fmt.Sprintf("lazywal: %s: %v", name, err)}
	}
	return envelope{OK: true, Value: value}
}

// timeoutOf reads an optional timeout_ms from any request.
func timeoutOf(request []byte) time.Duration {
	var bounded struct {
		TimeoutMS int64 `json:"timeout_ms"`
	}
	if json.Unmarshal(request, &bounded) == nil && bounded.TimeoutMS > 0 {
		return time.Duration(bounded.TimeoutMS) * time.Millisecond
	}
	return defaultTimeout
}

func encode(env envelope) []byte {
	out, err := json.Marshal(env)
	if err != nil {
		out, _ = json.Marshal(envelope{Error: fmt.Sprintf("lazywal: encoding the result: %v", err)})
	}
	return out
}

// sessionRequest names the environment to classify. A request without "env"
// reads the process environment instead.
type sessionRequest struct {
	Env      map[string]string `json:"env"`
	Override string            `json:"override"`
}

func (r sessionRequest) session() wallpaper.Session {
	if r.Env == nil {
		return wallpaper.DetectSession(os.Getenv)
	}
	return wallpaper.DetectSession(func(key string) string { return r.Env[key] })
}

func describeSession(s wallpaper.Session) map[string]any {
	desktops := s.Desktops
	if desktops == nil {
		desktops = []string{}
	}
	return map[string]any{
		"type":     string(s.Type),
		"desktops": desktops,
		"inferred": s.Inferred,
		"summary":  s.String(),
	}
}

func detectSession(_ context.Context, raw json.RawMessage) (any, error) {
	var req sessionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	return describeSession(req.session()), nil
}
