package main

/*
#include <jni.h>

static jsize array_length(JNIEnv *env, jarray a) {
	return (*env)->GetArrayLength(env, a);
}

static void get_bytes(JNIEnv *env, jbyteArray a, jsize n, void *buf) {
	(*env)->GetByteArrayRegion(env, a, 0, n, buf);
}
*/
import "C"

import (
	"errors"
	"io"
	"log"
	"sync"
	"unsafe"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

// androidMedia is the sound and camera of android/java/.../Media.java.
type androidMedia struct{}

func newMedia() media { return androidMedia{} }

// The recording and the camera call these, from Java's threads.
var (
	mediaMu  sync.Mutex
	recorder func([]int16)
	camera   func(width, height int, y, u, v []byte)
)

// The recording thread and the camera thread each have a buffer, for one
// frame at a time.
var (
	pcmBuf    []int16
	planeBufs [3][]byte
)

//export Java_chat_tox_gotox_Media_recorded
func Java_chat_tox_gotox_Media_recorded(env *C.JNIEnv, class C.jclass, pcm C.jbyteArray) {
	n := int(C.array_length(env, C.jarray(pcm))) / 2
	if n == 0 {
		return
	}
	pcmBuf = grow(pcmBuf, n)
	C.get_bytes(env, pcm, C.jsize(n*2), unsafe.Pointer(&pcmBuf[0]))
	mediaMu.Lock()
	record := recorder
	mediaMu.Unlock()
	if record != nil {
		record(pcmBuf)
	}
}

//export Java_chat_tox_gotox_Media_captured
func Java_chat_tox_gotox_Media_captured(env *C.JNIEnv, class C.jclass, width, height C.jint, y, u, v C.jbyteArray) {
	for i, a := range []C.jbyteArray{y, u, v} {
		n := C.array_length(env, C.jarray(a))
		planeBufs[i] = grow(planeBufs[i], int(n))
		if n != 0 {
			C.get_bytes(env, a, n, unsafe.Pointer(&planeBufs[i][0]))
		}
	}
	mediaMu.Lock()
	frame := camera
	mediaMu.Unlock()
	if frame != nil {
		frame(int(width), int(height), planeBufs[0], planeBufs[1], planeBufs[2])
	}
}

// androidAudio plays on a goroutine that stays attached to Java for the
// call: attaching for each frame would cost more than playing it.
type androidAudio struct {
	frames  chan pcmFrame
	free    chan []int16
	done    chan struct{}
	stopped chan struct{}
}

type pcmFrame struct {
	pcm            []int16
	channels, rate int
}

func (androidMedia) openAudio(record func([]int16)) (audioIO, error) {
	mediaMu.Lock()
	recorder = record
	mediaMu.Unlock()
	err := callMedia("startAudio", "(Landroid/content/Context;)V",
		func(env jni.Env, ctx jni.Object) []jni.Value { return []jni.Value{jni.Value(ctx)} })
	if err != nil {
		stopRecording()
		return nil, err
	}
	a := &androidAudio{
		frames:  make(chan pcmFrame, 4),
		free:    make(chan []int16, 4),
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go a.player()
	return a, nil
}

// play hands the player a copy of the frame, or drops it if the player is
// behind.
func (a *androidAudio) play(pcm []int16, channels, rate int) {
	if len(pcm) == 0 {
		return
	}
	var buf []int16
	select {
	case buf = <-a.free:
	default:
	}
	f := pcmFrame{append(buf[:0], pcm...), channels, rate}
	select {
	case a.frames <- f:
	default:
	}
}

func (a *androidAudio) player() {
	defer close(a.stopped)
	err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		class, err := mediaClass(env)
		if err != nil {
			return err
		}
		play := jni.GetStaticMethodID(env, class, "play", "([BII)V")
		for {
			var f pcmFrame
			select {
			case <-a.done:
				return nil
			case f = <-a.frames:
			}
			data := unsafe.Slice((*byte)(unsafe.Pointer(&f.pcm[0])), len(f.pcm)*2)
			arr := jni.NewByteArray(env, data)
			err := jni.CallStaticVoidMethod(env, class, play, jni.Value(arr), jni.Value(f.channels), jni.Value(f.rate))
			// The thread stays attached, so its references would pile up.
			jni.DeleteLocalRef(env, jni.Object(arr))
			if err != nil {
				return err
			}
			select {
			case a.free <- f.pcm:
			default:
			}
		}
	})
	if err != nil {
		log.Printf("playing sound: %v", err)
	}
}

func (a *androidAudio) Close() error {
	close(a.done)
	<-a.stopped
	stopRecording()
	return callMedia("stopAudio", "(Landroid/content/Context;)V",
		func(env jni.Env, ctx jni.Object) []jni.Value { return []jni.Value{jni.Value(ctx)} })
}

func stopRecording() {
	mediaMu.Lock()
	recorder = nil
	mediaMu.Unlock()
}

type androidCamera struct{}

func (androidMedia) openCamera(frame func(int, int, []byte, []byte, []byte)) (io.Closer, error) {
	mediaMu.Lock()
	camera = frame
	mediaMu.Unlock()
	var started bool
	err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		class, err := mediaClass(env)
		if err != nil {
			return err
		}
		m := jni.GetStaticMethodID(env, class, "startCamera", "(Landroid/content/Context;)Z")
		started, err = jni.CallStaticBooleanMethod(env, class, m, jni.Value(app.AppContext()))
		return err
	})
	if err == nil && !started {
		err = errors.New("there's no camera, or the app may not use it")
	}
	if err != nil {
		androidCamera{}.Close()
		return nil, err
	}
	return androidCamera{}, nil
}

func (androidCamera) Close() error {
	mediaMu.Lock()
	camera = nil
	mediaMu.Unlock()
	return callMedia("stopCamera", "()V", func(jni.Env, jni.Object) []jni.Value { return nil })
}

// callMedia calls a static method of chat.tox.gotox.Media.
func callMedia(method, signature string, args func(env jni.Env, ctx jni.Object) []jni.Value) error {
	return jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		class, err := mediaClass(env)
		if err != nil {
			return err
		}
		ctx := jni.Object(app.AppContext())
		return jni.CallStaticVoidMethod(env, class, jni.GetStaticMethodID(env, class, method, signature), args(env, ctx)...)
	})
}

// mediaClass finds chat.tox.gotox.Media, from any thread: FindClass only
// sees the app's classes on Java's own threads.
func mediaClass(env jni.Env) (jni.Class, error) {
	ctx := jni.Object(app.AppContext())
	return jni.LoadClass(env, jni.ClassLoaderFor(env, ctx), "chat.tox.gotox.Media")
}
