//go:build !linux

package main

import (
	"errors"

	"gioui.org/app"
)

// platform does nothing on platforms the app doesn't have code for yet.
type platform struct{}

func newPlatform(func(uri string)) *platform { return &platform{} }

// closesApps is set on platforms that close apps the user isn't using.
const closesApps = false

// opensFiles is set on platforms where the app opens files in other apps.
const opensFiles = false

func (*platform) openFile(string) error { return errors.ErrUnsupported }

func (*platform) notify(tag, title, text, uri string) {}

func (*platform) cancel(tag string) {}

func (*platform) stayConnected(bool) {}

func (*platform) askForCall(bool) {}

func (*platform) viewEvent(app.ViewEvent) {}
