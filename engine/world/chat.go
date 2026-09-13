package world

import (
	"context"
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	chatSystem         = 0x00
	chatSay            = 0x01
	chatParty          = 0x02
	chatRaid           = 0x03
	chatGuild          = 0x04
	chatOfficer        = 0x05
	chatYell           = 0x06
	chatWhisper        = 0x07
	chatEmote          = 0x0A
	chatChannel        = 0x11
	chatWhisperInform  = 0x09
	chatAFK            = 0x17
	chatDND            = 0x18
	chatIgnored        = 0x19
	chatBattleground   = 0x2C
	chatBattleLeader   = 0x2D
	chatPartyLeader    = 0x33
	maxChatMessageType = 0x34
	languageUniversal  = uint32(0)
	languageAddon      = ^uint32(0)
)

func (s *session) handleSetSelection(payload []byte) bool {
	if !s.playerLoaded {
		return true
	}
	b := protocol.NewReader(payload)
	selection, err := b.ReadU64()
	if err != nil {
		s.debug("selection rejected", "account", s.accountName, "error", err)
		return false
	}
	s.selection = selection
	return true
}

func (s *session) handleMessageChat(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	b := protocol.NewReader(payload)
	typeID, err := b.ReadU32()
	if err != nil {
		s.debug("chat rejected", "account", s.accountName, "reason", "malformed type", "error", err)
		return true
	}
	language, err := b.ReadU32()
	if err != nil {
		s.debug("chat rejected", "account", s.accountName, "reason", "malformed language", "error", err)
		return true
	}
	if typeID >= maxChatMessageType {
		s.debug("chat rejected", "account", s.accountName, "reason", "invalid message type", "type", typeID)
		return true
	}
	var targetName, channel, message string
	switch uint8(typeID) {
	case chatWhisper:
		targetName, err = b.ReadCString()
		if err == nil {
			message, err = b.ReadCString()
		}
	case chatChannel:
		channel, err = b.ReadCString()
		if err == nil {
			message, err = b.ReadCString()
		}
	default:
		message, err = b.ReadCString()
	}
	if err != nil {
		s.debug("chat rejected", "account", s.accountName, "reason", "malformed message", "error", err)
		return true
	}
	if len(message) > 255 || strings.ContainsAny(message, "\r\n") || strings.IndexFunc(message, func(r rune) bool { return r < 32 && r != '\t' }) >= 0 {
		s.debug("chat rejected", "account", s.accountName, "reason", "invalid characters")
		return true
	}
	if s.warden != nil && s.warden.processLuaCheckResponse(message) {
		return true
	}
	if strings.HasPrefix(message, ".") || strings.HasPrefix(message, "!") {
		command := strings.TrimSpace(message[1:])
		if command == "" {
			return true
		}
		if s.executeCommand(ctx, command) {
			return true
		}
		if s.server.Features != nil && s.server.Features.Scripts != nil {
			values, hookErr := s.server.Features.Scripts.TriggerPlayerEvent(ctx, 42, scripting.PlayerEventCommand, s.luaPlayer(), command)
			if hookErr != nil {
				s.debug("lua command hook failed", "account", s.accountName, "error", hookErr)
			}
			return !luaCancelled(values)
		}
		return true
	}
	if message == "" && typeID != chatAFK && typeID != chatDND {
		return true
	}
	isGM := s.player != nil && ((s.player.ExtraFlags&playerExtraGMOn != 0) || (s.player.PlayerFlags&playerFlagGM != 0))
	if language == languageUniversal && typeID != chatAFK && typeID != chatDND {
		if !isGM || (!s.gmChat && s.player.ExtraFlags&playerExtraGMChat == 0) {
			if s.playerAlliance() {
				language = 7 // Common
			} else {
				language = 1 // Orcish
			}
		}
	}
	if language != languageAddon && isGM && (s.gmChat || s.player.ExtraFlags&playerExtraGMChat != 0) {
		language = languageUniversal
	}
	if typeID == chatWhisper && language != languageAddon {
		language = languageUniversal
	}
	if (typeID == chatGuild || typeID == chatOfficer) && !s.guildChatSpeakAllowed(typeID == chatOfficer) {
		return true
	}
	if s.server.Features != nil && s.server.Features.Scripts != nil {
		values, hookErr := s.server.Features.Scripts.TriggerPlayerEvent(ctx, scripting.PlayerEventChat, scripting.PlayerEventChat, s.luaPlayer(), message, typeID, language)
		if hookErr != nil {
			s.debug("lua chat hook failed", "account", s.accountName, "error", hookErr)
		}
		if luaCancelled(values) {
			return true
		}
	}
	var receiver *session
	if typeID == chatWhisper {
		receiver = s.server.findSessionByName(targetName)
		if receiver == nil {
			return true
		}
	}
	if typeID == chatChannel && !s.server.isChannelMember(s, channel) {
		return s.sendChannelNotify(channelNotMemberNotice, channel, nil) == nil
	}
	if typeID == chatChannel && s.server.isChannelMuted(s, channel) {
		// Reference Channel::Say: muted members receive CHAT_MUTED_NOTICE and
		// the message is not delivered.
		return s.sendChannelNotify(channelMutedNotice, channel, nil) == nil
	}
	s.server.broadcastChat(s, receiver, uint8(typeID), language, message, channel)
	s.debug("chat accepted", "account", s.accountName, "type", typeID, "gm_chat", s.gmChat)
	return true
}

