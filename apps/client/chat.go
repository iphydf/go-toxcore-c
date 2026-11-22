package main

import (
	"encoding/hex"
	"errors"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tox "github.com/TokTok/go-toxcore-c"
)

type chatKind uint8

const (
	chatFriend chatKind = iota + 1
	chatGroup
	chatConference
	// chatPrivate is a private chat with a peer of a group.
	chatPrivate
)

// A chatID says which chat: a friend's public key, a group's chat ID or a
// conference's ID; for a private chat, the group's chat ID and the peer's key.
type chatID struct {
	kind chatKind
	key  [32]byte
	peer tox.PublicKey
}

func friendChat(key tox.PublicKey) chatID { return chatID{kind: chatFriend, key: key} }

func groupChat(id tox.GroupChatID) chatID { return chatID{kind: chatGroup, key: id} }

func conferenceChat(id tox.ConferenceID) chatID { return chatID{kind: chatConference, key: id} }

func privateChat(group tox.GroupChatID, peer tox.PublicKey) chatID {
	return chatID{kind: chatPrivate, key: group, peer: peer}
}

// bytes is how the store keeps the ID.
func (id chatID) bytes() []byte {
	b := append([]byte{byte(id.kind)}, id.key[:]...)
	if id.kind == chatPrivate {
		b = append(b, id.peer[:]...)
	}
	return b
}

func (id chatID) String() string { return strings.ToUpper(hex.EncodeToString(id.bytes())) }

func parseChatID(s string) (chatID, error) {
	b, err := hex.DecodeString(s)
	var id chatID
	switch {
	case err != nil:
		return id, err
	case len(b) == 1+len(id.key) && chatKind(b[0]) >= chatFriend && chatKind(b[0]) < chatPrivate:
	case len(b) == 1+len(id.key)+len(id.peer) && chatKind(b[0]) == chatPrivate:
		copy(id.peer[:], b[1+len(id.key):])
	default:
		return id, errors.New("not a chat ID")
	}
	id.kind = chatKind(b[0])
	copy(id.key[:], b[1:])
	return id, nil
}

// A chat is what friends, groups, conferences and private chats with group
// peers have in common: their messages.
type chat struct {
	id     chatID
	unread int
	// last is the newest message, for the chat list.
	last *message

	// The chat is one of these.
	friend     *friend
	group      *group
	conference *conference
	private    *private
}

// pageSize is how many messages a chat shows when it opens, and how many more
// each time the user scrolls to the top.
const pageSize = 50

// newChat starts keeping a chat. The caller holds c.mu once the core runs.
func (c *core) newChat(id chatID) *chat {
	ch := &chat{id: id}
	if m, ok, err := c.store.last(id); err != nil {
		log.Print(err)
	} else if ok {
		ch.last = &m
	}
	var err error
	if ch.unread, err = c.store.unread(id); err != nil {
		log.Print(err)
	}
	c.chats[id] = ch
	return ch
}

// title is the chat's name, as the user knows it.
func (ch *chat) title() string {
	switch {
	case ch.friend != nil:
		return ch.friend.title()
	case ch.group != nil:
		return ch.group.title()
	case ch.conference != nil:
		return ch.conference.title()
	case ch.private != nil:
		return ch.private.title()
	}
	return ""
}

// add stores a message and shows it, and returns it with its id. The caller
// holds c.mu.
func (c *core) add(ch *chat, m message) (message, bool) {
	if err := c.store.add(ch.id, &m); err != nil {
		log.Printf("storing a message: %v", err)
		return m, false
	}
	ch.last = &m
	if c.isOpen(ch) {
		c.chat = append(c.chat, m)
	}
	switch {
	case m.outgoing || m.notice:
	case c.isOpen(ch) && c.focused:
		c.markRead(ch)
	default:
		ch.unread++
		if c.alerting(ch, m) {
			title := ch.title()
			if ch.unread > 1 {
				title += " (" + strconv.Itoa(ch.unread) + ")"
			}
			text := m.preview()
			if m.senderName != "" {
				text = m.senderName + ": " + text
			}
			c.alerts = append(c.alerts, alert{ch.id, title, text})
		}
	}
	return m, true
}

// alerting says whether a message tells the user: all of them in a chat with
// one person, and in groups those that mention the user. The caller holds
// c.mu.
func (c *core) alerting(ch *chat, m message) bool {
	var self string
	switch {
	case ch.group != nil:
		self, _ = ch.group.SelfName()
	case ch.conference != nil:
		self = c.tox.Name()
	default:
		return true
	}
	return self != "" && strings.Contains(strings.ToLower(m.text), strings.ToLower(self))
}

// preview is the message in short, for the chat list and notifications.
func (m message) preview() string {
	switch {
	case m.file != nil:
		return "File: " + m.file.name
	case m.invite != nil:
		return "Invitation: " + m.invite.name
	case m.action:
		return "* " + m.text
	}
	return m.text
}

// notice adds a notice to a chat: something that happened in it. The caller
// holds c.mu.
func (c *core) notice(ch *chat, text string) {
	c.add(ch, message{time: time.Now(), notice: true, text: text})
}

// update changes a message that is on screen. The caller holds c.mu.
func (c *core) update(ch *chat, id int64, f func(*message)) {
	if ch.last != nil && ch.last.id == id {
		m := *ch.last
		f(&m)
		ch.last = &m
	}
	if c.isOpen(ch) {
		if i := slices.IndexFunc(c.chat, func(m message) bool { return m.id == id }); i >= 0 {
			f(&c.chat[i])
		}
	}
}

