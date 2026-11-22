package toxencryptsave

/*
#cgo pkg-config: toxcore
#include <tox/toxencryptsave.h>
*/
import "C"

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/TokTok/go-toxcore-c/internal/cname"
)

const PassEncryptionExtraLength = C.TOX_PASS_ENCRYPTION_EXTRA_LENGTH

type PassSalt [C.TOX_PASS_SALT_LENGTH]byte

// empty is what an empty input points to, as an empty slice has no address and
// toxencryptsave only takes NULL where its header says _Nullable.
var empty C.uint8_t

func cBytes(b []byte) *C.uint8_t {
	if len(b) == 0 {
		return &empty
	}
	return (*C.uint8_t)(unsafe.Pointer(&b[0]))
}

func Encrypt(plaintext, passphrase []byte) ([]byte, error) {
	ciphertext := make([]byte, len(plaintext)+PassEncryptionExtraLength)
	var err C.Tox_Err_Encryption
	C.tox_pass_encrypt(cBytes(plaintext), C.size_t(len(plaintext)), cBytes(passphrase), C.size_t(len(passphrase)),
		cBytes(ciphertext), &err)
	if err := toError[EncryptionError](uint32(err)); err != nil {
		return nil, err
	}
	return ciphertext, nil
}

func Decrypt(ciphertext, passphrase []byte) ([]byte, error) {
	if len(ciphertext) < PassEncryptionExtraLength {
		return nil, ErrDecryptionInvalidLength
	}
	plaintext := make([]byte, len(ciphertext)-PassEncryptionExtraLength)
	var err C.Tox_Err_Decryption
	C.tox_pass_decrypt(cBytes(ciphertext), C.size_t(len(ciphertext)), cBytes(passphrase), C.size_t(len(passphrase)),
		cBytes(plaintext), &err)
	if err := toError[DecryptionError](uint32(err)); err != nil {
		return nil, err
	}
	return plaintext, nil
}

func Salt(ciphertext []byte) (PassSalt, error) {
	var salt PassSalt
	if len(ciphertext) < PassEncryptionExtraLength {
		return salt, ErrGetSaltBadFormat
	}
	var err C.Tox_Err_Get_Salt
	C.tox_get_salt(cBytes(ciphertext), cBytes(salt[:]), &err)
	return salt, toError[GetSaltError](uint32(err))
}

func IsDataEncrypted(data []byte) bool {
	return len(data) >= PassEncryptionExtraLength && bool(C.tox_is_data_encrypted(cBytes(data)))
}

type PassKey struct {
	// mu keeps Close from freeing the key while it is in use.
	mu      sync.RWMutex
	ptr     *C.Tox_Pass_Key
	cleanup runtime.Cleanup
}

func PassKeyDerive(passphrase []byte) (*PassKey, error) {
	var err C.Tox_Err_Key_Derivation
	return newPassKey(C.tox_pass_key_derive(cBytes(passphrase), C.size_t(len(passphrase)), &err), err)
}

func PassKeyDeriveWithSalt(passphrase []byte, salt PassSalt) (*PassKey, error) {
	var err C.Tox_Err_Key_Derivation
	return newPassKey(C.tox_pass_key_derive_with_salt(cBytes(passphrase), C.size_t(len(passphrase)), cBytes(salt[:]), &err), err)
}

func newPassKey(ptr *C.Tox_Pass_Key, err C.Tox_Err_Key_Derivation) (*PassKey, error) {
	if ptr == nil {
		return nil, toError[KeyDerivationError](uint32(err))
	}
	k := &PassKey{ptr: ptr}
	// A key nobody closed is freed when it is garbage.
	k.cleanup = runtime.AddCleanup(k, func(ptr *C.Tox_Pass_Key) { C.tox_pass_key_free(ptr) }, ptr)
	return k, nil
}

func (k *PassKey) Close() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.ptr == nil {
		return
	}
	k.cleanup.Stop()
	C.tox_pass_key_free(k.ptr)
	k.ptr = nil
}

