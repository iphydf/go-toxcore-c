package main

import (
	"errors"
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"

	tox "github.com/TokTok/go-toxcore-c"
)

// A peerAction is what the user does to a group's peer.
type peerAction struct {
	key  tox.PublicKey
	what string
}

// groupPage shows a group, a conference or a private chat with a group's
// peer: who is in it, and what the user can change.
type groupPage struct {
	list     widget.List
	topic    widget.Editor
	setTopic widget.Clickable
	name     widget.Editor
	setName  widget.Clickable
	chatID   widget.Selectable
	copyID   widget.Clickable
	invites  map[chatID]*widget.Clickable
	peers    map[tox.PublicKey]*widget.Clickable
	actions  map[peerAction]*widget.Clickable
	// selected is the peer whose actions show.
	selected *tox.PublicKey

	public      widget.Bool
	topicLock   widget.Bool
	voice       widget.Enum
	password    widget.Editor
	setPassword widget.Clickable
	limit       widget.Editor
	setLimit    widget.Clickable

	toGroup       widget.Clickable
	deleteHistory widget.Bool
	leave         confirmation
	err           string
}

// open fills the page in for a chat.
func (p *groupPage) open(cv chatView) {
	p.list.Axis = layout.Vertical
	for _, e := range []*widget.Editor{&p.topic, &p.name, &p.password, &p.limit} {
		e.SingleLine = true
	}
	p.invites = map[chatID]*widget.Clickable{}
	p.peers = map[tox.PublicKey]*widget.Clickable{}
	p.actions = map[peerAction]*widget.Clickable{}
	p.selected = nil
	p.deleteHistory.Value = false
	p.leave.asking = false
	p.err = ""
	switch {
	case cv.group != nil:
		g := cv.group
		p.topic.SetText(g.topic)
		p.name.SetText(g.selfName)
		p.password.SetText(g.password)
		p.limit.SetText(strconv.Itoa(int(g.peerLimit)))
		p.public.Value = g.privacy == tox.GroupPrivacyStatePublic
		p.topicLock.Value = g.topicLock == tox.GroupTopicLockEnabled
		p.voice.Value = voiceKey(g.voice)
	case cv.conference != nil:
		p.name.SetText(cv.conference.name)
	}
}

func voiceKey(v tox.GroupVoiceState) string {
	switch v {
	case tox.GroupVoiceStateModerator:
		return "moderators"
	case tox.GroupVoiceStateFounder:
		return "founder"
	}
	return "all"
}

var voices = map[string]tox.GroupVoiceState{
	"all":        tox.GroupVoiceStateAll,
	"moderators": tox.GroupVoiceStateModerator,
	"founder":    tox.GroupVoiceStateFounder,
}

// show shows the outcome of what the user did.
func (p *groupPage) show(err error) {
	if err != nil {
		p.err = err.Error()
	} else {
		p.err = ""
	}
}

func (p *groupPage) layout(gtx layout.Context, u *ui) layout.Dimensions {
	cv, ok := u.view.chatView(u.chat)
	if !ok {
		u.navigate(pageFriends)
		return layout.Dimensions{}
	}
	var rows []layout.Widget
	switch {
	case cv.group != nil:
		rows = p.groupRows(gtx, u, cv)
	case cv.conference != nil:
		rows = p.conferenceRows(gtx, u, cv)
	case cv.private != nil:
		rows = p.privateRows(gtx, u, cv)
	}
	rows = append(rows, gap(16), u.errorText(p.err))
	if !u.chatOpen() {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return u.header(gtx, true, cv.title, cv.statusText())
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return u.layoutForm(gtx, &p.list, rows)
		}),
	)
}

// chatOpen reports whether the page still has a chat: leaving goes back.
func (u *ui) chatOpen() bool { return u.page == pageInfo }

