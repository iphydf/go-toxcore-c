package tox

/*
#include <tox/tox.h>
*/
import "C"

type GroupNewError uint32

const (
	ErrGroupNewTooLong  GroupNewError = C.TOX_ERR_GROUP_NEW_TOO_LONG
	ErrGroupNewEmpty    GroupNewError = C.TOX_ERR_GROUP_NEW_EMPTY
	ErrGroupNewInit     GroupNewError = C.TOX_ERR_GROUP_NEW_INIT
	ErrGroupNewState    GroupNewError = C.TOX_ERR_GROUP_NEW_STATE
	ErrGroupNewAnnounce GroupNewError = C.TOX_ERR_GROUP_NEW_ANNOUNCE
)

func (e GroupNewError) Error() string {
	return errorString(C.tox_err_group_new_to_string(C.Tox_Err_Group_New(e)), "TOX_ERR_GROUP_NEW_", uint32(e))
}

type GroupJoinError uint32

const (
	ErrGroupJoinInit      GroupJoinError = C.TOX_ERR_GROUP_JOIN_INIT
	ErrGroupJoinBadChatID GroupJoinError = C.TOX_ERR_GROUP_JOIN_BAD_CHAT_ID
	ErrGroupJoinEmpty     GroupJoinError = C.TOX_ERR_GROUP_JOIN_EMPTY
	ErrGroupJoinTooLong   GroupJoinError = C.TOX_ERR_GROUP_JOIN_TOO_LONG
	ErrGroupJoinPassword  GroupJoinError = C.TOX_ERR_GROUP_JOIN_PASSWORD
	ErrGroupJoinCore      GroupJoinError = C.TOX_ERR_GROUP_JOIN_CORE
)

func (e GroupJoinError) Error() string {
	return errorString(C.tox_err_group_join_to_string(C.Tox_Err_Group_Join(e)), "TOX_ERR_GROUP_JOIN_", uint32(e))
}

type GroupIsConnectedError uint32

const ErrGroupIsConnectedGroupNotFound GroupIsConnectedError = C.TOX_ERR_GROUP_IS_CONNECTED_GROUP_NOT_FOUND

func (e GroupIsConnectedError) Error() string {
	return errorString(C.tox_err_group_is_connected_to_string(C.Tox_Err_Group_Is_Connected(e)), "TOX_ERR_GROUP_IS_CONNECTED_", uint32(e))
}

type GroupDisconnectError uint32

const (
	ErrGroupDisconnectGroupNotFound       GroupDisconnectError = C.TOX_ERR_GROUP_DISCONNECT_GROUP_NOT_FOUND
	ErrGroupDisconnectAlreadyDisconnected GroupDisconnectError = C.TOX_ERR_GROUP_DISCONNECT_ALREADY_DISCONNECTED
)

func (e GroupDisconnectError) Error() string {
	return errorString(C.tox_err_group_disconnect_to_string(C.Tox_Err_Group_Disconnect(e)), "TOX_ERR_GROUP_DISCONNECT_", uint32(e))
}

type GroupLeaveError uint32

const (
	ErrGroupLeaveGroupNotFound GroupLeaveError = C.TOX_ERR_GROUP_LEAVE_GROUP_NOT_FOUND
	ErrGroupLeaveTooLong       GroupLeaveError = C.TOX_ERR_GROUP_LEAVE_TOO_LONG
	ErrGroupLeaveFailSend      GroupLeaveError = C.TOX_ERR_GROUP_LEAVE_FAIL_SEND
)

func (e GroupLeaveError) Error() string {
	return errorString(C.tox_err_group_leave_to_string(C.Tox_Err_Group_Leave(e)), "TOX_ERR_GROUP_LEAVE_", uint32(e))
}

type GroupSelfQueryError uint32

const ErrGroupSelfQueryGroupNotFound GroupSelfQueryError = C.TOX_ERR_GROUP_SELF_QUERY_GROUP_NOT_FOUND

func (e GroupSelfQueryError) Error() string {
	return errorString(C.tox_err_group_self_query_to_string(C.Tox_Err_Group_Self_Query(e)), "TOX_ERR_GROUP_SELF_QUERY_", uint32(e))
}

type GroupSelfNameSetError uint32