func (k *PassKey) rlock() *C.Tox_Pass_Key {
	k.mu.RLock()
	if k.ptr == nil {
		k.mu.RUnlock()
		panic("toxencryptsave: use of a closed PassKey")
	}
	return k.ptr
}

func (k *PassKey) Encrypt(plaintext []byte) ([]byte, error) {
	p := k.rlock()
	defer k.mu.RUnlock()
	ciphertext := make([]byte, len(plaintext)+PassEncryptionExtraLength)
	var err C.Tox_Err_Encryption
	C.tox_pass_key_encrypt(p, cBytes(plaintext), C.size_t(len(plaintext)), cBytes(ciphertext), &err)
	if err := toError[EncryptionError](uint32(err)); err != nil {
		return nil, err
	}
	return ciphertext, nil
}

func (k *PassKey) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < PassEncryptionExtraLength {
		return nil, ErrDecryptionInvalidLength
	}
	p := k.rlock()
	defer k.mu.RUnlock()
	plaintext := make([]byte, len(ciphertext)-PassEncryptionExtraLength)
	var err C.Tox_Err_Decryption
	C.tox_pass_key_decrypt(p, cBytes(ciphertext), C.size_t(len(ciphertext)), cBytes(plaintext), &err)
	if err := toError[DecryptionError](uint32(err)); err != nil {
		return nil, err
	}
	return plaintext, nil
}

type toxError interface {
	~uint32
	error
}

// toError is nil for the OK value, which is always 0.
func toError[E toxError](code uint32) error {
	if code == 0 {
		return nil
	}
	return E(code)
}

type KeyDerivationError uint32

const ErrKeyDerivationFailed KeyDerivationError = C.TOX_ERR_KEY_DERIVATION_FAILED

func (e KeyDerivationError) Error() string {
	return cname.Error("toxencryptsave", C.GoString(C.tox_err_key_derivation_to_string(C.Tox_Err_Key_Derivation(e))),
		"TOX_ERR_KEY_DERIVATION_", uint32(e))
}

type EncryptionError uint32

const (
	ErrEncryptionNull                EncryptionError = C.TOX_ERR_ENCRYPTION_NULL
	ErrEncryptionKeyDerivationFailed EncryptionError = C.TOX_ERR_ENCRYPTION_KEY_DERIVATION_FAILED
	ErrEncryptionFailed              EncryptionError = C.TOX_ERR_ENCRYPTION_FAILED
)

func (e EncryptionError) Error() string {
	return cname.Error("toxencryptsave", C.GoString(C.tox_err_encryption_to_string(C.Tox_Err_Encryption(e))),
		"TOX_ERR_ENCRYPTION_", uint32(e))
}

type DecryptionError uint32

const (
	ErrDecryptionInvalidLength       DecryptionError = C.TOX_ERR_DECRYPTION_INVALID_LENGTH
	ErrDecryptionBadFormat           DecryptionError = C.TOX_ERR_DECRYPTION_BAD_FORMAT
	ErrDecryptionKeyDerivationFailed DecryptionError = C.TOX_ERR_DECRYPTION_KEY_DERIVATION_FAILED
	ErrDecryptionFailed              DecryptionError = C.TOX_ERR_DECRYPTION_FAILED
)

func (e DecryptionError) Error() string {
	return cname.Error("toxencryptsave", C.GoString(C.tox_err_decryption_to_string(C.Tox_Err_Decryption(e))),
		"TOX_ERR_DECRYPTION_", uint32(e))
}

type GetSaltError uint32

const ErrGetSaltBadFormat GetSaltError = C.TOX_ERR_GET_SALT_BAD_FORMAT

func (e GetSaltError) Error() string {
	return cname.Error("toxencryptsave", C.GoString(C.tox_err_get_salt_to_string(C.Tox_Err_Get_Salt(e))),
		"TOX_ERR_GET_SALT_", uint32(e))
}
