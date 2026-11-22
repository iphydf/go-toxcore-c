package main

import (
	"crypto/sha256"
	"errors"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/toxav"
	"github.com/TokTok/go-toxcore-c/toxencryptsave"
)

// A node is a DHT node to bootstrap from.
type node struct {
	host     string
	port     tox.Port
	tcpPorts []tox.Port
	key      string
}

// An alert tells the user about a message or a friend request while they
// aren't looking, or, with no text, takes back the one for that chat.
type alert struct {
	key   chatID
	title string
	text  string
}

// A core runs the Tox instance and keeps what the UI shows.
type core struct {
	tox *tox.Tox
	av  *toxav.AV
	// media is the platform's sound and camera.
	media media
	// line is the one call there can be, which all profiles share.
	line  *callLine
	store *store
	dir   string
	nodes []node
	// ownNodes are the user's: Tox always asks them.
	ownNodes   []node
	invalidate func()
	// alert shows alerts. The core calls it without holding mu.
	alert func(alert)
	// passKey encrypts the profile, if the user gave it a password.
	passKey *toxencryptsave.PassKey
	// settings are the ones the Tox started with.
	settings settings

	mu sync.Mutex
	// version goes up with every change the UI should show.
	version     int
	connection  tox.Connection
	chats       map[chatID]*chat
	friends     map[tox.FriendNumber]*friend
	groups      map[tox.GroupNumber]*group
	conferences map[tox.ConferenceNumber]*conference
	requests    []request
	// open is the chat on screen. The user reads its messages while the
	// window is focused.
	open    *chatID
	focused bool
	// chat has the open chat's newest messages, oldest first.
	chat []message
	// older is set when the open chat has messages from before those in chat.
	older bool
	// unsaved is set when the profile changed since it was saved: a friend's
	// name or a group's topic, say. On a phone, the app may not get to save
	// when it stops.
	unsaved bool
	// alerts wait for mu to be unlocked.
	alerts []alert
	// progress is how much of each file that is going went.
	progress map[int64]uint64
	// avatar is ours.
	avatar *avatar
	// call is the call going on, if there is one.
	call *call
}

func profilePath(dir string) string { return filepath.Join(dir, "profile.tox") }

var (
	// errLocked is opening a profile with a password without it.
	errLocked = errors.New("the profile has a password")
	// errWrongPassword is opening a profile with the wrong password.
	errWrongPassword = errors.New("that's not the password")
)

// profileEncrypted reports whether the profile in dir has a password.
func profileEncrypted(dir string) bool {
	data, err := os.ReadFile(profilePath(dir))
	return err == nil && toxencryptsave.IsDataEncrypted(data)
}

// newCore loads the profile in dir, with its password if it has one, or makes
// a new one. It bootstraps from nodes.
func newCore(dir string, nodes []node, invalidate func(), password string) (*core, error) {
	for _, sub := range []string{"files", "avatars"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			return nil, err
		}
	}
	opts := tox.DefaultOptions()
	// Groups go in the profile, as friends do.
	opts.ExperimentalGroupsPersistence = true
	opts.LogCallback = func(level tox.LogLevel, file string, line uint32, function, message string) {
		if level >= tox.LogLevelWarning {
			log.Printf("toxcore: %s:%d: %s", file, line, message)
		}
	}
	set, err := loadSettings(dir)
	if err != nil {
		log.Printf("reading the settings: %v", err)
	}
	set.apply(opts)
	own, err := parseNodes(set.Nodes)
	if err != nil {
		log.Printf("your nodes: %v", err)
	}
	saved, err := os.ReadFile(profilePath(dir))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var passKey *toxencryptsave.PassKey
	if toxencryptsave.IsDataEncrypted(saved) {
		if password == "" {
			return nil, errLocked
		}
		if passKey, saved, err = decryptProfile(saved, password); err != nil {
			return nil, err
		}
	}
	if saved != nil {
		opts.SavedataType = tox.SavedataTypeToxSave
		opts.SavedataData = saved
	}
	t, err := tox.New(opts)
	if err != nil {
		closeKey(passKey)
		return nil, err
	}
	st, err := openStore(filepath.Join(dir, "chats.db"), storeKey(t.SecretKey()))
	if err != nil {
		t.Close()
		closeKey(passKey)
		return nil, err
	}
	av, err := toxav.New(t)
	if err != nil {
		st.close()
		t.Close()
		closeKey(passKey)
		return nil, err
	}
	c := &core{
		tox:         t,
		av:          av,
		media:       noMedia{},
		store:       st,
		dir:         dir,
		nodes:       nodes,
		invalidate:  invalidate,
		alert:       func(alert) {},
		passKey:     passKey,
		settings:    set,
		ownNodes:    own,
		chats:       map[chatID]*chat{},
		friends:     map[tox.FriendNumber]*friend{},
		groups:      map[tox.GroupNumber]*group{},
		conferences: map[tox.ConferenceNumber]*conference{},
		progress:    map[int64]uint64{},
	}
	c.avatar = loadAvatar(c.avatarPath())
	if saved == nil {
		if err := t.SetName("Tox user"); err != nil {
			c.close()
			return nil, err
		}
		c.saveProfile()
	}
	for _, f := range t.FriendList() {
		c.addFriend(f)
	}
	for _, g := range t.GroupList() {
		c.addGroup(g)
	}
	for _, cf := range t.ConferenceChatlist() {
		c.addConference(cf)
	}
	if c.requests, err = st.requests(); err != nil {
		log.Print(err)
	}
	log.Printf("Tox ID: %v", t.Address())
	return c, nil
}

