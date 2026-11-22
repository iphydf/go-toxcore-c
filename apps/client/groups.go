package main

import (
	"errors"
	"log"
	"slices"
	"strings"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
)

// A group is a group chat (NGC).
type group struct {
	tox.Group
	chat      *chat
	chatID    tox.GroupChatID
	name      string
	topic     string
	privacy   tox.GroupPrivacyState
	voice     tox.GroupVoiceState
	topicLock tox.GroupTopicLock
	peerLimit uint32
	password  string
	// peers are the peers toxcore told us about: those in the group now.
	peers map[tox.GroupPeerNumber]*groupPeer
	// privates are the private chats with the group's peers, by key.
	privates map[tox.PublicKey]*private
}

type groupPeer struct {
	key     tox.PublicKey
	name    string
	status  tox.UserStatus
	role    tox.GroupRole
	ignored bool
}

// A private is a private chat with a peer of a group.
type private struct {
	chat  *chat
	group *group
	key   tox.PublicKey
	// name is the peer's name, the last we knew it.
	name string
}

func (g *group) title() string {
	if g.name != "" {
		return g.name
	}
	return "Group " + g.chatID.String()[:8]
}

func (p *private) title() string {
	if p.name != "" {
		return p.name
	}
	return p.key.String()[:12] + "…"
}

// peer returns the private chat's peer while they are in the group.
func (p *private) peer() *tox.GroupPeer {
	for n, gp := range p.group.peers {
		if gp.key == p.key {
			peer := p.group.Peer(n)
			return &peer
		}
	}
	return nil
}

// addGroup starts keeping a group. The caller holds c.mu once the core runs.
func (c *core) addGroup(gr tox.Group) *group {
	g := &group{Group: gr, peers: map[tox.GroupPeerNumber]*groupPeer{}, privates: map[tox.PublicKey]*private{}}
	g.chatID, _ = gr.ChatID()
	g.refresh()
	g.chat = c.newChat(groupChat(g.chatID))
	g.chat.group = g
	privates, err := c.store.privateChats(g.chatID)
	if err != nil {
		log.Print(err)
	}
	for key, name := range privates {
		c.addPrivate(g, key, name)
	}
	c.groups[gr.Number()] = g
	return g
}

// refresh reads the group's state from toxcore.
func (g *group) refresh() {
	g.name, _ = g.Name()
	g.topic, _ = g.Topic()
	g.privacy, _ = g.PrivacyState()
	g.voice, _ = g.VoiceState()
	g.topicLock, _ = g.TopicLock()
	limit, _ := g.PeerLimit()
	g.peerLimit = uint32(limit)
	g.password, _ = g.Password()
}

// addPrivate starts keeping a private chat with a group's peer. The caller
// holds c.mu once the core runs.
func (c *core) addPrivate(g *group, key tox.PublicKey, name string) *private {
	p := &private{group: g, key: key, name: name}
	p.chat = c.newChat(privateChat(g.chatID, key))
	p.chat.private = p
	g.privates[key] = p
	return p
}

// newPrivate starts a private chat with a group's peer, and stores it. The
// caller holds c.mu.
func (c *core) newPrivate(g *group, key tox.PublicKey, name string) *private {
	p := c.addPrivate(g, key, name)
	if err := c.store.setPeerName(p.chat.id, name); err != nil {
		log.Print(err)
	}
	return p
}

// groupPeer returns what we know about a peer, and their handle.
func (c *core) groupPeer(peer tox.GroupPeer) (*group, *groupPeer) {
	g := c.groups[peer.Group().Number()]
	if g == nil {
		return nil, nil
	}
	return g, g.peers[peer.Number()]
}

// peerName is a peer's name, for notices.
func (g *group) peerName(peer tox.GroupPeer) string {
	if p := g.peers[peer.Number()]; p != nil && p.name != "" {
		return p.name
	}
	if name, err := peer.Name(); err == nil && name != "" {
		return name
	}
	return "someone"
}

func roleName(r tox.GroupRole) string {
	switch r {
	case tox.GroupRoleFounder:
		return "founder"
	case tox.GroupRoleModerator:
		return "moderator"
	case tox.GroupRoleObserver:
		return "observer"
	}
	return "user"
}

