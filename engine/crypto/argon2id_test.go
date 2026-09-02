package crypto

import (
	"bytes"
	"testing"
)

func TestArgon2idRoundTrip(t *testing.T) {
	hash, err := HashArgon2id("password", bytes.Repeat([]byte{7}, 16), 1, 8*1024)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyArgon2id("password", hash) {
		t.Fatal("valid password rejected")
	}
	if VerifyArgon2id("wrong", hash) {
		t.Fatal("invalid password accepted")
	}
}
