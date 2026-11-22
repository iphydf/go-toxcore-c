// Command client is Go Tox, a Tox chat client for desktops and phones.
package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"net/url"
	// toxcore uses the network from C. Importing net is how gogio knows to
	// give the Android app the INTERNET permission.
	_ "net"
	"os"
	"path/filepath"
	"sync/atomic"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/x/explorer"
)

// version is the app's version: builds for a release set it with
// -ldflags "-X main.version=...".
var version = "dev"

func main() {
	profile := flag.String("profile", "", "the directory to keep the profile and chats in (default: the app's data directory)")
	size := flag.String("size", "960x680", "the window's width and height in dp, on a desktop")
	flag.Parse()
	var width, height int
	if _, err := fmt.Sscanf(*size, "%dx%d", &width, &height); err != nil {
		log.Fatalf("-size %q: %v", *size, err)
	}

	// win is the window on screen.
	var win atomic.Pointer[app.Window]
	newWindow := func() *app.Window {
		w := new(app.Window)
		w.Option(app.Title("Go Tox"), app.Size(unit.Dp(width), unit.Dp(height)),
			// Light status bar icons, on the header's colour.
			app.StatusColor(headerBg))
		win.Store(w)
		return w
	}
	invalidate := func() {
		if w := win.Load(); w != nil {
			w.Invalidate()
		}
	}
	urls := make(chan *url.URL, 8)
	open := func(l *url.URL) {
		select {
		case urls <- l:
			invalidate()
		default:
		}
	}
	// A click on a desktop notification also brings the window up.
	clicked := func(l *url.URL) {
		open(l)
		if w := win.Load(); w != nil {
			w.Perform(system.ActionRaise)
		}
	}
	go func() {
		if err := run(*profile, newWindow, invalidate, urls, clicked); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	// Links come here, not to the window: a notification's on Android, or a
	// Tox ID's.
	for ev := range app.Events {
		if e, ok := ev.(app.URLEvent); ok {
			open(e.URL)
		}
	}
}

// run runs the app until its window closes. On Android, the window goes with
// the activity: then a new window waits for the next one, while Tox keeps
// running. It opens the urls, and calls clicked with a notification's.
func run(dir string, newWindow func() *app.Window, invalidate func(), urls <-chan *url.URL, clicked func(*url.URL)) error {
	plat := newPlatform(func(uri string) {
		if l, err := url.Parse(uri); err == nil {
			clicked(l)
		}
	})
	h := &home{fixed: dir, plat: plat, invalidate: invalidate}
	if dir == "" {
		data, err := app.DataDir()
		if err != nil {
			return err
		}
		h.base = filepath.Join(data, "go-tox")
	}
	defer h.close()
	u := newUI(h)
	u.invalidate = invalidate
	// Every profile without a password runs; the others wait to be unlocked,
	// on the start page if none runs.
	if err := h.openAll(); err != nil {
		return err
	}
	if ps := h.profiles(); h.core() == nil && len(ps) == 1 {
		u.start.chosen = ps[0].name
	}
	for {
		w := newWindow()
		closed, err := show(w, u, h.plat, urls)
		if closed {
			return err
		}
		// Gio may still send the old window events: it waits until they're
		// taken.
		go func() {
			for {
				if _, ok := w.Event().(app.DestroyEvent); ok {
					return
				}
			}
		}()
	}
}

// show runs the UI in a window until the window closes, or on Android until
// its activity is gone.
func show(w *app.Window, u *ui, plat *platform, urls <-chan *url.URL) (closed bool, err error) {
	var ops op.Ops
	u.explorer = explorer.NewExplorer(w)
	for {
		e := w.Event()
		u.explorer.ListenEvents(e)
		switch e := e.(type) {
		case app.DestroyEvent:
			return true, e.Err
		case app.ViewEvent:
			plat.viewEvent(e)
			if closesApps && !e.Valid() {
				return false, nil
			}
		case app.ConfigEvent:
			u.setFocused(e.Config.Focused)
		case app.FrameEvent:
		links:
			for {
				select {
				case l := <-urls:
					u.openURL(l)
				default:
					break links
				}
			}
			gtx := app.NewContext(&ops, e)
			paint.Fill(gtx.Ops, u.th.Palette.Bg)
			// The status bar is over the window on phones: the header's
			// colour goes on under it.
			top := image.Pt(gtx.Constraints.Max.X+e.Metric.Dp(e.Insets.Left+e.Insets.Right), e.Metric.Dp(e.Insets.Top))
			paint.FillShape(gtx.Ops, headerBg, clip.Rect{Min: image.Pt(-e.Metric.Dp(e.Insets.Left), -top.Y), Max: image.Pt(top.X, 0)}.Op())
			u.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
