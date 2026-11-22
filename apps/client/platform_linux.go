//go:build !android

package main

import (
	"fmt"
	"log"
	"os/exec"
	"sync"

	"gioui.org/app"
	"github.com/godbus/dbus/v5"
)

// platform shows notifications through the desktop's notification server.
type platform struct {
	// bus is nil without a session bus.
	bus *dbus.Conn

	mu sync.Mutex
	// ids are the server's IDs for our notifications, by tag.
	ids map[string]uint32
	// uris are what the notifications open when clicked, by ID.
	uris map[uint32]string
}

// newPlatform makes the platform, which calls open with a notification's URI
// when the user clicks it.
func newPlatform(open func(uri string)) *platform {
	p := &platform{ids: map[string]uint32{}, uris: map[uint32]string{}}
	var err error
	if p.bus, err = dbus.SessionBus(); err != nil {
		log.Printf("no notifications: %v", err)
		return p
	}
	signals := make(chan *dbus.Signal, 8)
	p.bus.Signal(signals)
	for _, member := range []string{"ActionInvoked", "NotificationClosed"} {
		if err := p.bus.AddMatchSignal(dbus.WithMatchObjectPath(notificationsPath),
			dbus.WithMatchInterface(notificationsName), dbus.WithMatchMember(member)); err != nil {
			log.Printf("notification clicks: %v", err)
		}
	}
	go func() {
		for sig := range signals {
			if len(sig.Body) < 2 {
				continue
			}
			id, _ := sig.Body[0].(uint32)
			p.mu.Lock()
			uri, ours := p.uris[id]
			if sig.Name == notificationsName+".NotificationClosed" {
				delete(p.uris, id)
			}
			p.mu.Unlock()
			if action, _ := sig.Body[1].(string); ours && sig.Name == notificationsName+".ActionInvoked" && action == "default" {
				open(uri)
			}
		}
	}()
	return p
}

const (
	notificationsName = "org.freedesktop.Notifications"
	notificationsPath = "/org/freedesktop/Notifications"
)

// closesApps is set on platforms that close apps the user isn't using.
const closesApps = false

// opensFiles is set on platforms where the app opens files in other apps.
const opensFiles = true

// openFile opens a file with the desktop's app for its kind.
func (p *platform) openFile(path string) error {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("opening the file: %w", err)
	}
	go cmd.Wait()
	return nil
}

func (p *platform) notifications() dbus.BusObject {
	return p.bus.Object(notificationsName, notificationsPath)
}

func (p *platform) notify(tag, title, text, uri string) {
	if p.bus == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var id uint32
	// The default action is a click on the notification.
	err := p.notifications().Call(notificationsName+".Notify", 0,
		"Go Tox", p.ids[tag], "", title, text, []string{"default", "Open"}, map[string]dbus.Variant{}, int32(-1)).Store(&id)
	if err != nil {
		log.Printf("notifying: %v", err)
		return
	}
	p.ids[tag] = id
	p.uris[id] = uri
}

func (p *platform) cancel(tag string) {
	if p.bus == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if id, ok := p.ids[tag]; ok {
		delete(p.ids, tag)
		delete(p.uris, id)
		p.notifications().Go(notificationsName+".CloseNotification", 0, nil, id)
	}
}

func (p *platform) stayConnected(bool) {}

func (p *platform) askForCall(bool) {}

func (p *platform) viewEvent(app.ViewEvent) {}
