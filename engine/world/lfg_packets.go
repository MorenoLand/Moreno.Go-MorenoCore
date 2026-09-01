package world

import "github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"

func (s *session) handleLFGJoin(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	b := protocol.NewReader(payload)
	roles, err := b.ReadU32()
	if err != nil {
		return false
	}
	if _, err := b.ReadBool(); err != nil {
		return false
	}
	if _, err := b.ReadBool(); err != nil {
		return false
	}
	count, err := b.ReadU8()
	if err != nil || count > 50 {
		return false
	}
	dungeons := make([]uint32, 0, count)
	for index := uint8(0); index < count; index++ {
		dungeon, readErr := b.ReadU32()
		if readErr != nil {
			return false
		}
		dungeons = append(dungeons, dungeon&0x00FFFFFF)
	}
	needsCount, err := b.ReadU8()
	if err != nil {
		return false
	}
	if needsCount > 16 || b.Remaining() < int(needsCount) {
		return false
	}
	if _, err := b.Read(int(needsCount)); err != nil {
		return false
	}
	comment, err := b.ReadCString()
	if err != nil {
		return false
	}
	if len(comment) > 255 {
		comment = comment[:255]
	}
	result, entry := s.server.Features.LFG.Join(s.playerGUID, uint8(roles), dungeons, comment)
	s.debug("lfg join", "account", s.accountName, "result", result, "dungeons", entry.Dungeons)
	if err := s.sendLFGJoinResult(result, uint32(LFGStateNone)); err != nil {
		return false
	}
	if result == LFGJoinOK {
		return s.sendLFGUpdatePlayer(LFGUpdateJoinQueue, entry) == nil
	}
	return true
}

func (s *session) handleLFGLeave() bool {
	if !s.playerLoaded {
		return true
	}
	if s.server.Features.LFG.Leave(s.playerGUID) {
		s.debug("lfg leave", "account", s.accountName)
	}
	return s.sendLFGUpdatePlayer(LFGUpdateRemovedFromQueue, LFGQueueEntry{GUID: s.playerGUID, State: LFGStateNone}) == nil
}

func (s *session) handleLFGGetStatus() bool {
	if !s.playerLoaded {
		return true
	}
	entry, _ := s.server.Features.LFG.Status(s.playerGUID)
	if err := s.sendLFGUpdatePlayer(LFGUpdateStatus, entry); err != nil {
		return false
	}
	return s.sendLFGUpdateParty(LFGUpdateStatus) == nil
}

func (s *session) sendLFGJoinResult(result uint32, state uint32) error {
	packet := protocol.NewBuffer(8)
	packet.WriteU32(result)
	packet.WriteU32(state)
	return s.write(uint16(protocol.OpcodeSMSG_LFG_JOIN_RESULT), packet.Bytes(), true)
}

func (s *session) sendLFGUpdatePlayer(updateType uint8, entry LFGQueueEntry) error {
	packet := protocol.NewBuffer(16 + len(entry.Dungeons)*4 + len(entry.Comment))
	packet.WriteU8(updateType)
	if len(entry.Dungeons) == 0 || entry.State == LFGStateNone {
		packet.WriteU8(0)
	} else {
		packet.WriteU8(1)
		packet.WriteU8(1)
		packet.WriteU8(0)
		packet.WriteU8(0)
		packet.WriteU8(uint8(len(entry.Dungeons)))
		for _, dungeon := range entry.Dungeons {
			packet.WriteU32(dungeon)
		}
		packet.WriteCString(entry.Comment)
	}
	return s.write(uint16(protocol.OpcodeSMSG_LFG_UPDATE_PLAYER), packet.Bytes(), true)
}

func (s *session) sendLFGUpdateParty(updateType uint8) error {
	packet := protocol.NewBuffer(2)
	packet.WriteU8(updateType)
	packet.WriteU8(0)
	return s.write(uint16(protocol.OpcodeSMSG_LFG_UPDATE_PARTY), packet.Bytes(), true)
}
