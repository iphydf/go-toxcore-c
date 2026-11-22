package tox

/*
#include <tox/tox.h>
#include <tox/tox_events.h>
*/
import "C"

import (
	"fmt"

	"github.com/TokTok/go-toxcore-c/internal/cname"
)

type toxError interface {
	~uint32
	error
}

// toError is nil for toxcore's OK value, which is always 0.
func toError[E toxError](code uint32) error {
	if code == 0 {
		return nil
	}
	return E(code)
}

// errorString turns C's name for an error, such as
// "TOX_ERR_FRIEND_ADD_TOO_LONG" for prefix "TOX_ERR_FRIEND_ADD_", into
// "tox: friend add: too long".
func errorString(s *C.char, prefix string, code uint32) string {
	return cname.Error("tox", C.GoString(s), prefix, code)
}

type NewError uint32

const (
	ErrNewMalloc        NewError = C.TOX_ERR_NEW_MALLOC
	ErrNewPortAlloc     NewError = C.TOX_ERR_NEW_PORT_ALLOC
	ErrNewProxyBadType  NewError = C.TOX_ERR_NEW_PROXY_BAD_TYPE
	ErrNewProxyBadHost  NewError = C.TOX_ERR_NEW_PROXY_BAD_HOST
	ErrNewProxyBadPort  NewError = C.TOX_ERR_NEW_PROXY_BAD_PORT
	ErrNewProxyNotFound NewError = C.TOX_ERR_NEW_PROXY_NOT_FOUND
	ErrNewLoadEncrypted NewError = C.TOX_ERR_NEW_LOAD_ENCRYPTED
	ErrNewLoadBadFormat NewError = C.TOX_ERR_NEW_LOAD_BAD_FORMAT
)

func (e NewError) Error() string {
	return errorString(C.tox_err_new_to_string(C.Tox_Err_New(e)), "TOX_ERR_NEW_", uint32(e))
}

type BootstrapError uint32

const (
	ErrBootstrapBadHost BootstrapError = C.TOX_ERR_BOOTSTRAP_BAD_HOST
	ErrBootstrapBadPort BootstrapError = C.TOX_ERR_BOOTSTRAP_BAD_PORT
)

func (e BootstrapError) Error() string {
	return errorString(C.tox_err_bootstrap_to_string(C.Tox_Err_Bootstrap(e)), "TOX_ERR_BOOTSTRAP_", uint32(e))
}

type SetInfoError uint32

const ErrSetInfoTooLong SetInfoError = C.TOX_ERR_SET_INFO_TOO_LONG

func (e SetInfoError) Error() string {
	return errorString(C.tox_err_set_info_to_string(C.Tox_Err_Set_Info(e)), "TOX_ERR_SET_INFO_", uint32(e))
}

type GetPortError uint32

const ErrGetPortNotBound GetPortError = C.TOX_ERR_GET_PORT_NOT_BOUND

func (e GetPortError) Error() string {
	return errorString(C.tox_err_get_port_to_string(C.Tox_Err_Get_Port(e)), "TOX_ERR_GET_PORT_", uint32(e))
}

type FriendAddError uint32

const (
	ErrFriendAddTooLong      FriendAddError = C.TOX_ERR_FRIEND_ADD_TOO_LONG
	ErrFriendAddNoMessage    FriendAddError = C.TOX_ERR_FRIEND_ADD_NO_MESSAGE
	ErrFriendAddOwnKey       FriendAddError = C.TOX_ERR_FRIEND_ADD_OWN_KEY
	ErrFriendAddAlreadySent  FriendAddError = C.TOX_ERR_FRIEND_ADD_ALREADY_SENT
	ErrFriendAddBadChecksum  FriendAddError = C.TOX_ERR_FRIEND_ADD_BAD_CHECKSUM
	ErrFriendAddSetNewNospam FriendAddError = C.TOX_ERR_FRIEND_ADD_SET_NEW_NOSPAM
	ErrFriendAddMalloc       FriendAddError = C.TOX_ERR_FRIEND_ADD_MALLOC
)

func (e FriendAddError) Error() string {
	return errorString(C.tox_err_friend_add_to_string(C.Tox_Err_Friend_Add(e)), "TOX_ERR_FRIEND_ADD_", uint32(e))
}

type FriendDeleteError uint32

const ErrFriendDeleteFriendNotFound FriendDeleteError = C.TOX_ERR_FRIEND_DELETE_FRIEND_NOT_FOUND

func (e FriendDeleteError) Error() string {
	return errorString(C.tox_err_friend_delete_to_string(C.Tox_Err_Friend_Delete(e)), "TOX_ERR_FRIEND_DELETE_", uint32(e))
}

type FriendByPublicKeyError uint32

const ErrFriendByPublicKeyNotFound FriendByPublicKeyError = C.TOX_ERR_FRIEND_BY_PUBLIC_KEY_NOT_FOUND

func (e FriendByPublicKeyError) Error() string {
	return errorString(C.tox_err_friend_by_public_key_to_string(C.Tox_Err_Friend_By_Public_Key(e)), "TOX_ERR_FRIEND_BY_PUBLIC_KEY_", uint32(e))
}

