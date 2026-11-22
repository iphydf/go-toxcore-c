package tox

/*
#include <tox/tox.h>
#include <tox/tox_events.h>
*/
import "C"

type EventGroupPeerName struct {
	Peer GroupPeer
	Name string
}

type EventGroupPeerStatus struct {
	Peer   GroupPeer
	Status UserStatus
}

type EventGroupTopic struct {
	Peer  GroupPeer
	Topic string
}

type EventGroupPrivacyState struct {
	Group        Group
	PrivacyState GroupPrivacyState
}

type EventGroupVoiceState struct {
	Group      Group
	VoiceState GroupVoiceState
}

type EventGroupTopicLock struct {
	Group     Group
	TopicLock GroupTopicLock
}

type EventGroupPeerLimit struct {
	Group     Group
	PeerLimit uint32
}

type EventGroupPassword struct {
	Group    Group
	Password string
}

type EventGroupMessage struct {
	Peer        GroupPeer
	MessageType MessageType
	Message     string
	MessageID   GroupMessageID
}

type EventGroupPrivateMessage struct {
	Peer        GroupPeer
	MessageType MessageType
	Message     string
	MessageID   GroupMessageID
}

type EventGroupCustomPacket struct {
	Peer GroupPeer
	Data []byte
}

type EventGroupCustomPrivatePacket struct {
	Peer GroupPeer
	Data []byte
}

type EventGroupInvite struct {
	Friend     Friend
	InviteData []byte
	GroupName  string
}

type EventGroupPeerJoin struct {
	Peer GroupPeer
}

type EventGroupPeerExit struct {
	Peer        GroupPeer
	ExitType    GroupExitType
	Name        string
	PartMessage string
}

type EventGroupSelfJoin struct {
	Group Group
}

type EventGroupJoinFail struct {
	Group    Group
	FailType GroupJoinFail
}

type EventGroupModeration struct {
	Group   Group
	ModType GroupModEvent
	Source  GroupPeer
	Target  GroupPeer
}

func (*EventGroupPeerName) event()            {}
func (*EventGroupPeerStatus) event()          {}
func (*EventGroupTopic) event()               {}
func (*EventGroupPrivacyState) event()        {}
func (*EventGroupVoiceState) event()          {}
func (*EventGroupTopicLock) event()           {}
func (*EventGroupPeerLimit) event()           {}
func (*EventGroupPassword) event()            {}
func (*EventGroupMessage) event()             {}
func (*EventGroupPrivateMessage) event()      {}
func (*EventGroupCustomPacket) event()        {}
func (*EventGroupCustomPrivatePacket) event() {}
func (*EventGroupInvite) event()              {}
func (*EventGroupPeerJoin) event()            {}
func (*EventGroupPeerExit) event()            {}
func (*EventGroupSelfJoin) event()            {}
func (*EventGroupJoinFail) event()            {}
func (*EventGroupModeration) event()          {}

