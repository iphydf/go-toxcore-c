package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"slices"
	"time"

	tox "github.com/TokTok/go-toxcore-c"
	_ "github.com/mutecomm/go-sqlcipher/v4"
)

// migrations make the history database, each from the one before it. The
// database's user_version is how many it had. A change to the schema is a new
// one at the end.
var migrations = []string{
	`-- chat is a chatID's bytes: what kind of chat, and whose.
	CREATE TABLE chats (
		chat BLOB PRIMARY KEY,
		-- read is the newest message the user saw.
		read INTEGER NOT NULL DEFAULT 0,
		-- nickname is the user's name for a friend, if they gave one.
		nickname TEXT NOT NULL DEFAULT '',
		-- name is a group peer's name, for a private chat with them.
		name TEXT NOT NULL DEFAULT ''
	);
	CREATE TABLE messages (
		id INTEGER PRIMARY KEY,
		chat BLOB NOT NULL,
		-- Unix time in milliseconds.
		time INTEGER NOT NULL,
		outgoing INTEGER NOT NULL,
		action INTEGER NOT NULL,
		text TEXT NOT NULL,
		-- Set when the friend's read receipt arrives, or a group took it.
		delivered INTEGER NOT NULL DEFAULT 0,
		-- In groups and conferences, who sent it, and their name then.
		sender BLOB,
		sender_name TEXT NOT NULL DEFAULT '',
		-- A notice is about the chat, not from someone: a new topic, say.
		notice INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX messages_by_chat ON messages (chat, id);
	-- A message that is a file. The file is files/MESSAGE in the profile.
	CREATE TABLE files (
		message INTEGER PRIMARY KEY REFERENCES messages (id) ON DELETE CASCADE,
		-- toxcore's ID for the transfer, which a transfer resumes with.
		file_id BLOB NOT NULL,
		name TEXT NOT NULL,
		size INTEGER NOT NULL,
		state INTEGER NOT NULL
	);
	-- A message that is an invitation to a group or a conference.
	CREATE TABLE invites (
		message INTEGER PRIMARY KEY REFERENCES messages (id) ON DELETE CASCADE,
		kind INTEGER NOT NULL,
		data BLOB NOT NULL,
		name TEXT NOT NULL,
		joined INTEGER NOT NULL DEFAULT 0
	);
	-- Friend requests the user hasn't answered.
	CREATE TABLE requests (
		key BLOB PRIMARY KEY,
		message TEXT NOT NULL
	);`,
}

// A message is one message in a chat.
type message struct {
	id        int64
	time      time.Time
	outgoing  bool
	action    bool
	text      string
	delivered bool
	// sender is who sent a message in a group or a conference, and senderName
	// their name then.
	sender     *tox.PublicKey
	senderName string
	// notice is set on a message about the chat: a new topic, say.
	notice bool
	// file is set on a file.
	file *file
	// invite is set on an invitation.
	invite *invite

	// sent is set when toxcore took the message in this session. Only views
	// set it.
	sent bool
}

// A file is a file in a chat, sent or received.
type file struct {
	id    tox.FileID
	name  string
	size  uint64
	state fileState
	// done is how much of it went: views set it.
	done uint64
}

type fileState int

const (
	// fileOffered is a file the friend offers, waiting for the user.
	fileOffered fileState = iota
	// fileActive is a file going, or waiting for the friend to be online.
	fileActive
	// filePaused is a file the user paused.
	filePaused
	fileDone
	// fileCancelled is a file either side cancelled or declined.
	fileCancelled
)

// An invite is a friend's invitation to a group or a conference.
type invite struct {
	kind   inviteKind
	data   []byte
	name   string
	joined bool
}

type inviteKind int

const (
	inviteGroup inviteKind = iota
	inviteConference
	// inviteAVConference is a conference with calls, which the app can't
	// join.
	inviteAVConference
)

// A store keeps the chats.
type store struct{ db *sql.DB }

