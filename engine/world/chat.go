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
	chatAFK            = 0x17
	chatDND            = 0x18
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
	selection, err := b.ReadPackedGUID()
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
		return false
	}
	language, err := b.ReadU32()
	if err != nil {
		return false
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
		return false
	}
	if len(message) > 255 || strings.ContainsAny(message, "\r\n") || strings.IndexFunc(message, func(r rune) bool { return r < 32 && r != '\t' }) >= 0 {
		s.debug("chat rejected", "account", s.accountName, "reason", "invalid characters")
		return true
	}
	if language == languageUniversal && typeID != chatAFK && typeID != chatDND {
		s.debug("chat rejected", "account", s.accountName, "reason", "universal language")
		return true
	}
	if message == "" && typeID != chatAFK && typeID != chatDND {
		return true
	}
	if strings.HasPrefix(message, ".") || strings.HasPrefix(message, "!") {
		command := strings.TrimSpace(message[1:])
		if command == "" {
			return true
		}
		values, hookErr := s.server.Features.Scripts.TriggerPlayerEvent(ctx, 42, scripting.PlayerEventCommand, s.luaPlayer(), command)
		if hookErr != nil {
			s.debug("lua command hook failed", "account", s.accountName, "error", hookErr)
		}
		return !luaCancelled(values)
	}
	values, hookErr := s.server.Features.Scripts.TriggerPlayerEvent(ctx, scripting.PlayerEventChat, scripting.PlayerEventChat, s.luaPlayer(), message, typeID, language)
	if hookErr != nil {
		s.debug("lua chat hook failed", "account", s.accountName, "error", hookErr)
	}
	if luaCancelled(values) {
		return true
	}
	var receiver *session
	if typeID == chatWhisper {
		receiver = s.server.findSessionByName(targetName)
		if receiver == nil {
			return true
		}
	}
	s.server.broadcastChat(s, receiver, uint8(typeID), language, message, channel)
	s.debug("chat accepted", "account", s.accountName, "type", typeID)
	return true
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

func (s *Server) broadcastChat(source, receiver *session, chatType uint8, language uint32, message, channel string) {
	if source == nil || source.player == nil {
		return
	}
	s.sessionsMu.RLock()
	targets := make([]*session, 0, len(s.sessions))
	for value := range s.sessions {
		if !value.authed || !value.playerLoaded || value.player == nil {
			continue
		}
		if receiver != nil {
			if value != source && value != receiver {
				continue
			}
		} else if value.player.Map != source.player.Map {
			continue
		}
		targets = append(targets, value)
	}
	s.sessionsMu.RUnlock()
	for _, target := range targets {
		receiverGUID := source.playerGUID
		payload := protocol.BuildChatMessage(chatType, language, source.playerGUID, receiverGUID, message, channel)
		if err := target.write(uint16(protocol.OpcodeSMSG_MESSAGECHAT), payload, true); err != nil {
			target.debug("chat delivery failed", "account", target.accountName, "error", err)
		}
	}
}
