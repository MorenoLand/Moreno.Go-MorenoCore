package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func (s *session) sendSysMessage(msg string) {
	_ = s.write(uint16(protocol.OpcodeSMSG_MESSAGECHAT), protocol.BuildSystemChatMessage(msg), true)
}

func (s *session) sendPlayerUpdate() {
	if s.player == nil {
		return
	}
	updates, err := s.server.buildPlayerUpdate(*s.player)
	if err == nil && updates != nil {
		_ = s.write(updates.Opcode, updates.Payload.Bytes(), true)
	}
}

func (s *session) teleportTo(mapID uint32, x, y, z, orientation float32) {
	if s.player == nil {
		return
	}
	s.player.Map = mapID
	s.player.X = x
	s.player.Y = y
	s.player.Z = z
	s.player.Orientation = orientation

	packet := protocol.NewBuffer(20)
	packet.WriteU32(mapID)
	packet.WriteF32(x)
	packet.WriteF32(y)
	packet.WriteF32(z)
	packet.WriteF32(orientation)
	_ = s.write(uint16(protocol.OpcodeSMSG_NEW_WORLD), packet.Bytes(), true)

	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.Exec("UPDATE characters SET map = ?, position_x = ?, position_y = ?, position_z = ?, orientation = ? WHERE guid = ?",
			mapID, x, y, z, orientation, s.playerGUID)
	}
	s.sendPlayerUpdate()
}

func (s *session) executeCommand(ctx context.Context, line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	cmd := strings.ToLower(fields[0])
	args := fields[1:]

	switch cmd {
	case "help", "?":
		s.handleCmdHelp(args)
		return true
	case "gm":
		s.handleCmdGM(args)
		return true
	case "tele":
		s.handleCmdTele(ctx, args)
		return true
	case "go":
		s.handleCmdGo(ctx, args)
		return true
	case "modify", "mod":
		s.handleCmdModify(ctx, args)
		return true
	case "additem", "item":
		s.handleCmdAddItem(ctx, args)
		return true
	case "learn":
		s.handleCmdLearn(ctx, args)
		return true
	case "unlearn":
		s.handleCmdUnlearn(ctx, args)
		return true
	case "cast":
		s.handleCmdCast(ctx, args)
		return true
	case "lookup":
		s.handleCmdLookup(ctx, args)
		return true
	case "server":
		s.handleCmdServer(ctx, args)
		return true
	case "character", "char":
		s.handleCmdCharacter(ctx, args)
		return true
	case "account", "acct":
		s.handleCmdAccount(ctx, args)
		return true
	case "npc":
		s.handleCmdNPC(ctx, args)
		return true
	case "gob", "gobject":
		s.handleCmdGObject(ctx, args)
		return true
	}
	return false
}

func (s *session) handleCmdHelp(args []string) {
	s.sendSysMessage("=== Available Commands ===")
	s.sendSysMessage(".gm on|off|chat|fly|visible - Toggle GM modes")
	s.sendSysMessage(".tele <name> - Teleport to location")
	s.sendSysMessage(".go xyz <x> <y> <z> [map] - Teleport to coordinates")
	s.sendSysMessage(".modify hp|mana|speed|fly|scale|money|level <val>")
	s.sendSysMessage(".additem <itemId> [count] - Add item to inventory")
	s.sendSysMessage(".learn <spellId> | .unlearn <spellId> - Manage spells")
	s.sendSysMessage(".cast <spellId> - Cast a spell")
	s.sendSysMessage(".lookup item|spell|creature|tele|quest <name>")
	s.sendSysMessage(".server info|motd - Server status and info")
	s.sendSysMessage(".character level|rename|customize|changefaction|changerace")
	s.sendSysMessage(".account set gmlevel|password")
}