// openStore opens the database at path, which SQLCipher encrypts with key.
func openStore(path string, key [32]byte) (*store, error) {
	// SQLite gives its other files the database's permissions.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		return nil, err
	}
	f.Close()
	// A raw key: SQLCipher derives none from it.
	pragmaKey := url.QueryEscape(fmt.Sprintf("x'%X'", key[:]))
	db, err := sql.Open("sqlite3", "file:"+path+"?_pragma_key="+pragmaKey+"&_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	// One connection: SQLite takes one writer, and the core calls it under its
	// lock anyway.
	db.SetMaxOpenConns(1)
	s := &store{db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

func (s *store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > len(migrations) {
		return errors.New("a newer Go Tox made this database")
	}
	for ; version < len(migrations); version++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.Exec(migrations[version]); err != nil {
			return err
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version+1)); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *store) close() error { return s.db.Close() }

// add stores a new message and sets its id.
func (s *store) add(chat chatID, m *message) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sender []byte
	if m.sender != nil {
		sender = m.sender[:]
	}
	r, err := tx.Exec(`INSERT INTO messages (chat, time, outgoing, action, text, delivered, sender, sender_name, notice)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		chat.bytes(), m.time.UnixMilli(), m.outgoing, m.action, m.text, m.delivered, sender, m.senderName, m.notice)
	if err != nil {
		return err
	}
	id, err := r.LastInsertId()
	if err != nil {
		return err
	}
	if f := m.file; f != nil {
		if _, err := tx.Exec("INSERT INTO files (message, file_id, name, size, state) VALUES (?, ?, ?, ?, ?)",
			id, f.id[:], f.name, int64(f.size), f.state); err != nil {
			return err
		}
	}
	if i := m.invite; i != nil {
		if _, err := tx.Exec("INSERT INTO invites (message, kind, data, name) VALUES (?, ?, ?, ?)",
			id, i.kind, i.data, i.name); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	m.id = id
	return nil
}

func (s *store) setDelivered(id int64) error {
	_, err := s.db.Exec("UPDATE messages SET delivered = 1 WHERE id = ?", id)
	return err
}

func (s *store) setFileState(message int64, state fileState) error {
	_, err := s.db.Exec("UPDATE files SET state = ? WHERE message = ?", state, message)
	return err
}

func (s *store) setJoined(message int64) error {
	_, err := s.db.Exec("UPDATE invites SET joined = 1 WHERE message = ?", message)
	return err
}

// files returns a chat's files in a state, oldest first.
func (s *store) files(chat chatID, state fileState) ([]message, error) {
	return s.query("WHERE chat = ? AND files.state = ? ORDER BY id", chat.bytes(), state)
}

// fileMessages returns the ids of all the chat's files.
func (s *store) fileMessages(chat chatID) ([]int64, error) {
	ms, err := s.query("WHERE chat = ? AND files.message IS NOT NULL", chat.bytes())
	ids := make([]int64, len(ms))
	for i, m := range ms {
		ids[i] = m.id
	}
	return ids, err
}

// fileByID returns the chat's incoming file with toxcore's ID for it.
func (s *store) fileByID(chat chatID, id tox.FileID) (message, bool, error) {
	ms, err := s.query("WHERE chat = ? AND NOT outgoing AND files.file_id = ? ORDER BY id DESC LIMIT 1", chat.bytes(), id[:])
	if len(ms) == 0 {
		return message{}, false, err
	}
	return ms[0], true, err
}

// message returns one of a chat's messages.
func (s *store) message(chat chatID, id int64) (message, bool, error) {
	ms, err := s.query("WHERE chat = ? AND id = ?", chat.bytes(), id)
	if len(ms) == 0 {
		return message{}, false, err
	}
	return ms[0], true, err
}

// messages returns up to n of the chat's newest messages from before the one
// with id before, oldest first.
func (s *store) messages(chat chatID, before int64, n int) ([]message, error) {
	ms, err := s.query("WHERE chat = ? AND id < ? ORDER BY id DESC LIMIT ?", chat.bytes(), before, n)
	slices.Reverse(ms)
	return ms, err
}

// last returns the chat's newest message, if there is one.
func (s *store) last(chat chatID) (message, bool, error) {
	ms, err := s.messages(chat, math.MaxInt64, 1)
	if len(ms) == 0 {
		return message{}, false, err
	}
	return ms[0], true, err
}

// undelivered returns the text messages in the chat that didn't go yet,
// oldest first.
func (s *store) undelivered(chat chatID) ([]message, error) {
	return s.query(`WHERE chat = ? AND outgoing AND NOT delivered AND NOT notice
		AND files.message IS NULL AND invites.message IS NULL ORDER BY id`, chat.bytes())
}

func (s *store) query(where string, args ...any) ([]message, error) {
	rows, err := s.db.Query(`SELECT id, time, outgoing, action, text, delivered, sender, sender_name, notice,
		files.file_id, files.name, files.size, files.state,
		invites.kind, invites.data, invites.name, invites.joined
		FROM messages
		LEFT JOIN files ON files.message = messages.id
		LEFT JOIN invites ON invites.message = messages.id `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ms []message
	for rows.Next() {
		var m message
		var t int64
		var sender, fileID, inviteData []byte
		var fileName, inviteName sql.NullString
		var size, state, kind sql.NullInt64
		var joined sql.NullBool
		if err := rows.Scan(&m.id, &t, &m.outgoing, &m.action, &m.text, &m.delivered, &sender, &m.senderName, &m.notice,
			&fileID, &fileName, &size, &state,
			&kind, &inviteData, &inviteName, &joined); err != nil {
			return ms, err
		}
		m.time = time.UnixMilli(t)
		if len(sender) == len(tox.PublicKey{}) {
			m.sender = new(tox.PublicKey)
			copy(m.sender[:], sender)
		}
		if fileName.Valid {
			m.file = &file{name: fileName.String, size: uint64(size.Int64), state: fileState(state.Int64)}
			copy(m.file.id[:], fileID)
		}
		if inviteName.Valid {
			m.invite = &invite{kind: inviteKind(kind.Int64), data: inviteData, name: inviteName.String, joined: joined.Bool}
		}
		ms = append(ms, m)
	}
	return ms, rows.Err()
}

// unread counts the chat's messages the user hasn't seen.
func (s *store) unread(chat chatID) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT count(*) FROM messages
		WHERE chat = ? AND NOT outgoing AND NOT notice AND id > coalesce((SELECT read FROM chats WHERE chat = ?), 0)`,
		chat.bytes(), chat.bytes()).Scan(&n)
	return n, err
}

