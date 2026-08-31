package protocol

func BuildSystemChatMessage(message string) []byte {
	packet := NewBuffer(32 + len(message))
	packet.WriteU8(0)
	packet.WriteU32(0)
	packet.WriteU64(0)
	packet.WriteU32(0)
	packet.WriteU64(0)
	packet.WriteU32(uint32(len(message) + 1))
	packet.WriteCString(message)
	packet.WriteU8(0)
	return packet.Bytes()
}
