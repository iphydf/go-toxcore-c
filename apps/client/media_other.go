//go:build !linux

package main

func newMedia() media { return noMedia{} }
