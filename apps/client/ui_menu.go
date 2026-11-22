package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// scrim darkens the page under the menu.
var scrim = color.NRGBA{A: 0x50}

// layoutMenu draws the menu over the page while it's open: the profiles to
// switch between, with their unread messages, a new one, and the settings.
func (u *ui) layoutMenu(gtx layout.Context) layout.Dimensions {
	if !u.menuOpen {
		return layout.Dimensions{}
	}
	if u.menuClicks == nil {
		u.menuClicks = map[string]*widget.Clickable{}
	}
	profiles := u.home.profiles()
	choose := func(action func()) {
		u.menuOpen = false
		action()
	}
	for _, pi := range profiles {
		if state(u.menuClicks, pi.name).Clicked(gtx) {
			choose(func() { u.openFromMenu(pi.name) })
		}
	}
	if u.menuAdd.Clicked(gtx) {
		choose(func() { u.openFromMenu("") })
	}
	if u.menuSettings.Clicked(gtx) {
		choose(u.openProfilePage)
	}
	if u.menuScrim.Clicked(gtx) {
		u.menuOpen = false
	}

	rows := []layout.FlexChild{
		layout.Rigid(material.H6(u.th, "Profiles").Layout),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
	}
	for _, pi := range profiles {
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.layoutMenuProfile(gtx, pi)
		}))
	}
	rows = append(rows,
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(u.lightButton(&u.menuAdd, "Add a profile")),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(u.lightButton(&u.menuSettings, "Settings")),
	)
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return u.menuScrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, scrim, clip.Rect{Max: gtx.Constraints.Max}.Op())
				return layout.Dimensions{Size: gtx.Constraints.Max}
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(min(gtx.Dp(300), gtx.Constraints.Max.X), gtx.Constraints.Max.Y)
			gtx.Constraints = layout.Exact(size)
			// Taps on the menu stay on it, not on the page under it.
			return u.menuPanel.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				paint.FillShape(gtx.Ops, u.th.Palette.Bg, clip.Rect{Max: size}.Op())
				return layout.UniformInset(16).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
				})
			})
		}),
	)
}

// layoutMenuProfile is a profile in the menu: running ones say how they're
// connected and how many messages wait; others wait for their password.
func (u *ui) layoutMenuProfile(gtx layout.Context, pi profileInfo) layout.Dimensions {
	var v *view
	if s := u.home.sessions[pi.name]; s != nil {
		v = s.core.snapshot(nil)
	}
	click := state(u.menuClicks, pi.name)
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Background{}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				if pi.name == u.home.shown || click.Hovered() {
					paint.FillShape(gtx.Ops, incomingBg, clip.Rect{Max: gtx.Constraints.Min}.Op())
				}
				return layout.Dimensions{Size: gtx.Constraints.Min}
			},
			func(gtx layout.Context) layout.Dimensions {
				return u.layoutMenuRow(gtx, pi, v)
			})
	})
}

func (u *ui) layoutMenuRow(gtx layout.Context, pi profileInfo, v *view) layout.Dimensions {
	return layout.Inset{Top: 8, Bottom: 8, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		about := "Locked: tap to open it with its password"
		var av *avatar
		unread := 0
		if v != nil {
			about, av = selfStatusText(v), v.avatar
			for _, cv := range v.chats {
				unread += cv.unread
			}
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return u.layoutAvatar(gtx, av, pi.name, pi.name, 40)
			}),
			layout.Rigid(layout.Spacer{Width: 12}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(material.Body1(u.th, pi.name).Layout),
					layout.Rigid(material.Caption(u.th, about).Layout),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if unread == 0 {
					return layout.Dimensions{}
				}
				return badge(gtx, u.th, unread)
			}),
		)
	})
}

// openFromMenu shows a running profile, or the page that opens a locked one
// with its password, or makes a new one with "".
func (u *ui) openFromMenu(name string) {
	if u.home.sessions[name] != nil {
		u.home.show(name)
		u.navigate(pageFriends)
		return
	}
	u.start.chosen = name
	u.start.pass.SetText("")
	u.start.err = ""
	u.navigate(pageProfiles)
}
