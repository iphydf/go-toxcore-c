package toxav

/*
#include <tox/toxav.h>
*/
import "C"

import (
	"fmt"
	"strings"
)

type BitRate uint32

type SamplingRate uint32

type Channels uint8

type Width uint16

type Height uint16

type CallControl uint32

const (
	CallControlResume      CallControl = C.TOXAV_CALL_CONTROL_RESUME
	CallControlPause       CallControl = C.TOXAV_CALL_CONTROL_PAUSE
	CallControlCancel      CallControl = C.TOXAV_CALL_CONTROL_CANCEL
	CallControlMuteAudio   CallControl = C.TOXAV_CALL_CONTROL_MUTE_AUDIO
	CallControlUnmuteAudio CallControl = C.TOXAV_CALL_CONTROL_UNMUTE_AUDIO
	CallControlHideVideo   CallControl = C.TOXAV_CALL_CONTROL_HIDE_VIDEO
	CallControlShowVideo   CallControl = C.TOXAV_CALL_CONTROL_SHOW_VIDEO
)

var callControlNames = map[CallControl]string{
	CallControlResume:      "resume",
	CallControlPause:       "pause",
	CallControlCancel:      "cancel",
	CallControlMuteAudio:   "mute audio",
	CallControlUnmuteAudio: "unmute audio",
	CallControlHideVideo:   "hide video",
	CallControlShowVideo:   "show video",
}

func (c CallControl) String() string {
	if name, ok := callControlNames[c]; ok {
		return name
	}
	return fmt.Sprintf("CallControl(%d)", uint32(c))
}

type FriendCallState uint32

const (
	FriendCallStateError      FriendCallState = C.TOXAV_FRIEND_CALL_STATE_ERROR
	FriendCallStateFinished   FriendCallState = C.TOXAV_FRIEND_CALL_STATE_FINISHED
	FriendCallStateSendingA   FriendCallState = C.TOXAV_FRIEND_CALL_STATE_SENDING_A
	FriendCallStateSendingV   FriendCallState = C.TOXAV_FRIEND_CALL_STATE_SENDING_V
	FriendCallStateAcceptingA FriendCallState = C.TOXAV_FRIEND_CALL_STATE_ACCEPTING_A
	FriendCallStateAcceptingV FriendCallState = C.TOXAV_FRIEND_CALL_STATE_ACCEPTING_V
)

var friendCallStateNames = []struct {
	flag FriendCallState
	name string
}{
	{FriendCallStateError, "error"},
	{FriendCallStateFinished, "finished"},
	{FriendCallStateSendingA, "sending audio"},
	{FriendCallStateSendingV, "sending video"},
	{FriendCallStateAcceptingA, "accepting audio"},
	{FriendCallStateAcceptingV, "accepting video"},
}

func (s FriendCallState) String() string {
	var names []string
	for _, f := range friendCallStateNames {
		if s&f.flag != 0 {
			names = append(names, f.name)
			s &^= f.flag
		}
	}
	if s != 0 {
		names = append(names, fmt.Sprintf("0x%x", uint32(s)))
	}
	if names == nil {
		return "none"
	}
	return strings.Join(names, "|")
}
