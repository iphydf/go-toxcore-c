package main

import (
	"os"
	"slices"
	"testing"
)

func TestProfilesRunAtOnce(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	base := t.TempDir()
	newHome := func() *home {
		h := &home{base: base, plat: &platform{}, invalidate: func() {}, nodes: nodes}
		t.Cleanup(h.close)
		return h
	}

	h := newHome()
	for _, name := range []string{"a", "b", "c"} {
		if err := h.open(name, ""); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.running(); !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("running %v", got)
	}
	if err := h.setPassword("secret"); err != nil {
		t.Fatal(err)
	}
	h.close()

	// The one with a password waits to be opened with it.
	h = newHome()
	if err := h.openAll(); err != nil {
		t.Fatal(err)
	}
	if got := h.running(); !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("running %v", got)
	}
	if h.shown != "a" {
		t.Errorf("showing %q", h.shown)
	}
	if err := h.open("c", "secret"); err != nil {
		t.Fatal(err)
	}

	if err := h.deleteProfile("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(h.dir("a")); !os.IsNotExist(err) {
		t.Errorf("a is still there: %v", err)
	}
	if err := h.deleteProfile("b"); err != nil {
		t.Fatal(err)
	}
	if err := h.deleteProfile("c"); err == nil {
		t.Error("the last profile went")
	}
}

func TestStayingConnectedIsEveryProfiles(t *testing.T) {
	t.Parallel()
	nodes := network(t)
	h := &home{base: t.TempDir(), plat: &platform{}, invalidate: func() {}, nodes: nodes}
	t.Cleanup(h.close)
	if err := h.open("a", ""); err != nil {
		t.Fatal(err)
	}
	set := h.core().settings
	set.Background = !set.Background
	if err := h.apply(set); err != nil {
		t.Fatal(err)
	}
	// A profile added later stays connected as the others do.
	if err := h.open("b", ""); err != nil {
		t.Fatal(err)
	}
	if got := h.sessions["b"].core.settings.Background; got != set.Background {
		t.Errorf("b stays connected: %v", got)
	}
}
