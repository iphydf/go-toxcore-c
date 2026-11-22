package tox

/*
#include <stdint.h>
#include <stdlib.h>
#include <tox/tox.h>

extern void goToxLog(uintptr_t handle, Tox_Log_Level level, char *file, uint32_t line, char *func, char *message);

static void tox_go_log(Tox *tox, Tox_Log_Level level, const char *file, uint32_t line, const char *func,
                       const char *message, void *user_data)
{
    goToxLog((uintptr_t)user_data, level, (char *)file, line, (char *)func, (char *)message);
}

static void tox_go_set_log(Tox_Options *options, uintptr_t handle)
{
    tox_options_set_log_callback(options, tox_go_log);
    tox_options_set_log_user_data(options, (void *)handle);
}
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"
)

type Options struct {
	IPv6Enabled             bool
	UDPEnabled              bool
	LocalDiscoveryEnabled   bool
	DHTAnnouncementsEnabled bool

	ProxyType ProxyType
	ProxyHost string
	ProxyPort Port

	StartPort Port
	EndPort   Port
	TCPPort   Port

	HolePunchingEnabled bool

	SavedataType SavedataType
	SavedataData []byte

	LogCallback func(level LogLevel, file string, line uint32, function, message string)

	ExperimentalGroupsPersistence bool
	ExperimentalDisableDNS        bool
}

type ProxyType uint32

const (
	ProxyTypeNone   ProxyType = C.TOX_PROXY_TYPE_NONE
	ProxyTypeHTTP   ProxyType = C.TOX_PROXY_TYPE_HTTP
	ProxyTypeSOCKS5 ProxyType = C.TOX_PROXY_TYPE_SOCKS5
)

func (t ProxyType) String() string {
	return enumString(C.tox_proxy_type_to_string(C.Tox_Proxy_Type(t)), "TOX_PROXY_TYPE_", "ProxyType", uint32(t))
}

type SavedataType uint32

const (
	SavedataTypeNone      SavedataType = C.TOX_SAVEDATA_TYPE_NONE
	SavedataTypeToxSave   SavedataType = C.TOX_SAVEDATA_TYPE_TOX_SAVE
	SavedataTypeSecretKey SavedataType = C.TOX_SAVEDATA_TYPE_SECRET_KEY
)

func (t SavedataType) String() string {
	return enumString(C.tox_savedata_type_to_string(C.Tox_Savedata_Type(t)), "TOX_SAVEDATA_TYPE_", "SavedataType", uint32(t))
}

type LogLevel uint32

const (
	LogLevelTrace   LogLevel = C.TOX_LOG_LEVEL_TRACE
	LogLevelDebug   LogLevel = C.TOX_LOG_LEVEL_DEBUG
	LogLevelInfo    LogLevel = C.TOX_LOG_LEVEL_INFO
	LogLevelWarning LogLevel = C.TOX_LOG_LEVEL_WARNING
	LogLevelError   LogLevel = C.TOX_LOG_LEVEL_ERROR
)

func (l LogLevel) String() string {
	return enumString(C.tox_log_level_to_string(C.Tox_Log_Level(l)), "TOX_LOG_LEVEL_", "LogLevel", uint32(l))
}

func DefaultOptions() *Options {
	o := newCOptions()
	defer C.tox_options_free(o)
	return &Options{
		IPv6Enabled:                   bool(C.tox_options_get_ipv6_enabled(o)),
		UDPEnabled:                    bool(C.tox_options_get_udp_enabled(o)),
		LocalDiscoveryEnabled:         bool(C.tox_options_get_local_discovery_enabled(o)),
		DHTAnnouncementsEnabled:       bool(C.tox_options_get_dht_announcements_enabled(o)),
		ProxyType:                     ProxyType(C.tox_options_get_proxy_type(o)),
		ProxyPort:                     Port(C.tox_options_get_proxy_port(o)),
		StartPort:                     Port(C.tox_options_get_start_port(o)),
		EndPort:                       Port(C.tox_options_get_end_port(o)),
		TCPPort:                       Port(C.tox_options_get_tcp_port(o)),
		HolePunchingEnabled:           bool(C.tox_options_get_hole_punching_enabled(o)),
		SavedataType:                  SavedataType(C.tox_options_get_savedata_type(o)),
		ExperimentalGroupsPersistence: bool(C.tox_options_get_experimental_groups_persistence(o)),
		ExperimentalDisableDNS:        bool(C.tox_options_get_experimental_disable_dns(o)),
	}
}

// newCOptions allocates toxcore's options. Like Go's own allocation, it
// panics when there is no memory.
func newCOptions() *C.Tox_Options {
	var err C.Tox_Err_Options_New
	o := C.tox_options_new(&err)
	if o == nil {
		panic(errorString(C.tox_err_options_new_to_string(err), "TOX_ERR_OPTIONS_NEW_", uint32(err)))
	}
	return o
}

// toC makes toxcore's options, with copies of the savedata and proxy host. The
// caller frees them with C.tox_options_free.
func (opts *Options) toC(log cgo.Handle) (*C.Tox_Options, error) {
	o := newCOptions()
	C.tox_options_set_experimental_owned_data(o, true)
	C.tox_options_set_ipv6_enabled(o, C.bool(opts.IPv6Enabled))
	C.tox_options_set_udp_enabled(o, C.bool(opts.UDPEnabled))
	C.tox_options_set_local_discovery_enabled(o, C.bool(opts.LocalDiscoveryEnabled))
	C.tox_options_set_dht_announcements_enabled(o, C.bool(opts.DHTAnnouncementsEnabled))
	C.tox_options_set_proxy_type(o, C.Tox_Proxy_Type(opts.ProxyType))
	C.tox_options_set_proxy_port(o, C.uint16_t(opts.ProxyPort))
	C.tox_options_set_start_port(o, C.uint16_t(opts.StartPort))
	C.tox_options_set_end_port(o, C.uint16_t(opts.EndPort))
	C.tox_options_set_tcp_port(o, C.uint16_t(opts.TCPPort))
	C.tox_options_set_hole_punching_enabled(o, C.bool(opts.HolePunchingEnabled))
	C.tox_options_set_experimental_groups_persistence(o, C.bool(opts.ExperimentalGroupsPersistence))
	C.tox_options_set_experimental_disable_dns(o, C.bool(opts.ExperimentalDisableDNS))
	C.tox_options_set_savedata_type(o, C.Tox_Savedata_Type(opts.SavedataType))
	if log != 0 {
		C.tox_go_set_log(o, C.uintptr_t(log))
	}

	ok := C.tox_options_set_savedata_data(o, cBytes(opts.SavedataData), C.size_t(len(opts.SavedataData)))
	if ok && opts.ProxyHost != "" {
		host := C.CString(opts.ProxyHost)
		ok = C.tox_options_set_proxy_host(o, host)
		C.free(unsafe.Pointer(host))
	}
	if !ok {
		C.tox_options_free(o)
		return nil, ErrNewMalloc
	}
	return o, nil
}
