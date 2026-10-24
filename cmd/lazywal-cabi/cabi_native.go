// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

//go:build !js && !wasip1

package main

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

// hive_call runs op on the NUL-terminated JSON request and returns a
// NUL-terminated JSON envelope. The caller owns the result and releases it
// with hive_free; it never frees it with its own allocator.
//
//export hive_call
func hive_call(op *C.char, request *C.char) *C.char {
	return C.CString(string(dispatch(C.GoString(op), []byte(C.GoString(request)))))
}

// hive_free releases a string returned by hive_call.
//
//export hive_free
func hive_free(p *C.char) {
	C.free(unsafe.Pointer(p))
}

// main is required by -buildmode=c-shared and never runs.
func main() {}
