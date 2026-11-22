package main

import (
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"sync"
	"time"

	"github.com/TokTok/go-toxcore-c/toxav"
)

const (
	// audioBitRate and videoBitRate are what calls send, in kbit/s.
	audioBitRate = 48
	videoBitRate = 1500
	// audioRate is the sampling rate of the sound we record, and audioFrame
	// how many samples go in a frame: 20 ms.
	audioRate  = 48000
	audioFrame = audioRate / 50
)

// An audioIO plays a call's sound, while it records the user's.
type audioIO interface {
	play(pcm []int16, channels, rate int)
	io.Closer
}

// A callLine is who has the one call there can be: one microphone and one
// speaker go to one profile at a time. A core without one has the line to
// itself.
type callLine struct {
	mu     sync.Mutex
	holder *core
}

// take takes the line for c, unless another profile has it.
func (l *callLine) take(c *core) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.holder == nil {
		l.holder = c
	}
	return l.holder == c
}

func (l *callLine) release(c *core) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.holder == c {
		l.holder = nil
	}
}

// caller is the profile with the call, or nil.
func (l *callLine) caller() *core {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.holder
}

// A media is the platform's sound and camera, for calls.
type media interface {
	// openAudio starts recording, giving record frames of audioFrame
	// samples, mono at audioRate, and returns the stream to play sound on.
	// The next frame reuses pcm.
	openAudio(record func(pcm []int16)) (audioIO, error)
	// openCamera starts the camera, giving frame its pictures in I420. The
	// next picture reuses the planes.
	openCamera(frame func(width, height int, y, u, v []byte)) (io.Closer, error)
}

// A call is a call with a friend. There is one at a time.
type call struct {
	friend *friend
	// outgoing is set on a call we made; incoming calls ring here until the
	// user answers.
	outgoing bool
	// answered is set once both sides are in the call, at started.
	answered bool
	started  time.Time
	// remote is what the friend's side does.
	remote toxav.FriendCallState
	muted  bool
	// video is set while we send video.
	video bool
	// frame is the friend's newest picture.
	frame image.Image

	audio  audioIO
	camera io.Closer
}

// runAV iterates the calls until done.
func (c *core) runAV(done <-chan struct{}) {
	for {
		events := c.av.Iterate()
		var play []*toxav.EventAudioReceiveFrame
		var audio audioIO
		if len(events) != 0 {
			c.mu.Lock()
			for _, ev := range events {
				if f, ok := ev.(*toxav.EventAudioReceiveFrame); ok {
					play = append(play, f)
					continue
				}
				c.handleAV(ev)
			}
			if c.call != nil {
				audio = c.call.audio
			}
			c.unlock()
		}
		// Playing waits for the sound card: not under the lock.
		if audio != nil {
			for _, f := range play {
				audio.play(f.PCM, int(f.Channels), int(f.SamplingRate))
			}
		}
		select {
		case <-done:
			return
		case <-time.After(c.av.IterationInterval()):
		}
	}
}

// handleAV applies an event about calls. The caller holds c.mu.
func (c *core) handleAV(ev toxav.Event) {
	switch ev := ev.(type) {
	case *toxav.EventCall:
		fr := c.friends[ev.Friend.Number()]
		if fr == nil {
			return
		}
		if c.call != nil || !c.line.take(c) {
			// Busy: one call at a time, here or in another profile.
			if err := c.av.CallControl(ev.Friend, toxav.CallControlCancel); err != nil {
				log.Print(err)
			}
			c.notice(fr.chat, "Missed call: you were in another call")
			return
		}
		cl := &call{friend: fr}
		if ev.AudioEnabled {
			cl.remote |= toxav.FriendCallStateSendingA
		}
		what := "Call"
		if ev.VideoEnabled {
			cl.remote |= toxav.FriendCallStateSendingV
			what = "Video call"
		}
		c.call = cl
		c.alerts = append(c.alerts, alert{fr.chat.id, fr.title(), what})
	case *toxav.EventCallState:
		cl := c.call
		if cl == nil || cl.friend.Number() != ev.Friend.Number() {
			return
		}
		if ev.State&(toxav.FriendCallStateFinished|toxav.FriendCallStateError) != 0 {
			c.endCall()
			return
		}
		cl.remote = ev.State
		if !cl.answered {
			c.startCall(cl)
		}
	case *toxav.EventVideoReceiveFrame:
		if cl := c.call; cl != nil && cl.friend.Number() == ev.Friend.Number() {
			cl.frame = i420(int(ev.Width), int(ev.Height), ev.Y, ev.U, ev.V)
		}
	}
}