const (
	ErrGroupSelfNameSetGroupNotFound GroupSelfNameSetError = C.TOX_ERR_GROUP_SELF_NAME_SET_GROUP_NOT_FOUND
	ErrGroupSelfNameSetTooLong       GroupSelfNameSetError = C.TOX_ERR_GROUP_SELF_NAME_SET_TOO_LONG
	ErrGroupSelfNameSetInvalid       GroupSelfNameSetError = C.TOX_ERR_GROUP_SELF_NAME_SET_INVALID
	ErrGroupSelfNameSetFailSend      GroupSelfNameSetError = C.TOX_ERR_GROUP_SELF_NAME_SET_FAIL_SEND
)

func (e GroupSelfNameSetError) Error() string {
	return errorString(C.tox_err_group_self_name_set_to_string(C.Tox_Err_Group_Self_Name_Set(e)), "TOX_ERR_GROUP_SELF_NAME_SET_", uint32(e))
}

type GroupSelfStatusSetError uint32

const (
	ErrGroupSelfStatusSetGroupNotFound GroupSelfStatusSetError = C.TOX_ERR_GROUP_SELF_STATUS_SET_GROUP_NOT_FOUND
	ErrGroupSelfStatusSetFailSend      GroupSelfStatusSetError = C.TOX_ERR_GROUP_SELF_STATUS_SET_FAIL_SEND
)

func (e GroupSelfStatusSetError) Error() string {
	return errorString(C.tox_err_group_self_status_set_to_string(C.Tox_Err_Group_Self_Status_Set(e)), "TOX_ERR_GROUP_SELF_STATUS_SET_", uint32(e))
}

type GroupPeerQueryError uint32

const (
	ErrGroupPeerQueryGroupNotFound GroupPeerQueryError = C.TOX_ERR_GROUP_PEER_QUERY_GROUP_NOT_FOUND
	ErrGroupPeerQueryPeerNotFound  GroupPeerQueryError = C.TOX_ERR_GROUP_PEER_QUERY_PEER_NOT_FOUND
)

func (e GroupPeerQueryError) Error() string {
	return errorString(C.tox_err_group_peer_query_to_string(C.Tox_Err_Group_Peer_Query(e)), "TOX_ERR_GROUP_PEER_QUERY_", uint32(e))
}

type GroupStateQueryError uint32

const ErrGroupStateQueryGroupNotFound GroupStateQueryError = C.TOX_ERR_GROUP_STATE_QUERY_GROUP_NOT_FOUND

func (e GroupStateQueryError) Error() string {
	return errorString(C.tox_err_group_state_query_to_string(C.Tox_Err_Group_State_Query(e)), "TOX_ERR_GROUP_STATE_QUERY_", uint32(e))
}

type GroupTopicSetError uint32

const (
	ErrGroupTopicSetGroupNotFound GroupTopicSetError = C.TOX_ERR_GROUP_TOPIC_SET_GROUP_NOT_FOUND
	ErrGroupTopicSetTooLong       GroupTopicSetError = C.TOX_ERR_GROUP_TOPIC_SET_TOO_LONG
	ErrGroupTopicSetPermissions   GroupTopicSetError = C.TOX_ERR_GROUP_TOPIC_SET_PERMISSIONS
	ErrGroupTopicSetFailCreate    GroupTopicSetError = C.TOX_ERR_GROUP_TOPIC_SET_FAIL_CREATE
	ErrGroupTopicSetFailSend      GroupTopicSetError = C.TOX_ERR_GROUP_TOPIC_SET_FAIL_SEND
	ErrGroupTopicSetDisconnected  GroupTopicSetError = C.TOX_ERR_GROUP_TOPIC_SET_DISCONNECTED
)

func (e GroupTopicSetError) Error() string {
	return errorString(C.tox_err_group_topic_set_to_string(C.Tox_Err_Group_Topic_Set(e)), "TOX_ERR_GROUP_TOPIC_SET_", uint32(e))
}

type GroupByIDError uint32

const ErrGroupByIDNotFound GroupByIDError = C.TOX_ERR_GROUP_BY_ID_NOT_FOUND

func (e GroupByIDError) Error() string {
	return errorString(C.tox_err_group_by_id_to_string(C.Tox_Err_Group_By_Id(e)), "TOX_ERR_GROUP_BY_ID_", uint32(e))
}

