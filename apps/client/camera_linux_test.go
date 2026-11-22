//go:build !android

package main

import (
	"bytes"
	"testing"
)

func TestYUYVToI420(t *testing.T) {
	// 4x2 pixels, rows padded to 10 bytes: Y0 U Y1 V for each pair.
	src := []byte{
		1, 10, 2, 20, 3, 30, 4, 40, 0, 0,
		5, 12, 6, 22, 7, 33, 8, 44, 0, 0,
	}
	y, u, v := yuyvToI420(src, 4, 2, 10, nil, nil, nil)
	if want := []byte{1, 2, 3, 4, 5, 6, 7, 8}; !bytes.Equal(y, want) {
		t.Errorf("Y %v, want %v", y, want)
	}
	if want := []byte{11, 32}; !bytes.Equal(u, want) {
		t.Errorf("U %v, want %v", u, want)
	}
	if want := []byte{21, 42}; !bytes.Equal(v, want) {
		t.Errorf("V %v, want %v", v, want)
	}
}
