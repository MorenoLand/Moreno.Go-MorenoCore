package world

import (
	"bytes"
	"compress/zlib"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func (s *session) handleNameQuery(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		s.debug("name query rejected", "account", s.accountName, "error", err)
		return false
	}
	packet := protocol.NewBuffer(32)
	packet.WritePackedGUID(guid)
	var name string
	var race, gender, class int64
	if s.player != nil && s.player.GUID == guid {
		name = s.player.Name
		race = int64(s.player.Race)
		gender = int64(s.player.Gender)
		class = int64(s.player.Class)
	} else if online := s.server.findSessionByGUID(guid); online != nil && online.player != nil {
		name = online.player.Name
		race = int64(online.player.Race)
		gender = int64(online.player.Gender)
		class = int64(online.player.Class)
	} else {
		err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT name, race, gender, class FROM characters WHERE guid = ? AND (deleteInfos_Name IS NULL OR deleteInfos_Name = '')", guid).Scan(&name, &race, &gender, &class)
		if errors.Is(err, sql.ErrNoRows) {
			packet.WriteU8(1)
			return s.write(uint16(protocol.OpcodeSMSG_NAME_QUERY_RESPONSE), packet.Bytes(), true) == nil
		} else if err != nil {
			s.debug("name query failed", "account", s.accountName, "guid", guid, "error", err)
			return false
		}
	}
	packet.WriteU8(0)
	packet.WriteCString(name)
	packet.WriteU8(0)
	packet.WriteU8(uint8(race))
	packet.WriteU8(uint8(gender))
	packet.WriteU8(uint8(class))
	packet.WriteU8(0)
	return s.write(uint16(protocol.OpcodeSMSG_NAME_QUERY_RESPONSE), packet.Bytes(), true) == nil
}

func (s *session) handleQueryTime() bool {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	packet := protocol.NewBuffer(8)
	packet.WriteU32(uint32(now.Unix()))
	packet.WriteU32(uint32(next.Sub(now).Seconds()))
	return s.write(uint16(protocol.OpcodeSMSG_QUERY_TIME_RESPONSE), packet.Bytes(), true) == nil
}

func (s *session) handlePlayedTime(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	trigger, err := reader.ReadU8()
	if err != nil {
		return false
	}
	var total, level int64
	err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT totaltime, leveltime FROM characters WHERE guid = ? AND account = ?", s.playerGUID, s.accountID).Scan(&total, &level)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.debug("played time query failed", "account", s.accountName, "error", err)
		return false
	}
	packet := protocol.NewBuffer(9)
	packet.WriteU32(uint32(total))
	packet.WriteU32(uint32(level))
	packet.WriteU8(trigger)
	return s.write(uint16(protocol.OpcodeSMSG_PLAYED_TIME), packet.Bytes(), true) == nil
}

func (s *session) handleZoneUpdate(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	zone, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if !s.playerLoaded || s.player == nil {
		return true
	}
	s.player.Zone = zone
	if _, err := s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_ZONE", zone, s.playerGUID); err != nil {
		s.debug("zone update failed", "account", s.accountName, "zone", zone, "error", err)
		return false
	}
	return true
}

func (s *session) handleSetActionBarToggles(payload []byte) bool {
	reader := protocol.NewReader(payload)
	toggles, err := reader.ReadU8()
	if err != nil {
		return false
	}
	if s.player != nil {
		s.player.ActionBars = uint32(toggles)
	}
	return true
}

func (s *session) handleSetActionButton(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	button, err := reader.ReadU8()
	if err != nil {
		return false
	}
	data, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if button >= 144 || !s.playerLoaded {
		return true
	}
	var spec int64
	if err := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT activeTalentGroup FROM characters WHERE guid = ? AND account = ?", s.playerGUID, s.accountID).Scan(&spec); err != nil {
		s.debug("action button spec query failed", "account", s.accountName, "error", err)
		return false
	}
	if data == 0 {
		_, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_DEL_CHAR_ACTION_BY_BUTTON_SPEC", s.playerGUID, button, spec)
	} else {
		var action, kind int64
		action = int64(data & 0x00FFFFFF)
		kind = int64(data >> 24)
		result, updateErr := s.server.CharactersStore.ExecStatement(ctx, "CHAR_UPD_CHAR_ACTION", action, kind, s.playerGUID, button, spec)
		err = updateErr
		if err == nil {
			var affected int64
			affected, err = result.RowsAffected()
			if err == nil && affected == 0 {
				_, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_INS_CHAR_ACTION", s.playerGUID, spec, button, action, kind)
			}
		}
	}
	if err != nil {
		s.debug("action button update failed", "account", s.accountName, "button", button, "error", err)
		return false
	}
	return true
}

