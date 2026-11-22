package main

import (
	"errors"
	"log"
	"sync/atomic"

	"gioui.org/app"
	"git.wow.st/gmp/jni"
)

// platform calls the app's Java code, in android/java.
type platform struct {
	// view is the window's, while it has one.
	view atomic.Uintptr
}

// newPlatform makes the platform. A tap on a notification opens the activity
// with its URI, which comes as a URL event.
func newPlatform(func(uri string)) *platform { return &platform{} }

// closesApps is set on platforms that close apps the user isn't using.
const closesApps = true

// opensFiles is set on platforms where the app opens files in other apps.
const opensFiles = false

func (*platform) openFile(string) error { return errors.ErrUnsupported }

func (p *platform) notify(tag, title, text, uri string) {
	p.call("notify", "(Landroid/content/Context;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)V",
		func(env jni.Env, ctx jni.Object) []jni.Value {
			return []jni.Value{jni.Value(ctx), str(env, tag), str(env, title), str(env, text), str(env, uri)}
		})
}

func (p *platform) cancel(tag string) {
	p.call("cancel", "(Landroid/content/Context;Ljava/lang/String;)V",
		func(env jni.Env, ctx jni.Object) []jni.Value {
			return []jni.Value{jni.Value(ctx), str(env, tag)}
		})
}

// stayConnected keeps the app running while it's closed, or lets Android stop
// it.
func (p *platform) stayConnected(on bool) {
	b := jni.Value(jni.FALSE)
	if on {
		b = jni.TRUE
	}
	p.call("stayConnected", "(Landroid/content/Context;Z)V",
		func(env jni.Env, ctx jni.Object) []jni.Value { return []jni.Value{jni.Value(ctx), b} })
}

// viewEvent asks for what the app needs once it has a window: notifications.
func (p *platform) viewEvent(e app.ViewEvent) {
	view := e.(app.AndroidViewEvent).View
	p.view.Store(view)
	if view == 0 {
		return
	}
	p.call("askForNotifications", "(Landroid/view/View;)V",
		func(jni.Env, jni.Object) []jni.Value { return []jni.Value{jni.Value(view)} })
}

// askForCall asks the user to let the app record sound, and with video use
// the camera.
func (p *platform) askForCall(video bool) {
	view := p.view.Load()
	if view == 0 {
		return
	}
	v := jni.Value(jni.FALSE)
	if video {
		v = jni.TRUE
	}
	p.call("askForCall", "(Landroid/view/View;Z)V",
		func(jni.Env, jni.Object) []jni.Value { return []jni.Value{jni.Value(view), v} })
}

// call calls a static method of chat.tox.gotox.Platform.
func (p *platform) call(method, signature string, args func(env jni.Env, ctx jni.Object) []jni.Value) {
	err := jni.Do(jni.JVMFor(app.JavaVM()), func(env jni.Env) error {
		ctx := jni.Object(app.AppContext())
		class, err := jni.LoadClass(env, jni.ClassLoaderFor(env, ctx), "chat.tox.gotox.Platform")
		if err != nil {
			return err
		}
		return jni.CallStaticVoidMethod(env, class, jni.GetStaticMethodID(env, class, method, signature), args(env, ctx)...)
	})
	if err != nil {
		log.Printf("Platform.%s: %v", method, err)
	}
}

func str(env jni.Env, s string) jni.Value { return jni.Value(jni.JavaString(env, s)) }
