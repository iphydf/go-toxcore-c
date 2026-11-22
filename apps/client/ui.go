package main

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gioui.org/x/explorer"

	tox "github.com/TokTok/go-toxcore-c"
	"rsc.io/qr"
)

// A page is what a narrow window shows, or a wide one shows next to the
// friend list.
type page int

const (
	pageFriends page = iota
	pageChat
	// pageNew adds a friend, or makes or joins a group.
	pageNew
	pageProfile
	// pageInfo shows who the open chat is with.
	pageInfo
	// pageCall shows the call going on.
	pageCall
	// pageProfiles opens a profile with a password, or makes or imports one.
	pageProfiles
)

//go:embed appicon.png
var logoPNG []byte

// logo is the app's icon, for the empty friend list.
var logo = func() paint.ImageOp {
	img, err := png.Decode(bytes.NewReader(logoPNG))
	if err != nil {
		panic(err)
	}
	return paint.NewImageOp(img)
}()

// Windows at least this wide show the friend list and a page side by side.
const wideWidth = unit.Dp(720)

var (
	divider       = color.NRGBA{R: 0xe0, G: 0xe0, B: 0xe0, A: 0xff}
	headerBg      = color.NRGBA{R: 0x3f, G: 0x51, B: 0xb5, A: 0xff}
	outgoingBg    = color.NRGBA{R: 0xd9, G: 0xe6, B: 0xff, A: 0xff}
	incomingBg    = color.NRGBA{R: 0xef, G: 0xef, B: 0xef, A: 0xff}
	requestBg     = color.NRGBA{R: 0xff, G: 0xf4, B: 0xd6, A: 0xff}
	onlineColor   = color.NRGBA{R: 0x43, G: 0xa0, B: 0x47, A: 0xff}
	awayColor     = color.NRGBA{R: 0xf9, G: 0xa8, B: 0x25, A: 0xff}
	busyColor     = color.NRGBA{R: 0xe5, G: 0x39, B: 0x35, A: 0xff}
	offlineColor  = color.NRGBA{R: 0x9e, G: 0x9e, B: 0x9e, A: 0xff}
	secondaryText = color.NRGBA{R: 0x75, G: 0x75, B: 0x75, A: 0xff}
)

type ui struct {
	home *home
	// core is the home's running profile's, or nil before one runs.
	core *core
	th   *material.Theme
	view *view
	// focused is set while the window is focused.
	focused bool
	// invalidate draws the window again, from any goroutine.
	invalidate func()
	// explorer opens and saves files, in the current window.
	explorer *explorer.Explorer
	files    files
	// fileButtons are the buttons of files in chats.
	fileButtons map[fileButton]*widget.Clickable
	// avatars are the images of avatars on screen.
	avatars map[*avatar]paint.ImageOp

	page page
	// chat is the open chat, on pageChat and pageInfo.
	chat chatID
	// opened is the chat the core has open, if it has one.
	opened *chatID

	editorAreas  map[*widget.Editor]*widget.Clickable
	friendList   widget.List
	friendClicks map[chatID]*widget.Clickable
	accepts      map[tox.PublicKey]*widget.Clickable
	rejects      map[tox.PublicKey]*widget.Clickable
	addButton    widget.Clickable
	meButton     widget.Clickable
	backButton   widget.Clickable

	messages widget.List
	// selectables let the user select and copy messages' text, by their id.
	selectables map[int64]*widget.Selectable
	input       widget.Editor
	sendButton  widget.Clickable
	fileButton  widget.Clickable
	infoButton  widget.Clickable
	typing      bool
	// joins are the join buttons of invitations, by message.
	joins map[int64]*widget.Clickable

	friendPage friendPage
	groupPage  groupPage
	newPage    newPage
	start      startPage
	callPage   callPage
	callButton widget.Clickable

	addID     widget.Editor
	pasteID   widget.Clickable
	addText   widget.Editor
	addSubmit widget.Clickable
	addError  string

	nameEditor   widget.Editor
	statusEditor widget.Editor
	saveProfile  widget.Clickable
	copyID       widget.Clickable
	idText       widget.Selectable
	profileError string
	profileList  widget.List
	status       widget.Enum
	newID        confirmation
	avatarChoose widget.Clickable
	avatarRemove widget.Clickable
	// qr is the QR code of qrText.
	qr     paint.ImageOp
	qrText string

	// copiedBy is the copy button clicked last, at copied.
	copiedBy *widget.Clickable
	copied   time.Time

	settings settingsPage

	// menuOpen shows the menu: the profiles, a new one, and the settings.
	menuOpen     bool
	menuButton   widget.Clickable
	menuClicks   map[string]*widget.Clickable
	menuAdd      widget.Clickable
	menuSettings widget.Clickable
	menuScrim    widget.Clickable
	menuPanel    widget.Clickable
	// followed is the profile whose call was brought up, so a call shows
	// itself once.
	followed string
}