func (p *groupPage) groupRows(gtx layout.Context, u *ui, cv chatView) []layout.Widget {
	g := cv.group
	mod := g.selfRole == tox.GroupRoleFounder || g.selfRole == tox.GroupRoleModerator
	founder := g.selfRole == tox.GroupRoleFounder
	do := func(f func(g *group) error) { p.show(u.core.groupDo(cv.id, f)) }

	if p.setTopic.Clicked(gtx) {
		topic := strings.TrimSpace(p.topic.Text())
		do(func(g *group) error { return g.SetTopic(topic) })
	}
	if p.setName.Clicked(gtx) {
		name := strings.TrimSpace(p.name.Text())
		do(func(g *group) error { return g.SelfSetName(name) })
	}
	if p.public.Update(gtx) {
		state := tox.GroupPrivacyStatePrivate
		if p.public.Value {
			state = tox.GroupPrivacyStatePublic
		}
		do(func(g *group) error { return g.SetPrivacyState(state) })
	}
	if p.topicLock.Update(gtx) {
		lock := tox.GroupTopicLockDisabled
		if p.topicLock.Value {
			lock = tox.GroupTopicLockEnabled
		}
		do(func(g *group) error { return g.SetTopicLock(lock) })
	}
	if p.voice.Update(gtx) {
		state := voices[p.voice.Value]
		do(func(g *group) error { return g.SetVoiceState(state) })
	}
	if p.setPassword.Clicked(gtx) {
		password := p.password.Text()
		do(func(g *group) error { return g.SetPassword(password) })
	}
	if p.setLimit.Clicked(gtx) {
		if n, err := strconv.ParseUint(strings.TrimSpace(p.limit.Text()), 10, 16); err != nil || n == 0 {
			p.show(errors.New("the limit is a number from 1 to 65535"))
		} else {
			do(func(g *group) error { return g.SetPeerLimit(uint16(n)) })
		}
	}
	p.handleInvites(gtx, u, cv)
	p.handlePeers(gtx, u, cv)
	if p.leave.update(gtx) {
		if err := u.core.leaveGroup(cv.id, p.deleteHistory.Value); err != nil {
			p.show(err)
		} else {
			u.navigate(pageFriends)
		}
	}

	topicEditable := mod || g.topicLock == tox.GroupTopicLockDisabled
	rows := []layout.Widget{
		material.Body2(u.th, "Topic").Layout,
	}
	if topicEditable {
		rows = append(rows, u.field(&p.topic, "What the group is about"), gap(8),
			material.Button(u.th, &p.setTopic, "Set the topic").Layout)
	} else {
		topic := g.topic
		if topic == "" {
			topic = "none"
		}
		rows = append(rows, material.Body1(u.th, topic).Layout)
	}
	rows = append(rows,
		gap(24),
		material.Body2(u.th, "Your name in this group").Layout,
		u.field(&p.name, "Your name"),
		gap(8),
		material.Button(u.th, &p.setName, "Set your name").Layout,
		gap(24),
		material.Body2(u.th, "Chat ID: others join with it, if the group is public").Layout,
		func(gtx layout.Context) layout.Dimensions {
			l := material.Body1(u.th, g.chatID.String())
			l.WrapPolicy = text.WrapGraphemes
			l.State = &p.chatID
			return l.Layout(gtx)
		},
		gap(8),
		func(gtx layout.Context) layout.Dimensions { return u.copyButton(gtx, &p.copyID, g.chatID.String()) },
	)
	rows = append(rows, p.inviteRows(u, cv)...)
	rows = append(rows, gap(24), material.H6(u.th, "People ("+strconv.Itoa(len(g.peers))+")").Layout)
	for _, peer := range g.peers {
		rows = append(rows, p.peerRow(u, peer))
		if !peer.self && p.selected != nil && *p.selected == peer.key {
			rows = append(rows, p.peerActions(u, g, peer))
		}
	}
	if founder {
		rows = append(rows,
			gap(24),
			material.H6(u.th, "Settings").Layout,
			material.CheckBox(u.th, &p.public, "Public: anyone with the chat ID can join").Layout,
			material.CheckBox(u.th, &p.topicLock, "Only moderators set the topic").Layout,
			gap(8),
			material.Body2(u.th, "Who may write").Layout,
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(material.RadioButton(u.th, &p.voice, "all", "Everyone").Layout),
					layout.Rigid(material.RadioButton(u.th, &p.voice, "moderators", "Moderators").Layout),
					layout.Rigid(material.RadioButton(u.th, &p.voice, "founder", "You").Layout),
				)
			},
			gap(8),
			material.Body2(u.th, "Password (empty for none)").Layout,
			u.field(&p.password, ""),
			gap(8),
			material.Button(u.th, &p.setPassword, "Set the password").Layout,
			gap(8),
			material.Body2(u.th, "Most people at once").Layout,
			u.field(&p.limit, ""),
			gap(8),
			material.Button(u.th, &p.setLimit, "Set the limit").Layout,
		)
	}
	rows = append(rows,
		gap(32),
		material.CheckBox(u.th, &p.deleteHistory, "Also delete the chat").Layout,
		gap(8),
		func(gtx layout.Context) layout.Dimensions {
			return u.layoutConfirmation(gtx, &p.leave, "Leave the group", "Leave "+cv.title+"?", "Leave")
		},
	)
	return rows
}

