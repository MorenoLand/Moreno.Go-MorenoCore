package crypto

import (
	"crypto/rc4"
	"testing"
)

func TestAuthCryptDirectionRoundTrip(t *testing.T) {
	key := make([]byte, SRP6SessionKeyLength)
	for i := range key {
		key[i] = byte(i)
	}
	server, err := NewAuthCrypt(key)
	if err != nil {
		t.Fatal(err)
	}
	serverEncryptionKey := []byte{0xCC, 0x98, 0xAE, 0x04, 0xE8, 0x97, 0xEA, 0xCA, 0x12, 0xDD, 0xC0, 0x93, 0x42, 0x91, 0x53, 0x57}
	digest := hmacSHA1(serverEncryptionKey, key)
	clientDecrypt, err := rc4.NewCipher(digest[:])
	if err != nil {
		t.Fatal(err)
	}
	var drop [1024]byte
	clientDecrypt.XORKeyStream(drop[:], drop[:])
	payload := []byte("world packet payload")
	original := append([]byte(nil), payload...)
	if err := server.EncryptSend(payload); err != nil {
		t.Fatal(err)
	}
	if string(payload) == string(original) {
		t.Fatal("encryption did not change payload")
	}
	client := &AuthCrypt{clientDecrypt: clientDecrypt, initialized: true}
	if err := client.DecryptRecv(payload); err != nil {
		t.Fatal(err)
	}
	if string(payload) != string(original) {
		t.Fatalf("decrypted payload %q, want %q", payload, original)
	}
}

