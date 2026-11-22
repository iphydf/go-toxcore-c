package main

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"

	tox "github.com/TokTok/go-toxcore-c"
	"golang.org/x/image/draw"
)

const (
	// avatarSize is the width and height of the avatar we send, in pixels.
	avatarSize = 128
	// maxAvatarSize is the most an avatar a friend sends can be, in bytes.
	maxAvatarSize = 64 << 10
)

// An avatar is a picture of someone.
type avatar struct {
	data  []byte
	hash  tox.Hash
	image image.Image
}

// decodeAvatar reads an avatar, or returns nil if it isn't a picture.
func decodeAvatar(data []byte) *avatar {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return &avatar{data: data, hash: tox.HashData(data), image: img}
}

func (c *core) avatarPath() string { return filepath.Join(c.dir, "avatar.png") }

func (c *core) friendAvatarPath(key tox.PublicKey) string {
	return filepath.Join(c.dir, "avatars", key.String())
}

// loadAvatar reads an avatar from a file, if there is one.
func loadAvatar(path string) *avatar {
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Print(err)
		}
		return nil
	}
	return decodeAvatar(data)
}

// makeAvatar makes our avatar from a picture: its middle square, small.
func makeAvatar(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, errors.New("that's not a picture the app can read")
	}
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	square := image.Rect(0, 0, side, side).Add(b.Min).Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))
	small := image.NewNRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	draw.CatmullRom.Scale(small, small.Bounds(), img, square, draw.Src, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, small); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// setAvatar makes a picture our avatar, or with nil takes ours away, and
// tells the friends who are online.
func (c *core) setAvatar(picture []byte) error {
	var a *avatar
	if picture != nil {
		data, err := makeAvatar(picture)
		if err != nil {
			return err
		}
		if a = decodeAvatar(data); a == nil {
			return errors.New("couldn't make the avatar")
		}
		if err := writeFile(c.avatarPath(), data); err != nil {
			return err
		}
	} else if err := os.Remove(c.avatarPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	c.change(func() {
		c.avatar = a
		for _, fr := range c.friends {
			if fr.connection != tox.ConnectionNone {
				c.sendAvatar(fr, true)
			}
		}
	})
	return nil
}

// sendAvatar offers the friend our avatar. With changed, it also tells them
// when we have none. The caller holds c.mu.
func (c *core) sendAvatar(fr *friend, changed bool) {
	if c.avatar == nil {
		if changed {
			if _, err := fr.FileSend(tox.FileKindAvatar, 0, nil, ""); err != nil {
				log.Print(err)
			}
		}
		return
	}
	// The ID is the hash: friends who have the avatar decline it.
	id := tox.FileID(c.avatar.hash)
	tf, err := fr.FileSend(tox.FileKindAvatar, uint64(len(c.avatar.data)), &id, "avatar.png")
	if err != nil {
		log.Print(err)
		return
	}
	fr.transfers[tf.Number()] = &transfer{avatar: c.avatar.data}
}

// avatarOffered handles a friend offering their avatar. The caller holds
// c.mu.
func (c *core) avatarOffered(fr *friend, ev *tox.EventFileRecv) {
	cancel := func() {
		if err := ev.File.Control(tox.FileControlCancel); err != nil {
			log.Print(err)
		}
	}
	if ev.FileSize == 0 {
		cancel()
		fr.avatar = nil
		if err := os.Remove(c.friendAvatarPath(fr.key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Print(err)
		}
		return
	}
	id, err := ev.File.FileID()
	if err != nil || ev.FileSize > maxAvatarSize || fr.avatar != nil && tox.Hash(id) == fr.avatar.hash {
		cancel()
		return
	}
	if err := ev.File.Control(tox.FileControlResume); err != nil {
		log.Print(err)
		return
	}
	fr.transfers[ev.File.Number()] = &transfer{incoming: true, avatar: make([]byte, ev.FileSize)}
}

// avatarChunk handles data of a friend's avatar. The caller holds c.mu.
func (c *core) avatarChunk(fr *friend, n tox.FileNumber, t *transfer, ev *tox.EventFileRecvChunk) {
	if len(ev.Data) != 0 {
		if ev.Position+uint64(len(ev.Data)) > uint64(len(t.avatar)) {
			c.cancelTransfer(fr, n, t)
			return
		}
		copy(t.avatar[ev.Position:], ev.Data)
		return
	}
	delete(fr.transfers, n)
	a := decodeAvatar(t.avatar)
	if a == nil {
		log.Printf("%s sent an avatar that isn't a picture", fr.title())
		return
	}
	if err := writeFile(c.friendAvatarPath(fr.key), a.data); err != nil {
		log.Print(err)
	}
	fr.avatar = a
}
