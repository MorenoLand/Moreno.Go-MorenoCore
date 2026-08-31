package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	Argon2HashLength        = 16
	Argon2EncodedLength     = 100
	Argon2DefaultIterations = 10
	Argon2DefaultMemoryKiB  = 1 << 17
	Argon2Parallelism       = 1
	Argon2SaltLength        = 16
)

func HashArgon2id(password string, salt []byte, iterations, memoryKiB uint32) (string, error) {
	if iterations == 0 {
		iterations = Argon2DefaultIterations
	}
	if memoryKiB == 0 {
		memoryKiB = Argon2DefaultMemoryKiB
	}
	if len(salt) == 0 {
		salt = make([]byte, Argon2SaltLength)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return "", err
		}
	}
	hash := argon2.IDKey([]byte(password), salt, iterations, memoryKiB, Argon2Parallelism, Argon2HashLength)
	encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memoryKiB, iterations, Argon2Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash))
	if len(encoded) > Argon2EncodedLength {
		return "", errors.New("encoded Argon2id hash exceeds reference length")
	}
	return encoded, nil
}

func VerifyArgon2id(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	params := map[string]uint32{}
	for _, part := range strings.Split(parts[3], ",") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 {
			return false
		}
		n, err := strconv.ParseUint(pair[1], 10, 32)
		if err != nil {
			return false
		}
		params[pair[0]] = uint32(n)
	}
	if params["m"] == 0 || params["t"] == 0 || params["p"] == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, params["t"], params["m"], uint8(params["p"]), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
