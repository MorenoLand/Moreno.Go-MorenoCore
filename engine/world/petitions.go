package world

import (
	"context"
	"database/sql"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	guildCharterItemID = 5863
	guildCharterCost   = 1000 // 10s
)

// handlePetitionBuy processes CMSG_PETITION_BUY (0x1B6).
// Reference: WorldSession::HandlePetitionBuyOpcode (PetitionsHandler.cpp:48).
func (s *session) handlePetitionBuy(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 16 {
		return true
	}
	r := protocol.NewReader(payload)
	_, _ = r.ReadU64() // npc GUID
	_, _ = r.ReadU32() // 0
	_, _ = r.ReadU64() // 0
	name, err := r.ReadCString()
	if err != nil || name == "" {
		return true
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	// Check if player already in guild
	var existingGuild int64
	_ = cdb.QueryRowContext(ctx, "SELECT guildid FROM guild_member WHERE guid = ? LIMIT 1", s.playerGUID).Scan(&existingGuild)
	if existingGuild != 0 {
		return true
	}

	petitionGUID := s.playerGUID + 100000
	_, _ = cdb.ExecContext(ctx, "REPLACE INTO petition (ownerguid, petitionguid, name, type) VALUES (?, ?, ?, 9)",
		s.playerGUID, petitionGUID, name)
	return true
}

// handlePetitionShowSignatures processes CMSG_PETITION_SHOW_SIGNATURES (0x1BE).
// Reference: WorldSession::HandlePetitionShowSignatures (PetitionsHandler.cpp:220).
func (s *session) handlePetitionShowSignatures(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	_ = cdb.QueryRowContext(ctx, "SELECT ownerguid FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID)

	rows, err := cdb.QueryContext(ctx, "SELECT playerguid FROM petition_sign WHERE petitionguid = ?", petitionGUID)
	var signs []uint64
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pguid int64
			if err := rows.Scan(&pguid); err == nil {
				signs = append(signs, uint64(pguid))
			}
		}
	}

	buf := protocol.NewBuffer(32 + len(signs)*12)
	buf.WriteU64(petitionGUID)
	buf.WriteU64(uint64(ownerGUID))
	buf.WriteU32(uint32(petitionGUID & 0xFFFFFFFF))
	buf.WriteU8(uint8(len(signs)))
	for _, sg := range signs {
		buf.WriteU64(sg)
		buf.WriteU32(0)
	}
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_SHOW_SIGNATURES), buf.Bytes(), true)
	return true
}

// handlePetitionQuery processes CMSG_PETITION_QUERY (0x1C6).
// Reference: WorldSession::HandleQueryPetition (PetitionsHandler.cpp:261).
func (s *session) handlePetitionQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 12 {
		return true
	}
	r := protocol.NewReader(payload)
	guildGUID, _ := r.ReadU32()
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	var name string
	var pType int64
	err = cdb.QueryRowContext(ctx, "SELECT ownerguid, name, type FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID, &name, &pType)
	if err != nil {
		return true
	}

	buf := protocol.NewBuffer(128 + len(name))
	buf.WriteU32(guildGUID)
	buf.WriteU64(uint64(ownerGUID))
	buf.WriteCString(name)
	buf.WriteU8(0)
	buf.WriteU32(9) // 9 signs needed for guild
	buf.WriteU32(9)
	buf.WriteU32(0)
	for i := 0; i < 10; i++ {
		buf.WriteCString("")
	}
	buf.WriteU32(uint32(pType))
	buf.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_QUERY_RESPONSE), buf.Bytes(), true)
	return true
}

// handlePetitionSign processes CMSG_PETITION_SIGN (0x1C0).
// Reference: WorldSession::HandlePetitionSignOpcode (PetitionsHandler.cpp:328).
func (s *session) handlePetitionSign(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	var pType int64
	err = cdb.QueryRowContext(ctx, "SELECT ownerguid, type FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID, &pType)
	if err != nil {
		return true
	}

	_, _ = cdb.ExecContext(ctx, "INSERT OR IGNORE INTO petition_sign (ownerguid, petitionguid, playerguid, player_account, type) VALUES (?, ?, ?, ?, ?)",
		ownerGUID, petitionGUID, s.playerGUID, s.accountID, pType)

	buf := protocol.NewBuffer(20)
	buf.WriteU64(petitionGUID)
	buf.WriteU64(s.playerGUID)
	buf.WriteU32(0) // success
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_SIGN_RESULTS), buf.Bytes(), true)
	return true
}

