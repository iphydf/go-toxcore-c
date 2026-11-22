package main

import (
	"errors"
	"log"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
)

type friend struct {
	tox.Friend
	chat          *chat
	key           tox.PublicKey
	name          string
	nickname      string
	statusMessage string
	status        tox.UserStatus
	connection    tox.Connection
	typing        bool
	// sending has the messages toxcore took in this session and that wait for
	// a read receipt, with toxcore's IDs for them.
	sending map[int64]tox.MessageID
	// transfers are the files going to or from the friend in this session.
	transfers map[tox.FileNumber]*transfer
	avatar    *avatar
}

type request struct {
	key     tox.PublicKey
	message string
}

// title is the friend's nickname, or their name, or the start of their key
// before we know it.
func (fr *friend) title() string {
	return title(fr.nickname, fr.name, fr.key)
}

func title(nickname, name string, key tox.PublicKey) string {
	switch {
	case nickname != "":
		return nickname
	case name != "":
		return name
	}
	return key.String()[:12] + "…"
}

func (fr *friend) online() bool { return fr.connection != tox.ConnectionNone }

// addFriend starts keeping a friend. The caller holds c.mu once the core runs.
func (c *core) addFriend(f tox.Friend) *friend {
	fr := &friend{Friend: f, sending: map[int64]tox.MessageID{}, transfers: map[tox.FileNumber]*transfer{}}
	fr.key, _ = f.PublicKey()
	fr.name, _ = f.Name()
	fr.statusMessage, _ = f.StatusMessage()
	fr.chat = c.newChat(friendChat(fr.key))
	fr.chat.friend = fr
	var err error
	if fr.nickname, err = c.store.nickname(fr.chat.id); err != nil {
		log.Print(err)
	}
	fr.avatar = loadAvatar(c.friendAvatarPath(fr.key))
	c.friends[f.Number()] = fr
	return fr
}

// handleFriend applies an event about friends, and says whether it was one.
// The caller holds c.mu.
func (c *core) handleFriend(ev tox.Event) bool {
	switch ev := ev.(type) {
	case *tox.EventFriendRequest:
		if !slices.ContainsFunc(c.requests, func(r request) bool { return r.key == ev.PublicKey }) {
			r := request{ev.PublicKey, ev.Message}
			c.requests = append(c.requests, r)
			if err := c.store.addRequest(r); err != nil {
				log.Print(err)
			}
			c.alerts = append(c.alerts, alert{friendChat(ev.PublicKey), "Friend request", ev.Message})
		}
	case *tox.EventFriendConnectionStatus:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			wasOnline := fr.online()
			fr.connection = ev.ConnectionStatus
			if !fr.online() {
				// toxcore forgets the messages it sent without a receipt, and
				// the files: they go again when the friend is back.
				clear(fr.sending)
				c.dropTransfers(fr)
				c.friendGone(fr)
			} else if !wasOnline {
				c.sendPending(fr.chat)
				c.startFiles(fr)
				c.sendAvatar(fr, false)
			}
		}
	case *tox.EventFriendName:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			c.unsaved = true
			fr.name = ev.Name
		}
	case *tox.EventFriendStatus:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			c.unsaved = true
			fr.status = ev.Status
		}
	case *tox.EventFriendStatusMessage:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			c.unsaved = true
			fr.statusMessage = ev.Message
		}
	case *tox.EventFriendTyping:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			fr.typing = ev.Typing
		}
	case *tox.EventFriendMessage:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			fr.typing = false
			c.add(fr.chat, message{
				time:   time.Now(),
				action: ev.Type == tox.MessageTypeAction,
				text:   ev.Message,
			})
		}
	case *tox.EventFriendReadReceipt:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			for id, sent := range fr.sending {
				if sent == ev.MessageID {
					delete(fr.sending, id)
					c.delivered(fr.chat, id)
					break
				}
			}
		}
	case *tox.EventFileRecv:
		if fr := c.friends[ev.File.Friend().Number()]; fr != nil {
			c.fileOffered(fr, ev)
		}
	case *tox.EventFileRecvChunk:
		if fr := c.friends[ev.File.Friend().Number()]; fr != nil {
			c.fileChunk(fr, ev)
		}
	case *tox.EventFileChunkRequest:
		if fr := c.friends[ev.File.Friend().Number()]; fr != nil {
			c.fileChunkRequest(fr, ev)
		}
	case *tox.EventFileRecvControl:
		if fr := c.friends[ev.File.Friend().Number()]; fr != nil {
			c.fileControl(fr, ev)
		}
	default:
		return false
	}
	return true
}

