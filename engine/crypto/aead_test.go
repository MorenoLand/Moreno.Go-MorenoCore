package crypto

import (
	"bytes"
	"testing"
)

func TestAESGCMRoundTrip(t *testing.T) {
	var key [AESKeySize]byte
	for i := range key {
		key[i] = byte(i)
	}
	data := []byte("authenticated payload")
	original := append([]byte(nil), data...)
	if err := EncryptWithRandomIV(&data, key); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(data, original) {
		t.Fatal("encryption did not change payload")
	}
	if err := DecryptWithTrailingIVAndTag(&data, key); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, original) {
		t.Fatalf("payload=%q", data)
	}
}
