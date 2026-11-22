package main

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/internal/toxtest"
)

// network returns the nodes of a Tox network of its own for a test.
func network(t *testing.T) []node {
	r := toxtest.NewRelay(t)
	return []node{{"127.0.0.1", r.Port, []tox.Port{r.TCPPort}, r.DhtID.String()}}
}

// A client is a running core, as the app runs it.
type client struct {
	*core
	stop func()

	alertsMu sync.Mutex
	alerts   []alert
}

// alerted returns the alerts the client showed and took back.
func (c *client) alerted() []alert {
	c.alertsMu.Lock()
	defer c.alertsMu.Unlock()
	return slices.Clone(c.alerts)
}

// start runs a core on the profile in dir until the test ends or it stops.
func start(t *testing.T, dir string, nodes []node) *client {
	t.Helper()
	if err := (settings{UDP: true}).save(dir); err != nil {
		t.Fatal(err)
	}
	c, err := newCore(dir, nodes, func() {}, "")
	if err != nil {
		t.Fatal(err)
	}
	cl := &client{core: c}
	c.alert = func(a alert) {
		cl.alertsMu.Lock()
		defer cl.alertsMu.Unlock()
		cl.alerts = append(cl.alerts, a)
	}
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		c.run(done)
	}()
	var once sync.Once
	cl.stop = func() {
		once.Do(func() {
			close(done)
			<-stopped
			c.close()
		})
	}
	t.Cleanup(cl.stop)
	return cl
}

// until waits for the client's view to satisfy cond.
func (c *client) until(t *testing.T, cond func(v *view) bool) {
	t.Helper()
	toxtest.Until(t, func() bool { return cond(c.snapshot(nil)) })
}

// friend returns the friend with the key, as the client shows them.
func (c *client) friend(key tox.PublicKey) (chatView, bool) {
	cv, ok := c.snapshot(nil).chatView(friendChat(key))
	return cv, ok && cv.friend != nil
}

func (c *client) online(key tox.PublicKey) bool {
	f, ok := c.friend(key)
	return ok && f.online()
}

// history returns the client's chat with the friend.
func (c *client) history(t *testing.T, key tox.PublicKey) []message {
	t.Helper()
	return c.chatHistory(t, friendChat(key))
}

// chatHistory returns one of the client's chats.
func (c *client) chatHistory(t *testing.T, id chatID) []message {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	ms, err := c.store.messages(id, math.MaxInt64, math.MaxInt32)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

// befriend makes a and b friends, the way users do: a sends a request and b
// accepts it.
func befriend(t *testing.T, a, b *client) {
	t.Helper()
	if err := a.friendAdd(b.tox.Address().String(), "hello"); err != nil {
		t.Fatal(err)
	}
	b.until(t, func(v *view) bool {
		return len(v.requests) == 1 && v.requests[0].key == a.tox.PublicKey()
	})
	if msg := b.snapshot(nil).requests[0].message; msg != "hello" {
		t.Errorf("request message %q, want %q", msg, "hello")
	}
	if err := b.accept(a.tox.PublicKey()); err != nil {
		t.Fatal(err)
	}
	if len(b.snapshot(nil).requests) != 0 {
		t.Error("the request is still there after accepting it")
	}
	toxtest.Until(t, func() bool { return a.online(b.tox.PublicKey()) && b.online(a.tox.PublicKey()) })
}

func TestProfileKeepsFriendNames(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	dirB := t.TempDir()
	a, b := start(t, t.TempDir(), nodes), start(t, dirB, nodes)
	befriend(t, a, b)

	if err := a.setName("Alice"); err != nil {
		t.Fatal(err)
	}
	b.until(t, func(*view) bool {
		f, _ := b.friend(a.tox.PublicKey())
		return f.friend != nil && f.friend.name == "Alice"
	})
	// A phone may stop the app without letting it save: the name is in the
	// profile already, before b closes.
	toxtest.Until(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return !b.unsaved
	})
	saved, err := os.ReadFile(profilePath(dirB))
	if err != nil {
		t.Fatal(err)
	}
	dirC := t.TempDir()
	if err := writeFile(profilePath(dirC), saved); err != nil {
		t.Fatal(err)
	}
	c := start(t, dirC, nil)
	if f, ok := c.friend(a.tox.PublicKey()); !ok || f.friend.name != "Alice" {
		t.Errorf("from the saved profile: friend %+v, %v; want Alice", f, ok)
	}
}