// handleInvites invites the friends the user picked.
func (p *groupPage) handleInvites(gtx layout.Context, u *ui, cv chatView) {
	for _, f := range u.view.friends() {
		if state(p.invites, f.id).Clicked(gtx) {
			p.show(u.core.inviteFriend(cv.id, f.id))
		}
	}
}

// inviteRows are the friends online to invite.
func (p *groupPage) inviteRows(u *ui, cv chatView) []layout.Widget {
	rows := []layout.Widget{gap(24), material.H6(u.th, "Invite a friend").Layout}
	n := 0
	for _, f := range u.view.friends() {
		if !f.online() {
			continue
		}
		n++
		rows = append(rows, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.layoutAvatar(gtx, f.avatar(), f.title, f.id.String(), 32)
					}),
					layout.Rigid(layout.Spacer{Width: 12}.Layout),
					layout.Flexed(1, material.Body1(u.th, f.title).Layout),
					layout.Rigid(u.lightButton(state(p.invites, f.id), "Invite")),
				)
			})
		})
	}
	if n == 0 {
		rows = append(rows, material.Body2(u.th, "No friends are online.").Layout)
	}
	return rows
}

// handlePeers does what the user picked for a peer.
func (p *groupPage) handlePeers(gtx layout.Context, u *ui, cv chatView) {
	for _, peer := range cv.group.peers {
		if state(p.peers, peer.key).Clicked(gtx) {
			if p.selected != nil && *p.selected == peer.key {
				p.selected = nil
			} else {
				key := peer.key
				p.selected = &key
			}
		}
		for _, what := range []string{"private", "ignore", "unignore", "moderator", "user", "observer", "kick"} {
			if !state(p.actions, peerAction{peer.key, what}).Clicked(gtx) {
				continue
			}
			key := peer.key
			switch what {
			case "private":
				if id, err := u.core.openPrivate(cv.id, key); err != nil {
					p.show(err)
				} else {
					u.openChat(id)
				}
			case "ignore", "unignore":
				p.show(u.core.setIgnore(cv.id, key, what == "ignore"))
			case "kick":
				p.show(u.core.peerDo(cv.id, key, func(gp tox.GroupPeer) error { return gp.Kick() }))
			default:
				role := map[string]tox.GroupRole{
					"moderator": tox.GroupRoleModerator,
					"user":      tox.GroupRoleUser,
					"observer":  tox.GroupRoleObserver,
				}[what]
				p.show(u.core.peerDo(cv.id, key, func(gp tox.GroupPeer) error { return gp.SetRole(role) }))
			}
		}
	}
}

func (p *groupPage) peerRow(u *ui, peer peerView) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		row := func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 6, Bottom: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return u.layoutAvatar(gtx, nil, peer.name, peer.key.String(), 32)
					}),
					layout.Rigid(layout.Spacer{Width: 12}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						name := peer.name
						if name == "" {
							name = peer.key.String()[:12] + "…"
						}
						if peer.self {
							name += " (you)"
						}
						return material.Body1(u.th, name).Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						what := roleName(peer.role)
						if peer.ignored {
							what += ", ignored"
						}
						l := material.Caption(u.th, what)
						l.Color = secondaryText
						return l.Layout(gtx)
					}),
				)
			})
		}
		if peer.self {
			return row(gtx)
		}
		return state(p.peers, peer.key).Layout(gtx, row)
	}
}