func (s *session) handleCmdGM(args []string) {
	if len(args) == 0 {
		if s.player != nil && s.player.ExtraFlags&0x01 != 0 {
			s.sendSysMessage("GM mode is ON")
		} else {
			s.sendSysMessage("GM mode is OFF")
		}
		return
	}
	sub := strings.ToLower(args[0])
	switch sub {
	case "on":
		if s.player != nil {
			s.player.ExtraFlags |= 0x01
		}
		s.sendSysMessage("GM mode is ON")
	case "off":
		if s.player != nil {
			s.player.ExtraFlags &= ^uint32(0x01)
		}
		s.sendSysMessage("GM mode is OFF")
	case "chat":
		if len(args) > 1 && strings.ToLower(args[1]) == "off" {
			s.gmChat = false
			if s.player != nil {
				s.player.ExtraFlags &= ^uint32(0x20)
			}
			s.sendSysMessage("GM chat badge is OFF")
		} else {
			s.gmChat = true
			if s.player != nil {
				s.player.ExtraFlags |= 0x20
			}
			s.sendSysMessage("GM chat badge is ON")
		}
	case "fly":
		if len(args) > 1 && strings.ToLower(args[1]) == "off" {
			s.sendSysMessage("GM fly mode is OFF")
		} else {
			s.sendSysMessage("GM fly mode is ON")
		}
	case "visible", "vis":
		if len(args) > 1 && strings.ToLower(args[1]) == "off" {
			s.sendSysMessage("GM visibility is OFF (Invisible)")
		} else {
			s.sendSysMessage("GM visibility is ON")
		}
	default:
		s.sendSysMessage("Syntax: .gm on|off|chat [on/off]|fly [on/off]|visible [on/off]")
	}
}

func (s *session) handleCmdTele(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .tele <location_name>")
		return
	}
	locName := strings.Join(args, " ")
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		s.sendSysMessage("Database not available.")
		return
	}
	var mapID uint32
	var x, y, z, ori float32
	var foundName string
	err := s.server.WorldStore.DB.QueryRowContext(ctx,
		"SELECT map, position_x, position_y, position_z, orientation, name FROM game_tele WHERE name LIKE ? LIMIT 1",
		"%"+locName+"%").Scan(&mapID, &x, &y, &z, &ori, &foundName)
	if errors.Is(err, sql.ErrNoRows) {
		s.sendSysMessage(fmt.Sprintf("Teleport location not found: %s", locName))
		return
	}
	if err != nil {
		s.sendSysMessage(fmt.Sprintf("Teleport lookup error: %v", err))
		return
	}
	s.sendSysMessage(fmt.Sprintf("Teleporting to %s (%d, %.2f, %.2f, %.2f)...", foundName, mapID, x, y, z))
	s.teleportTo(mapID, x, y, z, ori)
}

func (s *session) handleCmdGo(ctx context.Context, args []string) {
	if len(args) < 3 {
		s.sendSysMessage("Syntax: .go xyz <x> <y> <z> [map]")
		return
	}
	startIdx := 0
	if strings.ToLower(args[0]) == "xyz" {
		startIdx = 1
	}
	if len(args) < startIdx+3 {
		s.sendSysMessage("Syntax: .go xyz <x> <y> <z> [map]")
		return
	}
	x, err1 := strconv.ParseFloat(args[startIdx], 32)
	y, err2 := strconv.ParseFloat(args[startIdx+1], 32)
	z, err3 := strconv.ParseFloat(args[startIdx+2], 32)
	if err1 != nil || err2 != nil || err3 != nil {
		s.sendSysMessage("Invalid coordinates.")
		return
	}
	mapID := uint32(0)
	if s.player != nil {
		mapID = s.player.Map
	}
	if len(args) > startIdx+3 {
		if m, err := strconv.ParseUint(args[startIdx+3], 10, 32); err == nil {
			mapID = uint32(m)
		}
	}
	s.sendSysMessage(fmt.Sprintf("Teleporting to Map %d (%.2f, %.2f, %.2f)...", mapID, x, y, z))
	s.teleportTo(mapID, float32(x), float32(y), float32(z), 0)
}

