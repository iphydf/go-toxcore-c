package main

import (
	"math"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
)

func TestStorePages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chats.db")
	s, err := openStore(path, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	var alice, bob tox.PublicKey
	bob[0] = 1
	for i := range 120 {
		for _, k := range []tox.PublicKey{alice, bob} {
			if err := s.add(friendChat(k), &message{time: time.Now(), text: strconv.Itoa(i)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	s.close()

	// What's stored stays.
	if s, err = openStore(path, [32]byte{1}); err != nil {
		t.Fatal(err)
	}
	defer s.close()
	var texts []string
	for before := int64(math.MaxInt64); ; {
		page, err := s.messages(friendChat(bob), before, pageSize)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		for i := len(page) - 1; i >= 0; i-- {
			texts = append(texts, page[i].text)
		}
		before = page[0].id
	}
	if len(texts) != 120 {
		t.Fatalf("%d messages, want 120", len(texts))
	}
	for i, text := range texts {
		if want := strconv.Itoa(119 - i); text != want {
			t.Fatalf("message %d from the end is %q, want %q", i, text, want)
		}
	}
}
