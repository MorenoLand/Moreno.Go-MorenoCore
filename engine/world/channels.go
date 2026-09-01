package world

import (
	"strings"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	channelJoinedNotice        uint8 = 0x00
	channelLeftNotice          uint8 = 0x01
	channelYouJoinedNotice     uint8 = 0x02
	channelYouLeftNotice       uint8 = 0x03
	channelWrongPasswordNotice uint8 = 0x04
	channelNotMemberNotice     uint8 = 0x05
	channelAlreadyMemberNotice uint8 = 0x17
	channelInvalidNameNotice   uint8 = 0x1B
	channelFlagCustom          uint8 = 0x01
	channelFlagTrade           uint8 = 0x04
	channelFlagNotLFG          uint8 = 0x08
	channelFlagGeneral         uint8 = 0x10
	channelFlagCity            uint8 = 0x20
	channelFlagLFG             uint8 = 0x40
)

type worldChannel struct {
	ID      uint32
	Name    string
	Flags   uint8
	Members map[*session]struct{}
}

func (s *session) handleJoinChannel(payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	reader := protocol.NewReader(payload)
	channelID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	if _, err = reader.ReadU8(); err != nil {
		return false
	}
	if _, err = reader.ReadU8(); err != nil {
		return false
	}
	name, err := reader.ReadCString()
	if err != nil {
		return false
	}
	password, err := reader.ReadCString()
	if err != nil {
		return false
	}
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return s.sendChannelNotify(channelInvalidNameNotice, name, nil) == nil
	}
	if len(name) > 31 || len(password) > 31 {
		return true
	}
	key := channelKey(name)
	flags := channelFlags(channelID, name)
	s.server.channelsMu.Lock()
	if s.server.channels == nil {
		s.server.channels = make(map[string]*worldChannel)
	}
	channel := s.server.channels[key]
	if channel == nil {
		channel = &worldChannel{ID: channelID, Name: name, Flags: flags, Members: make(map[*session]struct{})}
		s.server.channels[key] = channel
	}
	if _, exists := channel.Members[s]; exists {
		s.server.channelsMu.Unlock()
		return s.sendChannelNotify(channelAlreadyMemberNotice, channel.Name, nil) == nil
	}
	channel.Members[s] = struct{}{}
	if s.channels == nil {
		s.channels = make(map[string]struct{})
	}
	s.channels[key] = struct{}{}
	others := make([]*session, 0, len(channel.Members)-1)
	for member := range channel.Members {
		if member != s {
			others = append(others, member)
		}
	}
	channelName, channelFlagsValue, channelIDValue := channel.Name, channel.Flags, channel.ID
	s.server.channelsMu.Unlock()
	for _, member := range others {
		_ = member.sendChannelNotify(channelJoinedNotice, channelName, &channelNotifyGUID{GUID: s.playerGUID})
	}
	if err := s.sendChannelNotify(channelYouJoinedNotice, channelName, &channelNotifyChannel{Flags: channelFlagsValue, ID: channelIDValue}); err != nil {
		return false
	}
	s.debug("channel joined", "account", s.accountName, "channel", channelName, "id", channelIDValue)
	return true
}