// sendFriend hands toxcore a message for a friend, unless it has it, and says
// whether more can go. Messages wait for a read receipt: toxcore forgets them
// when the friend goes offline or the app stops. The caller holds c.mu.
func (c *core) sendFriend(fr *friend, m message, kind tox.MessageType) bool {
	if !fr.online() {
		return false
	}
	if _, ok := fr.sending[m.id]; ok {
		return true
	}
	id, err := fr.SendMessage(kind, m.text)
	if err != nil {
		// Not connected after all, or toxcore's queue is full: the next
		// connection tries again.
		return false
	}
	fr.sending[m.id] = id
	return true
}

func (c *core) setTyping(id chatID, typing bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch := c.chats[id]; ch != nil && ch.friend != nil && ch.friend.online() {
		if err := ch.friend.SetTyping(typing); err != nil {
			log.Print(err)
		}
	}
}

// friendAdd sends a friend request to an address in hex.
func (c *core) friendAdd(address, message string) error {
	address = strings.TrimSpace(address)
	// The form QR codes and links have.
	address, _ = strings.CutPrefix(address, "tox:")
	a, err := tox.ParseAddress(address)
	if err != nil {
		return errors.New("that's not a Tox ID")
	}
	if strings.TrimSpace(message) == "" {
		message = "Hi, I'd like to add you as a friend."
	}
	f, err := c.tox.FriendAdd(a, message)
	if err != nil {
		return friendAddMessage(err)
	}
	c.change(func() {
		c.addFriend(f)
		c.saveProfile()
	})
	return nil
}

func friendAddMessage(err error) error {
	switch err {
	case tox.ErrFriendAddOwnKey:
		return errors.New("that's your own Tox ID")
	case tox.ErrFriendAddAlreadySent:
		return errors.New("you already sent them a request, or they're your friend")
	case tox.ErrFriendAddBadChecksum:
		return errors.New("that Tox ID has a typo")
	case tox.ErrFriendAddSetNewNospam:
		return errors.New("they changed their Tox ID: they're already your friend")
	case tox.ErrFriendAddTooLong:
		return errors.New("the message is too long")
	}
	return err
}

// accept makes the request's sender a friend.
func (c *core) accept(key tox.PublicKey) error {
	f, err := c.tox.FriendAddNoRequest(key)
	if err != nil {
		return err
	}
	c.change(func() {
		c.addFriend(f)
		c.removeRequest(key)
		c.saveProfile()
	})
	return nil
}

func (c *core) reject(key tox.PublicKey) {
	c.change(func() { c.removeRequest(key) })
}

func (c *core) removeRequest(key tox.PublicKey) {
	c.requests = slices.DeleteFunc(c.requests, func(r request) bool { return r.key == key })
	if err := c.store.removeRequest(key); err != nil {
		log.Print(err)
	}
	c.alerts = append(c.alerts, alert{key: friendChat(key)})
}

// setNickname gives a friend a name only the user sees, or takes it away with
// "".
func (c *core) setNickname(id chatID, name string) {
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.friend == nil {
			return
		}
		if err := c.store.setNickname(id, name); err != nil {
			log.Print(err)
			return
		}
		ch.friend.nickname = name
	})
}

// deleteFriend removes a friend, and with history their chat.
func (c *core) deleteFriend(id chatID, history bool) error {
	var err error
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.friend == nil {
			return
		}
		fr := ch.friend
		if err = fr.Delete(); err != nil {
			return
		}
		c.dropTransfers(fr)
		delete(c.friends, fr.Number())
		if err := os.Remove(c.friendAvatarPath(fr.key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Print(err)
		}
		c.saveProfile()
		err = c.closeChat(ch, history)
	})
	return err
}

func (c *core) setStatus(status tox.UserStatus) {
	c.change(func() {
		c.tox.SetStatus(status)
		c.saveProfile()
	})
}

// newNospam changes the Tox ID: friend requests to the old one stop working.
// Friends stay.
func (c *core) newNospam() {
	c.change(func() {
		c.tox.SetNospam(rand.Uint32())
		c.saveProfile()
	})
}

func (c *core) setName(name string) error {
	if err := c.tox.SetName(name); err != nil {
		return err
	}
	c.change(c.saveProfile)
	return nil
}

func (c *core) setStatusMessage(message string) error {
	if err := c.tox.SetStatusMessage(message); err != nil {
		return err
	}
	c.change(c.saveProfile)
	return nil
}
