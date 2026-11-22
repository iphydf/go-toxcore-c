package tox

/*
#include <tox/tox.h>
*/
import "C"

import "fmt"

const (
	GroupMaxTopicLength                = C.TOX_GROUP_MAX_TOPIC_LENGTH
	GroupMaxPartLength                 = C.TOX_GROUP_MAX_PART_LENGTH
	GroupMaxMessageLength              = C.TOX_GROUP_MAX_MESSAGE_LENGTH
	GroupMaxCustomLossyPacketLength    = C.TOX_GROUP_MAX_CUSTOM_LOSSY_PACKET_LENGTH
	GroupMaxCustomLosslessPacketLength = C.TOX_GROUP_MAX_CUSTOM_LOSSLESS_PACKET_LENGTH
	GroupMaxGroupNameLength            = C.TOX_GROUP_MAX_GROUP_NAME_LENGTH
	GroupMaxPasswordSize               = C.TOX_GROUP_MAX_PASSWORD_SIZE
)

type GroupChatID [C.TOX_GROUP_CHAT_ID_SIZE]byte

func (id GroupChatID) String() string { return hexString(id[:]) }

func ParseGroupChatID(s string) (GroupChatID, error) {
	var id GroupChatID
	return id, parseHex(id[:], s, "group chat ID")
}

type GroupNumber uint32

type GroupPeerNumber uint32

type GroupMessageID uint32

type GroupPrivacyState uint32

const (
	GroupPrivacyStatePublic  GroupPrivacyState = C.TOX_GROUP_PRIVACY_STATE_PUBLIC
	GroupPrivacyStatePrivate GroupPrivacyState = C.TOX_GROUP_PRIVACY_STATE_PRIVATE
)

func (s GroupPrivacyState) String() string {
	return enumString(C.tox_group_privacy_state_to_string(C.Tox_Group_Privacy_State(s)), "TOX_GROUP_PRIVACY_STATE_", "GroupPrivacyState", uint32(s))
}

type GroupTopicLock uint32

const (
	GroupTopicLockEnabled  GroupTopicLock = C.TOX_GROUP_TOPIC_LOCK_ENABLED
	GroupTopicLockDisabled GroupTopicLock = C.TOX_GROUP_TOPIC_LOCK_DISABLED
)

func (l GroupTopicLock) String() string {
	return enumString(C.tox_group_topic_lock_to_string(C.Tox_Group_Topic_Lock(l)), "TOX_GROUP_TOPIC_LOCK_", "GroupTopicLock", uint32(l))
}

type GroupVoiceState uint32

const (
	GroupVoiceStateAll       GroupVoiceState = C.TOX_GROUP_VOICE_STATE_ALL
	GroupVoiceStateModerator GroupVoiceState = C.TOX_GROUP_VOICE_STATE_MODERATOR
	GroupVoiceStateFounder   GroupVoiceState = C.TOX_GROUP_VOICE_STATE_FOUNDER
)

func (s GroupVoiceState) String() string {
	return enumString(C.tox_group_voice_state_to_string(C.Tox_Group_Voice_State(s)), "TOX_GROUP_VOICE_STATE_", "GroupVoiceState", uint32(s))
}

type GroupRole uint32

const (
	GroupRoleFounder   GroupRole = C.TOX_GROUP_ROLE_FOUNDER
	GroupRoleModerator GroupRole = C.TOX_GROUP_ROLE_MODERATOR
	GroupRoleUser      GroupRole = C.TOX_GROUP_ROLE_USER
	GroupRoleObserver  GroupRole = C.TOX_GROUP_ROLE_OBSERVER
)

func (r GroupRole) String() string {
	return enumString(C.tox_group_role_to_string(C.Tox_Group_Role(r)), "TOX_GROUP_ROLE_", "GroupRole", uint32(r))
}

type Group struct {
	tox    *Tox
	number GroupNumber
}

func (t *Tox) Group(number GroupNumber) Group {
	return Group{t, number}
}

func (g Group) Number() GroupNumber {
	return g.number
}

func (g Group) String() string {
	return fmt.Sprintf("group %d", g.number)
}

func (g Group) c() C.Tox_Group_Number {
	return C.Tox_Group_Number(g.number)
}

type GroupPeer struct {
	group  Group
	number GroupPeerNumber
}

func (g Group) Peer(number GroupPeerNumber) GroupPeer {
	return GroupPeer{g, number}
}

func (gp GroupPeer) Group() Group {
	return gp.group
}

func (gp GroupPeer) Number() GroupPeerNumber {
	return gp.number
}

func (gp GroupPeer) String() string {
	return fmt.Sprintf("peer %d of group %d", gp.number, gp.group.number)
}

func (gp GroupPeer) c() C.Tox_Group_Peer_Number {
	return C.Tox_Group_Peer_Number(gp.number)
}

