package encrypter

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
)

type Encrypter interface {
	Encrypt(rkey string, data []byte) ([]byte, error)
	Decrypt(rkey string, encrypted []byte) ([]byte, error)
}

type AesEncrypter struct {
	blocks []cipher.Block
}

func NewFromKeys(keys [][]byte) (Encrypter, error) {
	var err error

	blocks := make([]cipher.Block, len(keys))
	for i := range len(keys) {
		blocks[i], err = aes.NewCipher(keys[i])
		if err != nil {
			return nil, err
		}

	}

	return &AesEncrypter{
		blocks: blocks,
	}, nil
}

// Takes in an atproto Record Key and bytes of data that must be a valid lexicon.
// Returns the data post-encryption.
//
// Encrypts the data using the cipher given by e.keys[hash(rkey)]
func (e *AesEncrypter) Encrypt(rkey string, data []byte) ([]byte, error) {
	i := e.cipherIndexFromRkey(rkey)
	block := e.blocks[i]

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// TODO: GCM-SIV
	enc := aesgcm.Seal(nil, nil, data, nil)
	return enc, nil
}

// Takes in an atproto Record Key and bytes of data encrypted.
// Returns the data post-encryption.
//
// Decrypts the data using the cipher given by e.keys[hash(rkey)]
func (e *AesEncrypter) Decrypt(rkey string, encrypted []byte) ([]byte, error) {
	i := e.cipherIndexFromRkey(rkey)
	cipher := e.blocks[i]

	var src []byte
	cipher.Decrypt(encrypted, src)
	return src, nil
}

func (e *AesEncrypter) cipherIndexFromRkey(rkey string) int {
	hashed := sha256.New().Sum([]byte(rkey))
	r := int(binary.BigEndian.Uint64(hashed[:8] /* first 8 bytes */))
	return r % len(e.blocks)
}
