package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

const ClientHeaderSize = 6

type ClientFrameHeader struct {
	Size        uint16
	Opcode      uint32
	PayloadSize int
}

func EncodeServerFrame(opcode uint16, payload []byte) ([]byte, int, error) {
	size := len(payload) + 2
	if size > 0xffffff {
		return nil, 0, errors.New("world packet is too large")
	}
	if size > 0x7fff {
		frame := make([]byte, 5+len(payload))
		frame[0] = 0x80 | byte(size>>16)
		frame[1] = byte(size >> 8)
		frame[2] = byte(size)
		binary.LittleEndian.PutUint16(frame[3:5], opcode)
		copy(frame[5:], payload)
		return frame, 5, nil
	}
	frame := make([]byte, 4+len(payload))
	frame[0] = byte(size >> 8)
	frame[1] = byte(size)
	binary.LittleEndian.PutUint16(frame[2:4], opcode)
	copy(frame[4:], payload)
	return frame, 4, nil
}

func DecodeClientFrameHeader(header []byte) (ClientFrameHeader, error) {
	if len(header) != ClientHeaderSize {
		return ClientFrameHeader{}, errors.New("world client header must be six bytes")
	}
	size := binary.BigEndian.Uint16(header[:2])
	if size < 4 || size >= 10240 {
		return ClientFrameHeader{}, errors.New("invalid world client packet size")
	}
	opcode := binary.LittleEndian.Uint32(header[2:])
	return ClientFrameHeader{Size: size, Opcode: opcode, PayloadSize: int(size) - 4}, nil
}

func ReadClientFrame(r io.Reader, decrypt func([]byte) error) (ClientFrameHeader, []byte, error) {
	header := make([]byte, ClientHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return ClientFrameHeader{}, nil, err
	}
	if decrypt != nil {
		if err := decrypt(header); err != nil {
			return ClientFrameHeader{}, nil, err
		}
	}
	parsed, err := DecodeClientFrameHeader(header)
	if err != nil {
		return ClientFrameHeader{}, nil, err
	}
	payload := make([]byte, parsed.PayloadSize)
	if _, err := io.ReadFull(r, payload); err != nil {
		return ClientFrameHeader{}, nil, err
	}
	return parsed, payload, nil
}
