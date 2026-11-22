package toxav

/*
#cgo pkg-config: toxcore
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <tox/toxav.h>

extern void goToxavCall(uintptr_t av, uint32_t friend_number, bool audio_enabled, bool video_enabled);
extern void goToxavCallState(uintptr_t av, uint32_t friend_number, uint32_t state);
extern void goToxavAudioBitRate(uintptr_t av, uint32_t friend_number, uint32_t audio_bit_rate);
extern void goToxavVideoBitRate(uintptr_t av, uint32_t friend_number, uint32_t video_bit_rate);
extern void goToxavAudioReceiveFrame(uintptr_t av, uint32_t friend_number, int16_t *pcm, size_t sample_count,
                                     uint8_t channels, uint32_t sampling_rate);
extern void goToxavVideoReceiveFrame(uintptr_t av, uint32_t friend_number, uint16_t width, uint16_t height,
                                     uint8_t *y, uint8_t *u, uint8_t *v, int32_t ystride, int32_t ustride,
                                     int32_t vstride);

static void toxav_go_call(ToxAV *av, Tox_Friend_Number friend_number, bool audio_enabled, bool video_enabled,
                          void *user_data)
{
    goToxavCall((uintptr_t)user_data, friend_number, audio_enabled, video_enabled);
}

static void toxav_go_call_state(ToxAV *av, Tox_Friend_Number friend_number, uint32_t state, void *user_data)
{
    goToxavCallState((uintptr_t)user_data, friend_number, state);
}

static void toxav_go_audio_bit_rate(ToxAV *av, Tox_Friend_Number friend_number, uint32_t audio_bit_rate,
                                    void *user_data)
{
    goToxavAudioBitRate((uintptr_t)user_data, friend_number, audio_bit_rate);
}

static void toxav_go_video_bit_rate(ToxAV *av, Tox_Friend_Number friend_number, uint32_t video_bit_rate,
                                    void *user_data)
{
    goToxavVideoBitRate((uintptr_t)user_data, friend_number, video_bit_rate);
}

static void toxav_go_audio_receive_frame(ToxAV *av, Tox_Friend_Number friend_number, const int16_t pcm[],
                                         size_t sample_count, uint8_t channels, uint32_t sampling_rate,
                                         void *user_data)
{
    goToxavAudioReceiveFrame((uintptr_t)user_data, friend_number, (int16_t *)pcm, sample_count, channels,
                             sampling_rate);
}

static void toxav_go_video_receive_frame(ToxAV *av, Tox_Friend_Number friend_number, uint16_t width,
                                         uint16_t height, const uint8_t y[], const uint8_t u[], const uint8_t v[],
                                         int32_t ystride, int32_t ustride, int32_t vstride, void *user_data)
{
    goToxavVideoReceiveFrame((uintptr_t)user_data, friend_number, width, height, (uint8_t *)y, (uint8_t *)u,
                             (uint8_t *)v, ystride, ustride, vstride);
}

// The callbacks only queue events, which Iterate returns.
static void toxav_go_register(ToxAV *av, uintptr_t handle)
{
    void *user_data = (void *)handle;
    toxav_callback_call(av, toxav_go_call, user_data);
    toxav_callback_call_state(av, toxav_go_call_state, user_data);
    toxav_callback_audio_bit_rate(av, toxav_go_audio_bit_rate, user_data);
    toxav_callback_video_bit_rate(av, toxav_go_video_bit_rate, user_data);
    toxav_callback_audio_receive_frame(av, toxav_go_audio_receive_frame, user_data);
    toxav_callback_video_receive_frame(av, toxav_go_video_receive_frame, user_data);
}
*/
import "C"

import (
	"runtime/cgo"
	"time"
	"unsafe"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/internal/handle"
)

type AV struct {
	tox *tox.Tox
	// h is the Tox's: toxav calls into the Tox, so it takes the Tox's mutex.
	h *handle.Handle
	// self is what the callbacks find the AV by.
	self cgo.Handle

	// These are guarded by h.
	ptr    *C.ToxAV
	cancel func()
	// events were queued by the callbacks, during an iteration of the AV or
	// of the Tox.
	events []Event
}

func New(t *tox.Tox) (*AV, error) {
	h := handle.Of(t)
	p := (*C.Tox)(h.Lock())
	defer h.Unlock()
	var err C.Toxav_Err_New
	ptr := C.toxav_new(p, &err)
	if ptr == nil {
		return nil, toError[NewError](uint32(err))
	}
	av := &AV{tox: t, h: h, ptr: ptr}
	av.self = cgo.NewHandle(av)
	C.toxav_go_register(ptr, C.uintptr_t(av.self))
	av.cancel = h.BeforeClose(av.kill)
	return av, nil
}

