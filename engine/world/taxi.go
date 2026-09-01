package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func (s *session) handleTaxiNodeStatusQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	packet := protocol.NewBuffer(9)
	packet.WriteU64(guid)
	packet.WriteU8(1) // Known status
	_ = s.write(uint16(protocol.OpcodeSMSG_TAXINODE_STATUS), packet.Bytes(), true)
	return true
}

func (s *session) handleTaxiQueryAvailableNodes(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	return s.sendTaxiMenu(ctx, guid)
}

func (s *session) sendTaxiMenu(ctx context.Context, flightMasterGUID uint64) bool {
	creatureEntry := uint32((flightMasterGUID >> 24) & 0xFFFFFF)
	curNode := creatureEntry
	if curNode == 0 {
		curNode = 1
	}
	// Send SMSG_NEW_TAXI_PATH
	_ = s.write(uint16(protocol.OpcodeSMSG_NEW_TAXI_PATH), nil, true)
	// Build SMSG_SHOWTAXINODES (0x1A9)
	packet := protocol.NewBuffer(4 + 8 + 4 + 8*4)
	packet.WriteU32(1)                // unk
	packet.WriteU64(flightMasterGUID) // Flight Master GUID
	packet.WriteU32(curNode)          // Current Node
	// 8 uint32s of taximask (all discovered)
	for i := 0; i < 8; i++ {
		packet.WriteU32(0xFFFFFFFF)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_SHOWTAXINODES), packet.Bytes(), true)
	s.debug("taxi menu sent", "account", s.accountName, "master", flightMasterGUID, "node", curNode)
	return true
}

func (s *session) handleActivateTaxi(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 16 {
		return true
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	sourceNode, err := reader.ReadU32()
	if err != nil {
		return false
	}
	destNode, err := reader.ReadU32()
	if err != nil {
		return false
	}
	// Reply with Success (0)
	packet := protocol.NewBuffer(4)
	packet.WriteU32(0) // ERR_TAXIOK = 0
	_ = s.write(uint16(protocol.OpcodeSMSG_ACTIVATETAXIREPLY), packet.Bytes(), true)
	s.debug("taxi flight activated", "account", s.accountName, "master", guid, "source", sourceNode, "dest", destNode)
	return true
}