func (s *session) guildChatSpeakAllowed(officer bool) bool {
	if s == nil || s.player == nil || s.player.GuildID == 0 || s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return s != nil && s.player != nil && s.player.GuildID != 0
	}
	var rights int64
	if err := s.server.CharactersStore.DB.QueryRowContext(context.Background(), `SELECT COALESCE(gr.rights, 0)
		FROM guild_member gm LEFT JOIN guild_rank gr ON gr.guildid = gm.guildid AND gr.rid = gm.rank
		WHERE gm.guildid = ? AND gm.guid = ? LIMIT 1`, s.player.GuildID, s.playerGUID).Scan(&rights); err != nil {
		return false
	}
	required := int64(0x42)
	if officer {
		required = 0x48
	}
	return rights&required == required
}

func (s *Server) guildChatListenAllowed(target *session, officer bool) bool {
	if target == nil || target.player == nil || target.player.GuildID == 0 || s == nil || s.CharactersStore == nil || s.CharactersStore.DB == nil {
		return target != nil && target.player != nil && target.player.GuildID != 0
	}
	var rights int64
	if err := s.CharactersStore.DB.QueryRowContext(context.Background(), `SELECT COALESCE(gr.rights, 0)
		FROM guild_member gm LEFT JOIN guild_rank gr ON gr.guildid = gm.guildid AND gr.rid = gm.rank
		WHERE gm.guildid = ? AND gm.guid = ? LIMIT 1`, target.player.GuildID, target.playerGUID).Scan(&rights); err != nil {
		return false
	}
	required := int64(0x41)
	if officer {
		required = 0x44
	}
	return rights&required == required
}

func (s *Server) chatIgnoredBy(targetGUID, sourceGUID uint64) bool {
	if s == nil || s.CharactersStore == nil || s.CharactersStore.DB == nil {
		return false
	}
	var flags int64
	if err := s.CharactersStore.DB.QueryRowContext(context.Background(), "SELECT flags FROM character_social WHERE guid = ? AND friend = ? LIMIT 1", targetGUID, sourceGUID).Scan(&flags); err != nil {
		return false
	}
	return uint64(flags)&uint64(socialFlagIgnored) != 0
}

func luaCancelled(values []any) bool {
	for _, value := range values {
		if cancelled, ok := value.(bool); ok && !cancelled {
			return true
		}
	}
	return false
}

func (s *Server) findSessionByName(name string) *session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for value := range s.sessions {
		if value.playerLoaded && value.player != nil && strings.EqualFold(value.player.Name, name) {
			return value
		}
	}
	return nil
}