func TestMessagesArriveWithReceipts(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	a, b := start(t, t.TempDir(), nodes), start(t, t.TempDir(), nodes)
	befriend(t, a, b)
	ka, kb := a.tox.PublicKey(), b.tox.PublicKey()
	a.send(friendChat(kb), "hi")
	a.send(friendChat(kb), "/me waves")
	b.until(t, func(*view) bool { return len(b.history(t, ka)) == 2 })
	got := b.history(t, ka)
	if got[0].text != "hi" || got[0].action || got[0].outgoing ||
		got[1].text != "waves" || !got[1].action {
		t.Errorf("b got %+v", got)
	}
	a.until(t, func(*view) bool {
		ms := a.history(t, kb)
		return ms[0].delivered && ms[1].delivered
	})

	// Unread, with alerts, until b opens the chat.
	if f, _ := b.friend(ka); f.unread != 2 {
		t.Errorf("unread %d, want 2", f.unread)
	}
	fa := friendChat(ka)
	b.setOpen(&fa)
	if v := b.snapshot(nil); len(v.chat) != 2 || v.chats[0].unread != 0 {
		t.Errorf("open chat: %+v, unread %d", v.chat, v.chats[0].unread)
	}
	want := []alert{
		{fa, "Friend request", "hello"},
		{key: fa},
		{fa, "Tox user", "hi"},
		{fa, "Tox user (2)", "* waves"},
		{key: fa},
	}
	if got := b.alerted(); !slices.Equal(got, want) {
		t.Errorf("alerts %+v, want %+v", got, want)
	}
}

func TestMessagesWaitForTheFriend(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	dirA, dirB := t.TempDir(), t.TempDir()
	a, b := start(t, dirA, nodes), start(t, dirB, nodes)
	befriend(t, a, b)
	ka, kb := a.tox.PublicKey(), b.tox.PublicKey()

	b.stop()
	toxtest.Until(t, func() bool { return !a.online(kb) })
	a.send(friendChat(kb), "while you were out")
	// toxcore doesn't keep messages across restarts: the client does.
	a.stop()
	a = start(t, dirA, nodes)
	if ms := a.history(t, kb); len(ms) != 1 || ms[0].delivered {
		t.Fatalf("a's history after a restart: %+v", ms)
	}

	b = start(t, dirB, nodes)
	b.until(t, func(*view) bool { return len(b.history(t, ka)) == 1 })
	if got := b.history(t, ka)[0].text; got != "while you were out" {
		t.Errorf("b got %q", got)
	}
	a.until(t, func(*view) bool { return a.history(t, kb)[0].delivered })
}

func TestSplitMessage(t *testing.T) {
	long := strings.Repeat("word ", 400) + strings.Repeat("ü", 1000)
	parts := splitMessage(long, tox.MaxMessageLength)
	for _, p := range parts {
		if len(p) > tox.MaxMessageLength || !utf8.ValidString(p) {
			t.Errorf("part of %d bytes, valid UTF-8 %v", len(p), utf8.ValidString(p))
		}
	}
	// The cut at a space drops it; the cut in a word keeps every byte.
	if got := strings.Join(parts, ""); strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(long, " ", "") {
		t.Error("the parts don't make up the message")
	}
	if !strings.HasSuffix(parts[0], "word") {
		t.Errorf("first part ends %q, want a whole word", parts[0][len(parts[0])-8:])
	}
	if got := splitMessage("short", tox.MaxMessageLength); !slices.Equal(got, []string{"short"}) {
		t.Errorf("short message: %q", got)
	}
}

// fileMessage returns the client's newest file in the chat with key.
func (c *client) fileMessage(t *testing.T, key tox.PublicKey) (message, bool) {
	ms := c.history(t, key)
	for i := len(ms) - 1; i >= 0; i-- {
		if ms[i].file != nil {
			return ms[i], true
		}
	}
	return message{}, false
}

