package tox

/*
#include <tox/tox.h>
#include <tox/tox_events.h>
*/
import "C"

type Event interface {
	event()
}

type EventSelfConnectionStatus struct {
	ConnectionStatus Connection
}

type EventFriendRequest struct {
	PublicKey PublicKey
	Message   string
}

type EventFriendConnectionStatus struct {
	Friend           Friend
	ConnectionStatus Connection
}

type EventFriendName struct {
	Friend Friend
	Name   string
}

type EventFriendStatus struct {
	Friend Friend
	Status UserStatus
}

type EventFriendStatusMessage struct {
	Friend  Friend
	Message string
}

type EventFriendTyping struct {
	Friend Friend
	Typing bool
}

type EventFriendMessage struct {
	Friend  Friend
	Type    MessageType
	Message string
}

type EventFriendReadReceipt struct {
	Friend    Friend
	MessageID MessageID
}

type EventFriendLossyPacket struct {
	Friend Friend
	Data   []byte
}

type EventFriendLosslessPacket struct {
	Friend Friend
	Data   []byte
}

type EventFileRecv struct {
	File     File
	Kind     FileKind
	FileSize uint64
	Filename string
}

type EventFileRecvChunk struct {
	File     File
	Position uint64
	Data     []byte
}

type EventFileChunkRequest struct {
	File     File
	Position uint64
	Length   int
}

type EventFileRecvControl struct {
	File    File
	Control FileControl
}

func (*EventSelfConnectionStatus) event()   {}
func (*EventFriendRequest) event()          {}
func (*EventFriendConnectionStatus) event() {}
func (*EventFriendName) event()             {}
func (*EventFriendStatus) event()           {}
func (*EventFriendStatusMessage) event()    {}
func (*EventFriendTyping) event()           {}
func (*EventFriendMessage) event()          {}
func (*EventFriendReadReceipt) event()      {}
func (*EventFriendLossyPacket) event()      {}
func (*EventFriendLosslessPacket) event()   {}
func (*EventFileRecv) event()               {}
func (*EventFileRecvChunk) event()          {}
func (*EventFileChunkRequest) event()       {}
func (*EventFileRecvControl) event()        {}

func (t *Tox) Iterate() ([]Event, error) {
	p := t.lock()
	defer t.h.Unlock()
	var err C.Tox_Err_Events_Iterate
	events := C.tox_events_iterate(p, nil, &err)
	defer C.tox_events_free(events)

	size := C.tox_events_get_size(events)
	out := make([]Event, 0, size)
	for i := range size {
		if ev := t.goEvent(C.tox_events_get(events, i)); ev != nil {
			out = append(out, ev)
		}
	}
	return out, toError[EventsIterateError](uint32(err))
}