// handleTurnInPetition processes CMSG_TURN_IN_PETITION (0x1C4).
// Reference: WorldSession::HandleTurnInPetitionOpcode (PetitionsHandler.cpp:557).
func (s *session) handleTurnInPetition(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var ownerGUID int64
	var guildName string
	err = cdb.QueryRowContext(ctx, "SELECT ownerguid, name FROM petition WHERE petitionguid = ? LIMIT 1", petitionGUID).Scan(&ownerGUID, &guildName)
	if err != nil || ownerGUID == 0 || guildName == "" {
		return true
	}

	var maxGuildID sql.NullInt64
	_ = cdb.QueryRowContext(ctx, "SELECT MAX(guildid) FROM guild").Scan(&maxGuildID)
	newGuildID := int64(1)
	if maxGuildID.Valid {
		newGuildID = maxGuildID.Int64 + 1
	}

	// Create guild
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild (guildid, name, leaderguid, createdate) VALUES (?, ?, ?, ?)",
		newGuildID, guildName, ownerGUID, timeNow())

	// Default ranks
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 0, 'Guild Master', 4294967295, 1000000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 1, 'Officer', 255, 500000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 2, 'Veteran', 64, 100000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 3, 'Member', 64, 50000)", newGuildID)
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_rank (guildid, rid, rname, rights, BankMoneyPerDay) VALUES (?, 4, 'Initiate', 64, 0)", newGuildID)

	// Add leader
	_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_member (guildid, guid, rank, pnote, offnote) VALUES (?, ?, 0, '', '')", newGuildID, ownerGUID)

	// Add signers
	var signers []int64
	rows, _ := cdb.QueryContext(ctx, "SELECT playerguid FROM petition_sign WHERE petitionguid = ?", petitionGUID)
	if rows != nil {
		for rows.Next() {
			var signerGUID int64
			if err := rows.Scan(&signerGUID); err == nil && signerGUID != ownerGUID {
				signers = append(signers, signerGUID)
			}
		}
		rows.Close()
	}
	for _, signerGUID := range signers {
		_, _ = cdb.ExecContext(ctx, "INSERT INTO guild_member (guildid, guid, rank, pnote, offnote) VALUES (?, ?, 4, '', '')", newGuildID, signerGUID)
	}

	// Clean up petition
	_, _ = cdb.ExecContext(ctx, "DELETE FROM petition WHERE petitionguid = ?", petitionGUID)
	_, _ = cdb.ExecContext(ctx, "DELETE FROM petition_sign WHERE petitionguid = ?", petitionGUID)

	buf := protocol.NewBuffer(4)
	buf.WriteU32(0) // success
	_ = s.write(uint16(protocol.OpcodeSMSG_TURN_IN_PETITION_RESULTS), buf.Bytes(), true)
	return true
}

// handleOfferPetition processes CMSG_OFFER_PETITION (0x1B3).
// Reference: WorldSession::HandlePetitionOfferOpcode (PetitionsHandler.cpp:446).
func (s *session) handleOfferPetition(ctx context.Context, payload []byte) bool {
	return true
}

// handlePetitionShowList processes CMSG_PETITION_SHOWLIST (0x1BB).
// Reference: WorldSession::HandlePetitionShowListOpcode (PetitionsHandler.cpp:408).
func (s *session) handlePetitionShowList(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	npcGUID, _ := r.ReadU64()

	buf := protocol.NewBuffer(24)
	buf.WriteU64(npcGUID)
	buf.WriteU8(1)                       // count = 1 petition item
	buf.WriteU32(1)                       // index = 1
	buf.WriteU32(guildCharterItemID)      // 5863 Guild Charter
	buf.WriteU32(CHARTER_DISPLAY_ID)      // 16161
	buf.WriteU32(guildCharterCost)        // 1000 copper
	buf.WriteU32(0)                       // 0
	buf.WriteU32(9)                       // 9 required signs
	_ = s.write(uint16(protocol.OpcodeSMSG_PETITION_SHOWLIST), buf.Bytes(), true)
	return true
}