// handleGroup applies an event about groups, and says whether it was one. The
// caller holds c.mu.
func (c *core) handleGroup(ev tox.Event) bool {
	switch ev := ev.(type) {
	case *tox.EventGroupPeerJoin:
		if g := c.groups[ev.Peer.Group().Number()]; g != nil {
			p := &groupPeer{}
			p.key, _ = ev.Peer.PublicKey()
			p.name, _ = ev.Peer.Name()
			p.status, _ = ev.Peer.Status()
			p.role, _ = ev.Peer.Role()
			g.peers[ev.Peer.Number()] = p
			if pc := g.privates[p.key]; pc != nil {
				c.setPeerName(pc, p.name)
				c.sendPending(pc.chat)
			}
		}
	case *tox.EventGroupPeerExit:
		if g := c.groups[ev.Peer.Group().Number()]; g != nil {
			delete(g.peers, ev.Peer.Number())
		}
	case *tox.EventGroupPeerName:
		if g, p := c.groupPeer(ev.Peer); p != nil {
			p.name = ev.Name
			if pc := g.privates[p.key]; pc != nil {
				c.setPeerName(pc, p.name)
			}
		}
	case *tox.EventGroupPeerStatus:
		if _, p := c.groupPeer(ev.Peer); p != nil {
			p.status = ev.Status
		}
	case *tox.EventGroupTopic:
		if g := c.groups[ev.Peer.Group().Number()]; g != nil {
			c.unsaved = true
			g.topic = ev.Topic
			c.notice(g.chat, g.peerName(ev.Peer)+" set the topic: "+ev.Topic)
		}
	case *tox.EventGroupPrivacyState, *tox.EventGroupVoiceState, *tox.EventGroupTopicLock,
		*tox.EventGroupPeerLimit, *tox.EventGroupPassword, *tox.EventGroupSelfJoin:
		g := c.groups[groupOf(ev).Number()]
		if g == nil {
			break
		}
		c.unsaved = true
		g.refresh()
		if _, ok := ev.(*tox.EventGroupSelfJoin); ok {
			c.sendPending(g.chat)
		}
	case *tox.EventGroupJoinFail:
		if g := c.groups[ev.Group.Number()]; g != nil {
			reason := "the group is full"
			switch ev.FailType {
			case tox.GroupJoinFailInvalidPassword:
				reason = "the password is wrong"
			case tox.GroupJoinFailUnknown:
				reason = "something went wrong"
			}
			c.notice(g.chat, "Couldn't join: "+reason)
		}
	case *tox.EventGroupModeration:
		if g := c.groups[ev.Group.Number()]; g != nil {
			source, target := g.peerName(ev.Source), g.peerName(ev.Target)
			if p := g.peers[ev.Target.Number()]; p != nil {
				p.role, _ = ev.Target.Role()
			}
			text := source + " kicked " + target
			switch ev.ModType {
			case tox.GroupModEventObserver:
				text = source + " made " + target + " an observer"
			case tox.GroupModEventUser:
				text = source + " made " + target + " a user"
			case tox.GroupModEventModerator:
				text = source + " made " + target + " a moderator"
			}
			c.notice(g.chat, text)
		}
	case *tox.EventGroupMessage:
		if g, p := c.groupPeer(ev.Peer); p != nil {
			c.add(g.chat, message{
				time:       time.Now(),
				action:     ev.MessageType == tox.MessageTypeAction,
				text:       ev.Message,
				sender:     &p.key,
				senderName: p.name,
			})
		}
	case *tox.EventGroupPrivateMessage:
		if g, p := c.groupPeer(ev.Peer); p != nil {
			pc := g.privates[p.key]
			if pc == nil {
				pc = c.newPrivate(g, p.key, p.name)
			}
			c.add(pc.chat, message{
				time:   time.Now(),
				action: ev.MessageType == tox.MessageTypeAction,
				text:   ev.Message,
			})
		}
	case *tox.EventGroupInvite:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			c.add(fr.chat, message{
				time:   time.Now(),
				invite: &invite{kind: inviteGroup, data: ev.InviteData, name: ev.GroupName},
			})
		}
	case *tox.EventGroupCustomPacket, *tox.EventGroupCustomPrivatePacket:
	default:
		return false
	}
	return true
}