func (t *Tox) GroupNew(privacy GroupPrivacyState, groupName, name string) (Group, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Group_New
	n := C.tox_group_new(p, C.Tox_Group_Privacy_State(privacy), cString(groupName), C.size_t(len(groupName)),
		cString(name), C.size_t(len(name)), &err)
	return Group{t, GroupNumber(n)}, toError[GroupNewError](uint32(err))
}

func (t *Tox) GroupJoin(chatID GroupChatID, name, password string) (Group, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Group_Join
	n := C.tox_group_join(p, cBytes(chatID[:]), cString(name), C.size_t(len(name)),
		cString(password), C.size_t(len(password)), &err)
	return Group{t, GroupNumber(n)}, toError[GroupJoinError](uint32(err))
}

func (t *Tox) GroupInviteAccept(friend Friend, inviteData []byte, name, password string) (Group, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Group_Invite_Accept
	n := C.tox_group_invite_accept(p, friend.c(), cBytes(inviteData), C.size_t(len(inviteData)),
		cString(name), C.size_t(len(name)), cString(password), C.size_t(len(password)), &err)
	return Group{t, GroupNumber(n)}, toError[GroupInviteAcceptError](uint32(err))
}

func (t *Tox) GroupByID(chatID GroupChatID) (Group, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Group_By_Id
	n := C.tox_group_by_id(p, cBytes(chatID[:]), &err)
	return Group{t, GroupNumber(n)}, toError[GroupByIDError](uint32(err))
}

func (t *Tox) GroupList() []Group {
	p := t.lock()
	defer t.h.Unlock()
	numbers := make([]C.Tox_Group_Number, C.tox_group_get_group_list_size(p))
	if len(numbers) == 0 {
		return nil
	}
	C.tox_group_get_group_list(p, &numbers[0])
	groups := make([]Group, len(numbers))
	for i, n := range numbers {
		groups[i] = Group{t, GroupNumber(n)}
	}
	return groups
}

func (g Group) IsConnected() (bool, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Is_Connected
	connected := C.tox_group_is_connected(p, g.c(), &err)
	return bool(connected), toError[GroupIsConnectedError](uint32(err))
}

func (g Group) Disconnect() error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Disconnect
	C.tox_group_disconnect(p, g.c(), &err)
	return toError[GroupDisconnectError](uint32(err))
}

func (g Group) Leave(partMessage string) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Leave
	C.tox_group_leave(p, g.c(), cString(partMessage), C.size_t(len(partMessage)), &err)
	return toError[GroupLeaveError](uint32(err))
}

func (g Group) SelfSetName(name string) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Self_Name_Set
	C.tox_group_self_set_name(p, g.c(), cString(name), C.size_t(len(name)), &err)
	return toError[GroupSelfNameSetError](uint32(err))
}

func (g Group) SelfName() (string, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Self_Query
	size := C.tox_group_self_get_name_size(p, g.c(), &err)
	if err := toError[GroupSelfQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_group_self_get_name(p, g.c(), buf, nil)
	}), nil
}

func (g Group) SelfSetStatus(status UserStatus) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Self_Status_Set
	C.tox_group_self_set_status(p, g.c(), C.Tox_User_Status(status), &err)
	return toError[GroupSelfStatusSetError](uint32(err))
}

func (g Group) SelfStatus() (UserStatus, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Self_Query
	status := C.tox_group_self_get_status(p, g.c(), &err)
	return UserStatus(status), toError[GroupSelfQueryError](uint32(err))
}

func (g Group) SelfRole() (GroupRole, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Self_Query
	role := C.tox_group_self_get_role(p, g.c(), &err)
	return GroupRole(role), toError[GroupSelfQueryError](uint32(err))
}

func (g Group) SelfPeer() (GroupPeer, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Self_Query
	n := C.tox_group_self_get_peer_id(p, g.c(), &err)
	return GroupPeer{g, GroupPeerNumber(n)}, toError[GroupSelfQueryError](uint32(err))
}

func (g Group) SelfPublicKey() (PublicKey, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var k PublicKey
	var err C.Tox_Err_Group_Self_Query
	C.tox_group_self_get_public_key(p, g.c(), cOut(k[:]), &err)
	return k, toError[GroupSelfQueryError](uint32(err))
}

func (g Group) SetTopic(topic string) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Topic_Set
	C.tox_group_set_topic(p, g.c(), cString(topic), C.size_t(len(topic)), &err)
	return toError[GroupTopicSetError](uint32(err))
}

func (g Group) Topic() (string, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_State_Query
	size := C.tox_group_get_topic_size(p, g.c(), &err)
	if err := toError[GroupStateQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_group_get_topic(p, g.c(), buf, nil)
	}), nil
}