func (s *session) handleLeaveChannel(payload []byte) bool {
	if !s.playerLoaded {
		return true
	}
	reader := protocol.NewReader(payload)
	channelID, err := reader.ReadU32()
	if err != nil {
		return false
	}
	name, err := reader.ReadCString()
	if err != nil {
		return false
	}
	if channelID == 0 && name == "" {
		return true
	}
	key := channelKey(name)
	s.server.channelsMu.Lock()
	channel := s.server.channels[key]
	if channel == nil && channelID != 0 {
		for candidateKey, candidate := range s.server.channels {
			if candidate.ID == channelID {
				key, channel = candidateKey, candidate
				break
			}
		}
	}
	if channel == nil {
		s.server.channelsMu.Unlock()
		return s.sendChannelNotify(channelNotMemberNotice, name, nil) == nil
	}
	if _, exists := channel.Members[s]; !exists {
		s.server.channelsMu.Unlock()
		return s.sendChannelNotify(channelNotMemberNotice, channel.Name, nil) == nil
	}
	delete(channel.Members, s)
	if s.channels != nil {
		delete(s.channels, key)
	}
	others := make([]*session, 0, len(channel.Members))
	for member := range channel.Members {
		others = append(others, member)
	}
	channelName, channelFlagsValue, channelIDValue := channel.Name, channel.Flags, channel.ID
	if len(channel.Members) == 0 {
		delete(s.server.channels, key)
	}
	s.server.channelsMu.Unlock()
	for _, member := range others {
		_ = member.sendChannelNotify(channelLeftNotice, channelName, &channelNotifyGUID{GUID: s.playerGUID})
	}
	if err := s.sendChannelNotify(channelYouLeftNotice, channelName, &channelNotifyChannel{Flags: channelFlagsValue, ID: channelIDValue}); err != nil {
		return false
	}
	s.debug("channel left", "account", s.accountName, "channel", channelName, "id", channelIDValue)
	return true
}

func (s *session) sendChannelNotify(notice uint8, name string, extra any) error {
	return s.write(uint16(protocol.OpcodeSMSG_CHANNEL_NOTIFY), buildChannelNotify(notice, name, extra), true)
}

type channelNotifyGUID struct{ GUID uint64 }

type channelNotifyChannel struct {
	Flags uint8
	ID    uint32
}

func buildChannelNotify(notice uint8, name string, extra any) []byte {
	packet := protocol.NewBuffer(48)
	packet.WriteU8(notice)
	packet.WriteCString(name)
	switch value := extra.(type) {
	case *channelNotifyGUID:
		packet.WriteU64(value.GUID)
	case *channelNotifyChannel:
		if notice == channelYouLeftNotice {
			packet.WriteU32(value.ID)
			if value.ID != 0 {
				packet.WriteU8(1)
			} else {
				packet.WriteU8(0)
			}
		} else {
			packet.WriteU8(value.Flags)
			packet.WriteU32(value.ID)
			packet.WriteU32(0)
		}
	}
	return packet.Bytes()
}

func (s *Server) channelMembers(name string) map[*session]struct{} {
	key := channelKey(name)
	s.channelsMu.RLock()
	channel := s.channels[key]
	result := make(map[*session]struct{})
	if channel != nil {
		for member := range channel.Members {
			result[member] = struct{}{}
		}
	}
	s.channelsMu.RUnlock()
	return result
}

func (s *Server) isChannelMember(member *session, name string) bool {
	key := channelKey(name)
	s.channelsMu.RLock()
	channel := s.channels[key]
	ok := false
	if channel != nil {
		_, ok = channel.Members[member]
	}
	s.channelsMu.RUnlock()
	return ok
}

func channelKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

func channelFlags(id uint32, name string) uint8 {
	if id == 0 {
		return channelFlagCustom
	}
	switch strings.ToLower(strings.ReplaceAll(name, " ", "")) {
	case "trade":
		return channelFlagGeneral | channelFlagNotLFG | channelFlagTrade | channelFlagCity
	case "lookingforgroup":
		return channelFlagGeneral | channelFlagLFG
	case "guildrecruitment":
		return channelFlagGeneral | channelFlagNotLFG | channelFlagCity
	default:
		return channelFlagGeneral | channelFlagNotLFG
	}
}

func (s *Server) removeSessionChannels(member *session) {
	s.channelsMu.Lock()
	if s.channels == nil {
		s.channelsMu.Unlock()
		return
	}
	for key, channel := range s.channels {
		if _, ok := channel.Members[member]; !ok {
			continue
		}
		delete(channel.Members, member)
		if len(channel.Members) == 0 {
			delete(s.channels, key)
		}
	}
	s.channelsMu.Unlock()
	member.channels = nil
}
