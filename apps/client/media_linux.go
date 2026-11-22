//go:build !android

package main

/*
#cgo pkg-config: libpulse-simple
#include <stdlib.h>
#include <pulse/simple.h>
#include <pulse/error.h>

static pa_simple *open_stream(pa_stream_direction_t dir, int rate, int channels, int frame_bytes, int *error) {
	pa_sample_spec spec = {
		.format = PA_SAMPLE_S16LE,
		.rate = rate,
		.channels = channels,
	};
	// Small buffers: a call wants sound now, not smooth.
	pa_buffer_attr attr = {
		.maxlength = (uint32_t)-1,
		.tlength = frame_bytes * 4,
		.prebuf = (uint32_t)-1,
		.minreq = (uint32_t)-1,
		.fragsize = frame_bytes,
	};
	return pa_simple_new(NULL, "Go Tox", dir, NULL,
		dir == PA_STREAM_RECORD ? "Call (microphone)" : "Call", &spec, NULL, &attr, error);
}
*/
import "C"

import (
	"errors"
	"log"
	"sync"
	"unsafe"
)

// pulseMedia is sound through PulseAudio, or PipeWire's PulseAudio server.
type pulseMedia struct{}

func newMedia() media { return pulseMedia{} }

func pulseError(code C.int) error {
	return errors.New("PulseAudio: " + C.GoString(C.pa_strerror(code)))
}

// pulseAudio records in a goroutine, and plays on a stream that it opens for
// the rate and channels of what comes.
type pulseAudio struct {
	record *C.pa_simple
	done   chan struct{}
	ended  chan struct{}

	mu       sync.Mutex
	out      *C.pa_simple
	rate     int
	channels int
	closed   bool
}

func (pulseMedia) openAudio(record func([]int16)) (audioIO, error) {
	var code C.int
	rec := C.open_stream(C.PA_STREAM_RECORD, audioRate, 1, audioFrame*2, &code)
	if rec == nil {
		return nil, pulseError(code)
	}
	a := &pulseAudio{record: rec, done: make(chan struct{}), ended: make(chan struct{})}
	go func() {
		defer close(a.ended)
		pcm := make([]int16, audioFrame)
		for {
			select {
			case <-a.done:
				return
			default:
			}
			var code C.int
			if C.pa_simple_read(rec, unsafe.Pointer(&pcm[0]), C.size_t(len(pcm)*2), &code) < 0 {
				log.Print(pulseError(code))
				return
			}
			record(pcm)
		}
	}()
	return a, nil
}

func (a *pulseAudio) play(pcm []int16, channels, rate int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || len(pcm) == 0 {
		return
	}
	if a.out == nil || a.rate != rate || a.channels != channels {
		if a.out != nil {
			C.pa_simple_free(a.out)
		}
		var code C.int
		a.out = C.open_stream(C.PA_STREAM_PLAYBACK, C.int(rate), C.int(channels), C.int(rate/50*channels*2), &code)
		if a.out == nil {
			log.Print(pulseError(code))
			return
		}
		a.rate, a.channels = rate, channels
	}
	var code C.int
	if C.pa_simple_write(a.out, unsafe.Pointer(&pcm[0]), C.size_t(len(pcm)*2), &code) < 0 {
		log.Print(pulseError(code))
	}
}

func (a *pulseAudio) Close() error {
	close(a.done)
	<-a.ended
	C.pa_simple_free(a.record)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	if a.out != nil {
		C.pa_simple_free(a.out)
		a.out = nil
	}
	return nil
}
