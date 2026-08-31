package protocol

import "time"

type Packet struct {
	Opcode     uint16
	Payload    *Buffer
	ReceivedAt time.Time
}

func NewPacket(opcode uint16, capacity int) *Packet {
	return &Packet{Opcode: opcode, Payload: NewBuffer(capacity)}
}
func PacketFrom(opcode uint16, payload []byte) *Packet {
	return &Packet{Opcode: opcode, Payload: NewReader(payload)}
}
