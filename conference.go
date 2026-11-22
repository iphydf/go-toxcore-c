package tox

/*
#include <tox/tox.h>
*/
import "C"

import (
	"fmt"
	"time"
)

type ConferenceID [C.TOX_CONFERENCE_ID_SIZE]byte

func (id ConferenceID) String() string { return hexString(id[:]) }

func ParseConferenceID(s string) (ConferenceID, error) {
	var id ConferenceID
	return id, parseHex(id[:], s, "conference ID")
}

type ConferenceNumber uint32

type ConferencePeerNumber uint32

type ConferenceOfflinePeerNumber uint32

type ConferenceType uint32

const (
	ConferenceTypeText ConferenceType = C.TOX_CONFERENCE_TYPE_TEXT
	ConferenceTypeAV   ConferenceType = C.TOX_CONFERENCE_TYPE_AV
)

func (t ConferenceType) String() string {
	return enumString(C.tox_conference_type_to_string(C.Tox_Conference_Type(t)), "TOX_CONFERENCE_TYPE_", "ConferenceType", uint32(t))
}

type Conference struct {
	tox    *Tox
	number ConferenceNumber
}

func (t *Tox) Conference(number ConferenceNumber) Conference {
	return Conference{t, number}
}

func (c Conference) Number() ConferenceNumber {
	return c.number
}

func (c Conference) String() string {
	return fmt.Sprintf("conference %d", c.number)
}

func (c Conference) c() C.Tox_Conference_Number {
	return C.Tox_Conference_Number(c.number)
}

type ConferencePeer struct {
	conference Conference
	number     ConferencePeerNumber
}

func (c Conference) Peer(number ConferencePeerNumber) ConferencePeer {
	return ConferencePeer{c, number}
}

func (cp ConferencePeer) Conference() Conference {
	return cp.conference
}

func (cp ConferencePeer) Number() ConferencePeerNumber {
	return cp.number
}

func (cp ConferencePeer) String() string {
	return fmt.Sprintf("peer %d of conference %d", cp.number, cp.conference.number)
}

func (cp ConferencePeer) c() C.Tox_Conference_Peer_Number {
	return C.Tox_Conference_Peer_Number(cp.number)
}

type ConferenceOfflinePeer struct {
	conference Conference
	number     ConferenceOfflinePeerNumber
}

func (c Conference) OfflinePeer(number ConferenceOfflinePeerNumber) ConferenceOfflinePeer {
	return ConferenceOfflinePeer{c, number}
}

func (op ConferenceOfflinePeer) Conference() Conference {
	return op.conference
}

func (op ConferenceOfflinePeer) Number() ConferenceOfflinePeerNumber {
	return op.number
}

func (op ConferenceOfflinePeer) String() string {
	return fmt.Sprintf("offline peer %d of conference %d", op.number, op.conference.number)
}

func (op ConferenceOfflinePeer) c() C.Tox_Conference_Offline_Peer_Number {
	return C.Tox_Conference_Offline_Peer_Number(op.number)
}

func (t *Tox) ConferenceNew() (Conference, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Conference_New
	n := C.tox_conference_new(p, &err)
	return Conference{t, ConferenceNumber(n)}, toError[ConferenceNewError](uint32(err))
}

func (t *Tox) ConferenceJoin(friend Friend, cookie []byte) (Conference, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Conference_Join
	n := C.tox_conference_join(p, friend.c(), cBytes(cookie), C.size_t(len(cookie)), &err)
	return Conference{t, ConferenceNumber(n)}, toError[ConferenceJoinError](uint32(err))
}

func (t *Tox) ConferenceByID(id ConferenceID) (Conference, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Conference_By_Id
	n := C.tox_conference_by_id(p, cBytes(id[:]), &err)
	return Conference{t, ConferenceNumber(n)}, toError[ConferenceByIDError](uint32(err))
}

func (t *Tox) ConferenceChatlist() []Conference {
	p := t.lock()
	defer t.h.Unlock()
	numbers := make([]C.Tox_Conference_Number, C.tox_conference_get_chatlist_size(p))
	if len(numbers) == 0 {
		return nil
	}
	C.tox_conference_get_chatlist(p, &numbers[0])
	conferences := make([]Conference, len(numbers))
	for i, n := range numbers {
		conferences[i] = Conference{t, ConferenceNumber(n)}
	}
	return conferences
}

func (c Conference) Delete() error {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Delete
	C.tox_conference_delete(p, c.c(), &err)
	return toError[ConferenceDeleteError](uint32(err))
}

func (c Conference) PeerCount() (uint32, error) {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Peer_Query
	count := C.tox_conference_peer_count(p, c.c(), &err)
	return uint32(count), toError[ConferencePeerQueryError](uint32(err))
}

