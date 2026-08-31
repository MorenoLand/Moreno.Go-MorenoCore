package protocol

func BuildChatMessage(chatType uint8, language uint32, senderGUID, receiverGUID uint64, message, channel string) []byte {
	packet := NewBuffer(48 + len(message) + len(channel))
	packet.WriteU8(chatType)
	packet.WriteU32(language)
	packet.WriteU64(senderGUID)
	packet.WriteU32(0)
	if chatType == 0x11 && channel != "" {
		packet.WriteCString(channel)
	}
	packet.WriteU64(receiverGUID)
	packet.WriteU32(uint32(len(message) + 1))
	packet.WriteCString(message)
	packet.WriteU8(0)
	return packet.Bytes()
}

func BuildSystemChatMessage(message string) []byte {
	return BuildChatMessage(0, 0, 0, 0, message, "")
}