type GroupSendMessageError uint32

const (
	ErrGroupSendMessageGroupNotFound GroupSendMessageError = C.TOX_ERR_GROUP_SEND_MESSAGE_GROUP_NOT_FOUND
	ErrGroupSendMessageTooLong       GroupSendMessageError = C.TOX_ERR_GROUP_SEND_MESSAGE_TOO_LONG
	ErrGroupSendMessageEmpty         GroupSendMessageError = C.TOX_ERR_GROUP_SEND_MESSAGE_EMPTY
	ErrGroupSendMessageBadType       GroupSendMessageError = C.TOX_ERR_GROUP_SEND_MESSAGE_BAD_TYPE
	ErrGroupSendMessagePermissions   GroupSendMessageError = C.TOX_ERR_GROUP_SEND_MESSAGE_PERMISSIONS
	ErrGroupSendMessageFailSend      GroupSendMessageError = C.TOX_ERR_GROUP_SEND_MESSAGE_FAIL_SEND
	ErrGroupSendMessageDisconnected  GroupSendMessageError = C.TOX_ERR_GROUP_SEND_MESSAGE_DISCONNECTED
)

func (e GroupSendMessageError) Error() string {
	return errorString(C.tox_err_group_send_message_to_string(C.Tox_Err_Group_Send_Message(e)), "TOX_ERR_GROUP_SEND_MESSAGE_", uint32(e))
}

type GroupSendPrivateMessageError uint32

const (
	ErrGroupSendPrivateMessageGroupNotFound GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_GROUP_NOT_FOUND
	ErrGroupSendPrivateMessagePeerNotFound  GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_PEER_NOT_FOUND
	ErrGroupSendPrivateMessageTooLong       GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_TOO_LONG
	ErrGroupSendPrivateMessageEmpty         GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_EMPTY
	ErrGroupSendPrivateMessageBadType       GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_BAD_TYPE
	ErrGroupSendPrivateMessagePermissions   GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_PERMISSIONS
	ErrGroupSendPrivateMessageFailSend      GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_FAIL_SEND
	ErrGroupSendPrivateMessageDisconnected  GroupSendPrivateMessageError = C.TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_DISCONNECTED
)

func (e GroupSendPrivateMessageError) Error() string {
	return errorString(C.tox_err_group_send_private_message_to_string(C.Tox_Err_Group_Send_Private_Message(e)), "TOX_ERR_GROUP_SEND_PRIVATE_MESSAGE_", uint32(e))
}

type GroupSendCustomPacketError uint32

const (
	ErrGroupSendCustomPacketGroupNotFound GroupSendCustomPacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PACKET_GROUP_NOT_FOUND
	ErrGroupSendCustomPacketTooLong       GroupSendCustomPacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PACKET_TOO_LONG
	ErrGroupSendCustomPacketEmpty         GroupSendCustomPacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PACKET_EMPTY
	ErrGroupSendCustomPacketDisconnected  GroupSendCustomPacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PACKET_DISCONNECTED
	ErrGroupSendCustomPacketFailSend      GroupSendCustomPacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PACKET_FAIL_SEND
)

func (e GroupSendCustomPacketError) Error() string {
	return errorString(C.tox_err_group_send_custom_packet_to_string(C.Tox_Err_Group_Send_Custom_Packet(e)), "TOX_ERR_GROUP_SEND_CUSTOM_PACKET_", uint32(e))
}

type GroupSendCustomPrivatePacketError uint32

const (
	ErrGroupSendCustomPrivatePacketGroupNotFound GroupSendCustomPrivatePacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PRIVATE_PACKET_GROUP_NOT_FOUND
	ErrGroupSendCustomPrivatePacketTooLong       GroupSendCustomPrivatePacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PRIVATE_PACKET_TOO_LONG
	ErrGroupSendCustomPrivatePacketEmpty         GroupSendCustomPrivatePacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PRIVATE_PACKET_EMPTY
	ErrGroupSendCustomPrivatePacketPeerNotFound  GroupSendCustomPrivatePacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PRIVATE_PACKET_PEER_NOT_FOUND
	ErrGroupSendCustomPrivatePacketFailSend      GroupSendCustomPrivatePacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PRIVATE_PACKET_FAIL_SEND
	ErrGroupSendCustomPrivatePacketDisconnected  GroupSendCustomPrivatePacketError = C.TOX_ERR_GROUP_SEND_CUSTOM_PRIVATE_PACKET_DISCONNECTED
)

