package encrypter

import (
	"crypto/rand"
	"encoding/hex"
)

func randomKey(numBytes int) string {
	bytes := make([]byte, numBytes) //generate a random numBytes byte key
	if _, err := rand.Read(bytes); err != nil {
		panic(err.Error())
	}

	return hex.EncodeToString(bytes) //encode key in bytes to string and keep as secret, put in a vault
}

/*
func TestAesEncrypter(t *testing.T) {
	var err error
	ciphers := make([]cipher.Block, 5)
	for i := range 5 {
		ciphers[i], err = aes.NewCipher([]byte(randomKey(16)))
		require.NoError(t, err)
	}
	e := NewFromKeys(ciphers)

	rkey := "my-rkey"
	data := []byte("this is my data lalalala")

	// Make sure decrypt(encrypted) == encrypt(decrypted)
	encrypted := e.Encrypt(rkey, data)
	require.Equal(t, data, e.Decrypt(rkey, encrypted))

	// Make sure the right cipher was used
	i := e.cipherIndexFromRkey(rkey)
	cipher := e.ciphers[i]
	var src []byte
	cipher.Decrypt(encrypted, src)
	require.Equal(t, data, src)
}
*/
