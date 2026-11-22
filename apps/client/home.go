package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/TokTok/go-toxcore-c/toxencryptsave"
)

// A home keeps profiles, each a Tox identity with its chats in a directory of
// its own. Every unlocked profile runs, and the UI shows one. The UI goroutine
// calls it.
type home struct {
	// base has the profiles, in base/profiles. With fixed, the app has one
	// profile, in that directory.
	base  string
	fixed string
	plat  *platform
	// invalidate draws the window again.
	invalidate func()
	// nodes are where profiles join the network: a test's own, or with nil
	// the built-in ones.
	nodes []node

	// sessions are the running profiles, by name.
	sessions map[string]*session
	// shown is the one the UI shows, of sessions, or "".
	shown string
	// passwords are the running profiles', to start them again with new
	// settings.
	passwords map[string]string
	// line is the one call there can be, for all of them.
	line callLine
}

// A session is one run of a profile's Tox, until the profile or the network
// settings change.
type session struct {
	core          *core
	done, stopped chan struct{}
}

func (a *home) profilesDir() string { return filepath.Join(a.base, "profiles") }

// dir is where a profile is.
func (a *home) dir(name string) string {
	if a.fixed != "" {
		return a.fixed
	}
	return filepath.Join(a.profilesDir(), name)
}

// A profileInfo is a profile as the start page shows it.
type profileInfo struct {
	name      string
	encrypted bool
}

// profiles are the app's profiles, by name.
func (a *home) profiles() []profileInfo {
	if a.fixed != "" {
		return []profileInfo{{filepath.Base(a.fixed), profileEncrypted(a.fixed)}}
	}
	entries, err := os.ReadDir(a.profilesDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Print(err)
	}
	var ps []profileInfo
	for _, e := range entries {
		if e.IsDir() {
			ps = append(ps, profileInfo{e.Name(), profileEncrypted(a.dir(e.Name()))})
		}
	}
	slices.SortFunc(ps, func(x, y profileInfo) int { return strings.Compare(x.name, y.name) })
	return ps
}

func (a *home) lastPath() string { return filepath.Join(a.base, "last-profile") }

// openAll starts every profile without a password, and shows the one used
// last. With none at all, it makes one. Profiles with a password wait to be
// unlocked.
func (a *home) openAll() error {
	ps := a.profiles()
	if len(ps) == 0 {
		return a.open("Tox", "")
	}
	for _, p := range ps {
		if !p.encrypted {
			if err := a.start(p.name, ""); err != nil {
				return err
			}
		}
	}
	if last, err := os.ReadFile(a.lastPath()); err == nil && a.sessions[string(last)] != nil {
		a.shown = string(last)
	} else if names := a.running(); len(names) > 0 {
		a.shown = names[0]
	}
	return nil
}