func (e GroupSendCustomPrivatePacketError) Error() string {
	return errorString(C.tox_err_group_send_custom_private_packet_to_string(C.Tox_Err_Group_Send_Custom_Private_Packet(e)), "TOX_ERR_GROUP_SEND_CUSTOM_PRIVATE_PACKET_", uint32(e))
}

type GroupInviteFriendError uint32

const (
	ErrGroupInviteFriendGroupNotFound  GroupInviteFriendError = C.TOX_ERR_GROUP_INVITE_FRIEND_GROUP_NOT_FOUND
	ErrGroupInviteFriendFriendNotFound GroupInviteFriendError = C.TOX_ERR_GROUP_INVITE_FRIEND_FRIEND_NOT_FOUND
	ErrGroupInviteFriendInviteFail     GroupInviteFriendError = C.TOX_ERR_GROUP_INVITE_FRIEND_INVITE_FAIL
	ErrGroupInviteFriendFailSend       GroupInviteFriendError = C.TOX_ERR_GROUP_INVITE_FRIEND_FAIL_SEND
	ErrGroupInviteFriendDisconnected   GroupInviteFriendError = C.TOX_ERR_GROUP_INVITE_FRIEND_DISCONNECTED
)

func (e GroupInviteFriendError) Error() string {
	return errorString(C.tox_err_group_invite_friend_to_string(C.Tox_Err_Group_Invite_Friend(e)), "TOX_ERR_GROUP_INVITE_FRIEND_", uint32(e))
}

type GroupInviteAcceptError uint32

const (
	ErrGroupInviteAcceptBadInvite      GroupInviteAcceptError = C.TOX_ERR_GROUP_INVITE_ACCEPT_BAD_INVITE
	ErrGroupInviteAcceptInitFailed     GroupInviteAcceptError = C.TOX_ERR_GROUP_INVITE_ACCEPT_INIT_FAILED
	ErrGroupInviteAcceptTooLong        GroupInviteAcceptError = C.TOX_ERR_GROUP_INVITE_ACCEPT_TOO_LONG
	ErrGroupInviteAcceptEmpty          GroupInviteAcceptError = C.TOX_ERR_GROUP_INVITE_ACCEPT_EMPTY
	ErrGroupInviteAcceptPassword       GroupInviteAcceptError = C.TOX_ERR_GROUP_INVITE_ACCEPT_PASSWORD
	ErrGroupInviteAcceptFriendNotFound GroupInviteAcceptError = C.TOX_ERR_GROUP_INVITE_ACCEPT_FRIEND_NOT_FOUND
	ErrGroupInviteAcceptFailSend       GroupInviteAcceptError = C.TOX_ERR_GROUP_INVITE_ACCEPT_FAIL_SEND
)

func (e GroupInviteAcceptError) Error() string {
	return errorString(C.tox_err_group_invite_accept_to_string(C.Tox_Err_Group_Invite_Accept(e)), "TOX_ERR_GROUP_INVITE_ACCEPT_", uint32(e))
}

type GroupSetPasswordError uint32

const (
	ErrGroupSetPasswordGroupNotFound GroupSetPasswordError = C.TOX_ERR_GROUP_SET_PASSWORD_GROUP_NOT_FOUND
	ErrGroupSetPasswordPermissions   GroupSetPasswordError = C.TOX_ERR_GROUP_SET_PASSWORD_PERMISSIONS
	ErrGroupSetPasswordTooLong       GroupSetPasswordError = C.TOX_ERR_GROUP_SET_PASSWORD_TOO_LONG
	ErrGroupSetPasswordFailSend      GroupSetPasswordError = C.TOX_ERR_GROUP_SET_PASSWORD_FAIL_SEND
	ErrGroupSetPasswordMalloc        GroupSetPasswordError = C.TOX_ERR_GROUP_SET_PASSWORD_MALLOC
	ErrGroupSetPasswordDisconnected  GroupSetPasswordError = C.TOX_ERR_GROUP_SET_PASSWORD_DISCONNECTED
)

