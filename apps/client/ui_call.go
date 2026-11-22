package main

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// callPage shows the call going on.
type callPage struct {
	answer      widget.Clickable
	answerVideo widget.Clickable
	decline     widget.Clickable
	mute        widget.Clickable
	video       widget.Clickable
	hangUp      widget.Clickable
	toChat      widget.Clickable
	// frame is the newest picture, ready to paint.
	frame   paint.ImageOp
	frameOf image.Image
	// shown is set once the page showed the call: a new call shows it.
	shown bool
}

var (
	callBg   = color.NRGBA{R: 0x1a, G: 0x1c, B: 0x24, A: 0xff}
	callText = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

// followCall shows a call that starts, and leaves its page when it ends.
func (u *ui) followCall() {
	call := u.view.call
	switch {
	case call != nil && !u.callPage.shown:
		u.callPage.shown = true
		u.chat = call.chat
		u.navigate(pageCall)
	case call == nil && u.callPage.shown:
		u.callPage.shown = false
		if u.page == pageCall {
			u.navigate(pageChat)
		}
	}
}

// startCall calls a friend, after asking for the microphone, and the camera
// for video.
func (u *ui) startCall(id chatID, video bool) {
	u.clearNotice()
	u.home.plat.askForCall(video)
	u.report(u.core.callFriend(id, video))
}

func (u *ui) layoutCall(gtx layout.Context) layout.Dimensions {
	p := &u.callPage
	call := u.view.call
	if call == nil {
		u.navigate(pageChat)
		return layout.Dimensions{}
	}
	for _, b := range []struct {
		click *widget.Clickable
		video bool
	}{{&p.answer, false}, {&p.answerVideo, true}} {
		if b.click.Clicked(gtx) {
			u.home.plat.askForCall(b.video)
			u.report(u.core.answer(b.video))
		}
	}
	if p.decline.Clicked(gtx) || p.hangUp.Clicked(gtx) {
		u.core.hangUp()
	}
	if p.mute.Clicked(gtx) {
		u.core.setMuted(!call.muted)
	}
	if p.video.Clicked(gtx) {
		if !call.video {
			u.home.plat.askForCall(true)
		}
		u.core.setVideo(!call.video)
	}
	if p.toChat.Clicked(gtx) {
		u.navigate(pageChat)
	}

	status := "Calling…"
	switch {
	case call.answered:
		status = callLength(gtx.Now.Sub(call.started))
		// The clock goes on.
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Second - gtx.Now.Sub(call.started)%time.Second)})
	case !call.outgoing:
		status = "is calling you"
	}
	if call.frame != nil && call.frame != p.frameOf {
		p.frame, p.frameOf = paint.NewImageOp(call.frame), call.frame
	}

	paint.FillShape(gtx.Ops, callBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if call.frame != nil {
				return layout.Center.Layout(gtx, widget.Image{Src: p.frame, Fit: widget.Contain}.Layout)
			}
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.layoutAvatar(gtx, call.avatar, call.title, call.chat.String(), 120)
					}),
					layout.Rigid(layout.Spacer{Height: 24}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.H5(u.th, call.title)
						l.Color = callText
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Body1(u.th, status)
						l.Color = callText
						return l.Layout(gtx)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			notice := u.notice()
			if notice == "" {
				return layout.Dimensions{}
			}
			return layout.UniformInset(8).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				l := material.Body2(u.th, notice)
				l.Color = callText
				return l.Layout(gtx)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(24).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					button := func(c *widget.Clickable, label string, bg color.NRGBA) layout.FlexChild {
						return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								b := material.Button(u.th, c, label)
								b.Background = bg
								b.Inset = layout.UniformInset(14)
								return b.Layout(gtx)
							})
						})
					}
					grey := color.NRGBA{R: 0x44, G: 0x47, B: 0x55, A: 0xff}
					if !call.outgoing && !call.answered {
						return layout.Flex{}.Layout(gtx,
							button(&p.decline, "Decline", busyColor),
							button(&p.answer, "Answer", onlineColor),
							button(&p.answerVideo, "With video", onlineColor),
						)
					}
					mute, video := "Mute", "Video"
					if call.muted {
						mute = "Unmute"
					}
					if call.video {
						video = "No video"
					}
					return layout.Flex{}.Layout(gtx,
						button(&p.toChat, "Chat", grey),
						button(&p.mute, mute, grey),
						button(&p.video, video, grey),
						button(&p.hangUp, "Hang up", busyColor),
					)
				})
			})
		}),
	)
}
