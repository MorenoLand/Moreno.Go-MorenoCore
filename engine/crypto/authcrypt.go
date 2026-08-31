package crypto

import (
	"crypto/hmac"
	"crypto/rc4"
	"crypto/sha1"
	"errors"
)

type AuthCrypt struct {
	clientDecrypt *rc4.Cipher
	serverEncrypt *rc4.Cipher
	initialized   bool
}

func NewAuthCrypt(sessionKey []byte) (*AuthCrypt, error) {
	serverKey := []byte{0xCC, 0x98, 0xAE, 0x04, 0xE8, 0x97, 0xEA, 0xCA, 0x12, 0xDD, 0xC0, 0x93, 0x42, 0x91, 0x53, 0x57}
	clientKey := []byte{0xC2, 0xB3, 0x72, 0x3C, 0xC6, 0xAE, 0xD9, 0xB5, 0x34, 0x3C, 0x53, 0xEE, 0x2F, 0x43, 0x67, 0xCE}
	serverDigest := hmacSHA1(serverKey, sessionKey)
	clientDigest := hmacSHA1(clientKey, sessionKey)
	server, err := rc4.NewCipher(serverDigest[:])
	if err != nil {
		return nil, err
	}
	client, err := rc4.NewCipher(clientDigest[:])
	if err != nil {
		return nil, err
	}
	var drop [1024]byte
	server.XORKeyStream(drop[:], drop[:])
	client.XORKeyStream(drop[:], drop[:])
	return &AuthCrypt{clientDecrypt: client, serverEncrypt: server, initialized: true}, nil
}

func NewClientAuthCrypt(sessionKey []byte) (*AuthCrypt, error) {
	serverKey := []byte{0xCC, 0x98, 0xAE, 0x04, 0xE8, 0x97, 0xEA, 0xCA, 0x12, 0xDD, 0xC0, 0x93, 0x42, 0x91, 0x53, 0x57}
	clientKey := []byte{0xC2, 0xB3, 0x72, 0x3C, 0xC6, 0xAE, 0xD9, 0xB5, 0x34, 0x3C, 0x53, 0xEE, 0x2F, 0x43, 0x67, 0xCE}
	serverDigest := hmacSHA1(serverKey, sessionKey)
	clientDigest := hmacSHA1(clientKey, sessionKey)
	clientDecrypt, err := rc4.NewCipher(serverDigest[:])
	if err != nil {
		return nil, err
	}
	clientEncrypt, err := rc4.NewCipher(clientDigest[:])
	if err != nil {
		return nil, err
	}
	var drop [1024]byte
	clientDecrypt.XORKeyStream(drop[:], drop[:])
	clientEncrypt.XORKeyStream(drop[:], drop[:])
	return &AuthCrypt{clientDecrypt: clientDecrypt, serverEncrypt: clientEncrypt, initialized: true}, nil
}

func (c *AuthCrypt) EncryptSend(data []byte) error { return c.crypt(c.serverEncrypt, data) }
func (c *AuthCrypt) DecryptRecv(data []byte) error { return c.crypt(c.clientDecrypt, data) }
func (c *AuthCrypt) Initialized() bool             { return c.initialized }

func (c *AuthCrypt) crypt(cipher *rc4.Cipher, data []byte) error {
	if !c.initialized || cipher == nil {
		return errors.New("authentication crypt is not initialized")
	}
	cipher.XORKeyStream(data, data)
	return nil
}

func hmacSHA1(key, data []byte) [sha1.Size]byte {
	h := hmac.New(sha1.New, key)
	_, _ = h.Write(data)
	var digest [sha1.Size]byte
	copy(digest[:], h.Sum(nil))
	return digest
}
