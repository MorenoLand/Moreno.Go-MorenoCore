package crypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"time"
)

const TOTPInterval = 30 * time.Second

func GenerateTOTP(secret []byte, timestamp time.Time) uint32 {
	counter := uint64(timestamp.Unix()) / uint64(TOTPInterval/time.Second)
	var challenge [8]byte
	binary.BigEndian.PutUint64(challenge[:], counter)
	h := hmac.New(sha1.New, secret)
	_, _ = h.Write(challenge[:])
	digest := h.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	value := uint32(digest[offset])<<24 | uint32(digest[offset+1])<<16 | uint32(digest[offset+2])<<8 | uint32(digest[offset+3])
	return (value & 0x7fffffff) % 1000000
}

func ValidateTOTP(secret []byte, token uint32, now time.Time) bool {
	if token >= 1000000 {
		return false
	}
	for _, offset := range []time.Duration{-TOTPInterval, 0, TOTPInterval} {
		if subtle.ConstantTimeEq(int32(token), int32(GenerateTOTP(secret, now.Add(offset)))) == 1 {
			return true
		}
	}
	return false
}
