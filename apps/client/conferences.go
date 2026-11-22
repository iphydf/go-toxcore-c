package main

import (
	"errors"
	"log"
	"strings"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
)

// A conference is an old-style group chat. The app does text in them, not
// calls.
type conference struct {
	tox.Conference
	chat      *chat
	id        tox.ConferenceID
	name      string
	connected bool
	peers     []peerView
}

func (cf *conference) title() string {
	if cf.name != "" {
		return cf.name
	}
	return "Conference " + strings.ToUpper(cf.id.String()[:8])
}

// addConference starts keeping a conference. The caller holds c.mu once the
// core runs.
func (c *core) addConference(conf tox.Conference) *conference {
	cf := &conference{Conference: conf}
	cf.id, _ = conf.ID()
	cf.name, _ = conf.Title()
	cf.chat = c.newChat(conferenceChat(cf.id))
	cf.chat.conference = cf
	cf.refreshPeers()
	c.conferences[conf.Number()] = cf
	return cf
}

// refreshPeers reads the conference's peers from toxcore.
func (cf *conference) refreshPeers() {
	cf.peers = cf.peers[:0]
	n, err := cf.PeerCount()
	if err != nil {
		return
	}
	for i := range n {
		p := cf.Peer(tox.ConferencePeerNumber(i))
		v := peerView{role: tox.GroupRoleUser}
		v.name, _ = p.Name()
		v.key, _ = p.PublicKey()
		v.self, _ = p.IsOurs()
		cf.peers = append(cf.peers, v)
	}
	sortPeers(cf.peers)
}

// handleConference applies an event about conferences, and says whether it
// was one. The caller holds c.mu.
func (c *core) handleConference(ev tox.Event) bool {
	switch ev := ev.(type) {
	case *tox.EventConferenceConnected:
		if cf := c.conferences[ev.Conference.Number()]; cf != nil {
			cf.connected = true
			cf.refreshPeers()
			c.sendPending(cf.chat)
		}
	case *tox.EventConferencePeerListChanged:
		if cf := c.conferences[ev.Conference.Number()]; cf != nil {
			cf.refreshPeers()
		}
	case *tox.EventConferencePeerName:
		if cf := c.conferences[ev.Peer.Conference().Number()]; cf != nil {
			cf.refreshPeers()
		}
	case *tox.EventConferenceTitle:
		if cf := c.conferences[ev.Peer.Conference().Number()]; cf != nil {
			c.unsaved = true
			cf.name = ev.Title
			who := "someone"
			if name, err := ev.Peer.Name(); err == nil && name != "" {
				who = name
			}
			c.notice(cf.chat, who+" named it "+ev.Title)
		}
	case *tox.EventConferenceMessage:
		cf := c.conferences[ev.Peer.Conference().Number()]
		if cf == nil {
			break
		}
		// toxcore gives us our own messages back.
		if ours, err := ev.Peer.IsOurs(); err != nil || ours {
			break
		}
		m := message{time: time.Now(), action: ev.Type == tox.MessageTypeAction, text: ev.Message}
		if key, err := ev.Peer.PublicKey(); err == nil {
			m.sender = &key
		}
		m.senderName, _ = ev.Peer.Name()
		c.add(cf.chat, m)
	case *tox.EventConferenceInvite:
		if fr := c.friends[ev.Friend.Number()]; fr != nil {
			kind := inviteConference
			if ev.Type == tox.ConferenceTypeAV {
				kind = inviteAVConference
			}
			c.add(fr.chat, message{
				time:   time.Now(),
				invite: &invite{kind: kind, data: ev.Cookie, name: "a conference"},
			})
		}
	default:
		return false
	}
	return true
}

// createConference makes a conference and returns its chat.
func (c *core) createConference(name string) (chatID, error) {
	var id chatID
	conf, err := c.tox.ConferenceNew()
	if err != nil {
		return id, err
	}
	if name = strings.TrimSpace(name); name != "" {
		if err := conf.SetTitle(name); err != nil {
			log.Print(err)
		}
	}
	c.change(func() {
		cf := c.addConference(conf)
		cf.connected = true
		id = cf.chat.id
		c.saveProfile()
	})
	return id, nil
}

// setConferenceTitle renames a conference.
func (c *core) setConferenceTitle(id chatID, name string) error {
	var err error
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.conference == nil {
			err = errors.New("that conference is gone")
			return
		}
		if err = ch.conference.SetTitle(name); err == nil {
			ch.conference.name = name
			c.saveProfile()
		}
	})
	return err
}

// leaveConference leaves a conference, and with history forgets its chat.
func (c *core) leaveConference(id chatID, history bool) error {
	var err error
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.conference == nil {
			return
		}
		if err = ch.conference.Delete(); err != nil {
			return
		}
		delete(c.conferences, ch.conference.Number())
		c.saveProfile()
		err = c.closeChat(ch, history)
	})
	return err
}
