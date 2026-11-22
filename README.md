## go-toxcore-c

A Go binding to [c-toxcore](https://github.com/TokTok/c-toxcore).

### Installation

Install c-toxcore (see its
[INSTALL.md](https://github.com/TokTok/c-toxcore/blob/master/INSTALL.md)), then:

    go get github.com/TokTok/go-toxcore-c

### Example

An echo bot:

```go
package main

import (
	"log"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
)

func main() {
	t, err := tox.New(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer t.Close()
	log.Print("address: ", t.Address())

	for {
		events, err := t.Iterate()
		if err != nil {
			log.Print(err)
		}
		for _, ev := range events {
			switch ev := ev.(type) {
			case *tox.EventFriendRequest:
				t.FriendAddNoRequest(ev.PublicKey)
			case *tox.EventFriendMessage:
				ev.Friend.SendMessage(ev.Type, ev.Message)
			}
		}
		time.Sleep(t.IterationInterval())
	}
}
```

It still needs to bootstrap from a node on the network with `t.Bootstrap`.

Audio and video calls are in package `toxav`, and encrypting saved profiles
with a passphrase in package `toxencryptsave`.

### Tests

    bazel test //go-toxcore-c/...
