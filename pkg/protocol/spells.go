package protocol

const (
	SpellTargetFlagUnit           uint32 = 0x00000002
	SpellTargetFlagItem           uint32 = 0x00000010
	SpellTargetFlagSourceLocation uint32 = 0x00000020
	SpellTargetFlagDestLocation   uint32 = 0x00000040
	SpellTargetFlagCorpseEnemy    uint32 = 0x00000200
	SpellTargetFlagGameObject     uint32 = 0x00000800
	SpellTargetFlagTradeItem      uint32 = 0x00001000
	SpellTargetFlagString         uint32 = 0x00002000
	SpellTargetFlagCorpseAlly     uint32 = 0x00008000
	SpellTargetFlagUnitMinipet    uint32 = 0x00010000
	SpellTargetFlagDestTarget     uint32 = 0x00040000
	SpellTargetFlagUnitWireMask          = SpellTargetFlagUnit | SpellTargetFlagUnitMinipet | SpellTargetFlagGameObject | SpellTargetFlagCorpseEnemy | SpellTargetFlagCorpseAlly
	SpellTargetFlagItemWireMask          = SpellTargetFlagItem | SpellTargetFlagTradeItem
	SpellCastFlagVisualChain      uint32 = 0x00080000
	SpellMissReflect              uint8  = 2
)

type SpellTargetLocation struct {
	Transport uint64
	X         float32
	Y         float32
	Z         float32
}

type SpellTargetData struct {
	Flags        uint32
	UnitGUID     uint64
	ItemGUID     uint64
	Source       SpellTargetLocation
	Destination  SpellTargetLocation
	StringTarget string
}

type SpellMissStatus struct {
	TargetGUID    uint64
	Reason        uint8
	ReflectStatus uint8
}

func ReadSpellTargetData(reader *Buffer) (SpellTargetData, error) {
	var target SpellTargetData
	var err error
	if target.Flags, err = reader.ReadU32(); err != nil {
		return target, err
	}
	if target.Flags&SpellTargetFlagUnitWireMask != 0 {
		if target.UnitGUID, err = reader.ReadPackedGUID(); err != nil {
			return target, err
		}
	}
	if target.Flags&SpellTargetFlagItemWireMask != 0 {
		if target.ItemGUID, err = reader.ReadPackedGUID(); err != nil {
			return target, err
		}
	}
	if target.Flags&SpellTargetFlagSourceLocation != 0 {
		if target.Source, err = readSpellTargetLocation(reader); err != nil {
			return target, err
		}
	}
	if target.Flags&SpellTargetFlagDestLocation != 0 {
		if target.Destination, err = readSpellTargetLocation(reader); err != nil {
			return target, err
		}
	}
	if target.Flags&SpellTargetFlagString != 0 {
		if target.StringTarget, err = reader.ReadCString(); err != nil {
			return target, err
		}
	}
	return target, nil
}

func readSpellTargetLocation(reader *Buffer) (SpellTargetLocation, error) {
	var location SpellTargetLocation
	var err error
	if location.Transport, err = reader.ReadPackedGUID(); err != nil {
		return location, err
	}
	if location.X, err = reader.ReadF32(); err != nil {
		return location, err
	}
	if location.Y, err = reader.ReadF32(); err != nil {
		return location, err
	}
	if location.Z, err = reader.ReadF32(); err != nil {
		return location, err
	}
	return location, nil
}

func BuildSpellStart(casterGUID, casterUnitGUID uint64, castID uint8, spellID, castFlags, castTime uint32, target SpellTargetData) []byte {
	packet := NewBuffer(64)
	writeSpellCastHeader(packet, casterGUID, casterUnitGUID, castID, spellID, castFlags, castTime)
	writeSpellTargetData(packet, target)
	writeSpellCastTrailer(packet, castFlags, target.Flags)
	return packet.Bytes()
}

func BuildSpellGo(casterGUID, casterUnitGUID uint64, castID uint8, spellID, castFlags, castTime uint32, hitTargets []uint64, missStatus []SpellMissStatus, target SpellTargetData) []byte {
	packet := NewBuffer(96)
	writeSpellCastHeader(packet, casterGUID, casterUnitGUID, castID, spellID, castFlags, castTime)
	if len(hitTargets) > 255 {
		hitTargets = hitTargets[:255]
	}
	packet.WriteU8(uint8(len(hitTargets)))
	for _, guid := range hitTargets {
		packet.WriteU64(guid)
	}
	if len(missStatus) > 255 {
		missStatus = missStatus[:255]
	}
	packet.WriteU8(uint8(len(missStatus)))
	for _, status := range missStatus {
		packet.WriteU64(status.TargetGUID)
		packet.WriteU8(status.Reason)
		if status.Reason == SpellMissReflect {
			packet.WriteU8(status.ReflectStatus)
		}
	}
	writeSpellTargetData(packet, target)
	writeSpellCastTrailer(packet, castFlags, target.Flags)
	return packet.Bytes()
}

func BuildSpellFailure(castID uint8, spellID uint32, result uint8) []byte {
	packet := NewBuffer(6)
	packet.WriteU8(castID)
	packet.WriteU32(spellID)
	packet.WriteU8(result)
	return packet.Bytes()
}

func writeSpellCastHeader(packet *Buffer, casterGUID, casterUnitGUID uint64, castID uint8, spellID, castFlags, castTime uint32) {
	packet.WritePackedGUID(casterGUID)
	packet.WritePackedGUID(casterUnitGUID)
	packet.WriteU8(castID)
	packet.WriteU32(spellID)
	packet.WriteU32(castFlags)
	packet.WriteU32(castTime)
}

func writeSpellTargetData(packet *Buffer, target SpellTargetData) {
	packet.WriteU32(target.Flags)
	if target.Flags&SpellTargetFlagUnitWireMask != 0 {
		packet.WritePackedGUID(target.UnitGUID)
	}
	if target.Flags&SpellTargetFlagItemWireMask != 0 {
		packet.WritePackedGUID(target.ItemGUID)
	}
	if target.Flags&SpellTargetFlagSourceLocation != 0 {
		writeSpellTargetLocation(packet, target.Source)
	}
	if target.Flags&SpellTargetFlagDestLocation != 0 {
		writeSpellTargetLocation(packet, target.Destination)
	}
	if target.Flags&SpellTargetFlagString != 0 {
		packet.WriteCString(target.StringTarget)
	}
}

func writeSpellTargetLocation(packet *Buffer, location SpellTargetLocation) {
	packet.WritePackedGUID(location.Transport)
	packet.WriteF32(location.X)
	packet.WriteF32(location.Y)
	packet.WriteF32(location.Z)
}

func writeSpellCastTrailer(packet *Buffer, castFlags, targetFlags uint32) {
	if castFlags&SpellCastFlagVisualChain != 0 {
		packet.WriteU32(0)
		packet.WriteU32(0)
	}
	if targetFlags&SpellTargetFlagDestLocation != 0 {
		packet.WriteU8(0)
	}
}
