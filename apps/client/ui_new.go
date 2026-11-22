package main

import (
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// newPage makes and joins groups. Adding a friend uses the ui's own editors,
// which Tox ID links fill in.
type newPage struct {
	list         widget.List
	groupName    widget.Editor
	public       widget.Bool
	create       widget.Clickable
	joinID       widget.Editor
	joinPassword widget.Editor
	join         widget.Clickable
	conference   widget.Clickable
	err          string
}

func (u *ui) layoutNew(gtx layout.Context, wide bool) layout.Dimensions {
	p := &u.newPage
	p.list.Axis = layout.Vertical
	p.groupName.SingleLine = true
	p.joinID.SingleLine = true
	p.joinPassword.SingleLine = true
	p.joinPassword.Mask = '•'

	submit := u.addSubmit.Clicked(gtx)
	for {
		ev, ok := u.addID.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			submit = true
		}
	}
	if u.pasteID.Clicked(gtx) {
		u.addID.SetText("")
		gtx.Execute(clipboard.ReadCmd{Tag: &u.addID})
	}
	if submit {
		if err := u.core.friendAdd(u.addID.Text(), u.addText.Text()); err != nil {
			u.addError = err.Error()
		} else {
			u.addID.SetText("")
			u.addText.SetText("")
			u.addError = ""
			u.navigate(pageFriends)
		}
	}
	opened := func(id chatID, err error) {
		if err != nil {
			p.err = err.Error()
			return
		}
		p.err = ""
		u.openChat(id)
	}
	if p.create.Clicked(gtx) {
		id, err := u.core.createGroup(p.groupName.Text(), p.public.Value)
		if err == nil {
			p.groupName.SetText("")
		}
		opened(id, err)
	}
	if p.join.Clicked(gtx) {
		id, err := u.core.joinGroup(p.joinID.Text(), p.joinPassword.Text())
		if err == nil {
			p.joinID.SetText("")
			p.joinPassword.SetText("")
		}
		opened(id, err)
	}
	if p.conference.Clicked(gtx) {
		opened(u.core.createConference(p.groupName.Text()))
	}

	rows := []layout.Widget{
		material.H6(u.th, "Add a friend").Layout,
		gap(8),
		material.Body2(u.th, "Their Tox ID").Layout,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, u.field(&u.addID, "76 hex digits")),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(material.Button(u.th, &u.pasteID, "Paste").Layout),
			)
		},
		gap(16),
		material.Body2(u.th, "Message").Layout,
		u.field(&u.addText, "Hi, I'd like to add you as a friend."),
		gap(16),
		u.errorText(u.addError),
		material.Button(u.th, &u.addSubmit, "Send friend request").Layout,
		gap(32),
		material.H6(u.th, "Make a group").Layout,
		gap(8),
		material.Body2(u.th, "Name").Layout,
		u.field(&p.groupName, "What the group is called"),
		gap(8),
		material.CheckBox(u.th, &p.public, "Public: anyone with its chat ID can join").Layout,
		gap(8),
		material.Button(u.th, &p.create, "Make the group").Layout,
		gap(32),
		material.H6(u.th, "Join a group").Layout,
		gap(8),
		material.Body2(u.th, "Its chat ID").Layout,
		u.field(&p.joinID, "64 hex digits"),
		gap(8),
		material.Body2(u.th, "Password, if it has one").Layout,
		u.field(&p.joinPassword, ""),
		gap(8),
		material.Button(u.th, &p.join, "Join").Layout,
		gap(16),
		u.errorText(p.err),
		gap(16),
		material.Body2(u.th, "Old-style conferences still work with some apps: they take the name above.").Layout,
		gap(8),
		u.lightButton(&p.conference, "Make a conference"),
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.header(gtx, !wide, "New", "")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return u.layoutForm(gtx, &p.list, rows)
		}),
	)
}