func focusCmd(e *widget.Editor) key.FocusCmd { return key.FocusCmd{Tag: e} }

// fontFiles has fonts/emoji.ttf, Noto Color Emoji, if fonts/emoji.sh made it
// before the build. The systems' own emoji fonts may be in a format Gio can't
// draw, as Android's are.
//
//go:embed fonts
var fontFiles embed.FS

// fonts are the Go fonts and the emoji. Other scripts come from the system's
// fonts.
var fonts = sync.OnceValue(func() []font.FontFace {
	faces := gofont.Collection()
	data, err := fontFiles.ReadFile("fonts/emoji.ttf")
	if err != nil {
		log.Print("no emoji font: run fonts/emoji.sh before building")
		return faces
	}
	emoji, err := opentype.ParseCollection(data)
	if err != nil {
		log.Printf("emoji font: %v", err)
		return faces
	}
	// The shaper asks for fonts by family: by its own name, Android's emoji
	// font would do too.
	for i := range emoji {
		emoji[i].Font.Typeface = "Go Tox Emoji"
	}
	return append(faces, emoji...)
})

func newUI(h *home) *ui {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(fonts()))
	th.Palette.ContrastBg = headerBg
	u := &ui{
		home:         h,
		core:         h.core(),
		th:           th,
		editorAreas:  map[*widget.Editor]*widget.Clickable{},
		friendClicks: map[chatID]*widget.Clickable{},
		joins:        map[int64]*widget.Clickable{},
		accepts:      map[tox.PublicKey]*widget.Clickable{},
		rejects:      map[tox.PublicKey]*widget.Clickable{},
		selectables:  map[int64]*widget.Selectable{},
		fileButtons:  map[fileButton]*widget.Clickable{},
		avatars:      map[*avatar]paint.ImageOp{},
		invalidate:   func() {},
	}
	u.files.thumbnails = map[int64]*thumbnail{}
	u.friendList.Axis = layout.Vertical
	u.messages.Axis = layout.Vertical
	u.messages.ScrollToEnd = true
	u.input.Submit = true
	u.addID.SingleLine = true
	u.addID.Submit = true
	u.profileList.Axis = layout.Vertical
	u.nameEditor.SingleLine = true
	u.statusEditor.SingleLine = true
	return u
}

func (u *ui) Layout(gtx layout.Context) layout.Dimensions {
	// A call in another profile shows itself there, once.
	if caller := u.home.caller(); caller != u.followed {
		u.followed = caller
		if caller != "" {
			u.home.show(caller)
		}
	}
	if c := u.home.core(); c != u.core {
		u.setCore(c)
	}
	if u.core == nil {
		return u.layoutStart(gtx)
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(u.layoutMain),
		layout.Expanded(u.layoutMenu),
	)
}

func (u *ui) layoutMain(gtx layout.Context) layout.Dimensions {
	u.view = u.core.snapshot(u.view)
	u.setLook(u.view.settings)
	u.followCall()
	u.handleBack(gtx)

	if gtx.Constraints.Max.X < gtx.Dp(wideWidth) {
		return u.layoutPage(gtx, false)
	}
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(340)
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			return u.layoutFriends(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Dp(1), gtx.Constraints.Max.Y)
			paint.FillShape(gtx.Ops, divider, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if u.page == pageFriends {
				return layout.Center.Layout(gtx, material.Body1(u.th, "Choose a chat.").Layout)
			}
			return u.layoutPage(gtx, true)
		}),
	)
}

func (u *ui) layoutPage(gtx layout.Context, wide bool) layout.Dimensions {
	switch u.page {
	case pageChat:
		return u.layoutChat(gtx, wide)
	case pageNew:
		return u.layoutNew(gtx, wide)
	case pageProfile:
		return u.layoutProfile(gtx, wide)
	case pageCall:
		return u.layoutCall(gtx)
	case pageProfiles:
		return u.layoutStart(gtx)
	case pageInfo:
		if u.chat.kind == chatFriend {
			return u.friendPage.layout(gtx, u)
		}
		return u.groupPage.layout(gtx, u)
	}
	return u.layoutFriends(gtx)
}

