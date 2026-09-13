package mpq

import (
	"encoding/binary"
	"errors"
)

func decompressSparse(data []byte, expected uint32) ([]byte, error) {
	if len(data) < 5 {
		return nil, errors.New("invalid MPQ sparse stream")
	}
	declared := binary.BigEndian.Uint32(data[:4])
	if declared != expected {
		return nil, errors.New("MPQ sparse size mismatch")
	}
	output := make([]byte, 0, declared)
	remaining := declared
	position := 4
	for position < len(data) && remaining > 0 {
		marker := data[position]
		position++
		chunkSize := uint32(marker&0x7F) + 3
		if marker&0x80 != 0 {
			chunkSize = uint32(marker&0x7F) + 1
			if uint64(position)+uint64(chunkSize) > uint64(len(data)) {
				return nil, errors.New("truncated MPQ sparse data chunk")
			}
			if chunkSize > remaining {
				return nil, errors.New("MPQ sparse output overflow")
			}
			output = append(output, data[position:position+int(chunkSize)]...)
			position += int(chunkSize)
		} else {
			if chunkSize > remaining {
				return nil, errors.New("MPQ sparse output overflow")
			}
			output = append(output, make([]byte, chunkSize)...)
		}
		remaining -= chunkSize
	}
	if remaining != 0 {
		return nil, errors.New("truncated MPQ sparse stream")
	}
	return output, nil
}
