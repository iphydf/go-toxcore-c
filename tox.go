package tox

/*
#include <stdlib.h>
#include <tox/tox.h>
#include <tox/tox_events.h>
*/
import "C"

import (
	"runtime/cgo"
	"time"
	"unsafe"

	"github.com/TokTok/go-toxcore-c/internal/handle"
)

type Tox struct {
	// h guards every call into toxcore, which isn't thread-safe.
	h *handle.Handle
	// log holds Options.LogCallback while toxcore can call it.
	log cgo.Handle
}

func init() {
	handle.Of = func(tox any) *handle.Handle { return tox.(*Tox).h }
}

func New(opts *Options) (*Tox, error) {
	if opts == nil {
		opts = DefaultOptions()
	}
	var log cgo.Handle
	if opts.LogCallback != nil {
		log = cgo.NewHandle(opts.LogCallback)
	}
	o, err := opts.toC(log)
	if err != nil {
		deleteHandle(log)
		return nil, err
	}
	defer C.tox_options_free(o)

	var cerr C.Tox_Err_New
	p := C.tox_new(o, &cerr)
	if p == nil {
		deleteHandle(log)
		return nil, toError[NewError](uint32(cerr))
	}
	C.tox_events_init(p)
	return &Tox{h: handle.New(unsafe.Pointer(p)), log: log}, nil
}

func deleteHandle(h cgo.Handle) {
	if h != 0 {
		h.Delete()
	}
}

func (t *Tox) Close() {
	if t.h.Close(func(p unsafe.Pointer) { C.tox_kill((*C.Tox)(p)) }) {
		deleteHandle(t.log)
	}
}

// lock takes the mutex and returns the instance. The caller calls t.h.Unlock.
func (t *Tox) lock() *C.Tox {
	return (*C.Tox)(t.h.Lock())
}

func (t *Tox) Savedata() []byte {
	p := t.lock()
	defer t.h.Unlock()
	data := make([]byte, C.tox_get_savedata_size(p))
	if len(data) != 0 {
		C.tox_get_savedata(p, cOut(data))
	}
	return data
}

func (t *Tox) Bootstrap(host string, port Port, key DhtID) error {
	h := C.CString(host)
	defer C.free(unsafe.Pointer(h))
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Bootstrap
	C.tox_bootstrap(p, h, C.uint16_t(port), cBytes(key[:]), &err)
	return toError[BootstrapError](uint32(err))
}

func (t *Tox) AddTCPRelay(host string, port Port, key DhtID) error {
	h := C.CString(host)
	defer C.free(unsafe.Pointer(h))
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Bootstrap
	C.tox_add_tcp_relay(p, h, C.uint16_t(port), cBytes(key[:]), &err)
	return toError[BootstrapError](uint32(err))
}

func (t *Tox) IterationInterval() time.Duration {
	p := t.lock()
	defer t.h.Unlock()
	return time.Duration(C.tox_iteration_interval(p)) * time.Millisecond
}

func (t *Tox) Address() Address {
	p := t.lock()
	defer t.h.Unlock()
	var a Address
	C.tox_self_get_address(p, cOut(a[:]))
	return a
}

func (t *Tox) SetNospam(nospam uint32) {
	p := t.lock()
	defer t.h.Unlock()
	C.tox_self_set_nospam(p, C.uint32_t(nospam))
}

func (t *Tox) Nospam() uint32 {
	p := t.lock()
	defer t.h.Unlock()
	return uint32(C.tox_self_get_nospam(p))
}

func (t *Tox) PublicKey() PublicKey {
	p := t.lock()
	defer t.h.Unlock()
	var k PublicKey
	C.tox_self_get_public_key(p, cOut(k[:]))
	return k
}

func (t *Tox) SecretKey() SecretKey {
	p := t.lock()
	defer t.h.Unlock()
	var k SecretKey
	C.tox_self_get_secret_key(p, cOut(k[:]))
	return k
}

func (t *Tox) SetName(name string) error {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Set_Info
	C.tox_self_set_name(p, cString(name), C.size_t(len(name)), &err)
	return toError[SetInfoError](uint32(err))
}

func (t *Tox) Name() string {
	p := t.lock()
	defer t.h.Unlock()
	return readString(C.tox_self_get_name_size(p), func(buf *C.uint8_t) {
		C.tox_self_get_name(p, buf)
	})
}

func (t *Tox) SetStatusMessage(message string) error {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Set_Info
	C.tox_self_set_status_message(p, cString(message), C.size_t(len(message)), &err)
	return toError[SetInfoError](uint32(err))
}

func (t *Tox) StatusMessage() string {
	p := t.lock()
	defer t.h.Unlock()
	return readString(C.tox_self_get_status_message_size(p), func(buf *C.uint8_t) {
		C.tox_self_get_status_message(p, buf)
	})
}

func (t *Tox) SetStatus(status UserStatus) {
	p := t.lock()
	defer t.h.Unlock()
	C.tox_self_set_status(p, C.Tox_User_Status(status))
}

func (t *Tox) Status() UserStatus {
	p := t.lock()
	defer t.h.Unlock()
	return UserStatus(C.tox_self_get_status(p))
}

func (t *Tox) DhtID() DhtID {
	p := t.lock()
	defer t.h.Unlock()
	var k DhtID
	C.tox_self_get_dht_id(p, cOut(k[:]))
	return k
}

func (t *Tox) UDPPort() (Port, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Get_Port
	port := C.tox_self_get_udp_port(p, &err)
	return Port(port), toError[GetPortError](uint32(err))
}

func (t *Tox) TCPPort() (Port, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Get_Port
	port := C.tox_self_get_tcp_port(p, &err)
	return Port(port), toError[GetPortError](uint32(err))
}
