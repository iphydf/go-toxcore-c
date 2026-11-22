// Package toxtest runs Tox instances for tests, on a network of their own on
// this machine.
package toxtest

import (
	"errors"
	"net"
	"testing"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
)

// New starts an instance that the test closes when it ends.
func New(t testing.TB, opts *tox.Options) *tox.Tox {
	t.Helper()
	if opts == nil {
		opts = tox.DefaultOptions()
	}
	opts.IPv6Enabled = false
	opts.LocalDiscoveryEnabled = false
	opts.LogCallback = func(level tox.LogLevel, file string, line uint32, function, message string) {
		if level >= tox.LogLevelWarning {
			t.Logf("%s:%d: %s: %s", file, line, level, message)
		}
	}
	x, err := tox.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(x.Close)
	return x
}

// newRelayTox starts the relay's instance on a free TCP port.
func newRelayTox(t testing.TB) (*tox.Tox, tox.Port) {
	for tries := 1; ; tries++ {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		opts := tox.DefaultOptions()
		opts.IPv6Enabled = false
		opts.LocalDiscoveryEnabled = false
		opts.TCPPort = tox.Port(l.Addr().(*net.TCPAddr).Port)
		l.Close()
		x, err := tox.New(opts)
		// Between closing the listener and toxcore taking the port, another
		// connection can take it.
		if errors.Is(err, tox.ErrNewPortAlloc) && tries < 10 {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(x.Close)
		return x, opts.TCPPort
	}
}

// A Relay is an instance for others to bootstrap from, which is also their TCP
// relay. Group chats need one.
type Relay struct {
	Port    tox.Port
	TCPPort tox.Port
	DhtID   tox.DhtID
}

// NewRelay starts a relay that runs until the test ends.
func NewRelay(t testing.TB) Relay {
	x, tcpPort := newRelayTox(t)
	port, err := x.UDPPort()
	if err != nil {
		t.Fatal(err)
	}
	// Cleanups run last first, so this stops the relay before it is closed.
	done, stopped := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() {
		close(done)
		<-stopped
	})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			case <-time.After(x.IterationInterval()):
			}
			if _, err := x.Iterate(); err != nil {
				t.Error(err)
			}
		}
	}()
	return Relay{port, tcpPort, x.DhtID()}
}

// Join bootstraps an instance from the relay.
func (r Relay) Join(t testing.TB, x *tox.Tox) {
	t.Helper()
	if err := x.Bootstrap("127.0.0.1", r.Port, r.DhtID); err != nil {
		t.Fatal(err)
	}
	if err := x.AddTCPRelay("127.0.0.1", r.TCPPort, r.DhtID); err != nil {
		t.Fatal(err)
	}
}

// Pair returns two instances on a network of their own, around a relay.
func Pair(t testing.TB) (a, b *tox.Tox) {
	r := NewRelay(t)
	a, b = New(t, nil), New(t, nil)
	r.Join(t, a)
	r.Join(t, b)
	return a, b
}

// Until calls step every 10 ms until it reports done.
func Until(t testing.TB, step func() (done bool)) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if step() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out")
}

// Run iterates the instances, giving each event to handle, until handle
// reports done.
func Run(t testing.TB, toxes []*tox.Tox, handle func(x *tox.Tox, ev tox.Event) (done bool)) {
	t.Helper()
	Until(t, func() bool {
		return Iterate(t, toxes, handle)
	})
}

// Iterate iterates the instances once, giving each event to handle, and
// reports whether handle reported done.
func Iterate(t testing.TB, toxes []*tox.Tox, handle func(x *tox.Tox, ev tox.Event) (done bool)) bool {
	t.Helper()
	for _, x := range toxes {
		events, err := x.Iterate()
		if err != nil {
			t.Fatal(err)
		}
		for _, ev := range events {
			if handle(x, ev) {
				return true
			}
		}
	}
	return false
}

// Friends returns two instances that are friends and connected: fa is b in
// a's friend list, fb is a in b's.
func Friends(t testing.TB) (a, b *tox.Tox, fa, fb tox.Friend) {
	a, b = Pair(t)
	fa, err := a.FriendAdd(b.Address(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	online := 0
	Run(t, []*tox.Tox{a, b}, func(x *tox.Tox, ev tox.Event) bool {
		switch ev := ev.(type) {
		case *tox.EventFriendRequest:
			if ev.PublicKey != a.PublicKey() || ev.Message != "hi" {
				t.Errorf("request from %v: %q", ev.PublicKey, ev.Message)
			}
			if fb, err = b.FriendAddNoRequest(ev.PublicKey); err != nil {
				t.Fatal(err)
			}
		case *tox.EventFriendConnectionStatus:
			if ev.ConnectionStatus != tox.ConnectionNone {
				online++
			}
		}
		return online == 2
	})
	return a, b, fa, fb
}