type FriendGetPublicKeyError uint32

const ErrFriendGetPublicKeyFriendNotFound FriendGetPublicKeyError = C.TOX_ERR_FRIEND_GET_PUBLIC_KEY_FRIEND_NOT_FOUND

func (e FriendGetPublicKeyError) Error() string {
	return errorString(C.tox_err_friend_get_public_key_to_string(C.Tox_Err_Friend_Get_Public_Key(e)), "TOX_ERR_FRIEND_GET_PUBLIC_KEY_", uint32(e))
}

type FriendGetLastOnlineError uint32

const ErrFriendGetLastOnlineFriendNotFound FriendGetLastOnlineError = C.TOX_ERR_FRIEND_GET_LAST_ONLINE_FRIEND_NOT_FOUND

func (e FriendGetLastOnlineError) Error() string {
	return errorString(C.tox_err_friend_get_last_online_to_string(C.Tox_Err_Friend_Get_Last_Online(e)), "TOX_ERR_FRIEND_GET_LAST_ONLINE_", uint32(e))
}

type FriendQueryError uint32

const ErrFriendQueryFriendNotFound FriendQueryError = C.TOX_ERR_FRIEND_QUERY_FRIEND_NOT_FOUND

func (e FriendQueryError) Error() string {
	return errorString(C.tox_err_friend_query_to_string(C.Tox_Err_Friend_Query(e)), "TOX_ERR_FRIEND_QUERY_", uint32(e))
}

type SetTypingError uint32

const ErrSetTypingFriendNotFound SetTypingError = C.TOX_ERR_SET_TYPING_FRIEND_NOT_FOUND

func (e SetTypingError) Error() string {
	return errorString(C.tox_err_set_typing_to_string(C.Tox_Err_Set_Typing(e)), "TOX_ERR_SET_TYPING_", uint32(e))
}

type FriendSendMessageError uint32

const (
	ErrFriendSendMessageFriendNotFound     FriendSendMessageError = C.TOX_ERR_FRIEND_SEND_MESSAGE_FRIEND_NOT_FOUND
	ErrFriendSendMessageFriendNotConnected FriendSendMessageError = C.TOX_ERR_FRIEND_SEND_MESSAGE_FRIEND_NOT_CONNECTED
	ErrFriendSendMessageSendq              FriendSendMessageError = C.TOX_ERR_FRIEND_SEND_MESSAGE_SENDQ
	ErrFriendSendMessageTooLong            FriendSendMessageError = C.TOX_ERR_FRIEND_SEND_MESSAGE_TOO_LONG
	ErrFriendSendMessageEmpty              FriendSendMessageError = C.TOX_ERR_FRIEND_SEND_MESSAGE_EMPTY
)

func (e FriendSendMessageError) Error() string {
	return errorString(C.tox_err_friend_send_message_to_string(C.Tox_Err_Friend_Send_Message(e)), "TOX_ERR_FRIEND_SEND_MESSAGE_", uint32(e))
}

type FriendCustomPacketError uint32

const (
	ErrFriendCustomPacketFriendNotFound     FriendCustomPacketError = C.TOX_ERR_FRIEND_CUSTOM_PACKET_FRIEND_NOT_FOUND
	ErrFriendCustomPacketFriendNotConnected FriendCustomPacketError = C.TOX_ERR_FRIEND_CUSTOM_PACKET_FRIEND_NOT_CONNECTED
	ErrFriendCustomPacketInvalid            FriendCustomPacketError = C.TOX_ERR_FRIEND_CUSTOM_PACKET_INVALID
	ErrFriendCustomPacketEmpty              FriendCustomPacketError = C.TOX_ERR_FRIEND_CUSTOM_PACKET_EMPTY
	ErrFriendCustomPacketTooLong            FriendCustomPacketError = C.TOX_ERR_FRIEND_CUSTOM_PACKET_TOO_LONG
	ErrFriendCustomPacketSendq              FriendCustomPacketError = C.TOX_ERR_FRIEND_CUSTOM_PACKET_SENDQ
)

func (e FriendCustomPacketError) Error() string {
	return errorString(C.tox_err_friend_custom_packet_to_string(C.Tox_Err_Friend_Custom_Packet(e)), "TOX_ERR_FRIEND_CUSTOM_PACKET_", uint32(e))
}

type FileControlError uint32

const (
	ErrFileControlFriendNotFound     FileControlError = C.TOX_ERR_FILE_CONTROL_FRIEND_NOT_FOUND
	ErrFileControlFriendNotConnected FileControlError = C.TOX_ERR_FILE_CONTROL_FRIEND_NOT_CONNECTED
	ErrFileControlNotFound           FileControlError = C.TOX_ERR_FILE_CONTROL_NOT_FOUND
	ErrFileControlNotPaused          FileControlError = C.TOX_ERR_FILE_CONTROL_NOT_PAUSED
	ErrFileControlDenied             FileControlError = C.TOX_ERR_FILE_CONTROL_DENIED
	ErrFileControlAlreadyPaused      FileControlError = C.TOX_ERR_FILE_CONTROL_ALREADY_PAUSED
	ErrFileControlSendq              FileControlError = C.TOX_ERR_FILE_CONTROL_SENDQ
)