// groupOf is the group of an event about a group's state.
func groupOf(ev tox.Event) tox.Group {
	switch ev := ev.(type) {
	case *tox.EventGroupPrivacyState:
		return ev.Group
	case *tox.EventGroupVoiceState:
		return ev.Group
	case *tox.EventGroupTopicLock:
		return ev.Group
	case *tox.EventGroupPeerLimit:
		return ev.Group
	case *tox.EventGroupPassword:
		return ev.Group
	case *tox.EventGroupSelfJoin:
		return ev.Group
	}
	panic("not an event about a group's state")
}

// setPeerName records a private chat's peer's name. The caller holds c.mu.
func (c *core) setPeerName(p *private, name string) {
	if name == "" || name == p.name {
		return
	}
	p.name = name
	if err := c.store.setPeerName(p.chat.id, name); err != nil {
		log.Print(err)
	}
}

// createGroup makes a group, private if it isn't public, and returns its chat.
func (c *core) createGroup(name string, public bool) (chatID, error) {
	var id chatID
	name = strings.TrimSpace(name)
	if name == "" {
		return id, errors.New("give the group a name")
	}
	privacy := tox.GroupPrivacyStatePrivate
	if public {
		privacy = tox.GroupPrivacyStatePublic
	}
	gr, err := c.tox.GroupNew(privacy, name, c.tox.Name())
	if err != nil {
		return id, err
	}
	c.change(func() {
		id = c.addGroup(gr).chat.id
		c.saveProfile()
	})
	return id, nil
}

// joinGroup joins a group by its chat ID in hex.
func (c *core) joinGroup(hexID, password string) (chatID, error) {
	var id chatID
	gid, err := tox.ParseGroupChatID(strings.TrimSpace(hexID))
	if err != nil {
		return id, errors.New("that's not a group's chat ID")
	}
	gr, err := c.tox.GroupJoin(gid, c.tox.Name(), password)
	if err != nil {
		return id, err
	}
	c.change(func() {
		id = c.addGroup(gr).chat.id
		c.saveProfile()
	})
	return id, nil
}

// joinInvite accepts a friend's invitation, and returns the chat it joined.
func (c *core) joinInvite(friendChat chatID, msg int64) (chatID, error) {
	var id chatID
	var err error
	c.change(func() {
		ch := c.chats[friendChat]
		if ch == nil || ch.friend == nil {
			err = errors.New("they're not a friend any more")
			return
		}
		m, ok, serr := c.store.message(friendChat, msg)
		if serr != nil || !ok || m.invite == nil {
			err = errors.New("that invitation is gone")
			return
		}
		switch m.invite.kind {
		case inviteGroup:
			var gr tox.Group
			if gr, err = c.tox.GroupInviteAccept(ch.friend.Friend, m.invite.data, c.tox.Name(), ""); err != nil {
				return
			}
			id = c.addGroup(gr).chat.id
		case inviteConference:
			var cf tox.Conference
			if cf, err = c.tox.ConferenceJoin(ch.friend.Friend, m.invite.data); err != nil {
				return
			}
			id = c.addConference(cf).chat.id
		default:
			err = errors.New("the app can't join conferences with calls")
			return
		}
		if serr := c.store.setJoined(msg); serr != nil {
			log.Print(serr)
		}
		c.update(ch, msg, func(m *message) {
			i := *m.invite
			i.joined = true
			m.invite = &i
		})
		c.saveProfile()
	})
	return id, err
}

// inviteFriend invites a friend to a group or a conference.
func (c *core) inviteFriend(to, friend chatID) error {
	var err error
	c.change(func() {
		ch, fc := c.chats[to], c.chats[friend]
		if ch == nil || fc == nil || fc.friend == nil {
			err = errors.New("that chat is gone")
			return
		}
		switch {
		case ch.group != nil:
			err = ch.group.InviteFriend(fc.friend.Friend)
		case ch.conference != nil:
			err = ch.conference.Invite(fc.friend.Friend)
		default:
			err = errors.New("that's not a group")
		}
		if err == nil {
			c.notice(fc, "You invited them to "+ch.title())
		}
	})
	return err
}

