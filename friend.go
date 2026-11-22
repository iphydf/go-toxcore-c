package tox

/*
#include <tox/tox.h>
*/
import "C"

import (
	"fmt"
	"time"
)

type Friend struct {
	tox    *Tox
	number FriendNumber
}

func (t *Tox) Friend(number FriendNumber) Friend {
	return Friend{t, number}
}

func (f Friend) Number() FriendNumber {
	return f.number
}

func (f Friend) String() string {
	return fmt.Sprintf("friend %d", f.number)
}

func (f Friend) c() C.Tox_Friend_Number {
	return C.Tox_Friend_Number(f.number)
}

func (t *Tox) FriendAdd(address Address, message string) (Friend, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Friend_Add
	n := C.tox_friend_add(p, cBytes(address[:]), cString(message), C.size_t(len(message)), &err)
	return Friend{t, FriendNumber(n)}, toError[FriendAddError](uint32(err))
}

func (t *Tox) FriendAddNoRequest(key PublicKey) (Friend, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Friend_Add
	n := C.tox_friend_add_norequest(p, cBytes(key[:]), &err)
	return Friend{t, FriendNumber(n)}, toError[FriendAddError](uint32(err))
}

func (t *Tox) FriendByPublicKey(key PublicKey) (Friend, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Friend_By_Public_Key
	n := C.tox_friend_by_public_key(p, cBytes(key[:]), &err)
	return Friend{t, FriendNumber(n)}, toError[FriendByPublicKeyError](uint32(err))
}

func (t *Tox) FriendList() []Friend {
	p := t.lock()
	defer t.h.Unlock()
	numbers := make([]C.Tox_Friend_Number, C.tox_self_get_friend_list_size(p))
	if len(numbers) == 0 {
		return nil
	}
	C.tox_self_get_friend_list(p, &numbers[0])
	friends := make([]Friend, len(numbers))
	for i, n := range numbers {
		friends[i] = Friend{t, FriendNumber(n)}
	}
	return friends
}

func (f Friend) Delete() error {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Friend_Delete
	C.tox_friend_delete(p, f.c(), &err)
	return toError[FriendDeleteError](uint32(err))
}

func (f Friend) Exists() bool {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	return bool(C.tox_friend_exists(p, f.c()))
}

func (f Friend) PublicKey() (PublicKey, error) {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var k PublicKey
	var err C.Tox_Err_Friend_Get_Public_Key
	C.tox_friend_get_public_key(p, f.c(), cOut(k[:]), &err)
	return k, toError[FriendGetPublicKeyError](uint32(err))
}

func (f Friend) LastOnline() (time.Time, error) {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Friend_Get_Last_Online
	seconds := C.tox_friend_get_last_online(p, f.c(), &err)
	if err := toError[FriendGetLastOnlineError](uint32(err)); err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(seconds), 0), nil
}

func (f Friend) Name() (string, error) {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Friend_Query
	size := C.tox_friend_get_name_size(p, f.c(), &err)
	if err := toError[FriendQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_friend_get_name(p, f.c(), buf, nil)
	}), nil
}

func (f Friend) StatusMessage() (string, error) {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Friend_Query
	size := C.tox_friend_get_status_message_size(p, f.c(), &err)
	if err := toError[FriendQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_friend_get_status_message(p, f.c(), buf, nil)
	}), nil
}

func (f Friend) SetTyping(typing bool) error {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Set_Typing
	C.tox_self_set_typing(p, f.c(), C.bool(typing), &err)
	return toError[SetTypingError](uint32(err))
}

func (f Friend) SendMessage(kind MessageType, message string) (MessageID, error) {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Friend_Send_Message
	id := C.tox_friend_send_message(p, f.c(), C.Tox_Message_Type(kind), cString(message), C.size_t(len(message)), &err)
	return MessageID(id), toError[FriendSendMessageError](uint32(err))
}

func (f Friend) SendLossyPacket(data []byte) error {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Friend_Custom_Packet
	C.tox_friend_send_lossy_packet(p, f.c(), cBytes(data), C.size_t(len(data)), &err)
	return toError[FriendCustomPacketError](uint32(err))
}

func (f Friend) SendLosslessPacket(data []byte) error {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_Friend_Custom_Packet
	C.tox_friend_send_lossless_packet(p, f.c(), cBytes(data), C.size_t(len(data)), &err)
	return toError[FriendCustomPacketError](uint32(err))
}