func (e GroupSetPasswordError) Error() string {
	return errorString(C.tox_err_group_set_password_to_string(C.Tox_Err_Group_Set_Password(e)), "TOX_ERR_GROUP_SET_PASSWORD_", uint32(e))
}

type GroupSetTopicLockError uint32

const (
	ErrGroupSetTopicLockGroupNotFound GroupSetTopicLockError = C.TOX_ERR_GROUP_SET_TOPIC_LOCK_GROUP_NOT_FOUND
	ErrGroupSetTopicLockInvalid       GroupSetTopicLockError = C.TOX_ERR_GROUP_SET_TOPIC_LOCK_INVALID
	ErrGroupSetTopicLockPermissions   GroupSetTopicLockError = C.TOX_ERR_GROUP_SET_TOPIC_LOCK_PERMISSIONS
	ErrGroupSetTopicLockFailSet       GroupSetTopicLockError = C.TOX_ERR_GROUP_SET_TOPIC_LOCK_FAIL_SET
	ErrGroupSetTopicLockFailSend      GroupSetTopicLockError = C.TOX_ERR_GROUP_SET_TOPIC_LOCK_FAIL_SEND
	ErrGroupSetTopicLockDisconnected  GroupSetTopicLockError = C.TOX_ERR_GROUP_SET_TOPIC_LOCK_DISCONNECTED
)

func (e GroupSetTopicLockError) Error() string {
	return errorString(C.tox_err_group_set_topic_lock_to_string(C.Tox_Err_Group_Set_Topic_Lock(e)), "TOX_ERR_GROUP_SET_TOPIC_LOCK_", uint32(e))
}

type GroupSetVoiceStateError uint32

const (
	ErrGroupSetVoiceStateGroupNotFound GroupSetVoiceStateError = C.TOX_ERR_GROUP_SET_VOICE_STATE_GROUP_NOT_FOUND
	ErrGroupSetVoiceStatePermissions   GroupSetVoiceStateError = C.TOX_ERR_GROUP_SET_VOICE_STATE_PERMISSIONS
	ErrGroupSetVoiceStateFailSet       GroupSetVoiceStateError = C.TOX_ERR_GROUP_SET_VOICE_STATE_FAIL_SET
	ErrGroupSetVoiceStateFailSend      GroupSetVoiceStateError = C.TOX_ERR_GROUP_SET_VOICE_STATE_FAIL_SEND
	ErrGroupSetVoiceStateDisconnected  GroupSetVoiceStateError = C.TOX_ERR_GROUP_SET_VOICE_STATE_DISCONNECTED
)

func (e GroupSetVoiceStateError) Error() string {
	return errorString(C.tox_err_group_set_voice_state_to_string(C.Tox_Err_Group_Set_Voice_State(e)), "TOX_ERR_GROUP_SET_VOICE_STATE_", uint32(e))
}

type GroupSetPrivacyStateError uint32

const (
	ErrGroupSetPrivacyStateGroupNotFound GroupSetPrivacyStateError = C.TOX_ERR_GROUP_SET_PRIVACY_STATE_GROUP_NOT_FOUND
	ErrGroupSetPrivacyStatePermissions   GroupSetPrivacyStateError = C.TOX_ERR_GROUP_SET_PRIVACY_STATE_PERMISSIONS
	ErrGroupSetPrivacyStateFailSet       GroupSetPrivacyStateError = C.TOX_ERR_GROUP_SET_PRIVACY_STATE_FAIL_SET
	ErrGroupSetPrivacyStateFailSend      GroupSetPrivacyStateError = C.TOX_ERR_GROUP_SET_PRIVACY_STATE_FAIL_SEND
	ErrGroupSetPrivacyStateDisconnected  GroupSetPrivacyStateError = C.TOX_ERR_GROUP_SET_PRIVACY_STATE_DISCONNECTED
)

func (e GroupSetPrivacyStateError) Error() string {
	return errorString(C.tox_err_group_set_privacy_state_to_string(C.Tox_Err_Group_Set_Privacy_State(e)), "TOX_ERR_GROUP_SET_PRIVACY_STATE_", uint32(e))
}

type GroupSetPeerLimitError uint32

