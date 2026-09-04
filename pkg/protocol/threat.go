package protocol

// ThreatEntry represents a target on a creature's threat list.
// Reference: TrinityCore ThreatManager.h / ThreatReference.
type ThreatEntry struct {
	VictimGUID uint64
	Threat     uint32
}

// BuildThreatClear builds an SMSG_THREAT_CLEAR packet.
// Reference: TrinityCore ThreatManager.cpp:755-760.
func BuildThreatClear(creatureGUID uint64) []byte {
	buf := NewBuffer(9)
	buf.WritePackedGUID(creatureGUID)
	return buf.Bytes()
}

// BuildThreatRemove builds an SMSG_THREAT_REMOVE packet.
// Reference: TrinityCore ThreatManager.cpp:762-768.
func BuildThreatRemove(creatureGUID, victimGUID uint64) []byte {
	buf := NewBuffer(18)
	buf.WritePackedGUID(creatureGUID)
	buf.WritePackedGUID(victimGUID)
	return buf.Bytes()
}

// BuildThreatUpdate builds an SMSG_THREAT_UPDATE packet.
// Reference: TrinityCore ThreatManager.cpp:770-789.
func BuildThreatUpdate(creatureGUID uint64, list []ThreatEntry) []byte {
	buf := NewBuffer(9 + 4 + len(list)*13)
	buf.WritePackedGUID(creatureGUID)
	buf.WriteU32(uint32(len(list)))
	for _, entry := range list {
		buf.WritePackedGUID(entry.VictimGUID)
		buf.WriteU32(entry.Threat)
	}
	return buf.Bytes()
}

// BuildHighestThreatUpdate builds an SMSG_HIGHEST_THREAT_UPDATE packet.
// Reference: TrinityCore ThreatManager.cpp:770-789.
func BuildHighestThreatUpdate(creatureGUID, highestGUID uint64, list []ThreatEntry) []byte {
	buf := NewBuffer(18 + 4 + len(list)*13)
	buf.WritePackedGUID(creatureGUID)
	buf.WritePackedGUID(highestGUID)
	buf.WriteU32(uint32(len(list)))
	for _, entry := range list {
		buf.WritePackedGUID(entry.VictimGUID)
		buf.WriteU32(entry.Threat)
	}
	return buf.Bytes()
}
