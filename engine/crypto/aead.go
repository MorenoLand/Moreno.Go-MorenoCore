package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

const (
	AESKeySize = 16
	AESIVSize  = 12
	AESTagSize = 12
)

type AESGCM struct {
	key        [AESKeySize]byte
	encrypting bool
}

func NewAESGCM(encrypting bool, key [AESKeySize]byte) *AESGCM {
	return &AESGCM{key: key, encrypting: encrypting}
}

func (a *AESGCM) Process(iv [AESIVSize]byte, data []byte, tag *[AESTagSize]byte) error {
	if tag == nil {
		return errors.New("AES-GCM tag is nil")
	}
	block, err := aes.NewCipher(a.key[:])
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCMWithTagSize(block, AESTagSize)
	if err != nil {
		return err
	}
	if a.encrypting {
		sealed := gcm.Seal(nil, iv[:], data, nil)
		copy(data, sealed[:len(data)])
		copy(tag[:], sealed[len(data):])
		return nil
	}
	opened, err := gcm.Open(nil, iv[:], append(append([]byte(nil), data...), tag[:]...), nil)
	if err != nil {
		return err
	}
	copy(data, opened)
	return nil
}

func EncryptWithRandomIV(data *[]byte, key [AESKeySize]byte) error {
	if data == nil {
		return errors.New("AES-GCM data is nil")
	}
	var iv [AESIVSize]byte
	if _, err := io.ReadFull(rand.Reader, iv[:]); err != nil {
		return err
	}
	var tag [AESTagSize]byte
	ciphertext := append([]byte(nil), (*data)...)
	if err := NewAESGCM(true, key).Process(iv, ciphertext, &tag); err != nil {
		return err
	}
	*data = append(ciphertext, iv[:]...)
	*data = append(*data, tag[:]...)
	return nil
}

func DecryptWithTrailingIVAndTag(data *[]byte, key [AESKeySize]byte) error {
	if data == nil || len(*data) < AESIVSize+AESTagSize {
		return errors.New("AES-GCM data is too short")
	}
	value := *data
	ivStart := len(value) - AESIVSize - AESTagSize
	var iv [AESIVSize]byte
	var tag [AESTagSize]byte
	copy(iv[:], value[ivStart:ivStart+AESIVSize])
	copy(tag[:], value[ivStart+AESIVSize:])
	payload := value[:ivStart]
	if err := NewAESGCM(false, key).Process(iv, payload, &tag); err != nil {
		return err
	}
	*data = payload
	return nil
}
