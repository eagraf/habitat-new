package encrypter

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func randomKey(numBytes int) string {
	bytes := make([]byte, numBytes) //generate a random numBytes byte key
	if _, err := rand.Read(bytes); err != nil {
		panic(err.Error())
	}

	return hex.EncodeToString(bytes) //encode key in bytes to string and keep as secret, put in a vault
}

func TestAesEncrypter(t *testing.T) {
	e, err := NewFromKey([]byte(randomKey(16)))
	require.NoError(t, err)

	rkey := "my-rkey"
	data := []byte("this is my data lalalala")

	// Make sure decrypt(encrypted) == encrypt(decrypted)
	enc, err := e.Encrypt(rkey, data)
	require.NoError(t, err)
	dec, err := e.Decrypt(rkey, enc)
	require.NoError(t, err)
	require.Equal(t, dec, data)
}