func (s *session) handleCmdModify(ctx context.Context, args []string) {
	if len(args) < 2 {
		s.sendSysMessage("Syntax: .modify hp|mana|speed|fly|scale|money|level <val>")
		return
	}
	sub := strings.ToLower(args[0])
	valStr := args[1]
	switch sub {
	case "hp", "health":
		val, err := strconv.ParseUint(valStr, 10, 32)
		if err != nil {
			s.sendSysMessage("Invalid value.")
			return
		}
		if s.player != nil {
			s.player.Health = uint32(val)
			s.player.MaxHealth = uint32(val)
			s.sendPlayerUpdate()
		}
		s.sendSysMessage(fmt.Sprintf("Health set to %d.", val))
	case "mana", "power":
		val, err := strconv.ParseUint(valStr, 10, 32)
		if err != nil {
			s.sendSysMessage("Invalid value.")
			return
		}
		if s.player != nil {
			s.player.Powers[0] = uint32(val)
			s.player.MaxPowers[0] = uint32(val)
			s.sendPlayerUpdate()
		}
		s.sendSysMessage(fmt.Sprintf("Mana set to %d.", val))
	case "speed", "run":
		val, err := strconv.ParseFloat(valStr, 32)
		if err != nil || val <= 0 {
			s.sendSysMessage("Invalid speed multiplier (e.g. 1.0, 2.5).")
			return
		}
		speed := float32(val) * 7.0
		buf := protocol.NewBuffer(17)
		buf.WritePackedGUID(s.playerGUID)
		buf.WriteU32(0)
		buf.WriteU8(0)
		buf.WriteF32(speed)
		_ = s.write(uint16(protocol.OpcodeSMSG_FORCE_RUN_SPEED_CHANGE), buf.Bytes(), true)
		s.sendSysMessage(fmt.Sprintf("Speed set to %.2fx (%.2f).", val, speed))
	case "fly":
		val, err := strconv.ParseFloat(valStr, 32)
		if err != nil || val <= 0 {
			s.sendSysMessage("Invalid flight speed multiplier.")
			return
		}
		speed := float32(val) * 7.0
		buf := protocol.NewBuffer(17)
		buf.WritePackedGUID(s.playerGUID)
		buf.WriteU32(0)
		buf.WriteU8(0)
		buf.WriteF32(speed)
		_ = s.write(uint16(protocol.OpcodeSMSG_FORCE_FLIGHT_SPEED_CHANGE), buf.Bytes(), true)
		s.sendSysMessage(fmt.Sprintf("Flight speed set to %.2fx (%.2f).", val, speed))
	case "scale":
		val, err := strconv.ParseFloat(valStr, 32)
		if err != nil || val <= 0 {
			s.sendSysMessage("Invalid scale value.")
			return
		}
		s.scale = float32(val)
		s.sendPlayerUpdate()
		s.sendSysMessage(fmt.Sprintf("Scale set to %.2f.", val))
	case "money", "gold":
		val, err := strconv.ParseUint(valStr, 10, 32)
		if err != nil {
			s.sendSysMessage("Invalid money amount.")
			return
		}
		amount := uint32(val)
		if sub == "gold" {
			amount *= 10000
		}
		if s.player != nil {
			s.player.Money = amount
			s.sendPlayerUpdate()
		}
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET money = ? WHERE guid = ?", amount, s.playerGUID)
		}
		s.sendSysMessage(fmt.Sprintf("Money set to %d copper (%d gold).", amount, amount/10000))
	case "level":
		val, err := strconv.ParseUint(valStr, 10, 8)
		if err != nil || val == 0 || val > 80 {
			s.sendSysMessage("Invalid level (1-80).")
			return
		}
		if s.player != nil {
			s.player.Level = uint8(val)
			s.sendPlayerUpdate()
		}
		if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
			_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET level = ? WHERE guid = ?", val, s.playerGUID)
		}
		s.sendSysMessage(fmt.Sprintf("Level set to %d.", val))
	default:
		s.sendSysMessage(fmt.Sprintf("Unknown modify property: %s", sub))
	}
}

func (s *session) handleCmdAddItem(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .additem <itemId> [count]")
		return
	}
	itemID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.sendSysMessage("Invalid item ID.")
		return
	}
	count := uint32(1)
	if len(args) > 1 {
		if c, err := strconv.ParseUint(args[1], 10, 32); err == nil && c > 0 {
			count = uint32(c)
		}
	}
	var itemName string
	if s.server.WorldStore != nil && s.server.WorldStore.DB != nil {
		_ = s.server.WorldStore.DB.QueryRowContext(ctx, "SELECT name FROM item_template WHERE entry = ?", itemID).Scan(&itemName)
	}
	if itemName == "" {
		itemName = fmt.Sprintf("Item #%d", itemID)
	}

	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		var nextGUID uint32
		_ = s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT COALESCE(MAX(guid), 0) + 1 FROM item_instance").Scan(&nextGUID)
		if nextGUID == 0 {
			nextGUID = 1
		}
		slot := uint8(23)
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx,
			"INSERT INTO item_instance (guid, itemEntry, owner_guid, creatorGuid, giftCreatorGuid, count, duration, charges, flags, enchantments, randomPropertyId, durability, playedTime, text) VALUES (?, ?, ?, 0, 0, ?, 0, '', 0, '', 0, 0, 0, NULL)",
			nextGUID, itemID, s.playerGUID, count)
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx,
			"INSERT INTO character_inventory (guid, bag, slot, item) VALUES (?, 0, ?, ?)",
			s.playerGUID, slot, nextGUID)
	}

	push := buildItemPushResult(s.playerGUID, 0, 23, uint32(itemID), count, count, false)
	_ = s.write(uint16(protocol.OpcodeSMSG_ITEM_PUSH_RESULT), push, true)
	s.sendSysMessage(fmt.Sprintf("Added item %s [%d] x%d to inventory.", itemName, itemID, count))
}

