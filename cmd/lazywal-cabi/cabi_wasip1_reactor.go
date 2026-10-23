// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

//go:build wasip1

package main

// Reactor exports: the same dispatch as the WASI command, lifted through the
// Component Model Canonical ABI so a host can load the module ONCE and call
// it per operation. Built with
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o lazywal.reactor.wasm ./cmd/lazywal-cabi
//	wasm-tools component embed cmd/lazywal-cabi/wit lazywal.reactor.wasm -o lazywal.embed.wasm
//	wasm-tools component new lazywal.embed.wasm \
//	    --adapt wasi_snapshot_preview1=wasi_snapshot_preview1.reactor.wasm -o lazywal.component.wasm
//
// WIT (wit/lazywal.wit): export call: func(op: string, request: string) -> string.
// In a c-shared build main is never run; the command build ignores these exports.

import "unsafe"

// live keeps every buffer the host was handed reachable until the call that
// consumes it returns; Go's collector does not move objects, so the address
// stays valid while the slice is referenced here.
var live = map[uintptr][]byte{}

// retArea is the Canonical ABI return area for a string result: (ptr, len).
var retArea [2]uint32

// result is the envelope of the last call, alive until cabi_post_call.
var result []byte

func addr(b []byte) uintptr { return uintptr(unsafe.Pointer(unsafe.SliceData(b))) }

//go:wasmexport cabi_realloc
func cabiRealloc(old, oldSize, align, newSize uint32) uint32 {
	if newSize == 0 {
		return align
	}
	buf := make([]byte, newSize+align)
	start := addr(buf)
	off := (uintptr(align) - start%uintptr(align)) % uintptr(align)
	buf = buf[off : off+uintptr(newSize)]
	if old != 0 {
		if prev, ok := live[uintptr(old)]; ok {
			copy(buf, prev[:min(int(oldSize), len(prev))])
			delete(live, uintptr(old))
		}
	}
	live[addr(buf)] = buf
	return uint32(addr(buf))
}

func take(ptr, n uint32) []byte {
	if n == 0 {
		return nil
	}
	b := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), n)
	out := make([]byte, n)
	copy(out, b)
	delete(live, uintptr(ptr))
	return out
}

//go:wasmexport call
func reactorCall(opPtr, opLen, reqPtr, reqLen uint32) uint32 {
	op := string(take(opPtr, opLen))
	request := take(reqPtr, reqLen)
	result = dispatch(op, request)
	retArea[0] = uint32(addr(result))
	retArea[1] = uint32(len(result))
	return uint32(uintptr(unsafe.Pointer(&retArea[0])))
}

//go:wasmexport cabi_post_call
func cabiPostCall(uint32) { result = nil }