func (c Conference) OfflinePeerCount() (uint32, error) {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Peer_Query
	count := C.tox_conference_offline_peer_count(p, c.c(), &err)
	return uint32(count), toError[ConferencePeerQueryError](uint32(err))
}

func (c Conference) SetMaxOffline(maxOffline uint32) error {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Set_Max_Offline
	C.tox_conference_set_max_offline(p, c.c(), C.uint32_t(maxOffline), &err)
	return toError[ConferenceSetMaxOfflineError](uint32(err))
}

func (c Conference) Invite(friend Friend) error {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Invite
	C.tox_conference_invite(p, friend.c(), c.c(), &err)
	return toError[ConferenceInviteError](uint32(err))
}

func (c Conference) SendMessage(kind MessageType, message string) error {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Send_Message
	C.tox_conference_send_message(p, c.c(), C.Tox_Message_Type(kind), cString(message), C.size_t(len(message)), &err)
	return toError[ConferenceSendMessageError](uint32(err))
}

func (c Conference) Title() (string, error) {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Title
	size := C.tox_conference_get_title_size(p, c.c(), &err)
	if err := toError[ConferenceTitleError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_conference_get_title(p, c.c(), buf, nil)
	}), nil
}

func (c Conference) SetTitle(title string) error {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Title
	C.tox_conference_set_title(p, c.c(), cString(title), C.size_t(len(title)), &err)
	return toError[ConferenceTitleError](uint32(err))
}

func (c Conference) Type() (ConferenceType, error) {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var err C.Tox_Err_Conference_Get_Type
	kind := C.tox_conference_get_type(p, c.c(), &err)
	return ConferenceType(kind), toError[ConferenceGetTypeError](uint32(err))
}

func (c Conference) ID() (ConferenceID, bool) {
	p := c.tox.lock()
	defer c.tox.h.Unlock()
	var id ConferenceID
	ok := C.tox_conference_get_id(p, c.c(), cOut(id[:]))
	return id, bool(ok)
}

func (cp ConferencePeer) Name() (string, error) {
	p := cp.conference.tox.lock()
	defer cp.conference.tox.h.Unlock()
	var err C.Tox_Err_Conference_Peer_Query
	size := C.tox_conference_peer_get_name_size(p, cp.conference.c(), cp.c(), &err)
	if err := toError[ConferencePeerQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_conference_peer_get_name(p, cp.conference.c(), cp.c(), buf, nil)
	}), nil
}

func (cp ConferencePeer) PublicKey() (PublicKey, error) {
	p := cp.conference.tox.lock()
	defer cp.conference.tox.h.Unlock()
	var k PublicKey
	var err C.Tox_Err_Conference_Peer_Query
	C.tox_conference_peer_get_public_key(p, cp.conference.c(), cp.c(), cOut(k[:]), &err)
	return k, toError[ConferencePeerQueryError](uint32(err))
}

func (cp ConferencePeer) IsOurs() (bool, error) {
	p := cp.conference.tox.lock()
	defer cp.conference.tox.h.Unlock()
	var err C.Tox_Err_Conference_Peer_Query
	ours := C.tox_conference_peer_number_is_ours(p, cp.conference.c(), cp.c(), &err)
	return bool(ours), toError[ConferencePeerQueryError](uint32(err))
}

func (op ConferenceOfflinePeer) Name() (string, error) {
	p := op.conference.tox.lock()
	defer op.conference.tox.h.Unlock()
	var err C.Tox_Err_Conference_Peer_Query
	size := C.tox_conference_offline_peer_get_name_size(p, op.conference.c(), op.c(), &err)
	if err := toError[ConferencePeerQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_conference_offline_peer_get_name(p, op.conference.c(), op.c(), buf, nil)
	}), nil
}

func (op ConferenceOfflinePeer) PublicKey() (PublicKey, error) {
	p := op.conference.tox.lock()
	defer op.conference.tox.h.Unlock()
	var k PublicKey
	var err C.Tox_Err_Conference_Peer_Query
	C.tox_conference_offline_peer_get_public_key(p, op.conference.c(), op.c(), cOut(k[:]), &err)
	return k, toError[ConferencePeerQueryError](uint32(err))
}

func (op ConferenceOfflinePeer) LastActive() (time.Time, error) {
	p := op.conference.tox.lock()
	defer op.conference.tox.h.Unlock()
	var err C.Tox_Err_Conference_Peer_Query
	seconds := C.tox_conference_offline_peer_get_last_active(p, op.conference.c(), op.c(), &err)
	if err := toError[ConferencePeerQueryError](uint32(err)); err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(seconds), 0), nil
}