// peerActions are the buttons for what the user may do to a peer.
func (p *groupPage) peerActions(u *ui, g *groupView, peer peerView) layout.Widget {
	labels := []struct{ what, label string }{{"private", "Message privately"}}
	if peer.ignored {
		labels = append(labels, struct{ what, label string }{"unignore", "Show their messages"})
	} else {
		labels = append(labels, struct{ what, label string }{"ignore", "Ignore"})
	}
	// Moderators manage users and observers; the founder manages everyone.
	if g.selfRole < peer.role && g.selfRole <= tox.GroupRoleModerator {
		if g.selfRole == tox.GroupRoleFounder && peer.role != tox.GroupRoleModerator {
			labels = append(labels, struct{ what, label string }{"moderator", "Make moderator"})
		}
		if peer.role != tox.GroupRoleUser {
			labels = append(labels, struct{ what, label string }{"user", "Make user"})
		}
		if peer.role != tox.GroupRoleObserver {
			labels = append(labels, struct{ what, label string }{"observer", "Make observer"})
		}
		labels = append(labels, struct{ what, label string }{"kick", "Kick"})
	}
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = 0
		return layout.Inset{Left: 44, Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			var children []layout.FlexChild
			for _, l := range labels {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						click := state(p.actions, peerAction{peer.key, l.what})
						if l.what == "kick" {
							b := material.Button(u.th, click, l.label)
							b.Background = busyColor
							return b.Layout(gtx)
						}
						return u.lightButton(click, l.label)(gtx)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	}
}

func (p *groupPage) conferenceRows(gtx layout.Context, u *ui, cv chatView) []layout.Widget {
	cf := cv.conference
	if p.setName.Clicked(gtx) {
		p.show(u.core.setConferenceTitle(cv.id, strings.TrimSpace(p.name.Text())))
	}
	p.handleInvites(gtx, u, cv)
	if p.leave.update(gtx) {
		if err := u.core.leaveConference(cv.id, p.deleteHistory.Value); err != nil {
			p.show(err)
		} else {
			u.navigate(pageFriends)
		}
	}
	rows := []layout.Widget{
		material.Body2(u.th, "Name").Layout,
		u.field(&p.name, "What the conference is called"),
		gap(8),
		material.Button(u.th, &p.setName, "Rename").Layout,
	}
	rows = append(rows, p.inviteRows(u, cv)...)
	rows = append(rows, gap(24), material.H6(u.th, "People ("+strconv.Itoa(len(cf.peers))+")").Layout)
	for _, peer := range cf.peers {
		rows = append(rows, p.peerRow(u, peer))
	}
	rows = append(rows,
		gap(32),
		material.CheckBox(u.th, &p.deleteHistory, "Also delete the chat").Layout,
		gap(8),
		func(gtx layout.Context) layout.Dimensions {
			return u.layoutConfirmation(gtx, &p.leave, "Leave the conference", "Leave "+cv.title+"?", "Leave")
		},
	)
	return rows
}

func (p *groupPage) privateRows(gtx layout.Context, u *ui, cv chatView) []layout.Widget {
	pv := cv.private
	if p.toGroup.Clicked(gtx) {
		u.openChat(pv.group)
	}
	return []layout.Widget{
		material.Body1(u.th, "A private chat with "+pv.name+", from the group "+pv.groupName+". Only they see it.").Layout,
		gap(16),
		material.Body2(u.th, "Their key in the group").Layout,
		func(gtx layout.Context) layout.Dimensions {
			l := material.Body1(u.th, pv.key.String())
			l.WrapPolicy = text.WrapGraphemes
			return l.Layout(gtx)
		},
		gap(16),
		material.Button(u.th, &p.toGroup, "Go to "+pv.groupName).Layout,
	}
}
