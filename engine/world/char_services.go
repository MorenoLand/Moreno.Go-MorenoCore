package world

import (
	"context"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// handleCharRename processes CMSG_CHAR_RENAME (0x2C7).
// Reference: WorldSession::HandleCharRenameOpcode (CharacterHandler.cpp:1111).
func (s *session) handleCharRename(ctx context.Context, payload []byte) bool {
	if len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	guid, err := r.ReadU64()
	if err != nil {
		return false
	}
	newName, err := r.ReadCString()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "UPDATE characters SET name = ? WHERE guid = ?", newName, guid)
	}

	buf := protocol.NewBuffer(1 + 8 + len(newName) + 1)
	buf.WriteU8(0) // RESPONSE_SUCCESS
	buf.WriteU64(guid)
	buf.WriteCString(newName)
	_ = s.write(uint16(protocol.OpcodeSMSG_CHAR_RENAME), buf.Bytes(), true)
	s.debug("character renamed", "guid", guid, "name", newName)
	return true
}

// handleCharCustomize processes CMSG_CHAR_CUSTOMIZE (0x473).
// Reference: WorldSession::HandleCharCustomize (CharacterHandler.cpp:1230).
func (s *session) handleCharCustomize(ctx context.Context, payload []byte) bool {
	if len(payload) < 15 {
		return true
	}
	r := protocol.NewReader(payload)
	guid, _ := r.ReadU64()
	newName, _ := r.ReadCString()
	gender, _ := r.ReadU8()
	skin, _ := r.ReadU8()
	face, _ := r.ReadU8()
	hairStyle, _ := r.ReadU8()
	hairColor, _ := r.ReadU8()
	facialHair, _ := r.ReadU8()

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "UPDATE characters SET name = ?, gender = ?, skin = ?, face = ?, hairStyle = ?, hairColor = ?, facialStyle = ? WHERE guid = ?",
			newName, gender, skin, face, hairStyle, hairColor, facialHair, guid)
	}

	buf := protocol.NewBuffer(16 + len(newName))
	buf.WriteU8(0) // RESPONSE_SUCCESS
	buf.WriteU64(guid)
	buf.WriteCString(newName)
	buf.WriteU8(gender)
	buf.WriteU8(skin)
	buf.WriteU8(face)
	buf.WriteU8(hairStyle)
	buf.WriteU8(hairColor)
	buf.WriteU8(facialHair)
	_ = s.write(uint16(protocol.OpcodeSMSG_CHAR_CUSTOMIZE), buf.Bytes(), true)
	s.debug("character customized", "guid", guid, "name", newName)
	return true
}

// handleCharRaceChange processes CMSG_CHAR_RACE_CHANGE (0x4F8).
func (s *session) handleCharRaceChange(ctx context.Context, payload []byte) bool {
	if len(payload) < 16 {
		return true
	}
	r := protocol.NewReader(payload)
	guid, _ := r.ReadU64()
	newName, _ := r.ReadCString()
	gender, _ := r.ReadU8()
	skin, _ := r.ReadU8()
	face, _ := r.ReadU8()
	hairStyle, _ := r.ReadU8()
	hairColor, _ := r.ReadU8()
	facialHair, _ := r.ReadU8()
	race, _ := r.ReadU8()

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "UPDATE characters SET name = ?, gender = ?, skin = ?, face = ?, hairStyle = ?, hairColor = ?, facialStyle = ?, race = ? WHERE guid = ?",
			newName, gender, skin, face, hairStyle, hairColor, facialHair, race, guid)
	}

	buf := protocol.NewBuffer(17 + len(newName))
	buf.WriteU8(0) // RESPONSE_SUCCESS
	buf.WriteU64(guid)
	buf.WriteCString(newName)
	buf.WriteU8(gender)
	buf.WriteU8(skin)
	buf.WriteU8(face)
	buf.WriteU8(hairStyle)
	buf.WriteU8(hairColor)
	buf.WriteU8(facialHair)
	buf.WriteU8(race)
	_ = s.write(uint16(protocol.OpcodeSMSG_CHAR_FACTION_CHANGE), buf.Bytes(), true)
	return true
}

// handleCharFactionChange processes CMSG_CHAR_FACTION_CHANGE (0x4D9).
func (s *session) handleCharFactionChange(ctx context.Context, payload []byte) bool {
	return s.handleCharRaceChange(ctx, payload)
}

// handleCompleteMovie processes CMSG_COMPLETE_MOVIE (0x465).
// Reference: WorldSession::HandleCompleteMovie (MiscHandler.cpp:969).
func (s *session) handleCompleteMovie(ctx context.Context, payload []byte) bool {
	s.debug("movie completed", "account", s.accountName)
	return true
}

// handleComplain processes CMSG_COMPLAIN (0x3C7).
// Reference: WorldSession::HandleComplainOpcode (MiscHandler.cpp:1151).
func (s *session) handleComplain(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(1)
	buf.WriteU8(0) // complain result: 0 = complaint received
	return s.write(uint16(protocol.OpcodeSMSG_COMPLAIN_RESULT), buf.Bytes(), true) == nil
}
