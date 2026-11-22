package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/explorer"
	"golang.org/x/image/draw"

	tox "github.com/TokTok/go-toxcore-c"
)

// maxThumbnail is the width and height of pictures in a chat, in pixels.
const maxThumbnail = 480

// A fileButton is a button of a file in a chat.
type fileButton struct {
	msg    int64
	action fileAction
}

// The buttons for finished files: what they do isn't for the core.
const (
	fileSave fileAction = -1 - iota
	fileOpen
)

// A thumbnail is a picture in a chat, small.
type thumbnail struct {
	img paint.ImageOp
	// ok is unset while the picture loads, and when it isn't one.
	ok bool
}

// files holds the UI's state for files: what goroutines do for it.
type files struct {
	mu sync.Mutex
	// thumbnails are by message.
	thumbnails map[int64]*thumbnail
	// notice is the last thing that went wrong, for the user.
	notice string
	// imported is a profile file the user chose to import.
	imported *importedProfile
}

// report tells the user about an error from a goroutine.
func (u *ui) report(err error) {
	if err == nil || err == explorer.ErrUserDecline {
		return
	}
	log.Print(err)
	u.files.mu.Lock()
	u.files.notice = err.Error()
	u.files.mu.Unlock()
	u.invalidate()
}

func (u *ui) notice() string {
	u.files.mu.Lock()
	defer u.files.mu.Unlock()
	return u.files.notice
}

func (u *ui) clearNotice() {
	u.files.mu.Lock()
	defer u.files.mu.Unlock()
	u.files.notice = ""
}

// fileName is a chosen file's name, without its directory.
func fileName(r io.Reader) string {
	if f, ok := r.(interface{ Name() string }); ok && f.Name() != "" {
		return filepath.Base(f.Name())
	}
	return "file"
}

// attach asks the user for a file and sends it to the friend.
func (u *ui) attach(id chatID) {
	if u.explorer == nil {
		return
	}
	go func() {
		r, err := u.explorer.ChooseFile()
		if err != nil {
			u.report(err)
			return
		}
		defer r.Close()
		u.report(u.core.sendFile(id, fileName(r), r))
	}()
}

// save asks the user where to save a file, and copies it there.
func (u *ui) save(m message) {
	if u.explorer == nil {
		return
	}
	path := u.core.filePath(m.id)
	go func() {
		in, err := os.Open(path)
		if err != nil {
			u.report(err)
			return
		}
		defer in.Close()
		out, err := u.explorer.CreateFile(m.file.name)
		if err != nil {
			u.report(err)
			return
		}
		_, err = io.Copy(out, in)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		u.report(err)
	}()
}

// openFile opens a file with the app the system has for its kind.
func (u *ui) openFile(m message) {
	go func() {
		path, err := u.core.namedFile(m.id, m.file.name)
		if err == nil {
			err = u.home.plat.openFile(path)
		}
		u.report(err)
	}()
}

// chooseAvatar asks the user for a picture, and makes it their avatar.
func (u *ui) chooseAvatar() {
	if u.explorer == nil {
		return
	}
	go func() {
		r, err := u.explorer.ChooseFile(".png", ".jpg", ".jpeg", ".gif")
		if err != nil {
			u.report(err)
			return
		}
		defer r.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			u.report(err)
			return
		}
		u.report(u.core.setAvatar(data))
	}()
}

// isPicture reports whether a file's name says it's a picture the app can
// show.
func isPicture(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return true
	}
	return false
}

// thumbnail returns a finished picture's thumbnail, loading it the first time.
func (u *ui) thumbnail(m message) *thumbnail {
	u.files.mu.Lock()
	defer u.files.mu.Unlock()
	if t := u.files.thumbnails[m.id]; t != nil {
		return t
	}
	t := new(thumbnail)
	u.files.thumbnails[m.id] = t
	path := u.core.filePath(m.id)
	go func() {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		op := paint.NewImageOp(shrink(img, maxThumbnail))
		u.files.mu.Lock()
		t.img, t.ok = op, true
		u.files.mu.Unlock()
		u.invalidate()
	}()
	return t
}

// shrink scales a picture down to fit in a square.
func shrink(img image.Image, side int) image.Image {
	b := img.Bounds()
	if b.Dx() <= side && b.Dy() <= side {
		return img
	}
	w, h := side, b.Dy()*side/b.Dx()
	if b.Dy() > b.Dx() {
		w, h = b.Dx()*side/b.Dy(), side
	}
	small := image.NewNRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), img, b, draw.Src, nil)
	return small
}

// fileStatus says how a file is doing.
func fileStatus(m message) string {
	f := m.file
	size := byteSize(f.size)
	switch f.state {
	case fileOffered:
		return size
	case fileActive:
		if f.done == 0 && !m.outgoing {
			return size + " · waiting for them"
		}
		if f.done == 0 {
			return size + " · waiting"
		}
		if f.size == tox.FileSizeUnknown || f.size == 0 {
			return byteSize(f.done)
		}
		return fmt.Sprintf("%s of %s", byteSize(f.done), size)
	case filePaused:
		return size + " · paused"
	case fileCancelled:
		return size + " · cancelled"
	}
	return size
}