func TestFilesGoOnAfterARestart(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	dirB := t.TempDir()
	a, b := start(t, t.TempDir(), nodes), start(t, dirB, nodes)
	befriend(t, a, b)
	ka, kb := a.tox.PublicKey(), b.tox.PublicKey()

	data := make([]byte, 8<<20)
	rand.Read(data)
	if err := a.sendFile(friendChat(kb), "big.bin", bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	var offer message
	b.until(t, func(*view) bool {
		var ok bool
		offer, ok = b.fileMessage(t, ka)
		return ok
	})
	if f := offer.file; f.name != "big.bin" || f.size != uint64(len(data)) || f.state != fileOffered {
		t.Fatalf("offer %+v", f)
	}

	// b takes part of it, pauses, and restarts.
	b.fileDo(friendChat(ka), offer.id, fileAccept)
	toxtest.Until(t, func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		return b.progress[offer.id] > 0
	})
	b.fileDo(friendChat(ka), offer.id, filePause)
	b.stop()
	b = start(t, dirB, nodes)
	if m, _ := b.fileMessage(t, ka); m.file.state != filePaused {
		t.Fatalf("after pausing and a restart: %+v", m.file)
	}
	toxtest.Until(t, func() bool { return b.online(ka) })
	b.fileDo(friendChat(ka), offer.id, fileResume)
	b.until(t, func(*view) bool {
		m, _ := b.fileMessage(t, ka)
		return m.file.state == fileDone
	})
	got, err := os.ReadFile(b.filePath(offer.id))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("b got %d bytes, not what a sent", len(got))
	}
	// Other apps get it under its name.
	named, err := b.namedFile(offer.id, offer.file.name)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(named); filepath.Base(named) != offer.file.name || err != nil || !bytes.Equal(got, data) {
		t.Errorf("named file %s: %v", named, err)
	}
	a.until(t, func(*view) bool {
		m, _ := a.fileMessage(t, kb)
		return m.file.state == fileDone
	})
}

func TestAvatars(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	dirB := t.TempDir()
	a, b := start(t, t.TempDir(), nodes), start(t, dirB, nodes)
	befriend(t, a, b)
	ka := a.tox.PublicKey()

	picture := image.NewNRGBA(image.Rect(0, 0, 300, 200))
	picture.Set(150, 100, color.NRGBA{R: 0xff, A: 0xff})
	var data bytes.Buffer
	if err := png.Encode(&data, picture); err != nil {
		t.Fatal(err)
	}
	if err := a.setAvatar(data.Bytes()); err != nil {
		t.Fatal(err)
	}
	if v := a.snapshot(nil); v.avatar == nil || v.avatar.image.Bounds().Dx() != avatarSize {
		t.Fatalf("a's avatar: %+v", v.avatar)
	}
	want := a.snapshot(nil).avatar.hash
	b.until(t, func(*view) bool {
		f, _ := b.friend(ka)
		return f.avatar() != nil && f.avatar().hash == want
	})
	b.stop()
	b = start(t, dirB, nodes)
	if f, _ := b.friend(ka); f.avatar() == nil || f.avatar().hash != want {
		t.Errorf("after a restart, b has avatar %+v", f.avatar())
	}
}

