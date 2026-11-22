package toxav_test

import (
	"testing"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/internal/toxtest"
	"github.com/TokTok/go-toxcore-c/toxav"
)

func newAV(t *testing.T, x *tox.Tox) *toxav.AV {
	av, err := toxav.New(x)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(av.Close)
	return av
}

// iterate iterates the instances and their AVs once, giving each AV event to
// handle.
func iterate(t *testing.T, toxes []*tox.Tox, avs []*toxav.AV, handle func(av *toxav.AV, ev toxav.Event)) {
	toxtest.Iterate(t, toxes, func(*tox.Tox, tox.Event) bool { return false })
	for _, av := range avs {
		for _, ev := range av.Iterate() {
			handle(av, ev)
		}
	}
}

func TestCall(t *testing.T) {
	a, b, fa, fb := toxtest.Friends(t)
	avA, avB := newAV(t, a), newAV(t, b)
	toxes, avs := []*tox.Tox{a, b}, []*toxav.AV{avA, avB}

	if err := avA.Call(fa, 48, 1000); err != nil {
		t.Fatal(err)
	}
	answered := false
	toxtest.Until(t, func() bool {
		iterate(t, toxes, avs, func(av *toxav.AV, ev toxav.Event) {
			switch ev := ev.(type) {
			case *toxav.EventCall:
				if av != avB || ev.Friend != fb || !ev.AudioEnabled || !ev.VideoEnabled {
					t.Errorf("call %+v", ev)
				}
				if err := avB.Answer(ev.Friend, 48, 1000); err != nil {
					t.Fatal(err)
				}
			case *toxav.EventCallState:
				answered = av == avA && ev.Friend == fa && ev.State&toxav.FriendCallStateAcceptingA != 0
			}
		})
		return answered
	})

	pcm := make([]int16, 960) // 20 ms of mono at 48 kHz
	y, u, v := make([]byte, 64*48), make([]byte, 32*24), make([]byte, 32*24)
	gotAudio, gotVideo := false, false
	toxtest.Until(t, func() bool {
		if err := avA.AudioSendFrame(fa, pcm, 1, 48000); err != nil {
			t.Fatal(err)
		}
		if err := avA.VideoSendFrame(fa, 64, 48, y, u, v); err != nil {
			t.Fatal(err)
		}
		iterate(t, toxes, avs, func(av *toxav.AV, ev toxav.Event) {
			switch ev := ev.(type) {
			case *toxav.EventAudioReceiveFrame:
				gotAudio = av == avB && ev.Friend == fb && ev.Channels == 1 && ev.SamplingRate == 48000 && len(ev.PCM) > 0
			case *toxav.EventVideoReceiveFrame:
				gotVideo = av == avB && ev.Width == 64 && ev.Height == 48 && len(ev.Y) == 64*48 && len(ev.U) == 32*24
			}
		})
		return gotAudio && gotVideo
	})

	if err := avA.CallControl(fa, toxav.CallControlCancel); err != nil {
		t.Fatal(err)
	}
	if err := avA.AudioSendFrame(fa, pcm, 1, 48000); err != toxav.ErrSendFrameFriendNotInCall {
		t.Errorf("sending after hanging up: %v", err)
	}
}

func TestLifecycle(t *testing.T) {
	x := toxtest.New(t, nil)
	av := newAV(t, x)
	if _, err := toxav.New(x); err != toxav.ErrNewMultiple {
		t.Errorf("second AV: %v", err)
	}
	if err := av.AudioSendFrame(x.Friend(0), make([]int16, 3), 2, 48000); err != toxav.ErrSendFrameInvalid {
		t.Errorf("3 samples in stereo: %v", err)
	}
	if err := av.VideoSendFrame(x.Friend(0), 64, 48, make([]byte, 10), nil, nil); err != toxav.ErrSendFrameInvalid {
		t.Errorf("short planes: %v", err)
	}

	x.Close()
	av.Close()
	defer func() {
		if recover() == nil {
			t.Error("an AV of a closed Tox didn't panic")
		}
	}()
	av.Iterate()
}
