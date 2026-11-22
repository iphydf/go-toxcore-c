package tox

/*
#include <stdint.h>
#include <tox/tox.h>
*/
import "C"

import "runtime/cgo"

// goToxLog receives toxcore's log messages for Options.LogCallback.
//
//export goToxLog
func goToxLog(handle C.uintptr_t, level C.Tox_Log_Level, file *C.char, line C.uint32_t, function, message *C.char) {
	log := cgo.Handle(handle).Value().(func(LogLevel, string, uint32, string, string))
	log(LogLevel(level), C.GoString(file), uint32(line), C.GoString(function), C.GoString(message))
}