// kill frees the AV. The caller holds the mutex.
func (av *AV) kill() {
	C.toxav_kill(av.ptr)
	av.ptr = nil
	av.self.Delete()
}

func (av *AV) Close() {
	av.h.LockOrNil()
	defer av.h.Unlock()
	if av.ptr == nil {
		return
	}
	av.cancel()
	av.kill()
}

// lock takes the Tox's mutex and returns the AV. The caller calls av.h.Unlock.
func (av *AV) lock() *C.ToxAV {
	av.h.LockOrNil()
	if av.ptr == nil {
		av.h.Unlock()
		panic("toxav: use of a closed AV")
	}
	return av.ptr
}

func (av *AV) Tox() *tox.Tox {
	return av.tox
}

func friendNumber(friend tox.Friend) C.Tox_Friend_Number {
	return C.Tox_Friend_Number(friend.Number())
}

func (av *AV) IterationInterval() time.Duration {
	p := av.lock()
	defer av.h.Unlock()
	return time.Duration(C.toxav_iteration_interval(p)) * time.Millisecond
}

func (av *AV) Iterate() []Event {
	p := av.lock()
	defer av.h.Unlock()
	C.toxav_iterate(p)
	events := av.events
	av.events = nil
	return events
}

func (av *AV) Call(friend tox.Friend, audioBitRate, videoBitRate BitRate) error {
	p := av.lock()
	defer av.h.Unlock()
	var err C.Toxav_Err_Call
	C.toxav_call(p, friendNumber(friend), C.uint32_t(audioBitRate), C.uint32_t(videoBitRate), &err)
	return toError[CallError](uint32(err))
}

func (av *AV) Answer(friend tox.Friend, audioBitRate, videoBitRate BitRate) error {
	p := av.lock()
	defer av.h.Unlock()
	var err C.Toxav_Err_Answer
	C.toxav_answer(p, friendNumber(friend), C.uint32_t(audioBitRate), C.uint32_t(videoBitRate), &err)
	return toError[AnswerError](uint32(err))
}

func (av *AV) CallControl(friend tox.Friend, control CallControl) error {
	p := av.lock()
	defer av.h.Unlock()
	var err C.Toxav_Err_Call_Control
	C.toxav_call_control(p, friendNumber(friend), C.Toxav_Call_Control(control), &err)
	return toError[CallControlError](uint32(err))
}

func (av *AV) AudioSendFrame(friend tox.Friend, pcm []int16, channels Channels, samplingRate SamplingRate) error {
	if channels == 0 || len(pcm)%int(channels) != 0 {
		return ErrSendFrameInvalid
	}
	var data *C.int16_t
	if len(pcm) != 0 {
		data = (*C.int16_t)(unsafe.Pointer(&pcm[0]))
	}
	p := av.lock()
	defer av.h.Unlock()
	var err C.Toxav_Err_Send_Frame
	C.toxav_audio_send_frame(p, friendNumber(friend), data, C.size_t(len(pcm)/int(channels)), C.uint8_t(channels),
		C.uint32_t(samplingRate), &err)
	return toError[SendFrameError](uint32(err))
}

func (av *AV) AudioSetBitRate(friend tox.Friend, bitRate BitRate) error {
	p := av.lock()
	defer av.h.Unlock()
	var err C.Toxav_Err_Bit_Rate_Set
	C.toxav_audio_set_bit_rate(p, friendNumber(friend), C.uint32_t(bitRate), &err)
	return toError[BitRateSetError](uint32(err))
}

func (av *AV) VideoSendFrame(friend tox.Friend, width Width, height Height, y, u, v []byte) error {
	w, h := int(width), int(height)
	if w == 0 || h == 0 || len(y) < w*h || len(u) < (w/2)*(h/2) || len(v) < (w/2)*(h/2) {
		return ErrSendFrameInvalid
	}
	p := av.lock()
	defer av.h.Unlock()
	var err C.Toxav_Err_Send_Frame
	C.toxav_video_send_frame(p, friendNumber(friend), C.uint16_t(width), C.uint16_t(height), plane(y), plane(u),
		plane(v), &err)
	return toError[SendFrameError](uint32(err))
}

// plane points C at an I420 plane. A 1-pixel-wide frame has empty chroma
// planes.
func plane(b []byte) *C.uint8_t {
	if len(b) == 0 {
		return nil
	}
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}

func (av *AV) VideoSetBitRate(friend tox.Friend, bitRate BitRate) error {
	p := av.lock()
	defer av.h.Unlock()
	var err C.Toxav_Err_Bit_Rate_Set
	C.toxav_video_set_bit_rate(p, friendNumber(friend), C.uint32_t(bitRate), &err)
	return toError[BitRateSetError](uint32(err))
}
