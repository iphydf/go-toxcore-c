package tox

/*
#cgo pkg-config: toxcore
#include <stdint.h>
*/
import "C"

import (
	"unsafe"

	"github.com/TokTok/go-toxcore-c/internal/cname"
)

// empty is what an empty input points to, as an empty slice has no address and
// toxcore only takes NULL where tox.h says _Nullable.
var empty C.uint8_t

// cBytes points C at b's bytes for the duration of a call.
func cBytes(b []byte) *C.uint8_t {
	if len(b) == 0 {
		return &empty
	}
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}

// cString points C at s's bytes for the duration of a call. The result is not
// NUL-terminated: toxcore takes a length with it.
func cString(s string) *C.uint8_t {
	if len(s) == 0 {
		return &empty
	}
	return (*C.uint8_t)(unsafe.Pointer(unsafe.StringData(s)))
}

// cOut points C at b as an output buffer.
func cOut(b []byte) *C.uint8_t {
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}

// goBytes copies length bytes from C memory.
func goBytes(p *C.uint8_t, length C.uint32_t) []byte {
	return C.GoBytes(unsafe.Pointer(p), C.int(length))
}

// goString copies length bytes from C memory into a string.
func goString(p *C.uint8_t, length C.uint32_t) string {
	return C.GoStringN((*C.char)(unsafe.Pointer(p)), C.int(length))
}

// copyFromC fills dst from C memory of the same size.
func copyFromC(dst []byte, p *C.uint8_t) {
	copy(dst, unsafe.Slice((*byte)(unsafe.Pointer(p)), len(dst)))
}

// readString runs a size getter and a getter for a byte string, under the lock
// the caller holds.
func readString(size C.size_t, get func(*C.uint8_t)) string {
	if size == 0 {
		return ""
	}
	buf := make([]byte, size)
	get(cOut(buf))
	return string(buf)
}

// enumString is the String of an enum type named typeName.
func enumString(s *C.char, prefix, typeName string, value uint32) string {
	return cname.String(C.GoString(s), prefix, typeName, value)
}