const (
	ErrGroupSetPeerLimitGroupNotFound GroupSetPeerLimitError = C.TOX_ERR_GROUP_SET_PEER_LIMIT_GROUP_NOT_FOUND
	ErrGroupSetPeerLimitPermissions   GroupSetPeerLimitError = C.TOX_ERR_GROUP_SET_PEER_LIMIT_PERMISSIONS
	ErrGroupSetPeerLimitFailSet       GroupSetPeerLimitError = C.TOX_ERR_GROUP_SET_PEER_LIMIT_FAIL_SET
	ErrGroupSetPeerLimitFailSend      GroupSetPeerLimitError = C.TOX_ERR_GROUP_SET_PEER_LIMIT_FAIL_SEND
	ErrGroupSetPeerLimitDisconnected  GroupSetPeerLimitError = C.TOX_ERR_GROUP_SET_PEER_LIMIT_DISCONNECTED
)

func (e GroupSetPeerLimitError) Error() string {
	return errorString(C.tox_err_group_set_peer_limit_to_string(C.Tox_Err_Group_Set_Peer_Limit(e)), "TOX_ERR_GROUP_SET_PEER_LIMIT_", uint32(e))
}

type GroupSetIgnoreError uint32

const (
	ErrGroupSetIgnoreGroupNotFound GroupSetIgnoreError = C.TOX_ERR_GROUP_SET_IGNORE_GROUP_NOT_FOUND
	ErrGroupSetIgnorePeerNotFound  GroupSetIgnoreError = C.TOX_ERR_GROUP_SET_IGNORE_PEER_NOT_FOUND
	ErrGroupSetIgnoreSelf          GroupSetIgnoreError = C.TOX_ERR_GROUP_SET_IGNORE_SELF
)

func (e GroupSetIgnoreError) Error() string {
	return errorString(C.tox_err_group_set_ignore_to_string(C.Tox_Err_Group_Set_Ignore(e)), "TOX_ERR_GROUP_SET_IGNORE_", uint32(e))
}

type GroupSetRoleError uint32

const (
	ErrGroupSetRoleGroupNotFound GroupSetRoleError = C.TOX_ERR_GROUP_SET_ROLE_GROUP_NOT_FOUND
	ErrGroupSetRolePeerNotFound  GroupSetRoleError = C.TOX_ERR_GROUP_SET_ROLE_PEER_NOT_FOUND
	ErrGroupSetRolePermissions   GroupSetRoleError = C.TOX_ERR_GROUP_SET_ROLE_PERMISSIONS
	ErrGroupSetRoleAssignment    GroupSetRoleError = C.TOX_ERR_GROUP_SET_ROLE_ASSIGNMENT
	ErrGroupSetRoleFailAction    GroupSetRoleError = C.TOX_ERR_GROUP_SET_ROLE_FAIL_ACTION
	ErrGroupSetRoleSelf          GroupSetRoleError = C.TOX_ERR_GROUP_SET_ROLE_SELF
)

func (e GroupSetRoleError) Error() string {
	return errorString(C.tox_err_group_set_role_to_string(C.Tox_Err_Group_Set_Role(e)), "TOX_ERR_GROUP_SET_ROLE_", uint32(e))
}

type GroupKickPeerError uint32

const (
	ErrGroupKickPeerGroupNotFound GroupKickPeerError = C.TOX_ERR_GROUP_KICK_PEER_GROUP_NOT_FOUND
	ErrGroupKickPeerPeerNotFound  GroupKickPeerError = C.TOX_ERR_GROUP_KICK_PEER_PEER_NOT_FOUND
	ErrGroupKickPeerPermissions   GroupKickPeerError = C.TOX_ERR_GROUP_KICK_PEER_PERMISSIONS
	ErrGroupKickPeerFailAction    GroupKickPeerError = C.TOX_ERR_GROUP_KICK_PEER_FAIL_ACTION
	ErrGroupKickPeerFailSend      GroupKickPeerError = C.TOX_ERR_GROUP_KICK_PEER_FAIL_SEND
	ErrGroupKickPeerSelf          GroupKickPeerError = C.TOX_ERR_GROUP_KICK_PEER_SELF
)

func (e GroupKickPeerError) Error() string {
	return errorString(C.tox_err_group_kick_peer_to_string(C.Tox_Err_Group_Kick_Peer(e)), "TOX_ERR_GROUP_KICK_PEER_", uint32(e))
}
