package crypto

import (
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"errors"
	"io"
	"math/big"
)

const (
	SRP6SaltLength       = 32
	SRP6VerifierLength   = 32
	SRP6EphemeralLength  = 32
	SRP6SessionKeyLength = 40
)

var srpModulus, _ = new(big.Int).SetString("894B645E89E1535BBDAD5B8B290650530801B18EBFBF5E8FAB3C82872A3E9BB7", 16)
var srpGenerator = big.NewInt(7)

func SRP6ModulusBytes() [SRP6EphemeralLength]byte {
	return littleBytes(srpModulus, SRP6EphemeralLength)
}
func SRP6GeneratorBytes() []byte { return []byte{7} }
func SessionVerifier(A [SRP6EphemeralLength]byte, clientM [sha1.Size]byte, sessionKey [SRP6SessionKeyLength]byte) [sha1.Size]byte {
	return hash(A[:], clientM[:], sessionKey[:])
}

func MakeClientProof(username, password string, salt [SRP6SaltLength]byte, B [SRP6EphemeralLength]byte, privateKey []byte) ([SRP6EphemeralLength]byte, [sha1.Size]byte, [SRP6SessionKeyLength]byte, error) {
	if len(privateKey) == 0 {
		return [SRP6EphemeralLength]byte{}, [sha1.Size]byte{}, [SRP6SessionKeyLength]byte{}, errors.New("SRP6 client private key is empty")
	}
	a := littleInt(privateKey)
	A := littleBytes(new(big.Int).Exp(srpGenerator, a, srpModulus), SRP6EphemeralLength)
	uHash := hash(A[:], B[:])
	u := littleInt(uHash[:])
	inner := hash([]byte(username), []byte(":"), []byte(password))
	xHash := hash(salt[:], inner[:])
	x := littleInt(xHash[:])
	base := new(big.Int).Sub(littleInt(B[:]), new(big.Int).Mul(big.NewInt(3), new(big.Int).Exp(srpGenerator, x, srpModulus)))
	base.Mod(base, srpModulus)
	exponent := new(big.Int).Add(a, new(big.Int).Mul(u, x))
	shared := littleBytes(new(big.Int).Exp(base, exponent, srpModulus), SRP6EphemeralLength)
	K := interleave(shared)
	nBytes := littleBytes(srpModulus, SRP6EphemeralLength)
	nHash := hash(nBytes[:])
	gHash := hash([]byte{7})
	var ngHash [sha1.Size]byte
	for i := range ngHash {
		ngHash[i] = nHash[i] ^ gHash[i]
	}
	iHash := hash([]byte(username))
	M := hash(ngHash[:], iHash[:], salt[:], A[:], B[:], K[:])
	return A, M, K, nil
}

type SRP6 struct {
	username string
	salt     [SRP6SaltLength]byte
	verifier [SRP6VerifierLength]byte
	b        *big.Int
	B        [SRP6EphemeralLength]byte
	used     bool
}

func MakeRegistrationData(username, password string) ([SRP6SaltLength]byte, [SRP6VerifierLength]byte, error) {
	return MakeRegistrationDataWithReader(username, password, rand.Reader)
}

func MakeRegistrationDataWithReader(username, password string, reader io.Reader) ([SRP6SaltLength]byte, [SRP6VerifierLength]byte, error) {
	var salt [SRP6SaltLength]byte
	if _, err := io.ReadFull(reader, salt[:]); err != nil {
		return salt, [SRP6VerifierLength]byte{}, err
	}
	return salt, calculateVerifier(username, password, salt), nil
}

func CheckLogin(username, password string, salt [SRP6SaltLength]byte, verifier [SRP6VerifierLength]byte) bool {
	calculated := calculateVerifier(username, password, salt)
	return subtle.ConstantTimeCompare(calculated[:], verifier[:]) == 1
}

func NewSRP6(username string, salt [SRP6SaltLength]byte, verifier [SRP6VerifierLength]byte) (*SRP6, error) {
	return NewSRP6WithReader(username, salt, verifier, rand.Reader)
}

