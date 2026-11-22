package tox

/*
#include <tox/tox.h>
*/
import "C"

import (
	"fmt"
	"math"
)

type File struct {
	friend Friend
	number FileNumber
}

func (f Friend) File(number FileNumber) File {
	return File{f, number}
}

func (f File) Friend() Friend {
	return f.friend
}

func (f File) Number() FileNumber {
	return f.number
}

func (f File) String() string {
	return fmt.Sprintf("file %d of friend %d", f.number, f.friend.number)
}

func (f File) c() C.Tox_File_Number {
	return C.Tox_File_Number(f.number)
}

type FileKind uint32

const (
	FileKindData    FileKind = C.TOX_FILE_KIND_DATA
	FileKindAvatar  FileKind = C.TOX_FILE_KIND_AVATAR
	FileKindSticker FileKind = C.TOX_FILE_KIND_STICKER
	FileKindSHA1    FileKind = C.TOX_FILE_KIND_SHA1
	FileKindSHA256  FileKind = C.TOX_FILE_KIND_SHA256
)

func (k FileKind) String() string {
	switch k {
	case FileKindData:
		return "data"
	case FileKindAvatar:
		return "avatar"
	case FileKindSticker:
		return "sticker"
	case FileKindSHA1:
		return "sha1"
	case FileKindSHA256:
		return "sha256"
	}
	return fmt.Sprintf("FileKind(%d)", uint32(k))
}

const FileSizeUnknown uint64 = math.MaxUint64

type FileControl uint32

const (
	FileControlResume FileControl = C.TOX_FILE_CONTROL_RESUME
	FileControlPause  FileControl = C.TOX_FILE_CONTROL_PAUSE
	FileControlCancel FileControl = C.TOX_FILE_CONTROL_CANCEL
)

func (c FileControl) String() string {
	return enumString(C.tox_file_control_to_string(C.Tox_File_Control(c)), "TOX_FILE_CONTROL_", "FileControl", uint32(c))
}

func (f Friend) FileSend(kind FileKind, size uint64, id *FileID, name string) (File, error) {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var cid *C.uint8_t
	if id != nil {
		cid = cBytes(id[:])
	}
	var err C.Tox_Err_File_Send
	n := C.tox_file_send(p, f.c(), C.uint32_t(kind), C.uint64_t(size), cid, cString(name), C.size_t(len(name)), &err)
	return File{f, FileNumber(n)}, toError[FileSendError](uint32(err))
}

func (f Friend) FileByID(id FileID) (File, error) {
	p := f.tox.lock()
	defer f.tox.h.Unlock()
	var err C.Tox_Err_File_By_Id
	n := C.tox_file_by_id(p, f.c(), cBytes(id[:]), &err)
	return File{f, FileNumber(n)}, toError[FileByIDError](uint32(err))
}

func (f File) Control(control FileControl) error {
	p := f.friend.tox.lock()
	defer f.friend.tox.h.Unlock()
	var err C.Tox_Err_File_Control
	C.tox_file_control(p, f.friend.c(), f.c(), C.Tox_File_Control(control), &err)
	return toError[FileControlError](uint32(err))
}

func (f File) Seek(position uint64) error {
	p := f.friend.tox.lock()
	defer f.friend.tox.h.Unlock()
	var err C.Tox_Err_File_Seek
	C.tox_file_seek(p, f.friend.c(), f.c(), C.uint64_t(position), &err)
	return toError[FileSeekError](uint32(err))
}

func (f File) FileID() (FileID, error) {
	p := f.friend.tox.lock()
	defer f.friend.tox.h.Unlock()
	var id FileID
	var err C.Tox_Err_File_Get
	C.tox_file_get_file_id(p, f.friend.c(), f.c(), cOut(id[:]), &err)
	return id, toError[FileGetError](uint32(err))
}

func (f File) SendChunk(position uint64, data []byte) error {
	p := f.friend.tox.lock()
	defer f.friend.tox.h.Unlock()
	var err C.Tox_Err_File_Send_Chunk
	C.tox_file_send_chunk(p, f.friend.c(), f.c(), C.uint64_t(position), cBytes(data), C.size_t(len(data)), &err)
	return toError[FileSendChunkError](uint32(err))
}