// setCore shows a new core's state, after a restart, or the start page
// with nil.
func (u *ui) setCore(c *core) {
	if u.core != nil {
		// Only the profile on screen reads its chats.
		u.core.setFocused(false)
	}
	u.core = c
	u.view = nil
	u.opened = nil
	if c == nil {
		u.page = pageFriends
		return
	}
	c.setFocused(u.focused)
	u.view = c.snapshot(nil)
	switch u.page {
	case pageChat:
		id := u.chat
		u.opened = &id
		c.setOpen(&id)
	case pageInfo:
		u.page = pageFriends
	}
}

func (u *ui) setFocused(focused bool) {
	if focused != u.focused {
		u.focused = focused
		if u.core != nil {
			u.core.setFocused(focused)
		}
	}
}

// chatURL is the URL openURL opens a profile's chat, or its friend requests,
// with.
func chatURL(profile string, id chatID) string {
	return "gotox:chat/" + url.PathEscape(profile) + "/" + id.String()
}

// openURL opens a chat from a notification, or the add page from a Tox ID
// link.
func (u *ui) openURL(link *url.URL) {
	if u.core == nil {
		return
	}
	switch link.Scheme {
	case "gotox":
		profile, chat, ok := strings.Cut(strings.TrimPrefix(link.Opaque, "chat/"), "/")
		if !ok {
			return
		}
		if name, err := url.PathUnescape(profile); err == nil && name != u.home.shown {
			u.home.show(name)
			u.setCore(u.home.core())
		}
		id, err := parseChatID(chat)
		if err != nil {
			return
		}
		u.view = u.core.snapshot(u.view)
		if _, ok := u.view.chatView(id); ok {
			u.openChat(id)
			return
		}
		// A friend request.
		u.navigate(pageFriends)
	case "tox":
		u.addID.SetText(link.Opaque)
		u.addError = ""
		u.navigate(pageNew)
	}
}

func (u *ui) openChat(id chatID) {
	if u.page == pageChat && u.chat == id {
		return
	}
	u.chat = id
	u.input.SetText("")
	u.page = pageFriends
	u.navigate(pageChat)
}

// navigate shows a page. Opening a chat marks its messages read.
func (u *ui) navigate(p page) {
	if u.page == pageChat && u.typing {
		u.core.setTyping(u.chat, false)
		u.typing = false
	}
	from := u.page
	u.page = p
	switch {
	case p == pageChat && (u.opened == nil || *u.opened != u.chat || from == pageChat || from == pageFriends):
		u.messages.Position = layout.Position{}
		id := u.chat
		u.opened = &id
		u.core.setOpen(&id)
	case p == pageFriends || p == pageNew || p == pageProfile:
		u.opened = nil
		u.core.setOpen(nil)
	}
}

// handleBack goes back a page on Android's back button or Escape: from a
// friend's page to their chat, from others to the friend list.
func (u *ui) handleBack(gtx layout.Context) {
	if u.page == pageFriends {
		return
	}
	back := u.backButton.Clicked(gtx)
	for {
		ev, ok := gtx.Event(key.Filter{Name: key.NameBack}, key.Filter{Name: key.NameEscape})
		if !ok {
			break
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			back = true
		}
	}
	switch {
	case !back:
	case u.page == pageInfo || u.page == pageCall:
		u.navigate(pageChat)
	default:
		u.navigate(pageFriends)
	}
}

// header draws a page's title bar. back adds a back button, for narrow
// windows.
func (u *ui) header(gtx layout.Context, back bool, title, subtitle string, actions ...layout.FlexChild) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, headerBg, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			// Every header is as high, with buttons or without.
			gtx.Constraints.Min.Y = gtx.Dp(56)
			return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{}
				if back {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.headerButton(gtx, &u.backButton, "←")
					}))
				}
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.H6(u.th, title)
								l.Color = u.th.Palette.ContrastFg
								l.MaxLines = 1
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if subtitle == "" {
									return layout.Dimensions{}
								}
								l := material.Caption(u.th, subtitle)
								l.Color = u.th.Palette.ContrastFg
								l.MaxLines = 1
								return l.Layout(gtx)
							}),
						)
					})
				}))
				children = append(children, actions...)
				gtx.Constraints.Min.Y -= gtx.Dp(16)
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
			})
		})
}

func (u *ui) headerButton(gtx layout.Context, c *widget.Clickable, label string) layout.Dimensions {
	b := material.Button(u.th, c, label)
	b.Background = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x30}
	return b.Layout(gtx)
}