// i420 is an I420 picture as an image, without copying it.
func i420(width, height int, y, u, v []byte) image.Image {
	return &image.YCbCr{
		Y: y, Cb: u, Cr: v,
		YStride:        width,
		CStride:        (width + 1) / 2,
		SubsampleRatio: image.YCbCrSubsampleRatio420,
		Rect:           image.Rect(0, 0, width, height),
	}
}

// startCall starts sending sound once both sides are in the call. The caller
// holds c.mu.
func (c *core) startCall(cl *call) {
	cl.answered, cl.started = true, time.Now()
	fr := cl.friend.Friend
	audio, err := c.media.openAudio(func(pcm []int16) {
		c.mu.Lock()
		muted := c.call != cl || cl.muted
		c.mu.Unlock()
		if muted {
			return
		}
		if err := c.av.AudioSendFrame(fr, pcm, 1, audioRate); err != nil {
			log.Printf("sending sound: %v", err)
		}
	})
	if err != nil {
		c.notice(cl.friend.chat, "No sound: "+err.Error())
		return
	}
	cl.audio = audio
	if cl.video {
		c.startCamera(cl)
	}
}

// startCamera starts sending video. The caller holds c.mu.
func (c *core) startCamera(cl *call) {
	fr := cl.friend.Friend
	camera, err := c.media.openCamera(func(width, height int, y, u, v []byte) {
		c.mu.Lock()
		sending := c.call == cl && cl.video
		c.mu.Unlock()
		if !sending {
			return
		}
		if err := c.av.VideoSendFrame(fr, toxav.Width(width), toxav.Height(height), y, u, v); err != nil {
			log.Printf("sending video: %v", err)
		}
	})
	if err != nil {
		cl.video = false
		c.notice(cl.friend.chat, "No camera: "+err.Error())
		return
	}
	cl.camera = camera
}

// endCall stops the call, and notes it in the chat. The caller holds c.mu.
func (c *core) endCall() {
	cl := c.call
	if cl == nil {
		return
	}
	c.call = nil
	c.line.release(c)
	if cl.audio != nil {
		// The recording goroutine takes c.mu: closing waits for it.
		go cl.audio.Close()
	}
	if cl.camera != nil {
		go cl.camera.Close()
	}
	c.alerts = append(c.alerts, alert{key: cl.friend.chat.id})
	switch {
	case cl.answered:
		c.notice(cl.friend.chat, "Call, "+callLength(time.Since(cl.started)))
	case cl.outgoing:
		c.notice(cl.friend.chat, "Call, not answered")
	default:
		c.notice(cl.friend.chat, "Missed call")
	}
}