// run iterates the Tox and its calls until done.
func (c *core) run(done <-chan struct{}) {
	avDone := make(chan struct{})
	go func() {
		defer close(avDone)
		c.runAV(done)
	}()
	defer func() { <-avDone }()
	c.bootstrap()
	lastBootstrap := time.Now()
	for {
		events, err := c.tox.Iterate()
		if err != nil {
			log.Print(err)
		}
		if len(events) != 0 {
			c.mu.Lock()
			for _, ev := range events {
				c.handle(ev)
			}
			c.unlock()
		}
		// Without a connection, try other nodes now and then.
		c.mu.Lock()
		offline := c.connection == tox.ConnectionNone
		if c.unsaved {
			c.saveProfile()
			c.unsaved = false
		}
		c.mu.Unlock()
		if offline && time.Since(lastBootstrap) > 10*time.Second {
			c.bootstrap()
			lastBootstrap = time.Now()
		}
		select {
		case <-done:
			return
		case <-time.After(c.tox.IterationInterval()):
		}
	}
}

// bootstrap asks the user's nodes and a few random others for more, and adds
// their TCP relays.
func (c *core) bootstrap() {
	nodes := slices.Clone(c.ownNodes)
	for _, i := range rand.Perm(len(c.nodes))[:min(4, len(c.nodes))] {
		nodes = append(nodes, c.nodes[i])
	}
	for _, n := range nodes {
		key, err := tox.ParseDhtID(n.key)
		if err != nil {
			log.Print(err)
			continue
		}
		if err := c.tox.Bootstrap(n.host, n.port, key); err != nil {
			log.Printf("bootstrapping from %s: %v", n.host, err)
		}
		for _, port := range n.tcpPorts {
			if err := c.tox.AddTCPRelay(n.host, port, key); err != nil {
				log.Printf("adding TCP relay %s:%d: %v", n.host, port, err)
			}
		}
	}
}

// handle applies an event. The caller holds c.mu.
func (c *core) handle(ev tox.Event) {
	if e, ok := ev.(*tox.EventSelfConnectionStatus); ok {
		c.connection = e.ConnectionStatus
		return
	}
	_ = c.handleFriend(ev) || c.handleGroup(ev) || c.handleConference(ev)
}

// change runs f under the lock, then shows its result.
func (c *core) change(f func()) {
	c.mu.Lock()
	f()
	c.unlock()
}

// unlock unlocks mu after a change, then shows it.
func (c *core) unlock() {
	c.version++
	alerts := c.alerts
	c.alerts = nil
	c.mu.Unlock()
	for _, a := range alerts {
		c.alert(a)
	}
	c.invalidate()
}