// byteSize is a size for people to read.
func byteSize(n uint64) string {
	if n == tox.FileSizeUnknown {
		return "size not known"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// layoutFile draws a file in a chat, with what the user can do with it.
func (u *ui) layoutFile(gtx layout.Context, id chatID, m message) layout.Dimensions {
	f := m.file
	var actions []fileAction
	switch f.state {
	case fileOffered:
		actions = []fileAction{fileAccept, fileCancel}
	case fileActive:
		actions = []fileAction{filePause, fileCancel}
	case filePaused:
		actions = []fileAction{fileResume, fileCancel}
	case fileDone:
		if opensFiles && !isLauncher(f.name) {
			actions = append(actions, fileOpen)
		}
		actions = append(actions, fileSave)
	}
	for _, a := range actions {
		if state(u.fileButtons, fileButton{m.id, a}).Clicked(gtx) {
			switch a {
			case fileSave:
				u.save(m)
			case fileOpen:
				u.openFile(m)
			default:
				u.core.fileDo(id, m.id, a)
			}
		}
	}

	children := []layout.FlexChild{}
	if f.state == fileDone && isPicture(f.name) {
		if t := u.thumbnail(m); t.ok {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := t.img.Size()
					w := min(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(size.X)), gtx.Dp(240))
					gtx.Constraints = layout.Exact(image.Pt(w, w*size.Y/max(size.X, 1)))
					return widget.Image{Src: t.img, Fit: widget.Contain}.Layout(gtx)
				})
			}))
		}
	}
	children = append(children,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Body1(u.th, f.name)
			l.WrapPolicy = text.WrapGraphemes
			return l.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(u.th, fileStatus(m))
			l.Color = secondaryText
			return l.Layout(gtx)
		}),
	)
	if f.state == fileActive && f.size != 0 && f.size != tox.FileSizeUnknown && f.done != 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(200)
				return material.ProgressBar(u.th, float32(f.done)/float32(f.size)).Layout(gtx)
			})
		}))
	}
	if len(actions) != 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			buttons := []layout.FlexChild{}
			for i, a := range actions {
				if i != 0 {
					buttons = append(buttons, layout.Rigid(layout.Spacer{Width: 8}.Layout))
				}
				buttons = append(buttons, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					b := material.Button(u.th, state(u.fileButtons, fileButton{m.id, a}), fileActionLabel(a, m))
					b.TextSize = u.th.TextSize * 0.85
					b.Inset = layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}
					return b.Layout(gtx)
				}))
			}
			return layout.Inset{Top: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx, buttons...)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func fileActionLabel(a fileAction, m message) string {
	switch a {
	case fileAccept:
		return "Accept"
	case filePause:
		return "Pause"
	case fileResume:
		return "Resume"
	case fileCancel:
		if m.file.state == fileOffered {
			return "Decline"
		}
		return "Cancel"
	case fileOpen:
		return "Open"
	}
	return "Save"
}

// isLauncher reports whether a file is a desktop's launcher: opening one runs
// a command, which a friend's file mustn't do with a click.
func isLauncher(name string) bool {
	return strings.EqualFold(filepath.Ext(name), ".desktop")
}

// layoutAvatar draws someone's avatar in a circle, or the first letter of
// their name on a colour from their key.
func (u *ui) layoutAvatar(gtx layout.Context, a *avatar, name, key string, size unit.Dp) layout.Dimensions {
	px := gtx.Dp(size)
	dims := image.Pt(px, px)
	defer clip.Ellipse{Max: dims}.Push(gtx.Ops).Pop()
	if a != nil {
		img := u.avatarImage(a)
		gtx.Constraints = layout.Exact(dims)
		return widget.Image{Src: img, Fit: widget.Cover}.Layout(gtx)
	}
	paint.Fill(gtx.Ops, avatarColor(key))
	r, _ := utf8.DecodeRuneInString(name)
	gtx.Constraints = layout.Exact(dims)
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		l := material.Label(u.th, unit.Sp(float32(size)*0.45), string(unicode.ToUpper(r)))
		l.Color = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
		return l.Layout(gtx)
	})
}

// avatarImage returns the image to paint an avatar with, made once.
func (u *ui) avatarImage(a *avatar) paint.ImageOp {
	if img, ok := u.avatars[a]; ok {
		return img
	}
	img := paint.NewImageOp(a.image)
	u.avatars[a] = img
	return img
}

// avatarColor gives each key a colour of its own.
func avatarColor(key string) color.NRGBA {
	colors := []color.NRGBA{
		{R: 0x5c, G: 0x6b, B: 0xc0, A: 0xff},
		{R: 0x26, G: 0xa6, B: 0x9a, A: 0xff},
		{R: 0xef, G: 0x6c, B: 0x00, A: 0xff},
		{R: 0x8e, G: 0x24, B: 0xaa, A: 0xff},
		{R: 0x00, G: 0x89, B: 0x7b, A: 0xff},
		{R: 0xd8, G: 0x1b, B: 0x60, A: 0xff},
	}
	h := 0
	for _, r := range key {
		h = h*31 + int(r)
	}
	return colors[(h%len(colors)+len(colors))%len(colors)]
}

// withDot draws a status dot over the corner of w.
func withDot(gtx layout.Context, c color.NRGBA, w layout.Widget) layout.Dimensions {
	dims := w(gtx)
	d, ring := gtx.Dp(14), gtx.Dp(2)
	at := dims.Size.Sub(image.Pt(d, d))
	paint.FillShape(gtx.Ops, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
		clip.Ellipse{Min: at.Sub(image.Pt(ring, ring)), Max: at.Add(image.Pt(d+ring, d+ring))}.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Min: at, Max: at.Add(image.Pt(d, d))}.Op(gtx.Ops))
	return dims
}