// markRead records that the user saw the chat's messages up to id.
func (s *store) markRead(chat chatID, id int64) error {
	_, err := s.db.Exec(`INSERT INTO chats (chat, read) VALUES (?, ?)
		ON CONFLICT (chat) DO UPDATE SET read = max(read, excluded.read)`, chat.bytes(), id)
	return err
}

// nickname returns the user's name for a friend, or "".
func (s *store) nickname(chat chatID) (string, error) {
	var name string
	err := s.db.QueryRow("SELECT nickname FROM chats WHERE chat = ?", chat.bytes()).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

func (s *store) setNickname(chat chatID, name string) error {
	_, err := s.db.Exec(`INSERT INTO chats (chat, nickname) VALUES (?, ?)
		ON CONFLICT (chat) DO UPDATE SET nickname = excluded.nickname`, chat.bytes(), name)
	return err
}

// setPeerName records the name of the peer of a private chat.
func (s *store) setPeerName(chat chatID, name string) error {
	_, err := s.db.Exec(`INSERT INTO chats (chat, name) VALUES (?, ?)
		ON CONFLICT (chat) DO UPDATE SET name = excluded.name`, chat.bytes(), name)
	return err
}

// privateChats returns the private chats with peers of a group, with the
// peers' names.
func (s *store) privateChats(group tox.GroupChatID) (map[tox.PublicKey]string, error) {
	prefix := chatID{kind: chatPrivate, key: group}.bytes()[:1+len(group)]
	rows, err := s.db.Query("SELECT chat, name FROM chats WHERE length(chat) = ? AND substr(chat, 1, ?) = ?",
		len(prefix)+len(tox.PublicKey{}), len(prefix), prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chats := map[tox.PublicKey]string{}
	for rows.Next() {
		var chat []byte
		var name string
		if err := rows.Scan(&chat, &name); err != nil {
			return chats, err
		}
		var peer tox.PublicKey
		copy(peer[:], chat[len(prefix):])
		chats[peer] = name
	}
	return chats, rows.Err()
}

func (s *store) addRequest(r request) error {
	_, err := s.db.Exec("INSERT OR REPLACE INTO requests (key, message) VALUES (?, ?)", r.key[:], r.message)
	return err
}

func (s *store) removeRequest(key tox.PublicKey) error {
	_, err := s.db.Exec("DELETE FROM requests WHERE key = ?", key[:])
	return err
}

// requests returns the friend requests, oldest first.
func (s *store) requests() ([]request, error) {
	rows, err := s.db.Query("SELECT key, message FROM requests ORDER BY rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rs []request
	for rows.Next() {
		var r request
		var key []byte
		if err := rows.Scan(&key, &r.message); err != nil {
			return rs, err
		}
		copy(r.key[:], key)
		rs = append(rs, r)
	}
	return rs, rows.Err()
}

// forget deletes a chat and what the store knows about it.
func (s *store) forget(chat chatID) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM messages WHERE chat = ?", chat.bytes()); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM chats WHERE chat = ?", chat.bytes()); err != nil {
		return err
	}
	return tx.Commit()
}
