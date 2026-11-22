package tox

/*
#include <tox/tox.h>
*/
import "C"

type ConferenceNewError uint32

const ErrConferenceNewInit ConferenceNewError = C.TOX_ERR_CONFERENCE_NEW_INIT

func (e ConferenceNewError) Error() string {
	return errorString(C.tox_err_conference_new_to_string(C.Tox_Err_Conference_New(e)), "TOX_ERR_CONFERENCE_NEW_", uint32(e))
}

type ConferenceDeleteError uint32

const ErrConferenceDeleteConferenceNotFound ConferenceDeleteError = C.TOX_ERR_CONFERENCE_DELETE_CONFERENCE_NOT_FOUND

func (e ConferenceDeleteError) Error() string {
	return errorString(C.tox_err_conference_delete_to_string(C.Tox_Err_Conference_Delete(e)), "TOX_ERR_CONFERENCE_DELETE_", uint32(e))
}

type ConferencePeerQueryError uint32

const (
	ErrConferencePeerQueryConferenceNotFound ConferencePeerQueryError = C.TOX_ERR_CONFERENCE_PEER_QUERY_CONFERENCE_NOT_FOUND
	ErrConferencePeerQueryPeerNotFound       ConferencePeerQueryError = C.TOX_ERR_CONFERENCE_PEER_QUERY_PEER_NOT_FOUND
	ErrConferencePeerQueryNoConnection       ConferencePeerQueryError = C.TOX_ERR_CONFERENCE_PEER_QUERY_NO_CONNECTION
)

func (e ConferencePeerQueryError) Error() string {
	return errorString(C.tox_err_conference_peer_query_to_string(C.Tox_Err_Conference_Peer_Query(e)), "TOX_ERR_CONFERENCE_PEER_QUERY_", uint32(e))
}

type ConferenceSetMaxOfflineError uint32

const ErrConferenceSetMaxOfflineConferenceNotFound ConferenceSetMaxOfflineError = C.TOX_ERR_CONFERENCE_SET_MAX_OFFLINE_CONFERENCE_NOT_FOUND

func (e ConferenceSetMaxOfflineError) Error() string {
	return errorString(C.tox_err_conference_set_max_offline_to_string(C.Tox_Err_Conference_Set_Max_Offline(e)), "TOX_ERR_CONFERENCE_SET_MAX_OFFLINE_", uint32(e))
}

type ConferenceInviteError uint32

const (
	ErrConferenceInviteConferenceNotFound ConferenceInviteError = C.TOX_ERR_CONFERENCE_INVITE_CONFERENCE_NOT_FOUND
	ErrConferenceInviteFailSend           ConferenceInviteError = C.TOX_ERR_CONFERENCE_INVITE_FAIL_SEND
	ErrConferenceInviteNoConnection       ConferenceInviteError = C.TOX_ERR_CONFERENCE_INVITE_NO_CONNECTION
)

func (e ConferenceInviteError) Error() string {
	return errorString(C.tox_err_conference_invite_to_string(C.Tox_Err_Conference_Invite(e)), "TOX_ERR_CONFERENCE_INVITE_", uint32(e))
}

type ConferenceJoinError uint32

const (
	ErrConferenceJoinInvalidLength  ConferenceJoinError = C.TOX_ERR_CONFERENCE_JOIN_INVALID_LENGTH
	ErrConferenceJoinWrongType      ConferenceJoinError = C.TOX_ERR_CONFERENCE_JOIN_WRONG_TYPE
	ErrConferenceJoinFriendNotFound ConferenceJoinError = C.TOX_ERR_CONFERENCE_JOIN_FRIEND_NOT_FOUND
	ErrConferenceJoinDuplicate      ConferenceJoinError = C.TOX_ERR_CONFERENCE_JOIN_DUPLICATE
	ErrConferenceJoinInitFail       ConferenceJoinError = C.TOX_ERR_CONFERENCE_JOIN_INIT_FAIL
	ErrConferenceJoinFailSend       ConferenceJoinError = C.TOX_ERR_CONFERENCE_JOIN_FAIL_SEND
)

func (e ConferenceJoinError) Error() string {
	return errorString(C.tox_err_conference_join_to_string(C.Tox_Err_Conference_Join(e)), "TOX_ERR_CONFERENCE_JOIN_", uint32(e))
}

type ConferenceSendMessageError uint32

const (
	ErrConferenceSendMessageConferenceNotFound ConferenceSendMessageError = C.TOX_ERR_CONFERENCE_SEND_MESSAGE_CONFERENCE_NOT_FOUND
	ErrConferenceSendMessageTooLong            ConferenceSendMessageError = C.TOX_ERR_CONFERENCE_SEND_MESSAGE_TOO_LONG
	ErrConferenceSendMessageNoConnection       ConferenceSendMessageError = C.TOX_ERR_CONFERENCE_SEND_MESSAGE_NO_CONNECTION
	ErrConferenceSendMessageFailSend           ConferenceSendMessageError = C.TOX_ERR_CONFERENCE_SEND_MESSAGE_FAIL_SEND
)

func (e ConferenceSendMessageError) Error() string {
	return errorString(C.tox_err_conference_send_message_to_string(C.Tox_Err_Conference_Send_Message(e)), "TOX_ERR_CONFERENCE_SEND_MESSAGE_", uint32(e))
}

type ConferenceTitleError uint32

const (
	ErrConferenceTitleConferenceNotFound ConferenceTitleError = C.TOX_ERR_CONFERENCE_TITLE_CONFERENCE_NOT_FOUND
	ErrConferenceTitleInvalidLength      ConferenceTitleError = C.TOX_ERR_CONFERENCE_TITLE_INVALID_LENGTH
	ErrConferenceTitleFailSend           ConferenceTitleError = C.TOX_ERR_CONFERENCE_TITLE_FAIL_SEND
)

func (e ConferenceTitleError) Error() string {
	return errorString(C.tox_err_conference_title_to_string(C.Tox_Err_Conference_Title(e)), "TOX_ERR_CONFERENCE_TITLE_", uint32(e))
}

type ConferenceGetTypeError uint32

const ErrConferenceGetTypeConferenceNotFound ConferenceGetTypeError = C.TOX_ERR_CONFERENCE_GET_TYPE_CONFERENCE_NOT_FOUND

func (e ConferenceGetTypeError) Error() string {
	return errorString(C.tox_err_conference_get_type_to_string(C.Tox_Err_Conference_Get_Type(e)), "TOX_ERR_CONFERENCE_GET_TYPE_", uint32(e))
}

type ConferenceByIDError uint32

const ErrConferenceByIDNotFound ConferenceByIDError = C.TOX_ERR_CONFERENCE_BY_ID_NOT_FOUND

func (e ConferenceByIDError) Error() string {
	return errorString(C.tox_err_conference_by_id_to_string(C.Tox_Err_Conference_By_Id(e)), "TOX_ERR_CONFERENCE_BY_ID_", uint32(e))
}
