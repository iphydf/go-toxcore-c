package tox_test

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/internal/toxtest"
)

func TestFriendsTalk(t *testing.T) {
	t.Parallel()
	a, b, fa, fb := toxtest.Friends(t)

	id, err := fa.SendMessage(tox.MessageTypeAction, "waves")
	if err != nil {
		t.Fatal(err)
	}
	got, receipt := false, false
	toxtest.Run(t, []*tox.Tox{a, b}, func(x *tox.Tox, ev tox.Event) bool {
		switch ev := ev.(type) {
		case *tox.EventFriendMessage:
			got = ev.Friend == fb && ev.Type == tox.MessageTypeAction && ev.Message == "waves"
		case *tox.EventFriendReadReceipt:
			receipt = ev.Friend == fa && ev.MessageID == id
		}
		return got && receipt
	})

	data := bytes.Repeat([]byte("0123456789"), 300)
	sent, err := fa.FileSend(tox.FileKindData, uint64(len(data)), nil, "digits.txt")
	if err != nil {
		t.Fatal(err)
	}
	var received []byte
	toxtest.Run(t, []*tox.Tox{a, b}, func(x *tox.Tox, ev tox.Event) bool {
		switch ev := ev.(type) {
		case *tox.EventFileRecv:
			if ev.File.Friend() != fb || ev.Filename != "digits.txt" || ev.FileSize != uint64(len(data)) {
				t.Errorf("got offer %+v", ev)
			}
			if err := ev.File.Control(tox.FileControlResume); err != nil {
				t.Fatal(err)
			}
		case *tox.EventFileChunkRequest:
			if ev.File != sent {
				t.Errorf("chunk request for %v, not %v", ev.File, sent)
			}
			if ev.Length > 0 {
				chunk := data[ev.Position : ev.Position+uint64(ev.Length)]
				if err := ev.File.SendChunk(ev.Position, chunk); err != nil {
					t.Fatal(err)
				}
			}
		case *tox.EventFileRecvChunk:
			if ev.Position != uint64(len(received)) {
				t.Fatalf("chunk at %d after %d bytes", ev.Position, len(received))
			}
			received = append(received, ev.Data...)
			return len(ev.Data) == 0
		}
		return false
	})
	if !bytes.Equal(received, data) {
		t.Errorf("received %d bytes, not the %d sent", len(received), len(data))
	}
}

func TestSavedataKeepsIdentity(t *testing.T) {
	a, b := toxtest.New(t, nil), toxtest.New(t, nil)
	if err := a.SetName("alice"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetStatusMessage("here"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.FriendAddNoRequest(b.PublicKey()); err != nil {
		t.Fatal(err)
	}

	opts := tox.DefaultOptions()
	opts.SavedataType = tox.SavedataTypeToxSave
	opts.SavedataData = a.Savedata()
	restored := toxtest.New(t, opts)

	if restored.PublicKey() != a.PublicKey() || restored.Address() != a.Address() {
		t.Errorf("restored %v, not %v", restored.Address(), a.Address())
	}
	if restored.Name() != "alice" || restored.StatusMessage() != "here" {
		t.Errorf("restored %q, %q", restored.Name(), restored.StatusMessage())
	}
	friends := restored.FriendList()
	if len(friends) != 1 {
		t.Fatalf("restored %d friends", len(friends))
	}
	if key, err := friends[0].PublicKey(); err != nil || key != b.PublicKey() {
		t.Errorf("restored friend %v, %v", key, err)
	}
}

func TestErrors(t *testing.T) {
	x := toxtest.New(t, nil)

	err := x.SetName(strings.Repeat("x", 1000))
	if !errors.Is(err, tox.ErrSetInfoTooLong) {
		t.Errorf("long name: %v", err)
	}
	if err.Error() != "tox: set info: too long" {
		t.Errorf("message %q", err)
	}

	if _, err := x.Friend(7).SendMessage(tox.MessageTypeNormal, "hi"); err != tox.ErrFriendSendMessageFriendNotFound {
		t.Errorf("unknown friend: %v", err)
	}
	if _, err := x.FriendAdd(x.Address(), "hi"); err != tox.ErrFriendAddOwnKey {
		t.Errorf("own address: %v", err)
	}

	address, err := tox.ParseAddress(strings.ToLower(x.Address().String()))
	if err != nil || address != x.Address() {
		t.Errorf("parsed %v, %v", address, err)
	}
	if _, err := tox.ParsePublicKey("ABCD"); err == nil {
		t.Error("parsed a short public key")
	}
}

func TestConcurrentUse(t *testing.T) {
	x := toxtest.New(t, nil)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				if i == 0 {
					if _, err := x.Iterate(); err != nil {
						t.Error(err)
					}
				} else if err := x.SetName(strings.Repeat("n", i)); err != nil {
					t.Error(err)
				}
				_ = x.Name()
			}
		}()
	}
	wg.Wait()
}