func (s *Server) findSessionByGUID(guid uint64) *session {
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for value := range s.sessions {
		if value.playerLoaded && value.player != nil && value.player.GUID == guid {
			return value
		}
	}
	return nil
}

func (s *Server) broadcastChat(source, receiver *session, chatType uint8, language uint32, message, channel string) {
	if source == nil || source.player == nil {
		return
	}
	s.sessionsMu.RLock()
	targets := make([]*session, 0, len(s.sessions))
	channelTargets := s.channelMembers(channel)
	for value := range s.sessions {
		if !value.authed || !value.playerLoaded || value.player == nil {
			continue
		}
		if receiver != nil {
			if value != source && value != receiver {
				continue
			}
		} else if chatType == chatChannel {
			if _, ok := channelTargets[value]; !ok {
				continue
			}
		} else if chatType == chatParty || chatType == chatPartyLeader {
			if source.groupID == 0 || value.groupID != source.groupID {
				continue
			}
		} else if chatType == chatGuild || chatType == chatOfficer {
			if source.player.GuildID == 0 || value.player.GuildID != source.player.GuildID {
				continue
			}
			if !s.guildChatListenAllowed(value, chatType == chatOfficer) {
				continue
			}
			if s.chatIgnoredBy(value.playerGUID, source.playerGUID) {
				continue
			}
		} else if value.player.Map != source.player.Map {
			continue
		}
		targets = append(targets, value)
	}
	s.sessionsMu.RUnlock()
	for _, target := range targets {
		receiverGUID := uint64(0)
		switch chatType {
		case chatSay, chatYell, chatEmote, chatChannel:
			receiverGUID = source.playerGUID
		}
		outType, senderGUID := chatType, source.playerGUID
		if receiver != nil {
			if target == receiver {
				receiverGUID = source.playerGUID
			} else {
				outType, senderGUID, receiverGUID = chatWhisperInform, receiver.playerGUID, receiver.playerGUID
			}
		}
		tag := source.chatTag()
		isGM := tag&0x04 != 0
		opcode := uint16(protocol.OpcodeSMSG_MESSAGECHAT)
		senderName := ""
		if isGM && source.player != nil {
			opcode = uint16(protocol.OpcodeSMSG_GM_MESSAGECHAT)
			senderName = source.player.Name
		}
		payload := protocol.BuildChatMessageWithOptions(outType, language, senderGUID, receiverGUID, message, channel, isGM, senderName, tag)
		if err := target.write(opcode, payload, true); err != nil {
			target.debug("chat delivery failed", "account", target.accountName, "error", err)
		}
	}
}

func (s *session) chatTag() uint8 {
	if s.player == nil {
		return 0
	}
	isGM := (s.player.ExtraFlags&playerExtraGMOn != 0) || (s.player.PlayerFlags&playerFlagGM != 0) || (s.player.ExtraFlags&playerExtraGMChat != 0) || s.gmChat || s.security > 0
	var tag uint8
	if isGM {
		tag |= 0x04
	}
	if s.player.PlayerFlags&playerFlagDND != 0 {
		tag |= 0x02
	}
	if s.player.PlayerFlags&playerFlagAFK != 0 {
		tag |= 0x01
	}
	return tag
}

// handleChatIgnored processes CMSG_CHAT_IGNORED (0x225).
// Reference: WorldSession::HandleChatIgnoredOpcode (ChatHandler.cpp:745).
func (s *session) handleChatIgnored(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	b := protocol.NewReader(payload)
	targetGUID, err := b.ReadU64()
	if err != nil {
		return false
	}
	_, err = b.ReadU8() // unk (spam reporting flag in reference)
	if err != nil {
		return false
	}
	targetSess := s.server.findSessionByGUID(targetGUID)
	if targetSess == nil || !targetSess.playerLoaded || targetSess.player == nil {
		return true
	}
	msg := protocol.BuildChatMessageWithOptions(chatIgnored, languageUniversal, s.playerGUID, s.playerGUID, s.player.Name, "", false, "", s.chatTag())
	_ = targetSess.write(uint16(protocol.OpcodeSMSG_MESSAGECHAT), msg, true)
	return true
}
