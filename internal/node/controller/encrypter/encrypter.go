package encrypter

import (
	"crypto/sha256"

	"github.com/agl/gcmsiv"
)

type Encrypter interface {
	Encrypt(rkey string, data []byte) ([]byte, error)
	Decrypt(rkey string, encrypted []byte) ([]byte, error)
}

type AesEncrypter struct {
	gcm *gcmsiv.GCMSIV
}

func NewFromKey(key []byte) (Encrypter, error) {
	gcm, err := gcmsiv.NewGCMSIV(key)
	if err != nil {
		return nil, err
	}
	return &AesEncrypter{
		gcm: gcm,
	}, nil
}

// Takes in an atproto Record Key and bytes of data that must be a valid lexicon.
// Returns the data post-encryption.
//
// Encrypts the data using the cipher given by e.keys[hash(rkey)]
func (e *AesEncrypter) Encrypt(rkey string, plaintext []byte) ([]byte, error) {
	// TODO: is this nonce kosher?
	nonce := sha256.New().Sum([]byte(rkey))
	nonce = nonce[:e.gcm.NonceSize()]
	return e.gcm.Seal(nil, nonce, plaintext, nil), nil
}

// Takes in an atproto Record Key and bytes of data encrypted.
// Returns the data post-encryption.
//
// Decrypts the data using the cipher given by e.keys[hash(rkey)]
func (e *AesEncrypter) Decrypt(rkey string, ciphertext []byte) ([]byte, error) {
	nonce := sha256.New().Sum([]byte(rkey))
	nonce = nonce[:e.gcm.NonceSize()]
	return e.gcm.Open(nil, nonce, ciphertext, nil)
}