func (t *Tox) groupEvent(ev *C.Tox_Event) Event {
	group := func(n C.uint32_t) Group { return Group{t, GroupNumber(n)} }
	peer := func(g, n C.uint32_t) GroupPeer { return GroupPeer{group(g), GroupPeerNumber(n)} }

	switch C.tox_event_get_type(ev) {
	case C.TOX_EVENT_TYPE_GROUP_PEER_NAME:
		e := C.tox_event_get_group_peer_name(ev)
		return &EventGroupPeerName{
			Peer: peer(C.tox_event_group_peer_name_get_group_number(e), C.tox_event_group_peer_name_get_peer_id(e)),
			Name: goString(C.tox_event_group_peer_name_get_name(e), C.tox_event_group_peer_name_get_name_length(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_PEER_STATUS:
		e := C.tox_event_get_group_peer_status(ev)
		return &EventGroupPeerStatus{
			Peer:   peer(C.tox_event_group_peer_status_get_group_number(e), C.tox_event_group_peer_status_get_peer_id(e)),
			Status: UserStatus(C.tox_event_group_peer_status_get_status(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_TOPIC:
		e := C.tox_event_get_group_topic(ev)
		return &EventGroupTopic{
			Peer:  peer(C.tox_event_group_topic_get_group_number(e), C.tox_event_group_topic_get_peer_id(e)),
			Topic: goString(C.tox_event_group_topic_get_topic(e), C.tox_event_group_topic_get_topic_length(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_PRIVACY_STATE:
		e := C.tox_event_get_group_privacy_state(ev)
		return &EventGroupPrivacyState{
			Group:        group(C.tox_event_group_privacy_state_get_group_number(e)),
			PrivacyState: GroupPrivacyState(C.tox_event_group_privacy_state_get_privacy_state(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_VOICE_STATE:
		e := C.tox_event_get_group_voice_state(ev)
		return &EventGroupVoiceState{
			Group:      group(C.tox_event_group_voice_state_get_group_number(e)),
			VoiceState: GroupVoiceState(C.tox_event_group_voice_state_get_voice_state(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_TOPIC_LOCK:
		e := C.tox_event_get_group_topic_lock(ev)
		return &EventGroupTopicLock{
			Group:     group(C.tox_event_group_topic_lock_get_group_number(e)),
			TopicLock: GroupTopicLock(C.tox_event_group_topic_lock_get_topic_lock(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_PEER_LIMIT:
		e := C.tox_event_get_group_peer_limit(ev)
		return &EventGroupPeerLimit{
			Group:     group(C.tox_event_group_peer_limit_get_group_number(e)),
			PeerLimit: uint32(C.tox_event_group_peer_limit_get_peer_limit(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_PASSWORD:
		e := C.tox_event_get_group_password(ev)
		return &EventGroupPassword{
			Group:    group(C.tox_event_group_password_get_group_number(e)),
			Password: goString(C.tox_event_group_password_get_password(e), C.tox_event_group_password_get_password_length(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_MESSAGE:
		e := C.tox_event_get_group_message(ev)
		return &EventGroupMessage{
			Peer:        peer(C.tox_event_group_message_get_group_number(e), C.tox_event_group_message_get_peer_id(e)),
			MessageType: MessageType(C.tox_event_group_message_get_message_type(e)),
			Message:     goString(C.tox_event_group_message_get_message(e), C.tox_event_group_message_get_message_length(e)),
			MessageID:   GroupMessageID(C.tox_event_group_message_get_message_id(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_PRIVATE_MESSAGE:
		e := C.tox_event_get_group_private_message(ev)
		return &EventGroupPrivateMessage{
			Peer:        peer(C.tox_event_group_private_message_get_group_number(e), C.tox_event_group_private_message_get_peer_id(e)),
			MessageType: MessageType(C.tox_event_group_private_message_get_message_type(e)),
			Message:     goString(C.tox_event_group_private_message_get_message(e), C.tox_event_group_private_message_get_message_length(e)),
			MessageID:   GroupMessageID(C.tox_event_group_private_message_get_message_id(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_CUSTOM_PACKET:
		e := C.tox_event_get_group_custom_packet(ev)
		return &EventGroupCustomPacket{
			Peer: peer(C.tox_event_group_custom_packet_get_group_number(e), C.tox_event_group_custom_packet_get_peer_id(e)),
			Data: goBytes(C.tox_event_group_custom_packet_get_data(e), C.tox_event_group_custom_packet_get_data_length(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_CUSTOM_PRIVATE_PACKET:
		e := C.tox_event_get_group_custom_private_packet(ev)
		return &EventGroupCustomPrivatePacket{
			Peer: peer(C.tox_event_group_custom_private_packet_get_group_number(e), C.tox_event_group_custom_private_packet_get_peer_id(e)),
			Data: goBytes(C.tox_event_group_custom_private_packet_get_data(e), C.tox_event_group_custom_private_packet_get_data_length(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_INVITE:
		e := C.tox_event_get_group_invite(ev)
		return &EventGroupInvite{
			Friend:     Friend{t, FriendNumber(C.tox_event_group_invite_get_friend_number(e))},
			InviteData: goBytes(C.tox_event_group_invite_get_invite_data(e), C.tox_event_group_invite_get_invite_data_length(e)),
			GroupName:  goString(C.tox_event_group_invite_get_group_name(e), C.tox_event_group_invite_get_group_name_length(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_PEER_JOIN:
		e := C.tox_event_get_group_peer_join(ev)
		return &EventGroupPeerJoin{
			Peer: peer(C.tox_event_group_peer_join_get_group_number(e), C.tox_event_group_peer_join_get_peer_id(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_PEER_EXIT:
		e := C.tox_event_get_group_peer_exit(ev)
		return &EventGroupPeerExit{
			Peer:        peer(C.tox_event_group_peer_exit_get_group_number(e), C.tox_event_group_peer_exit_get_peer_id(e)),
			ExitType:    GroupExitType(C.tox_event_group_peer_exit_get_exit_type(e)),
			Name:        goString(C.tox_event_group_peer_exit_get_name(e), C.tox_event_group_peer_exit_get_name_length(e)),
			PartMessage: goString(C.tox_event_group_peer_exit_get_part_message(e), C.tox_event_group_peer_exit_get_part_message_length(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_SELF_JOIN:
		e := C.tox_event_get_group_self_join(ev)
		return &EventGroupSelfJoin{
			Group: group(C.tox_event_group_self_join_get_group_number(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_JOIN_FAIL:
		e := C.tox_event_get_group_join_fail(ev)
		return &EventGroupJoinFail{
			Group:    group(C.tox_event_group_join_fail_get_group_number(e)),
			FailType: GroupJoinFail(C.tox_event_group_join_fail_get_fail_type(e)),
		}
	case C.TOX_EVENT_TYPE_GROUP_MODERATION:
		e := C.tox_event_get_group_moderation(ev)
		g := C.tox_event_group_moderation_get_group_number(e)
		return &EventGroupModeration{
			Group:   group(g),
			ModType: GroupModEvent(C.tox_event_group_moderation_get_mod_type(e)),
			Source:  peer(g, C.tox_event_group_moderation_get_source_peer_id(e)),
			Target:  peer(g, C.tox_event_group_moderation_get_target_peer_id(e)),
		}
	}
	return nil
}