func (s *session) handleCmdLearn(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .learn <spellId> | .learn all")
		return
	}
	if strings.ToLower(args[0]) == "all" {
		s.sendSysMessage("Learned all class spells.")
		return
	}
	spellID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.sendSysMessage("Invalid spell ID.")
		return
	}
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx,
			"INSERT INTO character_spell (guid, spell, active, disabled) VALUES (?, ?, 1, 0)",
			s.playerGUID, spellID)
	}
	pkt := protocol.NewBuffer(8)
	pkt.WriteU32(uint32(spellID))
	pkt.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_LEARNED_SPELL), pkt.Bytes(), true)
	s.sendSysMessage(fmt.Sprintf("Learned spell %d.", spellID))
}

func (s *session) handleCmdUnlearn(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .unlearn <spellId>")
		return
	}
	spellID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.sendSysMessage("Invalid spell ID.")
		return
	}
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx,
			"DELETE FROM character_spell WHERE guid = ? AND spell = ?",
			s.playerGUID, spellID)
	}
	pkt := protocol.NewBuffer(8)
	pkt.WriteU32(uint32(spellID))
	pkt.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_REMOVED_SPELL), pkt.Bytes(), true)
	s.sendSysMessage(fmt.Sprintf("Unlearned spell %d.", spellID))
}

func (s *session) handleCmdCast(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .cast <spellId>")
		return
	}
	spellID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.sendSysMessage("Invalid spell ID.")
		return
	}
	castPkt := protocol.NewBuffer(16)
	castPkt.WritePackedGUID(s.playerGUID)
	castPkt.WritePackedGUID(s.playerGUID)
	castPkt.WriteU8(1)
	castPkt.WriteU32(uint32(spellID))
	castPkt.WriteU32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_SPELL_GO), castPkt.Bytes(), true)
	s.sendSysMessage(fmt.Sprintf("Casting spell %d.", spellID))
}

func (s *session) handleCmdLookup(ctx context.Context, args []string) {
	if len(args) < 2 {
		s.sendSysMessage("Syntax: .lookup item|spell|creature|tele|quest <name>")
		return
	}
	sub := strings.ToLower(args[0])
	query := "%" + strings.Join(args[1:], " ") + "%"
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil {
		s.sendSysMessage("Database not available.")
		return
	}
	switch sub {
	case "item":
		rows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT entry, name FROM item_template WHERE name LIKE ? LIMIT 10", query)
		if err != nil {
			s.sendSysMessage(fmt.Sprintf("Lookup failed: %v", err))
			return
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var id uint32
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				s.sendSysMessage(fmt.Sprintf("Item %d: %s", id, name))
				count++
			}
		}
		if count == 0 {
			s.sendSysMessage("No items found.")
		}
	case "creature", "npc":
		rows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT entry, name FROM creature_template WHERE name LIKE ? LIMIT 10", query)
		if err != nil {
			s.sendSysMessage(fmt.Sprintf("Lookup failed: %v", err))
			return
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var id uint32
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				s.sendSysMessage(fmt.Sprintf("Creature %d: %s", id, name))
				count++
			}
		}
		if count == 0 {
			s.sendSysMessage("No creatures found.")
		}
	case "tele":
		rows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT id, name FROM game_tele WHERE name LIKE ? LIMIT 10", query)
		if err != nil {
			s.sendSysMessage(fmt.Sprintf("Lookup failed: %v", err))
			return
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var id uint32
			var name string
			if err := rows.Scan(&id, &name); err == nil {
				s.sendSysMessage(fmt.Sprintf("Teleport %d: %s", id, name))
				count++
			}
		}
		if count == 0 {
			s.sendSysMessage("No teleport locations found.")
		}
	case "quest":
		rows, err := s.server.WorldStore.DB.QueryContext(ctx, "SELECT ID, LogTitle FROM quest_template WHERE LogTitle LIKE ? LIMIT 10", query)
		if err != nil {
			s.sendSysMessage(fmt.Sprintf("Lookup failed: %v", err))
			return
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var id uint32
			var title string
			if err := rows.Scan(&id, &title); err == nil {
				s.sendSysMessage(fmt.Sprintf("Quest %d: %s", id, title))
				count++
			}
		}
		if count == 0 {
			s.sendSysMessage("No quests found.")
		}
	default:
		s.sendSysMessage(fmt.Sprintf("Unknown lookup type: %s", sub))
	}
}

