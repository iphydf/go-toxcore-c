package toxav

/*
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
*/
import "C"

import (
	"runtime/cgo"
	"unsafe"

	tox "github.com/TokTok/go-toxcore-c"
)

type Event interface {
	event()
}

type EventCall struct {
	Friend       tox.Friend
	AudioEnabled bool
	VideoEnabled bool
}

type EventCallState struct {
	Friend tox.Friend
	State  FriendCallState
}

type EventAudioBitRate struct {
	Friend       tox.Friend
	AudioBitRate BitRate
}

type EventVideoBitRate struct {
	Friend       tox.Friend
	VideoBitRate BitRate
}

type EventAudioReceiveFrame struct {
	Friend       tox.Friend
	PCM          []int16
	Channels     Channels
	SamplingRate SamplingRate
}

type EventVideoReceiveFrame struct {
	Friend  tox.Friend
	Width   Width
	Height  Height
	Y, U, V []byte
}

func (*EventCall) event()              {}
func (*EventCallState) event()         {}
func (*EventAudioBitRate) event()      {}
func (*EventVideoBitRate) event()      {}
func (*EventAudioReceiveFrame) event() {}
func (*EventVideoReceiveFrame) event() {}

// queue adds an event for Iterate. The callbacks run in calls into toxcore,
// which hold the mutex that guards the queue.
func queue(handle C.uintptr_t, ev func(av *AV) Event) {
	av := cgo.Handle(handle).Value().(*AV)
	av.events = append(av.events, ev(av))
}

func friend(av *AV, n C.uint32_t) tox.Friend {
	return av.tox.Friend(tox.FriendNumber(n))
}

//export goToxavCall
func goToxavCall(handle C.uintptr_t, friendNumber C.uint32_t, audioEnabled, videoEnabled C.bool) {
	queue(handle, func(av *AV) Event {
		return &EventCall{friend(av, friendNumber), bool(audioEnabled), bool(videoEnabled)}
	})
}

//export goToxavCallState
func goToxavCallState(handle C.uintptr_t, friendNumber C.uint32_t, state C.uint32_t) {
	queue(handle, func(av *AV) Event {
		return &EventCallState{friend(av, friendNumber), FriendCallState(state)}
	})
}

//export goToxavAudioBitRate
func goToxavAudioBitRate(handle C.uintptr_t, friendNumber C.uint32_t, bitRate C.uint32_t) {
	queue(handle, func(av *AV) Event {
		return &EventAudioBitRate{friend(av, friendNumber), BitRate(bitRate)}
	})
}

//export goToxavVideoBitRate
func goToxavVideoBitRate(handle C.uintptr_t, friendNumber C.uint32_t, bitRate C.uint32_t) {
	queue(handle, func(av *AV) Event {
		return &EventVideoBitRate{friend(av, friendNumber), BitRate(bitRate)}
	})
}

//export goToxavAudioReceiveFrame
func goToxavAudioReceiveFrame(handle C.uintptr_t, friendNumber C.uint32_t, pcm *C.int16_t, sampleCount C.size_t,
	channels C.uint8_t, samplingRate C.uint32_t) {
	queue(handle, func(av *AV) Event {
		samples := make([]int16, int(sampleCount)*int(channels))
		if len(samples) != 0 {
			copy(samples, unsafe.Slice((*int16)(unsafe.Pointer(pcm)), len(samples)))
		}
		return &EventAudioReceiveFrame{friend(av, friendNumber), samples, Channels(channels), SamplingRate(samplingRate)}
	})
}

//export goToxavVideoReceiveFrame
func goToxavVideoReceiveFrame(handle C.uintptr_t, friendNumber C.uint32_t, width, height C.uint16_t,
	y, u, v *C.uint8_t, yStride, uStride, vStride C.int32_t) {
	queue(handle, func(av *AV) Event {
		w, h := int(width), int(height)
		cw, ch := (w+1)/2, (h+1)/2
		return &EventVideoReceiveFrame{
			Friend: friend(av, friendNumber),
			Width:  Width(width),
			Height: Height(height),
			Y:      copyPlane(y, int(yStride), w, h),
			U:      copyPlane(u, int(uStride), cw, ch),
			V:      copyPlane(v, int(vStride), cw, ch),
		}
	})
}

// copyPlane copies a plane without the padding at the end of its rows. A
// negative stride means the rows go up in memory.
func copyPlane(p *C.uint8_t, stride, width, height int) []byte {
	out := make([]byte, width*height)
	for row := range height {
		src := unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(p), row*stride)), width)
		copy(out[row*width:], src)
	}
	return out
}