func (g Group) Name() (string, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_State_Query
	size := C.tox_group_get_name_size(p, g.c(), &err)
	if err := toError[GroupStateQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_group_get_name(p, g.c(), buf, nil)
	}), nil
}

func (g Group) ChatID() (GroupChatID, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var id GroupChatID
	var err C.Tox_Err_Group_State_Query
	C.tox_group_get_chat_id(p, g.c(), cOut(id[:]), &err)
	return id, toError[GroupStateQueryError](uint32(err))
}

func (g Group) PrivacyState() (GroupPrivacyState, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_State_Query
	state := C.tox_group_get_privacy_state(p, g.c(), &err)
	return GroupPrivacyState(state), toError[GroupStateQueryError](uint32(err))
}

func (g Group) VoiceState() (GroupVoiceState, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_State_Query
	state := C.tox_group_get_voice_state(p, g.c(), &err)
	return GroupVoiceState(state), toError[GroupStateQueryError](uint32(err))
}

func (g Group) TopicLock() (GroupTopicLock, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_State_Query
	lock := C.tox_group_get_topic_lock(p, g.c(), &err)
	return GroupTopicLock(lock), toError[GroupStateQueryError](uint32(err))
}

func (g Group) PeerLimit() (uint16, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_State_Query
	limit := C.tox_group_get_peer_limit(p, g.c(), &err)
	return uint16(limit), toError[GroupStateQueryError](uint32(err))
}

func (g Group) Password() (string, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_State_Query
	size := C.tox_group_get_password_size(p, g.c(), &err)
	if err := toError[GroupStateQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_group_get_password(p, g.c(), buf, nil)
	}), nil
}

func (g Group) SendMessage(kind MessageType, message string) (GroupMessageID, error) {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Send_Message
	id := C.tox_group_send_message(p, g.c(), C.Tox_Message_Type(kind), cString(message), C.size_t(len(message)), &err)
	return GroupMessageID(id), toError[GroupSendMessageError](uint32(err))
}

func (g Group) SendCustomPacket(lossless bool, data []byte) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Send_Custom_Packet
	C.tox_group_send_custom_packet(p, g.c(), C.bool(lossless), cBytes(data), C.size_t(len(data)), &err)
	return toError[GroupSendCustomPacketError](uint32(err))
}

func (g Group) InviteFriend(friend Friend) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Invite_Friend
	C.tox_group_invite_friend(p, g.c(), friend.c(), &err)
	return toError[GroupInviteFriendError](uint32(err))
}

func (g Group) SetPassword(password string) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Set_Password
	C.tox_group_set_password(p, g.c(), cString(password), C.size_t(len(password)), &err)
	return toError[GroupSetPasswordError](uint32(err))
}

func (g Group) SetTopicLock(lock GroupTopicLock) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Set_Topic_Lock
	C.tox_group_set_topic_lock(p, g.c(), C.Tox_Group_Topic_Lock(lock), &err)
	return toError[GroupSetTopicLockError](uint32(err))
}

func (g Group) SetVoiceState(state GroupVoiceState) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Set_Voice_State
	C.tox_group_set_voice_state(p, g.c(), C.Tox_Group_Voice_State(state), &err)
	return toError[GroupSetVoiceStateError](uint32(err))
}

func (g Group) SetPrivacyState(state GroupPrivacyState) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Set_Privacy_State
	C.tox_group_set_privacy_state(p, g.c(), C.Tox_Group_Privacy_State(state), &err)
	return toError[GroupSetPrivacyStateError](uint32(err))
}

func (g Group) SetPeerLimit(limit uint16) error {
	p := g.tox.lock()
	defer g.tox.h.Unlock()
	var err C.Tox_Err_Group_Set_Peer_Limit
	C.tox_group_set_peer_limit(p, g.c(), C.uint16_t(limit), &err)
	return toError[GroupSetPeerLimitError](uint32(err))
}

func (gp GroupPeer) Name() (string, error) {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Peer_Query
	size := C.tox_group_peer_get_name_size(p, gp.group.c(), gp.c(), &err)
	if err := toError[GroupPeerQueryError](uint32(err)); err != nil {
		return "", err
	}
	return readString(size, func(buf *C.uint8_t) {
		C.tox_group_peer_get_name(p, gp.group.c(), gp.c(), buf, nil)
	}), nil
}

func (gp GroupPeer) Status() (UserStatus, error) {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Peer_Query
	status := C.tox_group_peer_get_status(p, gp.group.c(), gp.c(), &err)
	return UserStatus(status), toError[GroupPeerQueryError](uint32(err))
}

func (gp GroupPeer) Role() (GroupRole, error) {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Peer_Query
	role := C.tox_group_peer_get_role(p, gp.group.c(), gp.c(), &err)
	return GroupRole(role), toError[GroupPeerQueryError](uint32(err))
}