func (s *session) handleUpdateAccountData(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	typeID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	timestamp, err := reader.ReadU32()
	if err != nil {
		return false
	}
	decompressedSize, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if typeID >= 8 || decompressedSize > 0xFFFF {
		return true
	}
	compressed, err := reader.Read(reader.Remaining())
	if err != nil {
		return false
	}
	data, err := decompressAccountData(compressed, decompressedSize)
	if err != nil {
		s.debug("account data decompression failed", "account", s.accountName, "type", typeID, "error", err)
		return true
	}
	var result sql.Result
	if globalAccountDataMask&(1<<typeID) != 0 {
		result, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_REP_ACCOUNT_DATA", s.accountID, typeID, timestamp, data)
	} else if s.playerLoaded {
		result, err = s.server.CharactersStore.ExecStatement(ctx, "CHAR_REP_PLAYER_ACCOUNT_DATA", s.playerGUID, typeID, timestamp, data)
	}
	_ = result
	if err != nil {
		s.debug("account data update failed", "account", s.accountName, "type", typeID, "error", err)
		return false
	}
	response := protocol.NewBuffer(8)
	response.WriteU32(typeID)
	response.WriteU32(0)
	return s.write(uint16(protocol.OpcodeSMSG_UPDATE_ACCOUNT_DATA_COMPLETE), response.Bytes(), true) == nil
}

func (s *session) handleRequestAccountData(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	typeID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if typeID >= 8 {
		return true
	}
	var timestamp int64
	var data []byte
	if globalAccountDataMask&(1<<typeID) != 0 {
		err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT time, data FROM account_data WHERE accountId = ? AND type = ?", s.accountID, typeID).Scan(&timestamp, &data)
	} else if s.playerLoaded {
		err = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT time, data FROM character_account_data WHERE guid = ? AND type = ?", s.playerGUID, typeID).Scan(&timestamp, &data)
	} else {
		err = sql.ErrNoRows
	}
	if errors.Is(err, sql.ErrNoRows) {
		timestamp = 0
		data = nil
	} else if err != nil {
		s.debug("account data request failed", "account", s.accountName, "type", typeID, "error", err)
		return false
	}
	compressed, err := compressAccountData(data)
	if err != nil {
		return false
	}
	packet := protocol.NewBuffer(24 + len(compressed))
	if s.playerLoaded {
		packet.WriteU64(s.playerGUID)
	} else {
		packet.WriteU64(0)
	}
	packet.WriteU32(typeID)
	packet.WriteU32(uint32(timestamp))
	packet.WriteU32(uint32(len(data)))
	packet.Write(compressed)
	return s.write(uint16(protocol.OpcodeSMSG_UPDATE_ACCOUNT_DATA), packet.Bytes(), true) == nil
}

func (s *session) handleWorldStateUITimer() bool {
	packet := protocol.NewBuffer(4)
	packet.WriteU32(uint32(time.Now().Unix()))
	return s.write(uint16(protocol.OpcodeSMSG_WORLD_STATE_UI_TIMER_UPDATE), packet.Bytes(), true) == nil
}

func (s *session) handleRequestRaidInfo() bool {
	packet := protocol.NewBuffer(4)
	packet.WriteU32(0)
	return s.write(uint16(protocol.OpcodeSMSG_RAID_INSTANCE_INFO), packet.Bytes(), true) == nil
}

func decompressAccountData(compressed []byte, expected uint32) ([]byte, error) {
	if expected == 0 {
		return nil, nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, int64(expected)+1))
	if err != nil {
		return nil, err
	}
	if uint32(len(data)) != expected {
		return nil, fmt.Errorf("decompressed size %d does not match %d", len(data), expected)
	}
	return data, nil
}

func compressAccountData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}