// running are the running profiles' names, in order.
func (a *home) running() []string {
	names := make([]string, 0, len(a.sessions))
	for name := range a.sessions {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// show makes the UI show a running profile.
func (a *home) show(name string) {
	if a.sessions[name] == nil {
		return
	}
	a.shown = name
	if a.fixed == "" {
		if err := writeFile(a.lastPath(), []byte(name)); err != nil {
			log.Print(err)
		}
	}
}

// caller is the running profile with the call, or "".
func (a *home) caller() string {
	c := a.line.caller()
	for name, s := range a.sessions {
		if s.core == c {
			return name
		}
	}
	return ""
}

// checkName says whether a name can be a new profile's.
func (a *home) checkName(name string) error {
	switch {
	case name == "" || name == "." || name == "..":
		return errors.New("give it a name")
	case strings.ContainsAny(name, `/\`):
		return errors.New("a name can't have / or \\ in it")
	}
	if _, err := os.Stat(a.dir(name)); err == nil {
		return errors.New("there's a profile with that name")
	}
	return nil
}

// open starts a profile, unless it runs, and shows it.
func (a *home) open(name, password string) error {
	if a.sessions[name] == nil {
		if err := a.start(name, password); err != nil {
			return err
		}
	}
	a.show(name)
	return nil
}

// start runs a profile's Tox.
func (a *home) start(name, password string) error {
	nodes := a.nodes
	if nodes == nil {
		nodes = builtInNodes
	}
	c, err := newCore(a.dir(name), nodes, a.invalidate, password)
	if err != nil {
		return err
	}
	c.media = newMedia()
	c.line = &a.line
	c.alert = func(al alert) {
		tag := alertTag(name, al.key)
		if al.text == "" {
			a.plat.cancel(tag)
		} else {
			a.plat.notify(tag, al.title, al.text, chatURL(name, al.key))
		}
	}
	// Staying connected is the app's: a profile's setting follows the
	// others'.
	if len(a.sessions) > 0 && a.staysConnected() != c.settings.Background {
		set := c.settings
		set.Background = a.staysConnected()
		if err := set.save(c.dir); err != nil {
			log.Print(err)
		}
		c.settings = set
	}
	s := &session{c, make(chan struct{}), make(chan struct{})}
	go func() {
		defer close(s.stopped)
		c.run(s.done)
	}()
	if a.sessions == nil {
		a.sessions, a.passwords = map[string]*session{}, map[string]string{}
	}
	a.sessions[name], a.passwords[name] = s, password
	a.plat.stayConnected(c.settings.Background)
	return nil
}

// staysConnected is whether the running profiles stay connected while the
// app is closed: they all have the same setting.
func (a *home) staysConnected() bool {
	for _, s := range a.sessions {
		return s.core.settings.Background
	}
	return false
}

// alertTag is a notification's tag: what it's about, in a profile.
func alertTag(profile string, key chatID) string { return profile + ":" + key.String() }

// stop stops a running profile.
func (a *home) stop(name string) {
	s := a.sessions[name]
	if s == nil {
		return
	}
	close(s.done)
	<-s.stopped
	s.core.close()
	delete(a.sessions, name)
	delete(a.passwords, name)
	if a.shown == name {
		a.shown = ""
		if names := a.running(); len(names) > 0 {
			a.show(names[0])
		}
	}
}

// close stops every running profile.
func (a *home) close() {
	for _, name := range a.running() {
		a.stop(name)
	}
}

// core is the shown profile's, or nil.
func (a *home) core() *core {
	if s := a.sessions[a.shown]; s != nil {
		return s.core
	}
	return nil
}

// create makes a profile and opens it.
func (a *home) create(name string) error {
	name = strings.TrimSpace(name)
	if err := a.checkName(name); err != nil {
		return err
	}
	return a.open(name, "")
}

// importProfile makes a profile from another app's profile.tox. One with a
// password opens with it.
func (a *home) importProfile(name string, data []byte, password string) error {
	name = strings.TrimSpace(name)
	if err := a.checkName(name); err != nil {
		return err
	}
	if toxencryptsave.IsDataEncrypted(data) {
		if password == "" {
			return errLocked
		}
		key, _, err := decryptProfile(data, password)
		if err != nil {
			return err
		}
		key.Close()
	}
	dir := a.dir(name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := writeFile(profilePath(dir), data); err != nil {
		return err
	}
	return a.open(name, password)
}

// apply saves the shown profile's new settings, and starts its Tox again if
// they need it. Staying connected goes to every profile.
func (a *home) apply(set settings) error {
	c := a.core()
	if c == nil {
		return errors.New("no profile is open")
	}
	if set.Background != c.settings.Background {
		for _, name := range a.running() {
			other := a.sessions[name].core
			if other == c {
				continue
			}
			oset := other.settings
			oset.Background = set.Background
			if err := oset.save(other.dir); err != nil {
				return err
			}
			other.change(func() { other.settings = oset })
		}
		a.plat.stayConnected(set.Background)
	}
	old := c.settings
	if err := set.save(c.dir); err != nil {
		return err
	}
	if set.network() == old.network() {
		c.change(func() { c.settings = set })
		return nil
	}
	name, password := a.shown, a.passwords[a.shown]
	a.stop(name)
	if err := a.open(name, password); err != nil {
		// Back to what worked.
		if saveErr := old.save(c.dir); saveErr != nil {
			log.Print(saveErr)
		}
		if err2 := a.open(name, password); err2 != nil {
			log.Fatal(err2)
		}
		return err
	}
	return nil
}

// setPassword changes the running profile's password.
func (a *home) setPassword(password string) error {
	c := a.core()
	if c == nil {
		return errors.New("no profile is open")
	}
	if err := c.setPassword(password); err != nil {
		return err
	}
	a.passwords[a.shown] = password
	return nil
}

// deleteProfile stops a profile and deletes it with its chats. The last
// profile stays, as does one in a call.
func (a *home) deleteProfile(name string) error {
	switch {
	case len(a.profiles()) == 1:
		return errors.New("it's the only profile")
	case a.caller() == name:
		return errors.New("it's in a call")
	}
	if s := a.sessions[name]; s != nil {
		for _, ch := range s.core.snapshot(nil).chats {
			a.plat.cancel(alertTag(name, ch.id))
		}
	}
	a.stop(name)
	return os.RemoveAll(a.dir(name))
}
