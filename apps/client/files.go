package main

import (
	"crypto/rand"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
)

// A transfer is a file going to or from a friend in this session: toxcore
// forgets them when the friend goes offline.
type transfer struct {
	// msg is the file's message, or 0 for an avatar.
	msg      int64
	incoming bool
	// f is the file on disk while it goes. It is nil for a file the user
	// hasn't accepted or resumed.
	f *os.File
	// avatar is an avatar coming in, or ours going out.
	avatar []byte
}

func (c *core) filesDir() string { return filepath.Join(c.dir, "files") }

// filePath is where a file in a chat is: a received one as it comes in, a sent
// one as the app copied it.
func (c *core) filePath(msg int64) string {
	return filepath.Join(c.filesDir(), strconv.FormatInt(msg, 10))
}

// sendFile sends a file to a friend, or keeps it until they're online. It
// copies r first, so the file can go again after a restart.
func (c *core) sendFile(id chatID, name string, r io.Reader) error {
	tmp, err := os.CreateTemp(c.filesDir(), "new-")
	if err != nil {
		return err
	}
	size, err := io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp.Name())
		return err
	}
	f := &file{name: name, size: uint64(size), state: fileActive}
	rand.Read(f.id[:])
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.friend == nil {
			err = errors.New("they're not a friend any more")
			return
		}
		fr := ch.friend
		m, ok := c.add(fr.chat, message{time: time.Now(), outgoing: true, file: f})
		if !ok {
			err = errors.New("couldn't store the file")
			return
		}
		if err = os.Rename(tmp.Name(), c.filePath(m.id)); err != nil {
			c.setFileState(fr, m.id, fileCancelled)
			return
		}
		if fr.connection != tox.ConnectionNone {
			c.startFile(fr, m)
		}
	})
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// startFile offers the friend a file we send. The caller holds c.mu.
func (c *core) startFile(fr *friend, m message) {
	f, err := os.Open(c.filePath(m.id))
	if err != nil {
		log.Print(err)
		c.setFileState(fr, m.id, fileCancelled)
		return
	}
	tf, err := fr.FileSend(tox.FileKindData, m.file.size, &m.file.id, m.file.name)
	if err != nil {
		// Not connected after all: the next connection tries again.
		f.Close()
		return
	}
	fr.transfers[tf.Number()] = &transfer{msg: m.id, f: f}
}

// startFiles offers the friend the files we send that didn't finish, when
// they come online. The caller holds c.mu.
func (c *core) startFiles(fr *friend) {
	ms, err := c.store.files(fr.chat.id, fileActive)
	if err != nil {
		log.Print(err)
		return
	}
	for _, m := range ms {
		if m.outgoing {
			c.startFile(fr, m)
		}
	}
}

// dropTransfers forgets the transfers toxcore forgot when the friend went
// offline. Unfinished files stay active, to go on when the friend is back.
// The caller holds c.mu.
func (c *core) dropTransfers(fr *friend) {
	for n, t := range fr.transfers {
		if t.f != nil {
			t.f.Close()
		}
		delete(fr.transfers, n)
	}
}

// fileOffered handles a friend offering a file. The caller holds c.mu.
func (c *core) fileOffered(fr *friend, ev *tox.EventFileRecv) {
	if ev.Kind == tox.FileKindAvatar {
		c.avatarOffered(fr, ev)
		return
	}
	if ev.Kind != tox.FileKindData {
		ev.File.Control(tox.FileControlCancel)
		return
	}
	id, err := ev.File.FileID()
	if err != nil {
		log.Print(err)
		return
	}
	n := ev.File.Number()
	// The friend offers it again after a disconnection or a restart: it goes
	// on from where it got to.
	if m, ok, err := c.store.fileByID(fr.chat.id, id); err != nil {
		log.Print(err)
	} else if ok && m.file.state != fileDone && m.file.state != fileCancelled {
		t := &transfer{msg: m.id, incoming: true}
		fr.transfers[n] = t
		if m.file.state == fileActive {
			c.receive(fr, n, t)
		}
		return
	}
	m, ok := c.add(fr.chat, message{
		time: time.Now(),
		file: &file{id: id, name: ev.Filename, size: ev.FileSize, state: fileOffered},
	})
	if ok {
		fr.transfers[n] = &transfer{msg: m.id, incoming: true}
	}
}

// receive starts or resumes receiving a file. The caller holds c.mu.
func (c *core) receive(fr *friend, n tox.FileNumber, t *transfer) {
	f, err := os.OpenFile(c.filePath(t.msg), os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		log.Print(err)
		return
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		log.Print(err)
		return
	}
	tf := fr.File(n)
	if have := uint64(st.Size()); have != 0 {
		if err := tf.Seek(have); err != nil {
			log.Printf("resuming %s: %v", tf, err)
		}
		c.progress[t.msg] = have
	}
	if err := tf.Control(tox.FileControlResume); err != nil {
		f.Close()
		log.Print(err)
		return
	}
	t.f = f
}

// fileChunk handles data of a file we receive. The caller holds c.mu.
func (c *core) fileChunk(fr *friend, ev *tox.EventFileRecvChunk) {
	n := ev.File.Number()
	t := fr.transfers[n]
	switch {
	case t == nil:
	case t.msg == 0:
		c.avatarChunk(fr, n, t, ev)
	case len(ev.Data) == 0:
		t.f.Close()
		delete(fr.transfers, n)
		delete(c.progress, t.msg)
		c.setFileState(fr, t.msg, fileDone)
	case t.f != nil:
		if _, err := t.f.WriteAt(ev.Data, int64(ev.Position)); err != nil {
			log.Print(err)
			c.cancelTransfer(fr, n, t)
			return
		}
		c.progress[t.msg] = ev.Position + uint64(len(ev.Data))
	}
}