// delivered records that a message went. The caller holds c.mu.
func (c *core) delivered(ch *chat, id int64) {
	if err := c.store.setDelivered(id); err != nil {
		log.Print(err)
	}
	c.update(ch, id, func(m *message) { m.delivered = true })
}

func (c *core) isOpen(ch *chat) bool { return c.open != nil && *c.open == ch.id }

// markRead records that the user saw the chat's messages. The caller holds
// c.mu.
func (c *core) markRead(ch *chat) {
	if ch.unread != 0 {
		c.alerts = append(c.alerts, alert{key: ch.id})
	}
	ch.unread = 0
	if ch.last != nil {
		if err := c.store.markRead(ch.id, ch.last.id); err != nil {
			log.Print(err)
		}
	}
}

// send sends a message, or keeps it until it can go. "/me " makes it an
// action. Tox sends long messages in parts.
func (c *core) send(id chatID, text string) {
	text = strings.TrimSpace(text)
	action := false
	if rest, ok := strings.CutPrefix(text, "/me "); ok {
		action, text = true, strings.TrimSpace(rest)
	}
	if text == "" {
		return
	}
	c.change(func() {
		ch := c.chats[id]
		if ch == nil {
			return
		}
		limit := tox.MaxMessageLength
		if ch.group != nil || ch.private != nil {
			limit = tox.GroupMaxMessageLength
		}
		for _, part := range splitMessage(text, limit) {
			c.add(ch, message{time: time.Now(), outgoing: true, action: action, text: part})
		}
		c.sendPending(ch)
	})
}

// sendPending sends the chat's messages that didn't go yet, if it can. The
// caller holds c.mu.
func (c *core) sendPending(ch *chat) {
	pending, err := c.store.undelivered(ch.id)
	if err != nil {
		log.Print(err)
		return
	}
	for _, m := range pending {
		kind := tox.MessageTypeNormal
		if m.action {
			kind = tox.MessageTypeAction
		}
		switch {
		case ch.friend != nil:
			if !c.sendFriend(ch.friend, m, kind) {
				return
			}
		case ch.group != nil:
			if _, err := ch.group.SendMessage(kind, m.text); err != nil {
				return
			}
			c.delivered(ch, m.id)
		case ch.conference != nil:
			if !ch.conference.connected {
				return
			}
			if err := ch.conference.SendMessage(kind, m.text); err != nil {
				return
			}
			c.delivered(ch, m.id)
		case ch.private != nil:
			p := ch.private.peer()
			if p == nil {
				return
			}
			if _, err := p.SendPrivateMessage(kind, m.text); err != nil {
				return
			}
			c.delivered(ch, m.id)
		}
	}
}

// splitMessage cuts text into parts of at most n bytes, at a space or line
// break if there is one in the second half of a part.
func splitMessage(text string, n int) []string {
	var parts []string
	for len(text) > n {
		cut := n
		for !utf8.RuneStart(text[cut]) {
			cut--
		}
		next := cut
		if i := strings.LastIndexAny(text[:cut+1], " \n"); i > n/2 {
			cut, next = i, i+1
		}
		parts = append(parts, text[:cut])
		text = text[next:]
	}
	return append(parts, text)
}

// setOpen marks the chat as on screen, or none with nil.
func (c *core) setOpen(id *chatID) {
	c.change(func() {
		c.open, c.chat, c.older = id, nil, false
		ch := c.openChat()
		if ch == nil {
			return
		}
		var err error
		if c.chat, err = c.store.messages(ch.id, math.MaxInt64, pageSize); err != nil {
			log.Print(err)
		}
		c.older = len(c.chat) == pageSize
		c.markRead(ch)
	})
}

// setFocused records whether the window is focused: whether the user sees the
// open chat.
func (c *core) setFocused(focused bool) {
	c.change(func() {
		c.focused = focused
		if ch := c.openChat(); ch != nil && focused {
			c.markRead(ch)
		}
	})
}

// openChat is the chat on screen, or nil. The caller holds c.mu.
func (c *core) openChat() *chat {
	if c.open == nil {
		return nil
	}
	return c.chats[*c.open]
}

// loadOlder adds a page of older messages to the open chat, and says how many.
func (c *core) loadOlder() int {
	n := 0
	c.change(func() {
		ch := c.openChat()
		if ch == nil || !c.older || len(c.chat) == 0 {
			return
		}
		older, err := c.store.messages(ch.id, c.chat[0].id, pageSize)
		if err != nil {
			log.Print(err)
			return
		}
		c.chat = append(older, c.chat...)
		c.older = len(older) == pageSize
		n = len(older)
	})
	return n
}

// closeChat stops keeping a chat, and with history forgets its messages. The
// caller holds c.mu.
func (c *core) closeChat(ch *chat, history bool) error {
	delete(c.chats, ch.id)
	c.alerts = append(c.alerts, alert{key: ch.id})
	if c.isOpen(ch) {
		c.open, c.chat, c.older = nil, nil, false
	}
	if !history {
		return nil
	}
	files, err := c.store.fileMessages(ch.id)
	if err != nil {
		log.Print(err)
	}
	if err := c.store.forget(ch.id); err != nil {
		return err
	}
	for _, msg := range files {
		c.removeFile(msg)
	}
	return nil
}
