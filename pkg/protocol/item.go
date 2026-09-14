package protocol

func BuildItemEnchantTimeUpdate(playerGUID, itemGUID uint64, slot, duration uint32) []byte {
	packet := NewBuffer(24)
	packet.WriteU64(itemGUID)
	packet.WriteU32(slot)
	packet.WriteU32(duration)
	packet.WriteU64(playerGUID)
	return packet.Bytes()
}

func BuildItemTimeUpdate(itemGUID uint64, duration uint32) []byte {
	packet := NewBuffer(12)
	packet.WriteU64(itemGUID)
	packet.WriteU32(duration)
	return packet.Bytes()
}
