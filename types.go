package tox

/*
#include <tox/tox.h>
*/
import "C"

import (
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	MaxNameLength          = C.TOX_MAX_NAME_LENGTH
	MaxStatusMessageLength = C.TOX_MAX_STATUS_MESSAGE_LENGTH
	MaxFriendRequestLength = C.TOX_MAX_FRIEND_REQUEST_LENGTH
	MaxMessageLength       = C.TOX_MAX_MESSAGE_LENGTH
	MaxCustomPacketSize    = C.TOX_MAX_CUSTOM_PACKET_SIZE
	MaxFilenameLength      = C.TOX_MAX_FILENAME_LENGTH
	MaxHostnameLength      = C.TOX_MAX_HOSTNAME_LENGTH
)

type PublicKey [C.TOX_PUBLIC_KEY_SIZE]byte

type SecretKey [C.TOX_SECRET_KEY_SIZE]byte

type DhtID [C.TOX_DHT_ID_SIZE]byte

type Address [C.TOX_ADDRESS_SIZE]byte

type Hash [C.TOX_HASH_LENGTH]byte

type FileID [C.TOX_FILE_ID_LENGTH]byte

func (k PublicKey) String() string { return hexString(k[:]) }

func (k DhtID) String() string { return hexString(k[:]) }

func (a Address) String() string { return hexString(a[:]) }

func (h Hash) String() string { return hexString(h[:]) }

func (id FileID) String() string { return hexString(id[:]) }

func ParsePublicKey(s string) (PublicKey, error) {
	var k PublicKey
	return k, parseHex(k[:], s, "public key")
}

func ParseDhtID(s string) (DhtID, error) {
	var k DhtID
	return k, parseHex(k[:], s, "DHT ID")
}

func ParseAddress(s string) (Address, error) {
	var a Address
	return a, parseHex(a[:], s, "address")
}

func hexString(b []byte) string {
	return strings.ToUpper(hex.EncodeToString(b))
}

func parseHex(dst []byte, s, what string) error {
	if hex.DecodedLen(len(s)) != len(dst) {
		return fmt.Errorf("tox: %s must be %d hex digits, not %d", what, 2*len(dst), len(s))
	}
	if _, err := hex.Decode(dst, []byte(s)); err != nil {
		return fmt.Errorf("tox: %s: %w", what, err)
	}
	return nil
}

func HashData(data []byte) Hash {
	var h Hash
	C.tox_hash(cOut(h[:]), cBytes(data), C.size_t(len(data)))
	return h
}

type FriendNumber uint32

type FileNumber uint32

type MessageID uint32

type Port uint16

type Connection uint32

const (
	ConnectionNone Connection = C.TOX_CONNECTION_NONE
	ConnectionTCP  Connection = C.TOX_CONNECTION_TCP
	ConnectionUDP  Connection = C.TOX_CONNECTION_UDP
)

func (c Connection) String() string {
	return enumString(C.tox_connection_to_string(C.Tox_Connection(c)), "TOX_CONNECTION_", "Connection", uint32(c))
}

type UserStatus uint32

const (
	UserStatusNone UserStatus = C.TOX_USER_STATUS_NONE
	UserStatusAway UserStatus = C.TOX_USER_STATUS_AWAY
	UserStatusBusy UserStatus = C.TOX_USER_STATUS_BUSY
)

func (s UserStatus) String() string {
	return enumString(C.tox_user_status_to_string(C.Tox_User_Status(s)), "TOX_USER_STATUS_", "UserStatus", uint32(s))
}

type MessageType uint32

const (
	MessageTypeNormal MessageType = C.TOX_MESSAGE_TYPE_NORMAL
	MessageTypeAction MessageType = C.TOX_MESSAGE_TYPE_ACTION
)

func (t MessageType) String() string {
	return enumString(C.tox_message_type_to_string(C.Tox_Message_Type(t)), "TOX_MESSAGE_TYPE_", "MessageType", uint32(t))
}