// goEvent copies an event out of toxcore's memory. It returns nil for an event
// this package doesn't know, such as one from a newer toxcore.
func (t *Tox) goEvent(ev *C.Tox_Event) Event {
	friend := func(n C.uint32_t) Friend { return Friend{t, FriendNumber(n)} }
	file := func(f, n C.uint32_t) File { return File{friend(f), FileNumber(n)} }

	switch C.tox_event_get_type(ev) {
	case C.TOX_EVENT_TYPE_SELF_CONNECTION_STATUS:
		e := C.tox_event_get_self_connection_status(ev)
		return &EventSelfConnectionStatus{
			ConnectionStatus: Connection(C.tox_event_self_connection_status_get_connection_status(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_REQUEST:
		e := C.tox_event_get_friend_request(ev)
		r := &EventFriendRequest{
			Message: goString(C.tox_event_friend_request_get_message(e), C.tox_event_friend_request_get_message_length(e)),
		}
		copyFromC(r.PublicKey[:], C.tox_event_friend_request_get_public_key(e))
		return r
	case C.TOX_EVENT_TYPE_FRIEND_CONNECTION_STATUS:
		e := C.tox_event_get_friend_connection_status(ev)
		return &EventFriendConnectionStatus{
			Friend:           friend(C.tox_event_friend_connection_status_get_friend_number(e)),
			ConnectionStatus: Connection(C.tox_event_friend_connection_status_get_connection_status(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_NAME:
		e := C.tox_event_get_friend_name(ev)
		return &EventFriendName{
			Friend: friend(C.tox_event_friend_name_get_friend_number(e)),
			Name:   goString(C.tox_event_friend_name_get_name(e), C.tox_event_friend_name_get_name_length(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_STATUS:
		e := C.tox_event_get_friend_status(ev)
		return &EventFriendStatus{
			Friend: friend(C.tox_event_friend_status_get_friend_number(e)),
			Status: UserStatus(C.tox_event_friend_status_get_status(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_STATUS_MESSAGE:
		e := C.tox_event_get_friend_status_message(ev)
		return &EventFriendStatusMessage{
			Friend:  friend(C.tox_event_friend_status_message_get_friend_number(e)),
			Message: goString(C.tox_event_friend_status_message_get_message(e), C.tox_event_friend_status_message_get_message_length(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_TYPING:
		e := C.tox_event_get_friend_typing(ev)
		return &EventFriendTyping{
			Friend: friend(C.tox_event_friend_typing_get_friend_number(e)),
			Typing: bool(C.tox_event_friend_typing_get_typing(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_MESSAGE:
		e := C.tox_event_get_friend_message(ev)
		return &EventFriendMessage{
			Friend:  friend(C.tox_event_friend_message_get_friend_number(e)),
			Type:    MessageType(C.tox_event_friend_message_get_type(e)),
			Message: goString(C.tox_event_friend_message_get_message(e), C.tox_event_friend_message_get_message_length(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_READ_RECEIPT:
		e := C.tox_event_get_friend_read_receipt(ev)
		return &EventFriendReadReceipt{
			Friend:    friend(C.tox_event_friend_read_receipt_get_friend_number(e)),
			MessageID: MessageID(C.tox_event_friend_read_receipt_get_message_id(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_LOSSY_PACKET:
		e := C.tox_event_get_friend_lossy_packet(ev)
		return &EventFriendLossyPacket{
			Friend: friend(C.tox_event_friend_lossy_packet_get_friend_number(e)),
			Data:   goBytes(C.tox_event_friend_lossy_packet_get_data(e), C.tox_event_friend_lossy_packet_get_data_length(e)),
		}
	case C.TOX_EVENT_TYPE_FRIEND_LOSSLESS_PACKET:
		e := C.tox_event_get_friend_lossless_packet(ev)
		return &EventFriendLosslessPacket{
			Friend: friend(C.tox_event_friend_lossless_packet_get_friend_number(e)),
			Data:   goBytes(C.tox_event_friend_lossless_packet_get_data(e), C.tox_event_friend_lossless_packet_get_data_length(e)),
		}
	case C.TOX_EVENT_TYPE_FILE_RECV:
		e := C.tox_event_get_file_recv(ev)
		return &EventFileRecv{
			File:     file(C.tox_event_file_recv_get_friend_number(e), C.tox_event_file_recv_get_file_number(e)),
			Kind:     FileKind(C.tox_event_file_recv_get_kind(e)),
			FileSize: uint64(C.tox_event_file_recv_get_file_size(e)),
			Filename: goString(C.tox_event_file_recv_get_filename(e), C.tox_event_file_recv_get_filename_length(e)),
		}
	case C.TOX_EVENT_TYPE_FILE_RECV_CHUNK:
		e := C.tox_event_get_file_recv_chunk(ev)
		return &EventFileRecvChunk{
			File:     file(C.tox_event_file_recv_chunk_get_friend_number(e), C.tox_event_file_recv_chunk_get_file_number(e)),
			Position: uint64(C.tox_event_file_recv_chunk_get_position(e)),
			Data:     goBytes(C.tox_event_file_recv_chunk_get_data(e), C.tox_event_file_recv_chunk_get_data_length(e)),
		}
	case C.TOX_EVENT_TYPE_FILE_CHUNK_REQUEST:
		e := C.tox_event_get_file_chunk_request(ev)
		return &EventFileChunkRequest{
			File:     file(C.tox_event_file_chunk_request_get_friend_number(e), C.tox_event_file_chunk_request_get_file_number(e)),
			Position: uint64(C.tox_event_file_chunk_request_get_position(e)),
			Length:   int(C.tox_event_file_chunk_request_get_length(e)),
		}
	case C.TOX_EVENT_TYPE_FILE_RECV_CONTROL:
		e := C.tox_event_get_file_recv_control(ev)
		return &EventFileRecvControl{
			File:    file(C.tox_event_file_recv_control_get_friend_number(e), C.tox_event_file_recv_control_get_file_number(e)),
			Control: FileControl(C.tox_event_file_recv_control_get_control(e)),
		}
	}
	if e := t.groupEvent(ev); e != nil {
		return e
	}
	return t.conferenceEvent(ev)
}