// decryptProfile reads a profile with a password.
func decryptProfile(data []byte, password string) (*toxencryptsave.PassKey, []byte, error) {
	salt, err := toxencryptsave.Salt(data)
	if err != nil {
		return nil, nil, err
	}
	key, err := toxencryptsave.PassKeyDeriveWithSalt([]byte(password), salt)
	if err != nil {
		return nil, nil, err
	}
	plain, err := key.Decrypt(data)
	if err != nil {
		key.Close()
		return nil, nil, errWrongPassword
	}
	return key, plain, nil
}

// closeKey closes a profile's key, if it has one.
func closeKey(k *toxencryptsave.PassKey) {
	if k != nil {
		k.Close()
	}
}

// storeKey is the key of the chats' database. It comes from the profile's
// secret key: the chats are as safe as the profile.
func storeKey(secret tox.SecretKey) [32]byte {
	return sha256.Sum256(append([]byte("Go Tox chats\x00"), secret[:]...))
}

// saveProfile writes the Tox's savedata, encrypted if the profile has a
// password.
func (c *core) saveProfile() {
	data := c.tox.Savedata()
	if c.passKey != nil {
		var err error
		if data, err = c.passKey.Encrypt(data); err != nil {
			log.Printf("encrypting the profile: %v", err)
			return
		}
	}
	if err := writeFile(profilePath(c.dir), data); err != nil {
		log.Printf("saving the profile: %v", err)
	}
}

// setPassword gives the profile a password, or with "" takes it away.
func (c *core) setPassword(password string) error {
	var key *toxencryptsave.PassKey
	if password != "" {
		var err error
		// Slow on purpose: not under the lock.
		if key, err = toxencryptsave.PassKeyDerive([]byte(password)); err != nil {
			return err
		}
	}
	c.change(func() {
		closeKey(c.passKey)
		c.passKey = key
		c.saveProfile()
	})
	return nil
}

// profileData is the profile as it is on disk, for exporting it.
func (c *core) profileData() ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.saveProfile()
	return os.ReadFile(profilePath(c.dir))
}

// writeFile replaces a file, so a crash leaves the old one or the new one.
func writeFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// close saves the profile and stops the Tox.
func (c *core) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.saveProfile()
	for _, fr := range c.friends {
		c.dropTransfers(fr)
	}
	c.endCall()
	c.av.Close()
	c.tox.Close()
	if err := c.store.close(); err != nil {
		log.Print(err)
	}
	closeKey(c.passKey)
}

// A view is what the UI shows: a copy of the core's state.
type view struct {
	version       int
	settings      settings
	name          string
	statusMessage string
	status        tox.UserStatus
	address       tox.Address
	connection    tox.Connection
	// chats are newest first.
	chats    []chatView
	requests []request
	// chat has the open chat's messages that the core loaded.
	chat []message
	// older is set when the open chat has messages from before those in chat.
	older  bool
	avatar *avatar
	// encrypted is set when the profile has a password.
	encrypted bool
	// call is the call going on, if there is one.
	call *callView
}

// A chatView is a chat as the UI shows it. One of friend, group, conference
// and private is set.
type chatView struct {
	id     chatID
	title  string
	unread int
	last   *message

	friend     *friendView
	group      *groupView
	conference *conferenceView
	private    *privateView
}

type friendView struct {
	number        tox.FriendNumber
	key           tox.PublicKey
	name          string
	nickname      string
	statusMessage string
	status        tox.UserStatus
	connection    tox.Connection
	typing        bool
	avatar        *avatar
}

type conferenceView struct {
	name      string
	connected bool
	peers     []peerView
}

type privateView struct {
	group     chatID
	groupName string
	key       tox.PublicKey
	name      string
	// online is set while the peer is in the group.
	online bool
}