// openProfilePage shows the profile and its settings, as they are now.
func (u *ui) openProfilePage() {
	v := u.view
	u.nameEditor.SetText(v.name)
	u.statusEditor.SetText(v.statusMessage)
	u.status.Value = statusKey(v.status)
	u.newID.asking = false
	u.settings.open(v)
	u.profileError = ""
	u.navigate(pageProfile)
}

func (u *ui) layoutFriends(gtx layout.Context) layout.Dimensions {
	v := u.view
	if u.addButton.Clicked(gtx) {
		u.addError = ""
		u.newPage.err = ""
		u.navigate(pageNew)
	}
	if u.meButton.Clicked(gtx) {
		u.openProfilePage()
	}
	for _, cv := range v.chats {
		if state(u.friendClicks, cv.id).Clicked(gtx) {
			u.openChat(cv.id)
		}
	}
	for _, r := range v.requests {
		if state(u.accepts, r.key).Clicked(gtx) {
			if err := u.core.accept(r.key); err != nil {
				u.addError = err.Error()
			}
		}
		if state(u.rejects, r.key).Clicked(gtx) {
			u.core.reject(r.key)
		}
	}

	if u.menuButton.Clicked(gtx) {
		u.menuOpen = true
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.header(gtx, false, v.name, selfStatusText(v),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.headerButton(gtx, &u.menuButton, "Menu")
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.headerButton(gtx, &u.addButton, "New")
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.headerButton(gtx, &u.meButton, "Me")
				}))
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(v.chats) == 0 && len(v.requests) == 0 {
				return layout.UniformInset(24).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							img := widget.Image{Src: logo, Fit: widget.Contain}
							size := image.Pt(gtx.Dp(128), gtx.Dp(128))
							gtx.Constraints = layout.Exact(size)
							// The icon is full-bleed for Android's launcher shapes.
							defer clip.Ellipse{Max: size}.Push(gtx.Ops).Pop()
							return img.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Height: 24}.Layout),
						layout.Rigid(material.Body1(u.th, "No chats yet. Add a friend with their Tox ID from New, or give them yours from Me.").Layout),
					)
				})
			}
			l := material.List(u.th, &u.friendList)
			l.AnchorStrategy = material.Overlay
			return l.Layout(gtx, len(v.requests)+len(v.chats),
				func(gtx layout.Context, i int) layout.Dimensions {
					if i < len(v.requests) {
						return u.layoutRequest(gtx, v.requests[i])
					}
					return u.layoutChatRow(gtx, v.chats[i-len(v.requests)])
				})
		}),
	)
}

// state returns a widget's state for a key, making it the first time.
func state[K comparable, V any](m map[K]*V, k K) *V {
	if m[k] == nil {
		m[k] = new(V)
	}
	return m[k]
}

func (u *ui) layoutRequest(gtx layout.Context, r request) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, requestBg, clip.Rect{Max: gtx.Constraints.Min}.Op())
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(12).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Body2(u.th, "Friend request from").Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Caption(u.th, r.key.String())
						l.WrapPolicy = text.WrapGraphemes
						return l.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: 4}.Layout),
					layout.Rigid(material.Body1(u.th, r.message).Layout),
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(material.Button(u.th, state(u.accepts, r.key), "Accept").Layout),
							layout.Rigid(layout.Spacer{Width: 8}.Layout),
							layout.Rigid(material.Button(u.th, state(u.rejects, r.key), "Ignore").Layout),
						)
					}),
				)
			})
		})
}

func (u *ui) layoutChatRow(gtx layout.Context, cv chatView) layout.Dimensions {
	click := state(u.friendClicks, cv.id)
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		selected := (u.page == pageChat || u.page == pageInfo) && u.chat == cv.id
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				if selected || click.Hovered() {
					paint.FillShape(gtx.Ops, incomingBg, clip.Rect{Max: gtx.Constraints.Min}.Op())
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Inset{Top: 10, Bottom: 10, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return withDot(gtx, statusColor(cv), func(gtx layout.Context) layout.Dimensions {
								return u.layoutAvatar(gtx, cv.avatar(), cv.title, cv.id.String(), 44)
							})
						}),
						layout.Rigid(layout.Spacer{Width: 12}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Body1(u.th, cv.title)
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									l := material.Caption(u.th, preview(cv))
									l.Color = secondaryText
									l.MaxLines = 1
									return l.Layout(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if cv.unread == 0 {
								return layout.Dimensions{}
							}
							return badge(gtx, u.th, cv.unread)
						}),
					)
				})
			})
	})
}

// avatar is the friend's avatar, if the chat is with one who has one.
func (cv chatView) avatar() *avatar {
	if cv.friend != nil {
		return cv.friend.avatar
	}
	return nil
}

