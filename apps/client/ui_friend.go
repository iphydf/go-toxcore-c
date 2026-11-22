package main

import (
	"strings"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// A friendPage shows the open chat's friend: their name and key, the
// nickname the user gave them, and a button to delete them.
type friendPage struct {
	list          widget.List
	nickname      widget.Editor
	saveNickname  widget.Clickable
	key           widget.Selectable
	copyKey       widget.Clickable
	deleteHistory widget.Bool
	delete        confirmation
	err           string
}

// open fills the page in for a friend.
func (p *friendPage) open(cv chatView) {
	f := cv.friend
	p.list.Axis = layout.Vertical
	p.nickname.SingleLine = true
	p.nickname.Submit = true
	p.nickname.SetText(f.nickname)
	p.deleteHistory.Value = false
	p.delete.asking = false
	p.err = ""
}

func (p *friendPage) layout(gtx layout.Context, u *ui) layout.Dimensions {
	cv, ok := u.view.chatView(u.chat)
	if !ok || cv.friend == nil {
		u.navigate(pageFriends)
		return layout.Dimensions{}
	}
	f := cv.friend
	save := p.saveNickname.Clicked(gtx)
	for {
		ev, ok := p.nickname.Update(gtx)
		if !ok {
			break
		}
		if _, ok := ev.(widget.SubmitEvent); ok {
			save = true
		}
	}
	if save {
		u.core.setNickname(cv.id, strings.TrimSpace(p.nickname.Text()))
	}
	if p.delete.update(gtx) {
		if err := u.core.deleteFriend(cv.id, p.deleteHistory.Value); err != nil {
			p.err = err.Error()
		} else {
			u.navigate(pageFriends)
			return layout.Dimensions{}
		}
	}

	name := f.name
	if name == "" {
		name = "not known yet"
	}
	rows := []layout.Widget{
		material.Body2(u.th, "Nickname: only you see it").Layout,
		u.field(&p.nickname, name),
		gap(8),
		material.Button(u.th, &p.saveNickname, "Save").Layout,
		gap(24),
		material.Body2(u.th, "Name").Layout,
		material.Body1(u.th, name).Layout,
		gap(16),
	}
	if f.statusMessage != "" {
		rows = append(rows,
			material.Body2(u.th, "Status message").Layout,
			material.Body1(u.th, f.statusMessage).Layout,
			gap(16),
		)
	}
	rows = append(rows,
		material.Body2(u.th, "Public key").Layout,
		func(gtx layout.Context) layout.Dimensions {
			l := material.Body1(u.th, f.key.String())
			l.WrapPolicy = text.WrapGraphemes
			l.State = &p.key
			return l.Layout(gtx)
		},
		gap(8),
		func(gtx layout.Context) layout.Dimensions {
			return u.copyButton(gtx, &p.copyKey, f.key.String())
		},
		gap(32),
		material.CheckBox(u.th, &p.deleteHistory, "Also delete the chat").Layout,
		gap(8),
		func(gtx layout.Context) layout.Dimensions {
			return u.layoutConfirmation(gtx, &p.delete, "Delete friend",
				"Delete "+cv.title+"? They can send you a friend request again.", "Delete")
		},
		gap(16),
		u.errorText(p.err),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.header(gtx, true, cv.title, cv.statusText())
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return u.layoutForm(gtx, &p.list, rows)
		}),
	)
}
