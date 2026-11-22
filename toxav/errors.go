package toxav

/*
#include <tox/toxav.h>
*/
import "C"

import "fmt"

type toxavError interface {
	~uint32
	error
}

// toError is nil for toxav's OK value, which is always 0.
func toError[E toxavError](code uint32) error {
	if code == 0 {
		return nil
	}
	return E(code)
}

func errorString[E ~uint32](op string, names map[E]string, e E) string {
	if name, ok := names[e]; ok {
		return "toxav: " + op + ": " + name
	}
	return fmt.Sprintf("toxav: %s: error %d", op, uint32(e))
}

type NewError uint32

const (
	ErrNewMalloc   NewError = C.TOXAV_ERR_NEW_MALLOC
	ErrNewMultiple NewError = C.TOXAV_ERR_NEW_MULTIPLE
)

func (e NewError) Error() string {
	return errorString("new", map[NewError]string{
		ErrNewMalloc:   "malloc",
		ErrNewMultiple: "multiple",
	}, e)
}

type CallError uint32

const (
	ErrCallMalloc              CallError = C.TOXAV_ERR_CALL_MALLOC
	ErrCallSync                CallError = C.TOXAV_ERR_CALL_SYNC
	ErrCallFriendNotFound      CallError = C.TOXAV_ERR_CALL_FRIEND_NOT_FOUND
	ErrCallFriendNotConnected  CallError = C.TOXAV_ERR_CALL_FRIEND_NOT_CONNECTED
	ErrCallFriendAlreadyInCall CallError = C.TOXAV_ERR_CALL_FRIEND_ALREADY_IN_CALL
	ErrCallInvalidBitRate      CallError = C.TOXAV_ERR_CALL_INVALID_BIT_RATE
)

func (e CallError) Error() string {
	return errorString("call", map[CallError]string{
		ErrCallMalloc:              "malloc",
		ErrCallSync:                "sync",
		ErrCallFriendNotFound:      "friend not found",
		ErrCallFriendNotConnected:  "friend not connected",
		ErrCallFriendAlreadyInCall: "friend already in call",
		ErrCallInvalidBitRate:      "invalid bit rate",
	}, e)
}

type AnswerError uint32

const (
	ErrAnswerSync                AnswerError = C.TOXAV_ERR_ANSWER_SYNC
	ErrAnswerCodecInitialization AnswerError = C.TOXAV_ERR_ANSWER_CODEC_INITIALIZATION
	ErrAnswerFriendNotFound      AnswerError = C.TOXAV_ERR_ANSWER_FRIEND_NOT_FOUND
	ErrAnswerFriendNotCalling    AnswerError = C.TOXAV_ERR_ANSWER_FRIEND_NOT_CALLING
	ErrAnswerInvalidBitRate      AnswerError = C.TOXAV_ERR_ANSWER_INVALID_BIT_RATE
)

func (e AnswerError) Error() string {
	return errorString("answer", map[AnswerError]string{
		ErrAnswerSync:                "sync",
		ErrAnswerCodecInitialization: "codec initialization",
		ErrAnswerFriendNotFound:      "friend not found",
		ErrAnswerFriendNotCalling:    "friend not calling",
		ErrAnswerInvalidBitRate:      "invalid bit rate",
	}, e)
}

type CallControlError uint32

const (
	ErrCallControlSync              CallControlError = C.TOXAV_ERR_CALL_CONTROL_SYNC
	ErrCallControlFriendNotFound    CallControlError = C.TOXAV_ERR_CALL_CONTROL_FRIEND_NOT_FOUND
	ErrCallControlFriendNotInCall   CallControlError = C.TOXAV_ERR_CALL_CONTROL_FRIEND_NOT_IN_CALL
	ErrCallControlInvalidTransition CallControlError = C.TOXAV_ERR_CALL_CONTROL_INVALID_TRANSITION
)

func (e CallControlError) Error() string {
	return errorString("call control", map[CallControlError]string{
		ErrCallControlSync:              "sync",
		ErrCallControlFriendNotFound:    "friend not found",
		ErrCallControlFriendNotInCall:   "friend not in call",
		ErrCallControlInvalidTransition: "invalid transition",
	}, e)
}

type BitRateSetError uint32

const (
	ErrBitRateSetSync            BitRateSetError = C.TOXAV_ERR_BIT_RATE_SET_SYNC
	ErrBitRateSetInvalidBitRate  BitRateSetError = C.TOXAV_ERR_BIT_RATE_SET_INVALID_BIT_RATE
	ErrBitRateSetFriendNotFound  BitRateSetError = C.TOXAV_ERR_BIT_RATE_SET_FRIEND_NOT_FOUND
	ErrBitRateSetFriendNotInCall BitRateSetError = C.TOXAV_ERR_BIT_RATE_SET_FRIEND_NOT_IN_CALL
)

func (e BitRateSetError) Error() string {
	return errorString("bit rate set", map[BitRateSetError]string{
		ErrBitRateSetSync:            "sync",
		ErrBitRateSetInvalidBitRate:  "invalid bit rate",
		ErrBitRateSetFriendNotFound:  "friend not found",
		ErrBitRateSetFriendNotInCall: "friend not in call",
	}, e)
}

type SendFrameError uint32

const (
	ErrSendFrameFriendNotFound      SendFrameError = C.TOXAV_ERR_SEND_FRAME_FRIEND_NOT_FOUND
	ErrSendFrameFriendNotInCall     SendFrameError = C.TOXAV_ERR_SEND_FRAME_FRIEND_NOT_IN_CALL
	ErrSendFrameSync                SendFrameError = C.TOXAV_ERR_SEND_FRAME_SYNC
	ErrSendFrameInvalid             SendFrameError = C.TOXAV_ERR_SEND_FRAME_INVALID
	ErrSendFramePayloadTypeDisabled SendFrameError = C.TOXAV_ERR_SEND_FRAME_PAYLOAD_TYPE_DISABLED
	ErrSendFrameRTPFailed           SendFrameError = C.TOXAV_ERR_SEND_FRAME_RTP_FAILED
)

func (e SendFrameError) Error() string {
	return errorString("send frame", map[SendFrameError]string{
		ErrSendFrameFriendNotFound:      "friend not found",
		ErrSendFrameFriendNotInCall:     "friend not in call",
		ErrSendFrameSync:                "sync",
		ErrSendFrameInvalid:             "invalid",
		ErrSendFramePayloadTypeDisabled: "payload type disabled",
		ErrSendFrameRTPFailed:           "rtp failed",
	}, e)
}