func callLength(d time.Duration) string {
	d = d.Round(time.Second)
	if d >= time.Hour {
		return fmt.Sprintf("%d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

// callFriend calls a friend, with video if video is set.
func (c *core) callFriend(id chatID, video bool) error {
	var err error
	c.change(func() {
		ch := c.chats[id]
		switch {
		case ch == nil || ch.friend == nil:
			err = errors.New("calls are with friends")
			return
		case c.call != nil || !c.line.take(c):
			err = errors.New("you're in a call")
			return
		case !ch.friend.online():
			err = errors.New("they're offline")
			return
		}
		vbr := toxav.BitRate(0)
		if video {
			vbr = videoBitRate
		}
		if err = c.av.Call(ch.friend.Friend, audioBitRate, vbr); err != nil {
			c.line.release(c)
			return
		}
		c.call = &call{friend: ch.friend, outgoing: true, video: video}
	})
	return err
}

// answer answers the call ringing here, with video if video is set.
func (c *core) answer(video bool) error {
	var err error
	c.change(func() {
		cl := c.call
		if cl == nil || cl.outgoing || cl.answered {
			return
		}
		vbr := toxav.BitRate(0)
		if video {
			vbr = videoBitRate
		}
		if err = c.av.Answer(cl.friend.Friend, audioBitRate, vbr); err != nil {
			c.endCall()
			return
		}
		cl.video = video
		c.startCall(cl)
	})
	return err
}

// hangUp ends the call, or declines it.
func (c *core) hangUp() {
	c.change(func() {
		if c.call == nil {
			return
		}
		if err := c.av.CallControl(c.call.friend.Friend, toxav.CallControlCancel); err != nil {
			log.Print(err)
		}
		c.endCall()
	})
}

// setMuted stops sending sound, or starts again.
func (c *core) setMuted(muted bool) {
	c.change(func() {
		if cl := c.call; cl != nil {
			cl.muted = muted
			control := toxav.CallControlUnmuteAudio
			if muted {
				control = toxav.CallControlMuteAudio
			}
			if cl.answered {
				if err := c.av.CallControl(cl.friend.Friend, control); err != nil {
					log.Print(err)
				}
			}
		}
	})
}

// setVideo starts or stops sending video.
func (c *core) setVideo(on bool) {
	c.change(func() {
		cl := c.call
		if cl == nil || cl.video == on {
			return
		}
		cl.video = on
		switch {
		case !cl.answered:
		case on:
			if err := c.av.VideoSetBitRate(cl.friend.Friend, videoBitRate); err != nil {
				log.Print(err)
			}
			c.startCamera(cl)
		default:
			if cl.camera != nil {
				go cl.camera.Close()
				cl.camera = nil
			}
			if err := c.av.CallControl(cl.friend.Friend, toxav.CallControlHideVideo); err != nil {
				log.Print(err)
			}
		}
	})
}

// A callView is the call as the UI shows it.
type callView struct {
	chat     chatID
	title    string
	avatar   *avatar
	outgoing bool
	answered bool
	started  time.Time
	muted    bool
	video    bool
	// frame is the friend's newest picture, if they send video.
	frame image.Image
}

// callView is the call, if there is one. The caller holds c.mu.
func (c *core) callView() *callView {
	cl := c.call
	if cl == nil {
		return nil
	}
	v := &callView{
		chat:     cl.friend.chat.id,
		title:    cl.friend.title(),
		avatar:   cl.friend.avatar,
		outgoing: cl.outgoing,
		answered: cl.answered,
		started:  cl.started,
		muted:    cl.muted,
		video:    cl.video,
	}
	if cl.remote&toxav.FriendCallStateSendingV != 0 {
		v.frame = cl.frame
	}
	return v
}

// friendGone ends the call with a friend who went offline. The caller holds
// c.mu.
func (c *core) friendGone(fr *friend) {
	if c.call != nil && c.call.friend == fr {
		c.endCall()
	}
}

// noMedia is for platforms without sound or a camera for the app.
type noMedia struct{}

var errNoMedia = errors.New("calls don't work on this computer yet")

func (noMedia) openAudio(func([]int16)) (audioIO, error) { return nil, errNoMedia }

func (noMedia) openCamera(func(int, int, []byte, []byte, []byte)) (io.Closer, error) {
	return nil, errNoMedia
}

// grow returns buf with length n, in a new array only if buf is too short.
func grow[T any](buf []T, n int) []T {
	if cap(buf) < n {
		return make([]T, n)
	}
	return buf[:n]
}