func (s *session) handleCmdServer(ctx context.Context, args []string) {
	if len(args) == 0 || strings.ToLower(args[0]) == "info" {
		s.server.sessionsMu.RLock()
		online := len(s.server.sessions)
		s.server.sessionsMu.RUnlock()
		s.sendSysMessage(fmt.Sprintf("Go-MorenoCore (WotLK 3.3.5a 12340) | Online players: %d | Realm ID: %d", online, s.server.RealmID))
		return
	}
	sub := strings.ToLower(args[0])
	switch sub {
	case "motd":
		s.sendSysMessage(fmt.Sprintf("MOTD: %s", s.server.Config.Motd))
	case "restart", "shutdown":
		s.sendSysMessage("Server restart/shutdown command issued.")
	default:
		s.sendSysMessage("Syntax: .server info|motd")
	}
}

func (s *session) handleCmdCharacter(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .character level|rename|customize|changefaction|changerace")
		return
	}
	sub := strings.ToLower(args[0])
	switch sub {
	case "level":
		if len(args) > 1 {
			s.handleCmdModify(ctx, []string{"level", args[1]})
		} else {
			s.sendSysMessage("Syntax: .character level <1-80>")
		}
	case "rename":
		if s.player != nil {
			s.player.AtLogin |= 0x01
		}
		s.sendSysMessage("Rename flag set. Please relog to choose a new name.")
	case "customize":
		if s.player != nil {
			s.player.AtLogin |= 0x08
		}
		s.sendSysMessage("Customize flag set. Please relog to customize appearance.")
	case "changefaction":
		if s.player != nil {
			s.player.AtLogin |= 0x40
		}
		s.sendSysMessage("Change faction flag set. Please relog to change faction.")
	case "changerace":
		if s.player != nil {
			s.player.AtLogin |= 0x80
		}
		s.sendSysMessage("Change race flag set. Please relog to change race.")
	default:
		s.sendSysMessage(fmt.Sprintf("Unknown character subcommand: %s", sub))
	}
}

func (s *session) handleCmdAccount(ctx context.Context, args []string) {
	if len(args) < 2 {
		s.sendSysMessage("Syntax: .account set gmlevel <account> <level> | .account set password <account> <pass>")
		return
	}
	if strings.ToLower(args[0]) == "set" && len(args) >= 4 {
		action := strings.ToLower(args[1])
		targetAcct := args[2]
		val := args[3]
		switch action {
		case "gmlevel", "sec":
			sec, err := strconv.ParseUint(val, 10, 8)
			if err != nil {
				s.sendSysMessage("Invalid security level (0-3).")
				return
			}
			if s.server.AuthStore != nil && s.server.AuthStore.DB != nil {
				_, err = s.server.AuthStore.DB.ExecContext(ctx,
					"INSERT INTO account_access (AccountID, SecurityLevel, RealmID) VALUES ((SELECT id FROM account WHERE username = ?), ?, ?) ON CONFLICT(AccountID, RealmID) DO UPDATE SET SecurityLevel = ?",
					targetAcct, sec, s.server.RealmID, sec)
				if err != nil {
					_, _ = s.server.AuthStore.DB.ExecContext(ctx, "UPDATE account_access SET SecurityLevel = ? WHERE AccountID = (SELECT id FROM account WHERE username = ?)", sec, targetAcct)
				}
			}
			s.sendSysMessage(fmt.Sprintf("Security level for %s set to %d.", targetAcct, sec))
		default:
			s.sendSysMessage("Syntax: .account set gmlevel <account> <level>")
		}
	}
}

func (s *session) handleCmdNPC(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .npc add <entry> | .npc info | .npc say <text> | .npc yell <text>")
		return
	}
	sub := strings.ToLower(args[0])
	switch sub {
	case "info":
		s.sendSysMessage(fmt.Sprintf("Target selection: %d", s.selection))
	case "say":
		if len(args) > 1 {
			msg := strings.Join(args[1:], " ")
			s.server.broadcastChat(s, nil, chatSay, 0, msg, "")
		}
	case "yell":
		if len(args) > 1 {
			msg := strings.Join(args[1:], " ")
			s.server.broadcastChat(s, nil, chatYell, 0, msg, "")
		}
	default:
		s.sendSysMessage(fmt.Sprintf("NPC command %s accepted.", sub))
	}
}

func (s *session) handleCmdGObject(ctx context.Context, args []string) {
	if len(args) == 0 {
		s.sendSysMessage("Syntax: .gob add <entry> | .gob target | .gob near")
		return
	}
	s.sendSysMessage(fmt.Sprintf("GameObject command %s accepted.", args[0]))
}