// preview is the chat's last message, or what else there is to say.
func preview(cv chatView) string {
	if cv.friend != nil && cv.friend.typing {
		return "typing…"
	}
	if m := cv.last; m != nil {
		switch {
		case m.outgoing:
			return "You: " + m.preview()
		case m.senderName != "":
			return m.senderName + ": " + m.preview()
		}
		return m.preview()
	}
	if cv.friend != nil {
		return cv.friend.statusMessage
	}
	return cv.statusText()
}

func statusColor(cv chatView) color.NRGBA {
	switch {
	case !cv.online():
		return offlineColor
	case cv.friend == nil:
		return onlineColor
	}
	switch cv.friend.status {
	case tox.UserStatusAway:
		return awayColor
	case tox.UserStatusBusy:
		return busyColor
	}
	return onlineColor
}

func statusDot(gtx layout.Context, c color.NRGBA) layout.Dimensions {
	d := gtx.Dp(12)
	paint.FillShape(gtx.Ops, c, clip.Ellipse{Max: image.Pt(d, d)}.Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(d, d)}
}

func badge(gtx layout.Context, th *material.Theme, n int) layout.Dimensions {
	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			r := gtx.Constraints.Min.Y / 2
			paint.FillShape(gtx.Ops, headerBg, clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, r).Op(gtx.Ops))
			return layout.Dimensions{Size: gtx.Constraints.Min}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 7, Right: 7, Top: 2, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(th, itoa(n))
				l.Color = th.Palette.ContrastFg
				return l.Layout(gtx)
			})
		})
}

func itoa(n int) string {
	if n > 99 {
		return "99+"
	}
	s := ""
	for {
		s = string(rune('0'+n%10)) + s
		n /= 10
		if n == 0 {
			return s
		}
	}
}

func (u *ui) layoutChat(gtx layout.Context, wide bool) layout.Dimensions {
	cv, ok := u.view.chatView(u.chat)
	if !ok {
		u.navigate(pageFriends)
		return layout.Dimensions{}
	}
	if u.infoButton.Clicked(gtx) {
		if cv.friend != nil {
			u.friendPage.open(cv)
		} else {
			u.groupPage.open(cv)
		}
		u.navigate(pageInfo)
	}
	if u.fileButton.Clicked(gtx) {
		u.clearNotice()
		u.attach(cv.id)
	}
	inCall := u.view.call != nil && u.view.call.chat == cv.id
	if u.callButton.Clicked(gtx) {
		if inCall {
			u.navigate(pageCall)
		} else {
			u.startCall(cv.id, false)
		}
	}
	send := u.sendButton.Clicked(gtx)
	for {
		ev, ok := u.input.Update(gtx)
		if !ok {
			break
		}
		switch ev.(type) {
		case widget.SubmitEvent:
			send = true
		case widget.ChangeEvent:
			if typing := u.input.Text() != ""; typing != u.typing {
				u.typing = typing
				u.core.setTyping(cv.id, typing)
			}
		}
	}
	if send {
		u.core.send(cv.id, u.input.Text())
		u.input.SetText("")
		if u.typing {
			u.typing = false
			u.core.setTyping(cv.id, false)
		}
	}

	// At the top, the next page goes above what's on screen, which stays put.
	if u.view.older && u.messages.Position.First == 0 {
		if n := u.core.loadOlder(); n != 0 {
			u.messages.Position.First += n
			u.view = u.core.snapshot(u.view)
		}
	}
	chat := u.view.chat
	many := cv.group != nil || cv.conference != nil

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.header(gtx, !wide, cv.title, cv.statusText(),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if cv.friend == nil {
						return layout.Dimensions{}
					}
					label := "Call"
					if inCall {
						label = "In call"
					}
					return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return u.headerButton(gtx, &u.callButton, label)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.headerButton(gtx, &u.infoButton, "Info")
				}))
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if len(chat) == 0 {
				return layout.Center.Layout(gtx, material.Body2(u.th, "No messages yet.").Layout)
			}
			return material.List(u.th, &u.messages).Layout(gtx, len(chat), func(gtx layout.Context, i int) layout.Dimensions {
				m := chat[i]
				newDay := i == 0 || !sameDay(chat[i-1].time, m.time)
				// In a group, a run of messages from one peer shows their name
				// once.
				sender := many && !m.outgoing && !m.notice &&
					(newDay || chat[i-1].outgoing || chat[i-1].notice || chat[i-1].senderName != m.senderName)
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !newDay {
							return layout.Dimensions{}
						}
						return u.layoutDay(gtx, m.time, gtx.Now)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.layoutMessage(gtx, cv, m, sender)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
			paint.FillShape(gtx.Ops, divider, clip.Rect{Max: size}.Op())
			return layout.Dimensions{Size: size}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			notice := u.notice()
			if notice == "" {
				return layout.Dimensions{}
			}
			return layout.Inset{Top: 8, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(u.th, notice)
				l.Color = busyColor
				return l.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.End}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Files go to friends.
						if cv.friend == nil {
							return layout.Dimensions{}
						}
						return layout.Inset{Right: 4}.Layout(gtx, u.lightButton(&u.fileButton, "File"))
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.Y = gtx.Dp(136)
						return u.editorArea(gtx, &u.input, "Message", layout.UniformInset(8))
					}),
					layout.Rigid(material.Button(u.th, &u.sendButton, "Send").Layout),
				)
			})
		}),
	)
}