func (gp GroupPeer) ConnectionStatus() (Connection, error) {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Peer_Query
	c := C.tox_group_peer_get_connection_status(p, gp.group.c(), gp.c(), &err)
	return Connection(c), toError[GroupPeerQueryError](uint32(err))
}

func (gp GroupPeer) PublicKey() (PublicKey, error) {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var k PublicKey
	var err C.Tox_Err_Group_Peer_Query
	C.tox_group_peer_get_public_key(p, gp.group.c(), gp.c(), cOut(k[:]), &err)
	return k, toError[GroupPeerQueryError](uint32(err))
}

func (gp GroupPeer) SendPrivateMessage(kind MessageType, message string) (GroupMessageID, error) {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Send_Private_Message
	id := C.tox_group_send_private_message(p, gp.group.c(), gp.c(), C.Tox_Message_Type(kind), cString(message),
		C.size_t(len(message)), &err)
	return GroupMessageID(id), toError[GroupSendPrivateMessageError](uint32(err))
}

func (gp GroupPeer) SendCustomPrivatePacket(lossless bool, data []byte) error {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Send_Custom_Private_Packet
	C.tox_group_send_custom_private_packet(p, gp.group.c(), gp.c(), C.bool(lossless), cBytes(data), C.size_t(len(data)), &err)
	return toError[GroupSendCustomPrivatePacketError](uint32(err))
}

func (gp GroupPeer) SetIgnore(ignore bool) error {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Set_Ignore
	C.tox_group_set_ignore(p, gp.group.c(), gp.c(), C.bool(ignore), &err)
	return toError[GroupSetIgnoreError](uint32(err))
}

func (gp GroupPeer) SetRole(role GroupRole) error {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Set_Role
	C.tox_group_set_role(p, gp.group.c(), gp.c(), C.Tox_Group_Role(role), &err)
	return toError[GroupSetRoleError](uint32(err))
}

func (gp GroupPeer) Kick() error {
	p := gp.group.tox.lock()
	defer gp.group.tox.h.Unlock()
	var err C.Tox_Err_Group_Kick_Peer
	C.tox_group_kick_peer(p, gp.group.c(), gp.c(), &err)
	return toError[GroupKickPeerError](uint32(err))
}

type GroupExitType uint32

const (
	GroupExitTypeQuit             GroupExitType = C.TOX_GROUP_EXIT_TYPE_QUIT
	GroupExitTypeTimeout          GroupExitType = C.TOX_GROUP_EXIT_TYPE_TIMEOUT
	GroupExitTypeDisconnected     GroupExitType = C.TOX_GROUP_EXIT_TYPE_DISCONNECTED
	GroupExitTypeSelfDisconnected GroupExitType = C.TOX_GROUP_EXIT_TYPE_SELF_DISCONNECTED
	GroupExitTypeKick             GroupExitType = C.TOX_GROUP_EXIT_TYPE_KICK
	GroupExitTypeSyncError        GroupExitType = C.TOX_GROUP_EXIT_TYPE_SYNC_ERROR
)

func (t GroupExitType) String() string {
	return enumString(C.tox_group_exit_type_to_string(C.Tox_Group_Exit_Type(t)), "TOX_GROUP_EXIT_TYPE_", "GroupExitType", uint32(t))
}

type GroupJoinFail uint32

const (
	GroupJoinFailPeerLimit       GroupJoinFail = C.TOX_GROUP_JOIN_FAIL_PEER_LIMIT
	GroupJoinFailInvalidPassword GroupJoinFail = C.TOX_GROUP_JOIN_FAIL_INVALID_PASSWORD
	GroupJoinFailUnknown         GroupJoinFail = C.TOX_GROUP_JOIN_FAIL_UNKNOWN
)

func (f GroupJoinFail) String() string {
	return enumString(C.tox_group_join_fail_to_string(C.Tox_Group_Join_Fail(f)), "TOX_GROUP_JOIN_FAIL_", "GroupJoinFail", uint32(f))
}

type GroupModEvent uint32

const (
	GroupModEventKick      GroupModEvent = C.TOX_GROUP_MOD_EVENT_KICK
	GroupModEventObserver  GroupModEvent = C.TOX_GROUP_MOD_EVENT_OBSERVER
	GroupModEventUser      GroupModEvent = C.TOX_GROUP_MOD_EVENT_USER
	GroupModEventModerator GroupModEvent = C.TOX_GROUP_MOD_EVENT_MODERATOR
)

func (e GroupModEvent) String() string {
	return enumString(C.tox_group_mod_event_to_string(C.Tox_Group_Mod_Event(e)), "TOX_GROUP_MOD_EVENT_", "GroupModEvent", uint32(e))
}