// groupDo does something with a group, under the lock, and saves the profile
// after.
func (c *core) groupDo(id chatID, f func(g *group) error) error {
	var err error
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.group == nil {
			err = errors.New("that group is gone")
			return
		}
		if err = f(ch.group); err == nil {
			ch.group.refresh()
			c.saveProfile()
		}
	})
	return err
}

// peerDo does something with a group's peer, if they're in the group.
func (c *core) peerDo(id chatID, key tox.PublicKey, f func(p tox.GroupPeer) error) error {
	return c.groupDo(id, func(g *group) error {
		for n, p := range g.peers {
			if p.key == key {
				err := f(g.Peer(n))
				if err == nil {
					p.role, _ = g.Peer(n).Role()
				}
				return err
			}
		}
		return errors.New("they're not in the group now")
	})
}

// setIgnore stops showing a peer's messages, or shows them again.
func (c *core) setIgnore(id chatID, key tox.PublicKey, ignore bool) error {
	return c.peerDo(id, key, func(p tox.GroupPeer) error {
		if err := p.SetIgnore(ignore); err != nil {
			return err
		}
		if gp := c.chats[id].group.peers[p.Number()]; gp != nil {
			gp.ignored = ignore
		}
		return nil
	})
}

// openPrivate returns the private chat with a group's peer, making it the
// first time.
func (c *core) openPrivate(id chatID, key tox.PublicKey) (chatID, error) {
	var pid chatID
	err := c.groupDo(id, func(g *group) error {
		p := g.privates[key]
		if p == nil {
			name := ""
			for _, gp := range g.peers {
				if gp.key == key {
					name = gp.name
				}
			}
			p = c.newPrivate(g, key, name)
		}
		pid = p.chat.id
		return nil
	})
	return pid, err
}

// leaveGroup leaves a group, and with history forgets its chats.
func (c *core) leaveGroup(id chatID, history bool) error {
	var err error
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.group == nil {
			return
		}
		g := ch.group
		if err = g.Leave(""); err != nil {
			return
		}
		delete(c.groups, g.Number())
		for _, p := range g.privates {
			if cerr := c.closeChat(p.chat, history); cerr != nil {
				log.Print(cerr)
			}
		}
		c.saveProfile()
		err = c.closeChat(ch, history)
	})
	return err
}

// A peerView is a group's or a conference's peer, as the UI shows them.
type peerView struct {
	key     tox.PublicKey
	name    string
	status  tox.UserStatus
	role    tox.GroupRole
	ignored bool
	self    bool
}

type groupView struct {
	chatID    tox.GroupChatID
	name      string
	topic     string
	connected bool
	privacy   tox.GroupPrivacyState
	voice     tox.GroupVoiceState
	topicLock tox.GroupTopicLock
	peerLimit uint32
	password  string
	selfName  string
	selfRole  tox.GroupRole
	peers     []peerView
}

// view is the group as the UI shows it. The caller holds c.mu.
func (g *group) view() *groupView {
	v := &groupView{
		chatID:    g.chatID,
		name:      g.name,
		topic:     g.topic,
		privacy:   g.privacy,
		voice:     g.voice,
		topicLock: g.topicLock,
		peerLimit: g.peerLimit,
		password:  g.password,
	}
	v.connected, _ = g.IsConnected()
	v.selfName, _ = g.SelfName()
	v.selfRole, _ = g.SelfRole()
	self := peerView{name: v.selfName, role: v.selfRole, self: true}
	self.key, _ = g.SelfPublicKey()
	self.status, _ = g.SelfStatus()
	v.peers = append(v.peers, self)
	for _, p := range g.peers {
		v.peers = append(v.peers, peerView{key: p.key, name: p.name, status: p.status, role: p.role, ignored: p.ignored})
	}
	sortPeers(v.peers)
	return v
}

// sortPeers puts peers in order: by role, then by name.
func sortPeers(peers []peerView) {
	slices.SortFunc(peers, func(a, b peerView) int {
		if a.role != b.role {
			return int(a.role) - int(b.role)
		}
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})
}
