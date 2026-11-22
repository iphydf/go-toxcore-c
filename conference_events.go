package tox

/*
#include <tox/tox.h>
#include <tox/tox_events.h>
*/
import "C"

type EventConferenceConnected struct {
	Conference Conference
}

type EventConferenceInvite struct {
	Friend Friend
	Type   ConferenceType
	Cookie []byte
}

type EventConferenceMessage struct {
	Peer    ConferencePeer
	Type    MessageType
	Message string
}

type EventConferencePeerListChanged struct {
	Conference Conference
}

type EventConferencePeerName struct {
	Peer ConferencePeer
	Name string
}

type EventConferenceTitle struct {
	Peer  ConferencePeer
	Title string
}

func (*EventConferenceConnected) event()       {}
func (*EventConferenceInvite) event()          {}
func (*EventConferenceMessage) event()         {}
func (*EventConferencePeerListChanged) event() {}
func (*EventConferencePeerName) event()        {}
func (*EventConferenceTitle) event()           {}

func (t *Tox) conferenceEvent(ev *C.Tox_Event) Event {
	conference := func(n C.uint32_t) Conference { return Conference{t, ConferenceNumber(n)} }
	peer := func(c, n C.uint32_t) ConferencePeer { return ConferencePeer{conference(c), ConferencePeerNumber(n)} }

	switch C.tox_event_get_type(ev) {
	case C.TOX_EVENT_TYPE_CONFERENCE_CONNECTED:
		e := C.tox_event_get_conference_connected(ev)
		return &EventConferenceConnected{
			Conference: conference(C.tox_event_conference_connected_get_conference_number(e)),
		}
	case C.TOX_EVENT_TYPE_CONFERENCE_INVITE:
		e := C.tox_event_get_conference_invite(ev)
		return &EventConferenceInvite{
			Friend: Friend{t, FriendNumber(C.tox_event_conference_invite_get_friend_number(e))},
			Type:   ConferenceType(C.tox_event_conference_invite_get_type(e)),
			Cookie: goBytes(C.tox_event_conference_invite_get_cookie(e), C.tox_event_conference_invite_get_cookie_length(e)),
		}
	case C.TOX_EVENT_TYPE_CONFERENCE_MESSAGE:
		e := C.tox_event_get_conference_message(ev)
		return &EventConferenceMessage{
			Peer:    peer(C.tox_event_conference_message_get_conference_number(e), C.tox_event_conference_message_get_peer_number(e)),
			Type:    MessageType(C.tox_event_conference_message_get_type(e)),
			Message: goString(C.tox_event_conference_message_get_message(e), C.tox_event_conference_message_get_message_length(e)),
		}
	case C.TOX_EVENT_TYPE_CONFERENCE_PEER_LIST_CHANGED:
		e := C.tox_event_get_conference_peer_list_changed(ev)
		return &EventConferencePeerListChanged{
			Conference: conference(C.tox_event_conference_peer_list_changed_get_conference_number(e)),
		}
	case C.TOX_EVENT_TYPE_CONFERENCE_PEER_NAME:
		e := C.tox_event_get_conference_peer_name(ev)
		return &EventConferencePeerName{
			Peer: peer(C.tox_event_conference_peer_name_get_conference_number(e), C.tox_event_conference_peer_name_get_peer_number(e)),
			Name: goString(C.tox_event_conference_peer_name_get_name(e), C.tox_event_conference_peer_name_get_name_length(e)),
		}
	case C.TOX_EVENT_TYPE_CONFERENCE_TITLE:
		e := C.tox_event_get_conference_title(ev)
		return &EventConferenceTitle{
			Peer:  peer(C.tox_event_conference_title_get_conference_number(e), C.tox_event_conference_title_get_peer_number(e)),
			Title: goString(C.tox_event_conference_title_get_title(e), C.tox_event_conference_title_get_title_length(e)),
		}
	}
	return nil
}