func (e FileControlError) Error() string {
	return errorString(C.tox_err_file_control_to_string(C.Tox_Err_File_Control(e)), "TOX_ERR_FILE_CONTROL_", uint32(e))
}

type FileSeekError uint32

const (
	ErrFileSeekFriendNotFound     FileSeekError = C.TOX_ERR_FILE_SEEK_FRIEND_NOT_FOUND
	ErrFileSeekFriendNotConnected FileSeekError = C.TOX_ERR_FILE_SEEK_FRIEND_NOT_CONNECTED
	ErrFileSeekNotFound           FileSeekError = C.TOX_ERR_FILE_SEEK_NOT_FOUND
	ErrFileSeekDenied             FileSeekError = C.TOX_ERR_FILE_SEEK_DENIED
	ErrFileSeekInvalidPosition    FileSeekError = C.TOX_ERR_FILE_SEEK_INVALID_POSITION
	ErrFileSeekSendq              FileSeekError = C.TOX_ERR_FILE_SEEK_SENDQ
)

func (e FileSeekError) Error() string {
	return errorString(C.tox_err_file_seek_to_string(C.Tox_Err_File_Seek(e)), "TOX_ERR_FILE_SEEK_", uint32(e))
}

type FileGetError uint32

const (
	ErrFileGetFriendNotFound FileGetError = C.TOX_ERR_FILE_GET_FRIEND_NOT_FOUND
	ErrFileGetNotFound       FileGetError = C.TOX_ERR_FILE_GET_NOT_FOUND
)

func (e FileGetError) Error() string {
	return errorString(C.tox_err_file_get_to_string(C.Tox_Err_File_Get(e)), "TOX_ERR_FILE_GET_", uint32(e))
}

type FileByIDError uint32

const (
	ErrFileByIDFriendNotFound FileByIDError = C.TOX_ERR_FILE_BY_ID_FRIEND_NOT_FOUND
	ErrFileByIDNotFound       FileByIDError = C.TOX_ERR_FILE_BY_ID_NOT_FOUND
)

func (e FileByIDError) Error() string {
	return errorString(C.tox_err_file_by_id_to_string(C.Tox_Err_File_By_Id(e)), "TOX_ERR_FILE_BY_ID_", uint32(e))
}

type FileSendError uint32

const (
	ErrFileSendFriendNotFound     FileSendError = C.TOX_ERR_FILE_SEND_FRIEND_NOT_FOUND
	ErrFileSendFriendNotConnected FileSendError = C.TOX_ERR_FILE_SEND_FRIEND_NOT_CONNECTED
	ErrFileSendNameTooLong        FileSendError = C.TOX_ERR_FILE_SEND_NAME_TOO_LONG
	ErrFileSendTooMany            FileSendError = C.TOX_ERR_FILE_SEND_TOO_MANY
)

func (e FileSendError) Error() string {
	return errorString(C.tox_err_file_send_to_string(C.Tox_Err_File_Send(e)), "TOX_ERR_FILE_SEND_", uint32(e))
}

type FileSendChunkError uint32

const (
	ErrFileSendChunkFriendNotFound     FileSendChunkError = C.TOX_ERR_FILE_SEND_CHUNK_FRIEND_NOT_FOUND
	ErrFileSendChunkFriendNotConnected FileSendChunkError = C.TOX_ERR_FILE_SEND_CHUNK_FRIEND_NOT_CONNECTED
	ErrFileSendChunkNotFound           FileSendChunkError = C.TOX_ERR_FILE_SEND_CHUNK_NOT_FOUND
	ErrFileSendChunkNotTransferring    FileSendChunkError = C.TOX_ERR_FILE_SEND_CHUNK_NOT_TRANSFERRING
	ErrFileSendChunkInvalidLength      FileSendChunkError = C.TOX_ERR_FILE_SEND_CHUNK_INVALID_LENGTH
	ErrFileSendChunkSendq              FileSendChunkError = C.TOX_ERR_FILE_SEND_CHUNK_SENDQ
	ErrFileSendChunkWrongPosition      FileSendChunkError = C.TOX_ERR_FILE_SEND_CHUNK_WRONG_POSITION
)

func (e FileSendChunkError) Error() string {
	return errorString(C.tox_err_file_send_chunk_to_string(C.Tox_Err_File_Send_Chunk(e)), "TOX_ERR_FILE_SEND_CHUNK_", uint32(e))
}

type EventsIterateError uint32

const ErrEventsIterateMalloc EventsIterateError = C.TOX_ERR_EVENTS_ITERATE_MALLOC

func (e EventsIterateError) Error() string {
	if e == ErrEventsIterateMalloc {
		return "tox: events iterate: malloc"
	}
	return fmt.Sprintf("tox: events iterate: error %d", uint32(e))
}
