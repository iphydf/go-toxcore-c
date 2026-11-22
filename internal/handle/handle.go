// Package handle shares a Tox instance between the packages that call into
// it: its C pointer, and the mutex every call into toxcore takes.
package handle

import (
	"sync"
	"unsafe"
)

// A Handle is one Tox instance.
type Handle struct {
	mu  sync.Mutex
	ptr unsafe.Pointer
	// beforeClose has what must go before the instance, such as a ToxAV.
	beforeClose map[int]func()
	nextID      int
}

// Of returns the Handle of a *tox.Tox. Package tox sets it, since this package
// can't import it.
var Of func(tox any) *Handle

func New(ptr unsafe.Pointer) *Handle {
	return &Handle{ptr: ptr, beforeClose: map[int]func(){}}
}

// Lock takes the mutex and returns the instance. It panics if the instance is
// closed.
func (h *Handle) Lock() unsafe.Pointer {
	h.mu.Lock()
	if h.ptr == nil {
		h.mu.Unlock()
		panic("tox: use of a closed Tox")
	}
	return h.ptr
}

// LockOrNil takes the mutex and returns the instance, or nil if it is closed.
func (h *Handle) LockOrNil() unsafe.Pointer {
	h.mu.Lock()
	return h.ptr
}

func (h *Handle) Unlock() {
	h.mu.Unlock()
}

// BeforeClose makes Close call f, with the mutex held, before it kills the
// instance. The caller holds the mutex. The result cancels it, also with the
// mutex held.
func (h *Handle) BeforeClose(f func()) (cancel func()) {
	id := h.nextID
	h.nextID++
	h.beforeClose[id] = f
	return func() { delete(h.beforeClose, id) }
}

// Close calls the BeforeClose functions, then kill. It returns false if the
// instance was already closed.
func (h *Handle) Close(kill func(unsafe.Pointer)) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.ptr == nil {
		return false
	}
	for _, f := range h.beforeClose {
		f()
	}
	kill(h.ptr)
	h.ptr = nil
	return true
}