// handlePetitionDecline processes MSG_PETITION_DECLINE (0x1C2).
// Reference: WorldSession::HandlePetitionDeclineOpcode (PetitionsHandler.cpp:520).
func (s *session) handlePetitionDecline(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(8)
	buf.WriteU64(s.playerGUID)
	_ = s.write(uint16(protocol.OpcodeMSG_PETITION_DECLINE), buf.Bytes(), true)
	return true
}

// handlePetitionRename processes MSG_PETITION_RENAME (0x1C1).
// Reference: WorldSession::HandlePetitionRenameOpcode (PetitionsHandler.cpp:738).
func (s *session) handlePetitionRename(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 9 {
		return true
	}
	r := protocol.NewReader(payload)
	petitionGUID, err := r.ReadU64()
	if err != nil {
		return false
	}
	newName, err := r.ReadCString()
	if err != nil || newName == "" {
		return false
	}

	cdb := s.server.CharactersStore.DB
	if cdb != nil {
		_, _ = cdb.ExecContext(ctx, "UPDATE petition SET name = ? WHERE petitionguid = ? AND ownerguid = ?", newName, petitionGUID, s.playerGUID)
	}

	buf := protocol.NewBuffer(16 + len(newName))
	buf.WriteU64(petitionGUID)
	buf.WriteCString(newName)
	_ = s.write(uint16(protocol.OpcodeMSG_PETITION_RENAME), buf.Bytes(), true)
	return true
}

// handleTabardVendorActivate processes MSG_TABARDVENDOR_ACTIVATE (0x1FA).
// Reference: WorldSession::HandleTabardVendorActivateOpcode (NPCHandler.cpp:665).
func (s *session) handleTabardVendorActivate(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	vendorGUID, err := r.ReadU64()
	if err != nil {
		return false
	}

	buf := protocol.NewBuffer(8)
	buf.WriteU64(vendorGUID)
	_ = s.write(uint16(protocol.OpcodeMSG_TABARDVENDOR_ACTIVATE), buf.Bytes(), true)
	return true
}

// handleSaveGuildEmblem processes MSG_SAVE_GUILD_EMBLEM (0x1FB).
// Reference: WorldSession::HandleSaveGuildEmblemOpcode (GuildHandler.cpp:205).
func (s *session) handleSaveGuildEmblem(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 28 {
		return true
	}
	r := protocol.NewReader(payload)
	_, _ = r.ReadU64() // vendor
	style, _ := r.ReadU32()
	color, _ := r.ReadU32()
	bStyle, _ := r.ReadU32()
	bColor, _ := r.ReadU32()
	bgColor, _ := r.ReadU32()

	cdb := s.server.CharactersStore.DB
	if cdb == nil {
		return true
	}

	var guildID, leaderGUID int64
	err := cdb.QueryRowContext(ctx, `SELECT g.guildid, g.leaderguid FROM guild g
		JOIN guild_member gm ON gm.guildid = g.guildid
		WHERE gm.guid = ? LIMIT 1`, s.playerGUID).Scan(&guildID, &leaderGUID)
	if err != nil || guildID == 0 || uint64(leaderGUID) != s.playerGUID {
		return true
	}

	const tabardCost = 100000 // 10 gold
	if s.player.Money < tabardCost {
		return true
	}

	s.player.Money -= tabardCost
	s.sendPlayerMoneyUpdate()
	_, _ = cdb.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", s.player.Money, s.playerGUID)
	_, _ = cdb.ExecContext(ctx, "UPDATE guild SET EmblemStyle = ?, EmblemColor = ?, BorderStyle = ?, BorderColor = ?, BackgroundColor = ? WHERE guildid = ?",
		style, color, bStyle, bColor, bgColor, guildID)

	res := protocol.NewBuffer(4)
	res.WriteU32(0) // ERR_GUILDEMBLEM_SUCCESS
	_ = s.write(uint16(protocol.OpcodeMSG_SAVE_GUILD_EMBLEM), res.Bytes(), true)

	// GE_TABARDCHANGE = 9
	eventBuf := protocol.NewBuffer(8)
	eventBuf.WriteU8(9)
	eventBuf.WriteU8(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_GUILD_EVENT), eventBuf.Bytes(), true)
	return true
}

const CHARTER_DISPLAY_ID = 16161