// invited waits for an invitation from the friend, and joins it.
func (c *client) invited(t *testing.T, from tox.PublicKey) chatID {
	t.Helper()
	var inv message
	c.until(t, func(*view) bool {
		for _, m := range c.history(t, from) {
			if m.invite != nil {
				inv = m
				return true
			}
		}
		return false
	})
	id, err := c.joinInvite(friendChat(from), inv.id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// received waits for a message in a chat, from someone else.
func (c *client) received(t *testing.T, id chatID, text string) message {
	t.Helper()
	var got message
	c.until(t, func(*view) bool {
		for _, m := range c.chatHistory(t, id) {
			if !m.outgoing && m.text == text {
				got = m
				return true
			}
		}
		return false
	})
	return got
}

func TestGroups(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	dirA := t.TempDir()
	a, b := start(t, dirA, nodes), start(t, t.TempDir(), nodes)
	befriend(t, a, b)
	if err := a.setName("Alice"); err != nil {
		t.Fatal(err)
	}
	ka := a.tox.PublicKey()

	ga, err := a.createGroup("Gophers", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.inviteFriend(ga, friendChat(b.tox.PublicKey())); err != nil {
		t.Fatal(err)
	}
	gb := b.invited(t, ka)
	if gb != ga {
		t.Errorf("b joined %v, not %v", gb, ga)
	}
	people := func(n int) func(*view) bool {
		return func(v *view) bool {
			cv, _ := v.chatView(ga)
			return cv.group != nil && len(cv.group.peers) == n
		}
	}
	a.until(t, people(2))
	b.until(t, people(2))
	if cv, _ := b.snapshot(nil).chatView(gb); cv.title != "Gophers" {
		t.Errorf("b calls the group %q", cv.title)
	}

	a.send(ga, "hello group")
	if m := b.received(t, gb, "hello group"); m.senderName != "Alice" || m.sender == nil {
		t.Errorf("b got %+v", m)
	}
	b.send(gb, "hi Alice")
	a.received(t, ga, "hi Alice")

	if err := a.groupDo(ga, func(g *group) error { return g.SetTopic("Go") }); err != nil {
		t.Fatal(err)
	}
	b.until(t, func(*view) bool {
		for _, m := range b.chatHistory(t, gb) {
			if m.notice && m.text == "Alice set the topic: Go" {
				return true
			}
		}
		return false
	})

	// b talks to Alice alone.
	var aliceInGroup tox.PublicKey
	cv, _ := b.snapshot(nil).chatView(gb)
	for _, p := range cv.group.peers {
		if !p.self {
			aliceInGroup = p.key
		}
	}
	pb, err := b.openPrivate(gb, aliceInGroup)
	if err != nil {
		t.Fatal(err)
	}
	b.send(pb, "psst")
	var bobInGroup tox.PublicKey
	cv, _ = a.snapshot(nil).chatView(ga)
	for _, p := range cv.group.peers {
		if !p.self {
			bobInGroup = p.key
		}
	}
	a.received(t, privateChat(ga.key, bobInGroup), "psst")

	// The group and its chats stay.
	a.stop()
	a = start(t, dirA, nodes)
	if cv, ok := a.snapshot(nil).chatView(ga); !ok || cv.title != "Gophers" {
		t.Errorf("after a restart, a has the group: %v, %q", ok, cv.title)
	}
	if _, ok := a.snapshot(nil).chatView(privateChat(ga.key, bobInGroup)); !ok {
		t.Error("after a restart, a has no private chat with b")
	}
}

func TestConferences(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	dirA := t.TempDir()
	a, b := start(t, dirA, nodes), start(t, t.TempDir(), nodes)
	befriend(t, a, b)
	if err := a.setName("Alice"); err != nil {
		t.Fatal(err)
	}
	ca, err := a.createConference("Old friends")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.inviteFriend(ca, friendChat(b.tox.PublicKey())); err != nil {
		t.Fatal(err)
	}
	cb := b.invited(t, a.tox.PublicKey())
	b.until(t, func(v *view) bool {
		cv, _ := v.chatView(cb)
		return cv.conference != nil && cv.conference.connected && len(cv.conference.peers) == 2
	})
	a.send(ca, "hello conference")
	if m := b.received(t, cb, "hello conference"); m.senderName != "Alice" {
		t.Errorf("b got %+v", m)
	}
	b.send(cb, "hi")
	a.received(t, ca, "hi")

	a.stop()
	a = start(t, dirA, nodes)
	if cv, ok := a.snapshot(nil).chatView(ca); !ok || cv.title != "Old friends" {
		t.Errorf("after a restart, a has the conference: %v, %q", ok, cv.title)
	}
}

func TestPassword(t *testing.T) {
	dir := t.TempDir()
	c, err := newCore(dir, nil, func() {}, "")
	if err != nil {
		t.Fatal(err)
	}
	var bob tox.PublicKey
	bob[0] = 1
	c.mu.Lock()
	c.store.add(friendChat(bob), &message{time: time.Now(), text: "secret"})
	c.mu.Unlock()
	if err := c.setPassword("hunter2"); err != nil {
		t.Fatal(err)
	}
	key := c.tox.PublicKey()
	c.close()

	if !profileEncrypted(dir) {
		t.Error("the profile isn't encrypted")
	}
	if db, err := os.ReadFile(filepath.Join(dir, "chats.db")); err != nil || bytes.Contains(db, []byte("SQLite format 3")) {
		t.Errorf("the chats aren't encrypted: %v", err)
	}
	if _, err := newCore(dir, nil, func() {}, ""); err != errLocked {
		t.Errorf("without the password: %v", err)
	}
	if _, err := newCore(dir, nil, func() {}, "hunter3"); err != errWrongPassword {
		t.Errorf("with the wrong password: %v", err)
	}
	c, err = newCore(dir, nil, func() {}, "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if c.tox.PublicKey() != key || len(c.history(t, bob)) != 1 {
		t.Errorf("with the password: key %v, history %+v", c.tox.PublicKey(), c.history(t, bob))
	}
	if err := c.setPassword(""); err != nil {
		t.Fatal(err)
	}
	c.close()
	if profileEncrypted(dir) {
		t.Error("the profile is still encrypted")
	}
}

// history returns a core's chat with a friend.
func (c *core) history(t *testing.T, key tox.PublicKey) []message {
	t.Helper()
	ms, err := c.store.messages(friendChat(key), math.MaxInt64, math.MaxInt32)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

// fakeMedia records a tone and counts what it plays.
type fakeMedia struct {
	played atomic.Int64
}

type fakeStream struct {
	m    *fakeMedia
	done chan struct{}
	once sync.Once
}

func (m *fakeMedia) start(f func()) *fakeStream {
	s := &fakeStream{m: m, done: make(chan struct{})}
	go func() {
		for {
			select {
			case <-s.done:
				return
			case <-time.After(20 * time.Millisecond):
				f()
			}
		}
	}()
	return s
}

func (m *fakeMedia) openAudio(record func([]int16)) (audioIO, error) {
	pcm := make([]int16, audioFrame)
	for i := range pcm {
		pcm[i] = int16(8000 * math.Sin(float64(i)*2*math.Pi*440/audioRate))
	}
	return m.start(func() { record(pcm) }), nil
}

func (m *fakeMedia) openCamera(frame func(int, int, []byte, []byte, []byte)) (io.Closer, error) {
	const w, h = 320, 240
	y, u, v := bytes.Repeat([]byte{128}, w*h), bytes.Repeat([]byte{128}, w*h/4), bytes.Repeat([]byte{128}, w*h/4)
	return m.start(func() { frame(w, h, y, u, v) }), nil
}

func (s *fakeStream) play([]int16, int, int) { s.m.played.Add(1) }

func (s *fakeStream) Close() error {
	s.once.Do(func() { close(s.done) })
	return nil
}

func TestCalls(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	a, b := start(t, t.TempDir(), nodes), start(t, t.TempDir(), nodes)
	ma, mb := &fakeMedia{}, &fakeMedia{}
	a.mu.Lock()
	a.media = ma
	a.mu.Unlock()
	b.mu.Lock()
	b.media = mb
	b.mu.Unlock()
	befriend(t, a, b)
	ka, kb := a.tox.PublicKey(), b.tox.PublicKey()

	if err := a.callFriend(friendChat(kb), true); err != nil {
		t.Fatal(err)
	}
	b.until(t, func(v *view) bool { return v.call != nil && !v.call.outgoing && !v.call.answered })
	if err := b.answer(false); err != nil {
		t.Fatal(err)
	}
	a.until(t, func(v *view) bool { return v.call != nil && v.call.answered })
	// Sound both ways, and a's video to b.
	toxtest.Until(t, func() bool { return ma.played.Load() > 10 && mb.played.Load() > 10 })
	b.until(t, func(v *view) bool { return v.call.frame != nil && v.call.frame.Bounds().Dx() == 320 })

	a.hangUp()
	b.until(t, func(v *view) bool { return v.call == nil })
	for _, c := range []struct {
		cl   *client
		with tox.PublicKey
	}{{a, kb}, {b, ka}} {
		ms := c.cl.history(t, c.with)
		if last := ms[len(ms)-1]; !last.notice || !strings.HasPrefix(last.text, "Call, ") {
			t.Errorf("the chat ends with %+v", last)
		}
	}
}

func TestCallWhileAnotherProfileHasOne(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	a, b := start(t, t.TempDir(), nodes), start(t, t.TempDir(), nodes)
	var line callLine
	for _, c := range []*client{a, b} {
		c.mu.Lock()
		c.media = &fakeMedia{}
		c.mu.Unlock()
	}
	b.mu.Lock()
	b.line = &line
	b.mu.Unlock()
	befriend(t, a, b)
	ka, kb := a.tox.PublicKey(), b.tox.PublicKey()

	// Another of b's profiles is in a call.
	line.take(&core{})
	if err := a.callFriend(friendChat(kb), false); err != nil {
		t.Fatal(err)
	}
	toxtest.Until(t, func() bool {
		ms := b.history(t, ka)
		return len(ms) > 0 && ms[len(ms)-1].text == "Missed call: you were in another call"
	})
	a.until(t, func(v *view) bool { return v.call == nil })
	b.until(t, func(v *view) bool { return v.call == nil })
}