// fileChunkRequest sends data of a file. The caller holds c.mu.
func (c *core) fileChunkRequest(fr *friend, ev *tox.EventFileChunkRequest) {
	n := ev.File.Number()
	t := fr.transfers[n]
	switch {
	case t == nil:
	case ev.Length == 0:
		if t.f != nil {
			t.f.Close()
		}
		delete(fr.transfers, n)
		if t.msg != 0 {
			delete(c.progress, t.msg)
			c.setFileState(fr, t.msg, fileDone)
		}
	default:
		data := make([]byte, ev.Length)
		var got int
		var err error
		if t.msg == 0 {
			got = copy(data, t.avatar[min(int(ev.Position), len(t.avatar)):])
		} else {
			got, err = t.f.ReadAt(data, int64(ev.Position))
			if err != nil && !errors.Is(err, io.EOF) {
				log.Print(err)
				c.cancelTransfer(fr, n, t)
				return
			}
			c.progress[t.msg] = ev.Position + uint64(got)
		}
		if err := ev.File.SendChunk(ev.Position, data[:got]); err != nil {
			log.Printf("sending %s: %v", ev.File, err)
		}
	}
}

// fileControl handles the friend pausing, resuming or cancelling a file. The
// caller holds c.mu.
func (c *core) fileControl(fr *friend, ev *tox.EventFileRecvControl) {
	n := ev.File.Number()
	t := fr.transfers[n]
	if t == nil || ev.Control != tox.FileControlCancel {
		return
	}
	if t.f != nil {
		t.f.Close()
	}
	delete(fr.transfers, n)
	if t.msg != 0 {
		delete(c.progress, t.msg)
		c.setFileState(fr, t.msg, fileCancelled)
	}
}

// cancelTransfer cancels a transfer. The caller holds c.mu.
func (c *core) cancelTransfer(fr *friend, n tox.FileNumber, t *transfer) {
	if err := fr.File(n).Control(tox.FileControlCancel); err != nil {
		log.Print(err)
	}
	if t.f != nil {
		t.f.Close()
	}
	delete(fr.transfers, n)
	if t.msg != 0 {
		delete(c.progress, t.msg)
		c.setFileState(fr, t.msg, fileCancelled)
	}
}

// transferOf returns the transfer of a file message in this session.
func (fr *friend) transferOf(msg int64) (tox.FileNumber, *transfer) {
	for n, t := range fr.transfers {
		if t.msg == msg {
			return n, t
		}
	}
	return 0, nil
}

// A fileAction is what the user does with a file in a chat.
type fileAction int

const (
	fileAccept fileAction = iota
	filePause
	fileResume
	fileCancel
)

// fileDo does what the user asked with a file.
func (c *core) fileDo(id chatID, msg int64, action fileAction) {
	c.change(func() {
		ch := c.chats[id]
		if ch == nil || ch.friend == nil {
			return
		}
		fr := ch.friend
		n, t := fr.transferOf(msg)
		switch action {
		case fileAccept, fileResume:
			c.setFileState(fr, msg, fileActive)
			switch {
			case t == nil:
				// It goes on when the friend offers it again, or, if we send
				// it, when we offer it again.
				if m, ok, _ := c.store.message(id, msg); ok && m.outgoing && fr.online() {
					c.startFile(fr, m)
				}
			case t.incoming && t.f == nil:
				c.receive(fr, n, t)
			default:
				if err := fr.File(n).Control(tox.FileControlResume); err != nil {
					log.Print(err)
				}
			}
		case filePause:
			c.setFileState(fr, msg, filePaused)
			if t != nil && (t.f != nil || !t.incoming) {
				if err := fr.File(n).Control(tox.FileControlPause); err != nil {
					log.Print(err)
				}
			}
		case fileCancel:
			if t != nil {
				c.cancelTransfer(fr, n, t)
			}
			c.setFileState(fr, msg, fileCancelled)
			c.removeFile(msg)
		}
	})
}

// removeFile deletes a file in a chat from the disk. The caller holds c.mu.
func (c *core) removeFile(msg int64) {
	delete(c.progress, msg)
	if err := os.Remove(c.filePath(msg)); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Print(err)
	}
	if err := os.RemoveAll(c.namedDir(msg)); err != nil {
		log.Print(err)
	}
}

func (c *core) namedDir(msg int64) string {
	return filepath.Join(c.filesDir(), "named", strconv.FormatInt(msg, 10))
}

// namedFile returns the path of a link to a file in a chat, under its name in
// a directory of its own: the name tells other apps what kind of file it is.
func (c *core) namedFile(msg int64, name string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	dir := c.namedDir(msg)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	link := filepath.Join(dir, safeFileName(name))
	if err := os.Link(c.filePath(msg), link); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	return link, nil
}

// safeFileName is a name from a friend, as the name of a file in a directory.
func safeFileName(name string) string {
	name = filepath.Base(name)
	if name == "." || name == ".." || name == string(filepath.Separator) {
		return "file"
	}
	return name
}

// setFileState records a file's new state, and shows it. The caller holds
// c.mu.
func (c *core) setFileState(fr *friend, msg int64, state fileState) {
	if err := c.store.setFileState(msg, state); err != nil {
		log.Print(err)
	}
	c.update(fr.chat, msg, func(m *message) {
		if m.file != nil {
			f := *m.file
			f.state = state
			m.file = &f
		}
	})
}