// lightButton is a button that stands back from the main one.
func (u *ui) lightButton(c *widget.Clickable, label string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		b := material.Button(u.th, c, label)
		b.Background = incomingBg
		b.Color = u.th.Palette.Fg
		return b.Layout(gtx)
	}
}

func (u *ui) layoutMessage(gtx layout.Context, cv chatView, m message, sender bool) layout.Dimensions {
	if m.notice {
		return layout.Inset{Top: 6, Bottom: 6, Left: 24, Right: 24}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Caption(u.th, m.text)
				l.Color = secondaryText
				l.Alignment = text.Middle
				return l.Layout(gtx)
			})
		})
	}
	bg, align, inside := incomingBg, layout.W, layout.Start
	if m.outgoing {
		bg, align, inside = outgoingBg, layout.E, layout.End
	}
	body := m.text
	if m.action {
		body = "* " + body
	}
	info := m.time.Local().Format("15:04")
	if m.outgoing && m.file == nil && m.invite == nil {
		switch {
		case m.delivered && cv.friend != nil:
			info += " · delivered"
		case !m.sent:
			info += " · waiting"
		}
	}
	return layout.Inset{Top: 4, Bottom: 4, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return align.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = 0
			gtx.Constraints.Max.X = gtx.Constraints.Max.X * 4 / 5
			return layout.Background{}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					rr := gtx.Dp(10)
					paint.FillShape(gtx.Ops, bg, clip.UniformRRect(image.Rectangle{Max: gtx.Constraints.Min}, rr).Op(gtx.Ops))
					return layout.Dimensions{Size: gtx.Constraints.Min}
				},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Alignment: inside}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !sender {
									return layout.Dimensions{}
								}
								name := m.senderName
								if name == "" {
									name = "someone"
								}
								l := material.Body2(u.th, name)
								l.Color = avatarColor(senderKey(m))
								l.Font.Weight = font.Bold
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								switch {
								case m.file != nil:
									return u.layoutFile(gtx, cv.id, m)
								case m.invite != nil:
									return u.layoutInvite(gtx, cv.id, m)
								}
								l := material.Body1(u.th, body)
								l.State = state(u.selectables, m.id)
								if m.action {
									l.Font.Style = font.Italic
								}
								return l.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								l := material.Caption(u.th, info)
								l.Color = secondaryText
								return l.Layout(gtx)
							}),
						)
					})
				})
		})
	})
}

// senderKey is what gives a group message's sender their colour.
func senderKey(m message) string {
	if m.sender != nil {
		return m.sender.String()
	}
	return m.senderName
}

// layoutInvite draws a friend's invitation to a group, with a button to join.
func (u *ui) layoutInvite(gtx layout.Context, friend chatID, m message) layout.Dimensions {
	inv := m.invite
	join := state(u.joins, m.id)
	if join.Clicked(gtx) {
		u.clearNotice()
		if id, err := u.core.joinInvite(friend, m.id); err != nil {
			u.report(err)
		} else {
			u.openChat(id)
		}
	}
	what := "Invitation to the group " + inv.name
	switch inv.kind {
	case inviteConference:
		what = "Invitation to a conference"
	case inviteAVConference:
		what = "Invitation to a conference with calls, which Go Tox can't join"
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.Body1(u.th, what).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			switch {
			case inv.kind == inviteAVConference:
				return layout.Dimensions{}
			case inv.joined:
				l := material.Caption(u.th, "Joined")
				l.Color = secondaryText
				return l.Layout(gtx)
			}
			return layout.Inset{Top: 6}.Layout(gtx, material.Button(u.th, join, "Join").Layout)
		}),
	)
}

// sameDay reports whether two times are on the same day here.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Local().Date()
	by, bm, bd := b.Local().Date()
	return ay == by && am == bm && ad == bd
}

