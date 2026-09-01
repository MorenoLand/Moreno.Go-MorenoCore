package protocol

func BuildChatMessage(chatType uint8, language uint32, senderGUID, receiverGUID uint64, message, channel string) []byte {
	return BuildChatMessageWithOptions(chatType, language, senderGUID, receiverGUID, message, channel, false, "", 0)
}

func BuildChatMessageWithOptions(chatType uint8, language uint32, senderGUID, receiverGUID uint64, message, channel string, gmMessage bool, senderName string, chatTag uint8) []byte {
	packet := NewBuffer(48 + len(message) + len(channel))
	packet.WriteU8(chatType)
	packet.WriteU32(language)
	packet.WriteU64(senderGUID)
	packet.WriteU32(0)
	if gmMessage {
		packet.WriteU32(uint32(len(senderName) + 1))
		packet.WriteCString(senderName)
	}
	if chatType == 0x11 {
		packet.WriteCString(channel)
	}
	packet.WriteU64(receiverGUID)
	packet.WriteU32(uint32(len(message) + 1))
	packet.WriteCString(message)
	packet.WriteU8(chatTag)
	return packet.Bytes()
}

func BuildSystemChatMessage(message string) []byte {
	return BuildChatMessage(0, 0, 0, 0, message, "")
}