func TestClose(t *testing.T) {
	x := toxtest.New(t, nil)
	x.Close()
	x.Close()
	defer func() {
		if recover() == nil {
			t.Error("a closed Tox didn't panic")
		}
	}()
	x.Name()
}

func TestGroupInvite(t *testing.T) {
	t.Parallel()
	a, b, fa, _ := toxtest.Friends(t)
	ga, err := a.GroupNew(tox.GroupPrivacyStatePrivate, "lounge", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := ga.InviteFriend(fa); err != nil {
		t.Fatal(err)
	}

	// Group messages only go to peers we are connected to, so both wait to
	// see the other join.
	var gb tox.Group
	aSeesBob, bSeesAlice := false, false
	toxtest.Run(t, []*tox.Tox{a, b}, func(x *tox.Tox, ev tox.Event) bool {
		switch ev := ev.(type) {
		case *tox.EventGroupInvite:
			if ev.GroupName != "lounge" {
				t.Errorf("invited to %q", ev.GroupName)
			}
			if gb, err = b.GroupInviteAccept(ev.Friend, ev.InviteData, "bob", ""); err != nil {
				t.Fatal(err)
			}
		case *tox.EventGroupPeerJoin:
			name, err := ev.Peer.Name()
			if err != nil {
				t.Fatal(err)
			}
			if x == a {
				aSeesBob = ev.Peer.Group() == ga && name == "bob"
			} else {
				bSeesAlice = ev.Peer.Group() == gb && name == "alice"
			}
		}
		return aSeesBob && bSeesAlice
	})

	if name, err := gb.Name(); err != nil || name != "lounge" {
		t.Errorf("group name %q, %v", name, err)
	}
	if err := ga.SetTopic("tea"); err != nil {
		t.Fatal(err)
	}
	if _, err := gb.SendMessage(tox.MessageTypeNormal, "hello all"); err != nil {
		t.Fatal(err)
	}
	topic, message := false, false
	toxtest.Run(t, []*tox.Tox{a, b}, func(x *tox.Tox, ev tox.Event) bool {
		switch ev := ev.(type) {
		case *tox.EventGroupTopic:
			topic = topic || x == b && ev.Peer.Group() == gb && ev.Topic == "tea"
		case *tox.EventGroupMessage:
			name, _ := ev.Peer.Name()
			message = x == a && ev.Peer.Group() == ga && name == "bob" && ev.Message == "hello all"
		}
		return topic && message
	})
}

func TestConferenceInvite(t *testing.T) {
	t.Parallel()
	a, b, fa, _ := toxtest.Friends(t)
	ca, err := a.ConferenceNew()
	if err != nil {
		t.Fatal(err)
	}
	if err := ca.SetTitle("tea time"); err != nil {
		t.Fatal(err)
	}
	if err := ca.Invite(fa); err != nil {
		t.Fatal(err)
	}

	var cb tox.Conference
	toxtest.Run(t, []*tox.Tox{a, b}, func(x *tox.Tox, ev tox.Event) bool {
		switch ev := ev.(type) {
		case *tox.EventConferenceInvite:
			if ev.Type != tox.ConferenceTypeText {
				t.Errorf("invited to a %v conference", ev.Type)
			}
			if cb, err = b.ConferenceJoin(ev.Friend, ev.Cookie); err != nil {
				t.Fatal(err)
			}
		case *tox.EventConferenceConnected:
			return x == b && ev.Conference == cb
		}
		return false
	})

	if title, err := cb.Title(); err != nil || title != "tea time" {
		t.Errorf("title %q, %v", title, err)
	}
	if id, ok := cb.ID(); !ok {
		t.Error("joined conference has no ID")
	} else if found, err := a.ConferenceByID(id); err != nil || found != ca {
		t.Errorf("conference by ID %v: %v, %v", id, found, err)
	}
	if err := cb.SendMessage(tox.MessageTypeNormal, "hi all"); err != nil {
		t.Fatal(err)
	}
	toxtest.Run(t, []*tox.Tox{a, b}, func(x *tox.Tox, ev tox.Event) bool {
		if ev, ok := ev.(*tox.EventConferenceMessage); ok && x == a {
			ours, err := ev.Peer.IsOurs()
			return err == nil && !ours && ev.Peer.Conference() == ca && ev.Message == "hi all"
		}
		return false
	})
}