// layoutDay draws the day the messages under it are from.
func (u *ui) layoutDay(gtx layout.Context, day, now time.Time) layout.Dimensions {
	var s string
	switch {
	case sameDay(day, now):
		s = "Today"
	case sameDay(day, now.AddDate(0, 0, -1)):
		s = "Yesterday"
	case day.Local().Year() == now.Local().Year():
		s = day.Local().Format("Monday 2 January")
	default:
		s = day.Local().Format("Monday 2 January 2006")
	}
	return layout.Inset{Top: 12, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			l := material.Caption(u.th, s)
			l.Color = secondaryText
			return l.Layout(gtx)
		})
	})
}

// editorArea draws an editor in an area that focuses it when clicked: the
// editor alone is only as high as its text.
func (u *ui) editorArea(gtx layout.Context, e *widget.Editor, hint string, inset layout.Inset) layout.Dimensions {
	area := state(u.editorAreas, e)
	if area.Clicked(gtx) {
		gtx.Execute(key.FocusCmd{Tag: e})
	}
	return area.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return material.Editor(u.th, e, hint).Layout(gtx)
		})
	})
}

// field draws an editor with a line under it.
func (u *ui) field(e *widget.Editor, hint string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return u.editorArea(gtx, e, hint, layout.Inset{Top: 8, Bottom: 8})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(1))
				paint.FillShape(gtx.Ops, secondaryText, clip.Rect{Max: size}.Op())
				return layout.Dimensions{Size: size}
			}),
		)
	}
}

func (u *ui) layoutProfile(gtx layout.Context, wide bool) layout.Dimensions {
	v := u.view
	if u.saveProfile.Clicked(gtx) {
		u.profileError = ""
		if err := u.core.setName(strings.TrimSpace(u.nameEditor.Text())); err != nil {
			u.profileError = err.Error()
		} else if err := u.core.setStatusMessage(strings.TrimSpace(u.statusEditor.Text())); err != nil {
			u.profileError = err.Error()
		}
	}
	if u.avatarChoose.Clicked(gtx) {
		u.clearNotice()
		u.chooseAvatar()
	}
	if u.avatarRemove.Clicked(gtx) {
		u.clearNotice()
		u.report(u.core.setAvatar(nil))
	}
	if u.status.Update(gtx) {
		u.core.setStatus(statuses[u.status.Value])
	}
	if u.newID.update(gtx) {
		u.core.newNospam()
		u.view = u.core.snapshot(u.view)
		v = u.view
	}

	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return u.layoutAvatar(gtx, v.avatar, v.name, v.address.String(), 72)
				}),
				layout.Rigid(layout.Spacer{Width: 16}.Layout),
				layout.Rigid(material.Button(u.th, &u.avatarChoose, "Choose picture").Layout),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if v.avatar == nil {
						return layout.Dimensions{}
					}
					return u.lightButton(&u.avatarRemove, "Remove")(gtx)
				}),
			)
		},
		u.errorText(u.notice()),
		gap(16),
		material.Body2(u.th, "Name").Layout,
		u.field(&u.nameEditor, "Your name"),
		gap(16),
		material.Body2(u.th, "Status message").Layout,
		u.field(&u.statusEditor, "What you're up to"),
		gap(16),
		u.errorText(u.profileError),
		material.Button(u.th, &u.saveProfile, "Save").Layout,
		gap(24),
		material.Body2(u.th, "Status").Layout,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(material.RadioButton(u.th, &u.status, "online", "Online").Layout),
				layout.Rigid(material.RadioButton(u.th, &u.status, "away", "Away").Layout),
				layout.Rigid(material.RadioButton(u.th, &u.status, "busy", "Busy").Layout),
			)
		},
		gap(32),
		material.Body2(u.th, "Your Tox ID: give it to friends so they can add you.").Layout,
		gap(8),
		func(gtx layout.Context) layout.Dimensions {
			l := material.Body1(u.th, v.address.String())
			l.WrapPolicy = text.WrapGraphemes
			l.State = &u.idText
			return l.Layout(gtx)
		},
		gap(8),
		func(gtx layout.Context) layout.Dimensions {
			return u.copyButton(gtx, &u.copyID, v.address.String())
		},
		gap(16),
		func(gtx layout.Context) layout.Dimensions {
			return u.layoutQR(gtx, "tox:"+v.address.String())
		},
		gap(16),
		func(gtx layout.Context) layout.Dimensions {
			return u.layoutConfirmation(gtx, &u.newID, "New Tox ID",
				"Friend requests to your current Tox ID will stop working. Your friends stay.", "Change it")
		},
	}
	rows = append(rows, u.settingsRows(gtx, v)...)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.header(gtx, !wide, "Me", selfStatusText(v))
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return u.layoutForm(gtx, &u.profileList, rows)
		}),
	)
}

