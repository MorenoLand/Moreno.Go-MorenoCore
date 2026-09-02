//go:build ignore

package crypto

import (
	"bytes"
	"math/big"
	"testing"
)

func TestSRP6RegistrationAndChallenge(t *testing.T) {
	username := "TEST"
	password := "PASSWORD"
	var saltReader [SRP6SaltLength]byte
	salt, verifier, err := MakeRegistrationDataWithReader(username, password, bytes.NewReader(saltReader[:]))
	if err != nil {
		t.Fatal(err)
	}
	if !CheckLogin(username, password, salt, verifier) {
		t.Fatal("registration verifier rejected its password")
	}
	server, err := NewSRP6WithReader(username, salt, verifier, bytes.NewReader(bytes.Repeat([]byte{0x19}, SRP6EphemeralLength)))
	if err != nil {
		t.Fatal(err)
	}
	a := littleInt(bytes.Repeat([]byte{0x27}, SRP6EphemeralLength))
	A := littleBytes(new(big.Int).Exp(srpGenerator, a, srpModulus), SRP6EphemeralLength)
	uHash := hash(A[:], server.B[:])
	u := littleInt(uHash[:])
	inner := hash([]byte(username), []byte(":"), []byte(password))
	xHash := hash(salt[:], inner[:])
	x := littleInt(xHash[:])
	clientBase := new(big.Int).Sub(littleInt(server.B[:]), new(big.Int).Mul(big.NewInt(3), new(big.Int).Exp(srpGenerator, x, srpModulus)))
	clientBase.Mod(clientBase, srpModulus)
	clientExponent := new(big.Int).Add(a, new(big.Int).Mul(u, x))
	clientS := new(big.Int).Exp(clientBase, clientExponent, srpModulus)
	clientK := interleave(littleBytes(clientS, SRP6EphemeralLength))
	nBytes := littleBytes(srpModulus, SRP6EphemeralLength)
	var ng [20]byte
	nHash := hash(nBytes[:])
	gHash := hash([]byte{7})
	for i := range ng {
		ng[i] = nHash[i] ^ gHash[i]
	}
	iHash := hash([]byte(username))
	clientM := hash(ng[:], iHash[:], salt[:], A[:], server.B[:], clientK[:])
	serverK, ok, err := server.VerifyChallengeResponse(A, clientM)
	if err != nil || !ok {
		t.Fatalf("challenge failed: ok=%v err=%v", ok, err)
	}
	if serverK != clientK {
		t.Fatal("server and client session keys differ")
	}
	if _, ok, err := server.VerifyChallengeResponse(A, clientM); err == nil || ok {
		t.Fatal("reused SRP6 state accepted")
	}
}