func NewSRP6WithReader(username string, salt [SRP6SaltLength]byte, verifier [SRP6VerifierLength]byte, reader io.Reader) (*SRP6, error) {
	bBytes := make([]byte, SRP6EphemeralLength)
	if _, err := io.ReadFull(reader, bBytes); err != nil {
		return nil, err
	}
	b := littleInt(bBytes)
	value := new(big.Int).Exp(srpGenerator, b, srpModulus)
	value.Add(value, new(big.Int).Mul(big.NewInt(3), littleInt(verifier[:])))
	value.Mod(value, srpModulus)
	return &SRP6{username: username, salt: salt, verifier: verifier, b: b, B: littleBytes(value, SRP6EphemeralLength)}, nil
}

func (s *SRP6) VerifyChallengeResponse(A [SRP6EphemeralLength]byte, clientM [sha1.Size]byte) ([SRP6SessionKeyLength]byte, bool, error) {
	if s == nil {
		return [SRP6SessionKeyLength]byte{}, false, errors.New("nil SRP6 state")
	}
	if s.used {
		return [SRP6SessionKeyLength]byte{}, false, errors.New("SRP6 state already used")
	}
	s.used = true
	a := littleInt(A[:])
	if new(big.Int).Mod(new(big.Int).Set(a), srpModulus).Sign() == 0 {
		return [SRP6SessionKeyLength]byte{}, false, nil
	}
	uHash := hash(A[:], s.B[:])
	u := littleInt(uHash[:])
	v := littleInt(s.verifier[:])
	base := new(big.Int).Mul(a, new(big.Int).Exp(v, u, srpModulus))
	S := new(big.Int).Exp(base, s.b, srpModulus)
	shared := littleBytes(S, SRP6EphemeralLength)
	K := interleave(shared)
	nBytes := littleBytes(srpModulus, SRP6EphemeralLength)
	NHash := hash(nBytes[:])
	gHash := hash([]byte{7})
	var ng [sha1.Size]byte
	for i := range ng {
		ng[i] = NHash[i] ^ gHash[i]
	}
	I := hash([]byte(s.username))
	ourM := hash(ng[:], I[:], s.salt[:], A[:], s.B[:], K[:])
	if subtle.ConstantTimeCompare(ourM[:], clientM[:]) != 1 {
		return [SRP6SessionKeyLength]byte{}, false, nil
	}
	return K, true, nil
}

func calculateVerifier(username, password string, salt [SRP6SaltLength]byte) [SRP6VerifierLength]byte {
	inner := hash([]byte(username), []byte(":"), []byte(password))
	xHash := hash(salt[:], inner[:])
	x := littleInt(xHash[:])
	v := new(big.Int).Exp(srpGenerator, x, srpModulus)
	return littleBytes(v, SRP6VerifierLength)
}

func interleave(S [SRP6EphemeralLength]byte) [SRP6SessionKeyLength]byte {
	var even, odd [SRP6EphemeralLength / 2]byte
	for i := 0; i < len(even); i++ {
		even[i] = S[2*i]
		odd[i] = S[2*i+1]
	}
	p := 0
	for p < len(S) && S[p] == 0 {
		p++
	}
	if p&1 != 0 {
		p++
	}
	p /= 2
	h0 := hash(even[p:])
	h1 := hash(odd[p:])
	var K [SRP6SessionKeyLength]byte
	for i := 0; i < sha1.Size; i++ {
		K[2*i] = h0[i]
		K[2*i+1] = h1[i]
	}
	return K
}

func hash(parts ...[]byte) [sha1.Size]byte {
	h := sha1.New()
	for _, part := range parts {
		_, _ = h.Write(part)
	}
	var result [sha1.Size]byte
	copy(result[:], h.Sum(nil))
	return result
}

func littleInt(data []byte) *big.Int {
	reversed := make([]byte, len(data))
	for i := range data {
		reversed[len(data)-1-i] = data[i]
	}
	return new(big.Int).SetBytes(reversed)
}

func littleBytes(value *big.Int, size int) (result [SRP6VerifierLength]byte) {
	bytes := value.Bytes()
	for i := 0; i < len(bytes) && i < size; i++ {
		result[i] = bytes[len(bytes)-1-i]
	}
	return result
}
