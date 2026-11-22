package toxencryptsave_test

import (
	"bytes"
	"testing"

	tox "github.com/TokTok/go-toxcore-c"
	"github.com/TokTok/go-toxcore-c/internal/toxtest"
	"github.com/TokTok/go-toxcore-c/toxencryptsave"
)

var passphrase = []byte("correct horse")

func TestEncryptDecrypt(t *testing.T) {
	plaintext := []byte("a profile")
	ciphertext, err := toxencryptsave.Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	if !toxencryptsave.IsDataEncrypted(ciphertext) || toxencryptsave.IsDataEncrypted(plaintext) {
		t.Error("IsDataEncrypted can't tell them apart")
	}
	if got, err := toxencryptsave.Decrypt(ciphertext, passphrase); err != nil || !bytes.Equal(got, plaintext) {
		t.Errorf("decrypted %q, %v", got, err)
	}
	if _, err := toxencryptsave.Decrypt(ciphertext, []byte("wrong")); err != toxencryptsave.ErrDecryptionFailed {
		t.Errorf("wrong passphrase: %v", err)
	}
	if _, err := toxencryptsave.Decrypt(plaintext, passphrase); err != toxencryptsave.ErrDecryptionInvalidLength {
		t.Errorf("short data: %v", err)
	}
	if _, err := toxencryptsave.Salt(plaintext); err != toxencryptsave.ErrGetSaltBadFormat {
		t.Errorf("salt of short data: %v", err)
	}
}

// A key derived again from the salt decrypts what the passphrase encrypted.
func TestPassKeyFromSalt(t *testing.T) {
	ciphertext, err := toxencryptsave.Encrypt([]byte("a profile"), passphrase)
	if err != nil {
		t.Fatal(err)
	}
	salt, err := toxencryptsave.Salt(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	key, err := toxencryptsave.PassKeyDeriveWithSalt(passphrase, salt)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	if got, err := key.Decrypt(ciphertext); err != nil || string(got) != "a profile" {
		t.Errorf("decrypted %q, %v", got, err)
	}

	again, err := key.Encrypt([]byte("again"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := toxencryptsave.Decrypt(again, passphrase); err != nil || string(got) != "again" {
		t.Errorf("decrypted %q, %v", got, err)
	}
	if _, err := key.Encrypt(nil); err != toxencryptsave.ErrEncryptionNull {
		t.Errorf("empty plaintext: %v", err)
	}

	key.Close()
	key.Close()
	defer func() {
		if recover() == nil {
			t.Error("a closed PassKey didn't panic")
		}
	}()
	key.Encrypt([]byte("x"))
}

func TestEncryptedProfile(t *testing.T) {
	a := toxtest.New(t, nil)
	key, err := toxencryptsave.PassKeyDerive(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	defer key.Close()
	saved, err := key.Encrypt(a.Savedata())
	if err != nil {
		t.Fatal(err)
	}

	opts := tox.DefaultOptions()
	opts.SavedataType = tox.SavedataTypeToxSave
	opts.SavedataData = saved
	if _, err := tox.New(opts); err != tox.ErrNewLoadEncrypted {
		t.Errorf("loading the encrypted profile: %v", err)
	}
	if opts.SavedataData, err = toxencryptsave.Decrypt(saved, passphrase); err != nil {
		t.Fatal(err)
	}
	if b := toxtest.New(t, opts); b.PublicKey() != a.PublicKey() {
		t.Errorf("loaded %v, not %v", b.PublicKey(), a.PublicKey())
	}
}