// gap is space between a form's rows.
func gap(dp unit.Dp) layout.Widget { return layout.Spacer{Height: dp}.Layout }

// errorText draws an error, if there is one, over a gap.
func (u *ui) errorText(text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if text == "" {
			return layout.Dimensions{}
		}
		l := material.Body2(u.th, text)
		l.Color = busyColor
		return layout.Inset{Bottom: 16}.Layout(gtx, l.Layout)
	}
}

// layoutForm draws rows in a list that scrolls, with a margin round them.
func (u *ui) layoutForm(gtx layout.Context, list *widget.List, rows []layout.Widget) layout.Dimensions {
	return material.List(u.th, list).Layout(gtx, len(rows), func(gtx layout.Context, i int) layout.Dimensions {
		in := layout.Inset{Left: 16, Right: 16}
		if i == 0 {
			in.Top = 16
		}
		if i == len(rows)-1 {
			in.Bottom = 16
		}
		return in.Layout(gtx, rows[i])
	})
}

// statuses are the user statuses by their keys for widget.Enum.
var statuses = map[string]tox.UserStatus{
	"online": tox.UserStatusNone,
	"away":   tox.UserStatusAway,
	"busy":   tox.UserStatusBusy,
}

func statusKey(s tox.UserStatus) string {
	for k, v := range statuses {
		if v == s {
			return k
		}
	}
	return "online"
}

// selfStatusText is the user's connection, and their status if they're away
// or busy.
func selfStatusText(v *view) string {
	s := connectionText(v.connection)
	if v.status != tox.UserStatusNone {
		s += " · " + statusKey(v.status)
	}
	return s
}

// copyButton draws a button that copies text, and says so for a moment.
func (u *ui) copyButton(gtx layout.Context, c *widget.Clickable, text string) layout.Dimensions {
	if c.Clicked(gtx) {
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		u.copiedBy, u.copied = c, gtx.Now
	}
	label := "Copy"
	if u.copiedBy == c && gtx.Now.Sub(u.copied) < 2*time.Second {
		label = "Copied"
		gtx.Execute(op.InvalidateCmd{At: u.copied.Add(2 * time.Second)})
	}
	return material.Button(u.th, c, label).Layout(gtx)
}

// A confirmation is a button that asks again before it acts.
type confirmation struct {
	ask, yes, no widget.Clickable
	asking       bool
}

// update reports whether the user confirmed.
func (c *confirmation) update(gtx layout.Context) bool {
	if c.ask.Clicked(gtx) {
		c.asking = true
	}
	if c.no.Clicked(gtx) {
		c.asking = false
	}
	if c.yes.Clicked(gtx) {
		c.asking = false
		return true
	}
	return false
}

func (u *ui) layoutConfirmation(gtx layout.Context, c *confirmation, label, question, yes string) layout.Dimensions {
	if !c.asking {
		return material.Button(u.th, &c.ask, label).Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(material.Body1(u.th, question).Layout),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			b := material.Button(u.th, &c.yes, yes)
			b.Background = busyColor
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(b.Layout),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(material.Button(u.th, &c.no, "Cancel").Layout),
			)
		}),
	)
}

// qrImage draws a QR code a pixel a module, in the white border scanners
// need. The code's own Image leaves the border out.
func qrImage(code *qr.Code) image.Image {
	const border = 4
	img := image.NewGray(image.Rect(0, 0, code.Size+2*border, code.Size+2*border))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for y := range code.Size {
		for x := range code.Size {
			if code.Black(x, y) {
				img.Pix[img.PixOffset(x+border, y+border)] = 0
			}
		}
	}
	return img
}

// layoutQR draws text as a QR code.
func (u *ui) layoutQR(gtx layout.Context, text string) layout.Dimensions {
	if u.qrText != text {
		code, err := qr.Encode(text, qr.M)
		if err != nil {
			return material.Body2(u.th, err.Error()).Layout(gtx)
		}
		u.qr, u.qrText = paint.NewImageOp(qrImage(code)), text
		u.qr.Filter = paint.FilterNearest
	}
	size := image.Pt(gtx.Dp(200), gtx.Dp(200))
	gtx.Constraints = layout.Exact(size)
	return widget.Image{Src: u.qr, Fit: widget.Contain}.Layout(gtx)
}
