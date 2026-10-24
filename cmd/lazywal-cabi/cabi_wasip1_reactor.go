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

// bootArena serves cabi_realloc calls that arrive before the Go runtime has
// initialised its heap. The wasi_snapshot_preview1 adapter allocates its
// state and stack lazily, on the first WASI import call, through the main
// module's cabi_realloc; for a Go reactor that first call is clock_time_get
// inside runtime.schedinit, during _initialize. A make() there throws (no
// heap yet), and the throw printer's fd_write re-enters the adapter, which
// calls cabi_realloc again: unbounded recursion. A static, pointer-free
// arena needs no runtime, so those early requests are bump-allocated here.
// The adapter keeps what it gets for the life of the instance, so nothing
// is ever freed back.
var (
	bootArena [256 << 10]byte
	bootUsed  uintptr
	heapReady bool // set by init, once the Go heap is usable
)

func init() { heapReady = true }

// bootAlloc returns an aligned block of bootArena, or 0 when it is full.
func bootAlloc(align, size uint32) uint32 {
	base := uintptr(unsafe.Pointer(&bootArena[0]))
	a := uintptr(align)
	if a == 0 {
		a = 1
	}
	start := (base + bootUsed + a - 1) &^ (a - 1)
	end := start + uintptr(size)
	if end > base+uintptr(len(bootArena)) {
		return 0
	}
	bootUsed = end - base
	return uint32(start)
}

//go:wasmexport cabi_realloc
func cabiRealloc(old, oldSize, align, newSize uint32) uint32 {
	if newSize == 0 {
		return align
	}
	if !heapReady && old == 0 {
		if p := bootAlloc(align, newSize); p != 0 {
			return p
		}
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
