// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

//go:build wasip1

package main

import (
	"io"
	"os"
)

// main answers one call: argv[1] names the operation, stdin carries the
// request and stdout receives the envelope. The exit code is always 0; the
// envelope says whether the operation succeeded.
func main() {
	op := ""
	if len(os.Args) > 1 {
		op = os.Args[1]
	}
	request, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Stdout.Write(encode(envelope{Error: "lazywal: reading the request: " + err.Error()}))
		return
	}
	os.Stdout.Write(dispatch(op, request))
}