// snapshot copies the state, if it changed since old.
func (c *core) snapshot(old *view) *view {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old != nil && old.version == c.version {
		return old
	}
	v := &view{
		version:       c.version,
		settings:      c.settings,
		name:          c.tox.Name(),
		statusMessage: c.tox.StatusMessage(),
		status:        c.tox.Status(),
		address:       c.tox.Address(),
		connection:    c.connection,
		requests:      slices.Clone(c.requests),
		older:         c.older,
		avatar:        c.avatar,
		encrypted:     c.passKey != nil,
		call:          c.callView(),
	}
	if ch := c.openChat(); ch != nil {
		v.chat = make([]message, len(c.chat))
		for i, m := range c.chat {
			v.chat[i] = c.viewOf(ch, m)
		}
	}
	for _, ch := range c.chats {
		cv := chatView{id: ch.id, title: ch.title(), unread: ch.unread}
		if ch.last != nil {
			m := c.viewOf(ch, *ch.last)
			cv.last = &m
		}
		switch {
		case ch.friend != nil:
			fr := ch.friend
			cv.friend = &friendView{
				number:        fr.Number(),
				key:           fr.key,
				name:          fr.name,
				nickname:      fr.nickname,
				statusMessage: fr.statusMessage,
				status:        fr.status,
				connection:    fr.connection,
				typing:        fr.typing,
				avatar:        fr.avatar,
			}
		case ch.group != nil:
			cv.group = ch.group.view()
		case ch.conference != nil:
			cf := ch.conference
			cv.conference = &conferenceView{name: cf.name, connected: cf.connected, peers: slices.Clone(cf.peers)}
		case ch.private != nil:
			p := ch.private
			cv.private = &privateView{
				group:     p.group.chat.id,
				groupName: p.group.title(),
				key:       p.key,
				name:      p.title(),
				online:    p.peer() != nil,
			}
		}
		v.chats = append(v.chats, cv)
	}
	// Newest first, then by name.
	slices.SortFunc(v.chats, func(a, b chatView) int {
		var at, bt time.Time
		if a.last != nil {
			at = a.last.time
		}
		if b.last != nil {
			bt = b.last.time
		}
		if c := bt.Compare(at); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.title), strings.ToLower(b.title))
	})
	return v
}

// viewOf is a message as the UI shows it: with whether toxcore has it, and
// how much of a file went. The caller holds c.mu.
func (c *core) viewOf(ch *chat, m message) message {
	if ch.friend != nil {
		_, m.sent = ch.friend.sending[m.id]
	} else {
		m.sent = m.delivered
	}
	if m.file != nil {
		f := *m.file
		if f.state == fileDone {
			f.done = f.size
		} else {
			f.done = c.progress[m.id]
		}
		m.file = &f
	}
	return m
}

// chat returns a chat by its ID.
func (v *view) chatView(id chatID) (chatView, bool) {
	for _, cv := range v.chats {
		if cv.id == id {
			return cv, true
		}
	}
	return chatView{}, false
}

// friends returns the friends, by name.
func (v *view) friends() []chatView {
	var fs []chatView
	for _, cv := range v.chats {
		if cv.friend != nil {
			fs = append(fs, cv)
		}
	}
	slices.SortFunc(fs, func(a, b chatView) int {
		return strings.Compare(strings.ToLower(a.title), strings.ToLower(b.title))
	})
	return fs
}

// online says whether the other side of a chat is there: the friend, the
// group, the conference or the peer.
func (cv chatView) online() bool {
	switch {
	case cv.friend != nil:
		return cv.friend.connection != tox.ConnectionNone
	case cv.group != nil:
		return cv.group.connected
	case cv.conference != nil:
		return cv.conference.connected
	case cv.private != nil:
		return cv.private.online
	}
	return false
}

func connectionText(c tox.Connection) string {
	switch c {
	case tox.ConnectionUDP:
		return "online"
	case tox.ConnectionTCP:
		return "online (TCP)"
	}
	return "offline"
}

// statusText says how the chat's other side is.
func (cv chatView) statusText() string {
	switch {
	case cv.group != nil:
		g := cv.group
		if !g.connected {
			return "connecting…"
		}
		if g.topic != "" {
			return g.topic
		}
		return peopleText(len(g.peers))
	case cv.conference != nil:
		if !cv.conference.connected {
			return "connecting…"
		}
		return peopleText(len(cv.conference.peers))
	case cv.private != nil:
		if !cv.private.online {
			return "not in " + cv.private.groupName
		}
		return "in " + cv.private.groupName
	}
	f := cv.friend
	if f.connection == tox.ConnectionNone {
		return "offline"
	}
	switch f.status {
	case tox.UserStatusAway:
		return "away"
	case tox.UserStatusBusy:
		return "busy"
	}
	if f.typing {
		return "typing…"
	}
	return connectionText(f.connection)
}

func peopleText(n int) string {
	if n == 1 {
		return "1 person"
	}
	return itoa(n) + " people"
}
