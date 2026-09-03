package world

import (
	"context"
	"encoding/binary"
	"math/rand"
	"sync/atomic"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// groupState holds all state for a 5-man or raid group.
// Mirrors TrinityCore's Group class (Groups/Group.h).
type groupState struct {
	ID            uint64
	LeaderGUID    uint64
	Members       []groupMember // ordered; first entry is leader
	LootMethod    uint8         // 0=Free, 1=RR, 2=MasterLoot, 3=GroupLoot, 4=NeedBeforeGreed
	MasterLooter  uint64
	LootThreshold uint8 // item quality threshold (default 2 = uncommon)
	DungeonDiff   uint8
	RaidDiff      uint8
	IsRaid        bool
	TargetIcons   [8]uint64 // raid target icons, index=icon, value=target GUID
	counter       uint32
}

// groupMember mirrors Group::MemberSlot.
type groupMember struct {
	GUID     uint64
	Name     string
	SubGroup uint8 // 0-7 for raids, always 0 for 5-man
	Flags    uint8 // member flags (assistant etc)
	Roles    uint8 // LFG roles (unused at group level)
}

// groupNextID is a monotonic group ID counter.
var groupNextID uint64 = 1

func newGroupID() uint64 {
	return atomic.AddUint64(&groupNextID, 1)
}

// PartyOperation enum, mirrors TrinityCore's PartyOperation.
const (
	partyOpInvite   uint32 = 0
	partyOpUninvite uint32 = 1
	partyOpLeave    uint32 = 2
)

// PartyResult enum, mirrors TrinityCore's PartyResult.
const (
	errPartyResultOK        uint32 = 0
	errBadPlayerNameS       uint32 = 1
	errTargetNotInGroup     uint32 = 2
	errGroupFull            uint32 = 3
	errAlreadyInGroupS      uint32 = 4
	errNotLeader            uint32 = 5
	errPlayerWrongFaction   uint32 = 7
	errIgnoringYouS         uint32 = 8
	errTargetNotInInstanceS uint32 = 11
	errInviteRestricted     uint32 = 13
)

// MaxGroupSize (5-man). Raids can have up to 40.
const maxGroupSize = 5
const maxRaidSize = 40

// -----------------------------------------------------------------
// Wire helpers
// -----------------------------------------------------------------

// buildPartyCommandResult mirrors WorldSession::SendPartyResult.
// SMSG_PARTY_COMMAND_RESULT: uint32 operation, cstring member, uint32 result, uint32 val
func buildPartyCommandResult(operation uint32, member string, result uint32) []byte {
	b := protocol.NewBuffer(4 + len(member) + 1 + 4 + 4)
	b.WriteU32(operation)
	b.WriteCString(member)
	b.WriteU32(result)
	b.WriteU32(0) // LFD cooldown val
	return b.Bytes()
}

// buildGroupList sends SMSG_GROUP_LIST to a specific member, excluding themselves.
// Mirrors Group::SendUpdate (Group.cpp:1755).
// groupType: 0=party, 1=BG, 2=raid
func buildGroupList(g *groupState, forGUID uint64) []byte {
	// Find the member slot for the recipient.
	var slot *groupMember
	for i := range g.Members {
		if g.Members[i].GUID == forGUID {
			slot = &g.Members[i]
			break
		}
	}
	membersCount := len(g.Members) - 1 // exclude self
	if membersCount < 0 {
		membersCount = 0
	}

	groupType := uint8(0)
	if g.IsRaid {
		groupType = 1
	}

	subGroup := uint8(0)
	flags := uint8(0)
	roles := uint8(0)
	if slot != nil {
		subGroup = slot.SubGroup
		flags = slot.Flags
		roles = slot.Roles
	}

	b := protocol.NewBuffer(64 + membersCount*24)
	b.WriteU8(groupType)
	b.WriteU8(subGroup)
	b.WriteU8(flags)
	b.WriteU8(roles)
	// no LFG fields (not an LFG group)
	b.WriteU64(g.ID)
	b.WriteU32(g.counter)
	b.WriteU32(uint32(membersCount))
	for _, m := range g.Members {
		if m.GUID == forGUID {
			continue
		}
		b.WriteCString(m.Name)
		b.WriteU64(m.GUID)
		b.WriteU8(1) // online status (MEMBER_STATUS_ONLINE=1)
		b.WriteU8(m.SubGroup)
		b.WriteU8(m.Flags)
		b.WriteU8(m.Roles)
	}
	b.WriteU64(g.LeaderGUID)
	if membersCount > 0 {
		b.WriteU8(g.LootMethod)
		b.WriteU64(g.MasterLooter)
		b.WriteU8(g.LootThreshold)
		b.WriteU8(g.DungeonDiff)
		b.WriteU8(g.RaidDiff)
		b.WriteU8(0) // dynamic raid difficulty flag
	}
	return b.Bytes()
}

// buildGroupInvite builds SMSG_GROUP_INVITE sent to the invited player.
// flag: 1 = valid invite, 0 = already in group notification.
// Mirrors GroupHandler.cpp:149 and GroupHandler.cpp:207.
func buildGroupInvite(flag uint8, inviterName string) []byte {
	b := protocol.NewBuffer(2 + len(inviterName) + 8)
	b.WriteU8(flag)
	b.WriteCString(inviterName)
	b.WriteU32(0) // unk
	b.WriteU8(0)  // count
	b.WriteU32(0) // unk
	return b.Bytes()
}

// -----------------------------------------------------------------
// Server helpers
// -----------------------------------------------------------------

func (s *Server) findGroupByID(id uint64) *groupState {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	return s.groups[id]
}

func (s *Server) broadcastGroupList(g *groupState) {
	g.counter++
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for sess := range s.sessions {
		if sess.groupID == g.ID {
			pkt := buildGroupList(g, sess.playerGUID)
			_ = sess.write(uint16(protocol.OpcodeSMSG_GROUP_LIST), pkt, true)
		}
	}
}

func (s *Server) broadcastToGroup(groupID uint64, opcode uint16, payload []byte) {
	if groupID == 0 {
		return
	}
	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()
	for sess := range s.sessions {
		if sess.groupID == groupID {
			_ = sess.write(opcode, payload, true)
		}
	}
}

// -----------------------------------------------------------------
// Handlers
// -----------------------------------------------------------------

// handleGroupInvite processes CMSG_GROUP_INVITE (0x06E).
// TrinityCore: WorldSession::HandleGroupInviteOpcode.
func (s *session) handleGroupInvite(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return false
	}
	r := protocol.NewReader(payload)
	memberName, err := r.ReadCString()
	if err != nil || memberName == "" {
		return s.sendPartyResult(partyOpInvite, memberName, errBadPlayerNameS)
	}
	// skip uint32 unk
	_, _ = r.ReadU32()

	// Can't invite yourself
	if toLower(memberName) == toLower(s.player.Name) {
		return s.sendPartyResult(partyOpInvite, memberName, errBadPlayerNameS)
	}

	invitedSess := s.server.findSessionByName(memberName)
	if invitedSess == nil || invitedSess.player == nil {
		return s.sendPartyResult(partyOpInvite, memberName, errBadPlayerNameS)
	}

	// Invited player already in a group or has a pending invite
	if invitedSess.groupID != 0 || invitedSess.pendingGroupLeader != 0 {
		_ = s.sendPartyResult(partyOpInvite, memberName, errAlreadyInGroupS)
		if invitedSess.groupID != 0 {
			_ = invitedSess.write(uint16(protocol.OpcodeSMSG_GROUP_INVITE), buildGroupInvite(0, s.player.Name), true)
		}
		return true
	}

	// Inviting player must be leader if already in a group
	if s.groupID != 0 {
		g := s.server.findGroupByID(s.groupID)
		if g == nil {
			s.groupID = 0
		} else if g.LeaderGUID != s.playerGUID {
			return s.sendPartyResult(partyOpInvite, "", errNotLeader)
		} else if len(g.Members) >= maxGroupSize {
			return s.sendPartyResult(partyOpInvite, "", errGroupFull)
		}
	}

	// Set the pending invite on the target player
	invitedSess.pendingGroupLeader = s.playerGUID

	// Send SMSG_GROUP_INVITE to invited player
	_ = invitedSess.write(uint16(protocol.OpcodeSMSG_GROUP_INVITE), buildGroupInvite(1, s.player.Name), true)
	// Tell inviter that invite was sent OK
	return s.sendPartyResult(partyOpInvite, memberName, errPartyResultOK)
}

// handleGroupAccept processes CMSG_GROUP_ACCEPT (0x072).
// TrinityCore: WorldSession::HandleGroupAcceptOpcode.
func (s *session) handleGroupAccept(_ context.Context, _ []byte) bool {
	if !s.playerLoaded || s.player == nil || s.pendingGroupLeader == 0 {
		return false
	}
	leaderGUID := s.pendingGroupLeader
	s.pendingGroupLeader = 0

	// Can't accept your own invite
	if leaderGUID == s.playerGUID {
		return false
	}

	leaderSess := s.server.findSessionByGUID(leaderGUID)
	if leaderSess == nil || leaderSess.player == nil {
		return false
	}

	srv := s.server
	srv.groupsMu.Lock()

	var g *groupState
	if leaderSess.groupID != 0 {
		g = srv.groups[leaderSess.groupID]
	}

	if g == nil {
		// Create new group
		g = &groupState{
			ID:            newGroupID(),
			LeaderGUID:    leaderGUID,
			LootThreshold: 2, // uncommon
			DungeonDiff:   1,
			RaidDiff:      1,
		}
		g.Members = append(g.Members, groupMember{GUID: leaderGUID, Name: leaderSess.player.Name})
		srv.groups[g.ID] = g
		leaderSess.groupID = g.ID
	}

	if len(g.Members) >= maxGroupSize {
		srv.groupsMu.Unlock()
		_ = s.sendPartyResult(partyOpInvite, "", errGroupFull)
		return false
	}

	g.Members = append(g.Members, groupMember{GUID: s.playerGUID, Name: s.player.Name})
	s.groupID = g.ID
	srv.groupsMu.Unlock()

	srv.broadcastGroupList(g)
	return true
}

// handleGroupDecline processes CMSG_GROUP_DECLINE (0x073).
// TrinityCore: WorldSession::HandleGroupDeclineOpcode.
func (s *session) handleGroupDecline(_ context.Context, _ []byte) bool {
	if !s.playerLoaded || s.pendingGroupLeader == 0 {
		return false
	}
	leaderGUID := s.pendingGroupLeader
	s.pendingGroupLeader = 0

	leaderSess := s.server.findSessionByGUID(leaderGUID)
	if leaderSess == nil || leaderSess.player == nil {
		return false
	}
	// SMSG_GROUP_DECLINE: player name cstring
	name := ""
	if s.player != nil {
		name = s.player.Name
	}
	b := protocol.NewBuffer(len(name) + 1)
	b.WriteCString(name)
	_ = leaderSess.write(uint16(protocol.OpcodeSMSG_GROUP_DECLINE), b.Bytes(), true)
	return true
}

// handleGroupUninvite processes CMSG_GROUP_UNINVITE (0x075) - by name.
// TrinityCore: WorldSession::HandleGroupUninviteOpcode.
func (s *session) handleGroupUninvite(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.groupID == 0 {
		return false
	}
	r := protocol.NewReader(payload)
	name, err := r.ReadCString()
	if err != nil || toLower(name) == toLower(s.player.Name) {
		return false
	}

	g := s.server.findGroupByID(s.groupID)
	if g == nil || g.LeaderGUID != s.playerGUID {
		return s.sendPartyResult(partyOpUninvite, "", errNotLeader)
	}

	target := s.server.findSessionByName(name)
	if target == nil {
		return s.sendPartyResult(partyOpUninvite, name, errTargetNotInGroup)
	}
	return s.removeFromGroup(g, target)
}

// handleGroupUninviteGUID processes CMSG_GROUP_UNINVITE_GUID (0x076).
// TrinityCore: WorldSession::HandleGroupUninviteGuidOpcode.
func (s *session) handleGroupUninviteGUID(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || s.groupID == 0 {
		return false
	}
	r := protocol.NewReader(payload)
	guid, err := r.ReadU64()
	if err != nil || guid == s.playerGUID {
		return false
	}

	g := s.server.findGroupByID(s.groupID)
	if g == nil || g.LeaderGUID != s.playerGUID {
		return s.sendPartyResult(partyOpUninvite, "", errNotLeader)
	}

	target := s.server.findSessionByGUID(guid)
	if target == nil {
		return s.sendPartyResult(partyOpUninvite, "", errTargetNotInGroup)
	}
	return s.removeFromGroup(g, target)
}

// removeFromGroup kicks target from the group, dissolving if group goes to 1 member.
func (s *session) removeFromGroup(g *groupState, target *session) bool {
	srv := s.server
	srv.groupsMu.Lock()
	// Remove the target member
	for i, m := range g.Members {
		if m.GUID == target.playerGUID {
			g.Members = append(g.Members[:i], g.Members[i+1:]...)
			break
		}
	}
	target.groupID = 0
	target.pendingGroupLeader = 0

	// Send SMSG_GROUP_UNINVITE to the kicked player
	_ = target.write(uint16(protocol.OpcodeSMSG_GROUP_UNINVITE), nil, true)

	if len(g.Members) <= 1 {
		// Disband the group
		delete(srv.groups, g.ID)
		if len(g.Members) == 1 {
			if last := srv.findSessionByGUID(g.Members[0].GUID); last != nil {
				last.groupID = 0
				// SMSG_GROUP_DESTROYED
				_ = last.write(uint16(protocol.OpcodeSMSG_GROUP_DESTROYED), nil, true)
				// Also send empty group list to clear UI
				emptyList := buildGroupList(&groupState{ID: g.ID, LeaderGUID: g.Members[0].GUID}, g.Members[0].GUID)
				_ = last.write(uint16(protocol.OpcodeSMSG_GROUP_LIST), emptyList, true)
			}
		}
		srv.groupsMu.Unlock()
		return true
	}
	srv.groupsMu.Unlock()
	srv.broadcastGroupList(g)
	return true
}

// handleGroupSetLeader processes CMSG_GROUP_SET_LEADER (0x078).
// TrinityCore: WorldSession::HandleGroupSetLeaderOpcode.
func (s *session) handleGroupSetLeader(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.groupID == 0 {
		return false
	}
	r := protocol.NewReader(payload)
	guid, err := r.ReadU64()
	if err != nil {
		return false
	}

	srv := s.server
	srv.groupsMu.Lock()
	g := srv.groups[s.groupID]
	if g == nil || g.LeaderGUID != s.playerGUID {
		srv.groupsMu.Unlock()
		return false
	}
	// Check target is a member
	found := false
	for _, m := range g.Members {
		if m.GUID == guid {
			found = true
			break
		}
	}
	if !found {
		srv.groupsMu.Unlock()
		return false
	}
	g.LeaderGUID = guid

	// Move new leader to front of members list
	for i, m := range g.Members {
		if m.GUID == guid {
			g.Members[0], g.Members[i] = g.Members[i], g.Members[0]
			break
		}
	}
	srv.groupsMu.Unlock()

	// SMSG_GROUP_SET_LEADER: cstring name
	newLeader := s.server.findSessionByGUID(guid)
	name := ""
	if newLeader != nil && newLeader.player != nil {
		name = newLeader.player.Name
	}
	b := protocol.NewBuffer(len(name) + 1)
	b.WriteCString(name)
	pkt := b.Bytes()
	srv.sessionsMu.RLock()
	for sess := range srv.sessions {
		if sess.groupID == g.ID {
			_ = sess.write(uint16(protocol.OpcodeSMSG_GROUP_SET_LEADER), pkt, true)
		}
	}
	srv.sessionsMu.RUnlock()
	srv.broadcastGroupList(g)
	return true
}

// handleGroupDisband processes CMSG_GROUP_DISBAND (0x07B).
// TrinityCore: WorldSession::HandleGroupDisbandOpcode.
func (s *session) handleGroupDisband(_ context.Context, _ []byte) bool {
	if !s.playerLoaded {
		return false
	}
	if s.pendingGroupLeader != 0 {
		// Cancel a pending invite we initiated (not a real TC case but safe)
		s.pendingGroupLeader = 0
		return true
	}
	if s.groupID == 0 {
		return false
	}

	srv := s.server
	srv.groupsMu.Lock()
	g := srv.groups[s.groupID]
	if g == nil {
		s.groupID = 0
		srv.groupsMu.Unlock()
		return false
	}

	name := ""
	if s.player != nil {
		name = s.player.Name
	}

	if g.LeaderGUID == s.playerGUID {
		// Leader disbands entire group
		members := make([]uint64, len(g.Members))
		for i, m := range g.Members {
			members[i] = m.GUID
		}
		delete(srv.groups, g.ID)
		srv.groupsMu.Unlock()
		for _, guid := range members {
			sess := srv.findSessionByGUID(guid)
			if sess == nil {
				continue
			}
			sess.groupID = 0
			_ = sess.write(uint16(protocol.OpcodeSMSG_GROUP_DESTROYED), nil, true)
			empty := buildGroupList(&groupState{ID: g.ID, LeaderGUID: guid}, guid)
			_ = sess.write(uint16(protocol.OpcodeSMSG_GROUP_LIST), empty, true)
		}
	} else {
		// Non-leader leaves
		for i, m := range g.Members {
			if m.GUID == s.playerGUID {
				g.Members = append(g.Members[:i], g.Members[i+1:]...)
				break
			}
		}
		s.groupID = 0
		if len(g.Members) <= 1 {
			var lastGUID uint64
			if len(g.Members) == 1 {
				lastGUID = g.Members[0].GUID
			}
			delete(srv.groups, g.ID)
			srv.groupsMu.Unlock()
			if lastGUID != 0 {
				if last := srv.findSessionByGUID(lastGUID); last != nil {
					last.groupID = 0
					_ = last.write(uint16(protocol.OpcodeSMSG_GROUP_DESTROYED), nil, true)
					emptyG := buildGroupList(&groupState{ID: g.ID, LeaderGUID: lastGUID}, lastGUID)
					_ = last.write(uint16(protocol.OpcodeSMSG_GROUP_LIST), emptyG, true)
				}
			}
		} else {
			srv.groupsMu.Unlock()
			srv.broadcastGroupList(g)
		}
	}

	_ = s.sendPartyResult(partyOpLeave, name, errPartyResultOK)
	return true
}

// handleLootMethod processes CMSG_LOOT_METHOD (0x09A).
// TrinityCore: WorldSession::HandleLootMethodOpcode.
func (s *session) handleLootMethod(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.groupID == 0 {
		return false
	}
	r := protocol.NewReader(payload)
	lootMethod, err := r.ReadU32()
	if err != nil {
		return false
	}
	masterLooter, err := r.ReadU64()
	if err != nil {
		return false
	}
	lootThreshold, err := r.ReadU32()
	if err != nil {
		return false
	}

	if lootMethod > 4 {
		return false
	}
	if lootThreshold < 2 || lootThreshold > 6 {
		return false
	}

	srv := s.server
	srv.groupsMu.Lock()
	g := srv.groups[s.groupID]
	if g == nil || g.LeaderGUID != s.playerGUID {
		srv.groupsMu.Unlock()
		return false
	}
	g.LootMethod = uint8(lootMethod)
	g.MasterLooter = masterLooter
	g.LootThreshold = uint8(lootThreshold)
	srv.groupsMu.Unlock()
	srv.broadcastGroupList(g)
	return true
}

// handleMinimapPing processes MSG_MINIMAP_PING (0x1D5).
// TrinityCore: WorldSession::HandleMinimapPingOpcode.
// Sends a map ping to all group members.
func (s *session) handleMinimapPing(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.groupID == 0 {
		return false
	}
	r := protocol.NewReader(payload)
	x, err := r.ReadF32()
	if err != nil {
		return false
	}
	y, err := r.ReadF32()
	if err != nil {
		return false
	}

	b := protocol.NewBuffer(16)
	b.WriteU64(s.playerGUID)
	binary.LittleEndian.AppendUint32(b.Bytes(), 0) // placeholder
	_ = b.Bytes()
	buf := protocol.NewBuffer(16)
	buf.WriteU64(s.playerGUID)
	buf.WriteF32(x)
	buf.WriteF32(y)
	pkt := buf.Bytes()

	srv := s.server
	srv.sessionsMu.RLock()
	for sess := range srv.sessions {
		if sess.groupID == s.groupID && sess != s {
			_ = sess.write(uint16(protocol.OpcodeMSG_MINIMAP_PING), pkt, true)
		}
	}
	srv.sessionsMu.RUnlock()
	return true
}

// handleRaidTargetUpdate processes MSG_RAID_TARGET_UPDATE (0x321).
// TrinityCore: WorldSession::HandleRaidTargetUpdateOpcode.
func (s *session) handleRaidTargetUpdate(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.groupID == 0 {
		return false
	}
	r := protocol.NewReader(payload)
	x, err := r.ReadU8()
	if err != nil {
		return false
	}

	srv := s.server
	srv.groupsMu.Lock()
	g := srv.groups[s.groupID]
	if g == nil {
		srv.groupsMu.Unlock()
		return false
	}

	if x == 0xFF {
		// Query — send current icon list
		srv.groupsMu.Unlock()
		b := protocol.NewBuffer(2 + 8*8)
		b.WriteU8(1) // full update
		for _, iconGUID := range g.TargetIcons {
			b.WriteU64(iconGUID)
		}
		_ = s.write(uint16(protocol.OpcodeMSG_RAID_TARGET_UPDATE), b.Bytes(), true)
		return true
	}

	// Must be leader or assistant in raid
	if g.IsRaid && g.LeaderGUID != s.playerGUID {
		srv.groupsMu.Unlock()
		return false
	}

	guid, err := r.ReadU64()
	if err != nil {
		srv.groupsMu.Unlock()
		return false
	}
	if x >= 8 {
		srv.groupsMu.Unlock()
		return false
	}
	g.TargetIcons[x] = guid
	srv.groupsMu.Unlock()

	b := protocol.NewBuffer(11)
	b.WriteU8(0) // partial update
	b.WriteU8(x)
	b.WriteU64(guid)
	pkt := b.Bytes()
	srv.sessionsMu.RLock()
	for sess := range srv.sessions {
		if sess.groupID == s.groupID {
			_ = sess.write(uint16(protocol.OpcodeMSG_RAID_TARGET_UPDATE), pkt, true)
		}
	}
	srv.sessionsMu.RUnlock()
	return true
}

// handleGroupRaidConvert processes CMSG_GROUP_RAID_CONVERT (0x28E).
// TrinityCore: WorldSession::HandleGroupRaidConvertOpcode.
func (s *session) handleGroupRaidConvert(_ context.Context, _ []byte) bool {
	if !s.playerLoaded || s.groupID == 0 {
		return false
	}
	srv := s.server
	srv.groupsMu.Lock()
	g := srv.groups[s.groupID]
	if g == nil || g.LeaderGUID != s.playerGUID || len(g.Members) < 2 {
		srv.groupsMu.Unlock()
		return false
	}
	g.IsRaid = !g.IsRaid
	srv.groupsMu.Unlock()
	_ = s.sendPartyResult(partyOpInvite, "", errPartyResultOK)
	srv.broadcastGroupList(g)
	return true
}

// handlePartyAssignment processes MSG_PARTY_ASSIGNMENT (0x38E).
// Sets main assist / main tank flags.
// TrinityCore: WorldSession::HandlePartyAssignmentOpcode.
func (s *session) handlePartyAssignment(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.groupID == 0 {
		return false
	}
	r := protocol.NewReader(payload)
	assignment, err := r.ReadU8()
	if err != nil {
		return false
	}
	applyByte, err := r.ReadU8()
	if err != nil {
		return false
	}
	guid, err := r.ReadU64()
	if err != nil {
		return false
	}
	apply := applyByte != 0

	srv := s.server
	srv.groupsMu.Lock()
	g := srv.groups[s.groupID]
	if g == nil || g.LeaderGUID != s.playerGUID {
		srv.groupsMu.Unlock()
		return false
	}
	const (
		assignMainAssist = 0
		assignMainTank   = 1
		flagMainAssist   = 0x04
		flagMainTank     = 0x02
	)
	clearFlag := uint8(0)
	setFlag := uint8(0)
	switch assignment {
	case assignMainAssist:
		clearFlag = flagMainAssist
		setFlag = flagMainAssist
	case assignMainTank:
		clearFlag = flagMainTank
		setFlag = flagMainTank
	default:
		srv.groupsMu.Unlock()
		return false
	}
	// Clear flag from all members first
	for i := range g.Members {
		g.Members[i].Flags &^= clearFlag
	}
	if apply {
		for i := range g.Members {
			if g.Members[i].GUID == guid {
				g.Members[i].Flags |= setFlag
				break
			}
		}
	}
	srv.groupsMu.Unlock()
	srv.broadcastGroupList(g)
	return true
}

// handleReadyCheck processes MSG_RAID_READY_CHECK (0x322).
// TrinityCore: WorldSession::HandleRaidReadyCheckOpcode.
func (s *session) handleReadyCheck(_ context.Context, payload []byte) bool {
	if !s.playerLoaded || s.groupID == 0 {
		return false
	}
	srv := s.server
	srv.groupsMu.RLock()
	g := srv.groups[s.groupID]
	srv.groupsMu.RUnlock()
	if g == nil {
		return false
	}

	r := protocol.NewReader(payload)
	if len(payload) == 0 {
		// Request — must be leader/assistant
		if g.LeaderGUID != s.playerGUID {
			return false
		}
		b := protocol.NewBuffer(8)
		b.WriteU64(s.playerGUID)
		pkt := b.Bytes()
		srv.sessionsMu.RLock()
		for sess := range srv.sessions {
			if sess.groupID == s.groupID {
				_ = sess.write(uint16(protocol.OpcodeMSG_RAID_READY_CHECK), pkt, true)
			}
		}
		srv.sessionsMu.RUnlock()
	} else {
		// Answer from a group member
		state, err := r.ReadU8()
		if err != nil {
			return false
		}
		b := protocol.NewBuffer(9)
		b.WriteU64(s.playerGUID)
		b.WriteU8(state)
		pkt := b.Bytes()
		// Broadcast the reply to leader only
		if leaderSess := srv.findSessionByGUID(g.LeaderGUID); leaderSess != nil {
			_ = leaderSess.write(uint16(protocol.OpcodeMSG_RAID_READY_CHECK_CONFIRM), pkt, true)
		}
	}
	return true
}

// sendPartyResult is a helper to write SMSG_PARTY_COMMAND_RESULT.
func (s *session) sendPartyResult(op uint32, member string, result uint32) bool {
	return s.write(uint16(protocol.OpcodeSMSG_PARTY_COMMAND_RESULT), buildPartyCommandResult(op, member, result), true) == nil
}

// toLower is a simple ASCII lowercase helper.
func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// randomRollMax is the maximum roll value (matching TrinityCore).
const randomRollMax = 10000

// handleRandomRoll processes MSG_RANDOM_ROLL.
// TrinityCore: WorldSession::HandleRandomRollOpcode.
func (s *session) handleRandomRoll(_ context.Context, payload []byte) bool {
	if !s.playerLoaded {
		return false
	}
	r := protocol.NewReader(payload)
	minimum, err := r.ReadU32()
	if err != nil {
		return false
	}
	maximum, err := r.ReadU32()
	if err != nil {
		return false
	}
	if minimum > maximum || maximum > randomRollMax {
		return false
	}
	rolled := minimum + uint32(rand.Intn(int(maximum-minimum)+1))

	// SMSG_RANDOMIZE_CHAR_NAME uses MSG_RANDOM_ROLL opcode in 3.3.5a.
	b := protocol.NewBuffer(20)
	b.WriteU32(minimum)
	b.WriteU32(maximum)
	b.WriteU32(rolled)
	b.WriteU64(s.playerGUID)
	pkt := b.Bytes()

	_ = s.write(uint16(protocol.OpcodeMSG_RANDOM_ROLL), pkt, true)
	// Broadcast to group members if in a group
	if s.groupID != 0 {
		srv := s.server
		srv.sessionsMu.RLock()
		for sess := range srv.sessions {
			if sess.groupID == s.groupID && sess != s {
				_ = sess.write(uint16(protocol.OpcodeMSG_RANDOM_ROLL), pkt, true)
			}
		}
		srv.sessionsMu.RUnlock()
	}
	return true
}

// handleGroupAssistantLeader processes CMSG_GROUP_ASSISTANT_LEADER (0x28F).
func (s *session) handleGroupAssistantLeader(ctx context.Context, payload []byte) bool {
	return true
}

// handleGroupChangeSubGroup processes CMSG_GROUP_CHANGE_SUB_GROUP (0x27E).
func (s *session) handleGroupChangeSubGroup(ctx context.Context, payload []byte) bool {
	return true
}

// handleResetInstances processes CMSG_RESET_INSTANCES (0x31D).
// Reference: WorldSession::HandleResetInstancesOpcode (MiscHandler.cpp:1255).
func (s *session) handleResetInstances(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	if s.groupID != 0 && s.server != nil {
		grp := s.server.findGroupByID(s.groupID)
		if grp != nil && grp.LeaderGUID != s.playerGUID {
			buf := protocol.NewBuffer(8)
			buf.WriteU32(2) // RESET_FAILED_NOT_LEADER
			buf.WriteU32(0) // mapid
			_ = s.write(uint16(protocol.OpcodeSMSG_RESET_FAILED_NOTIFY), buf.Bytes(), true)
			return true
		}
	}
	buf := protocol.NewBuffer(4)
	buf.WriteU32(0) // success for map 0 / all
	_ = s.write(uint16(protocol.OpcodeSMSG_INSTANCE_RESET), buf.Bytes(), true)
	return true
}

// handleSetDungeonDifficulty processes MSG_SET_DUNGEON_DIFFICULTY (0x329).
// Reference: WorldSession::HandleSetDungeonDifficultyOpcode (MiscHandler.cpp:1268).
func (s *session) handleSetDungeonDifficulty(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	mode, err := r.ReadU32()
	if err != nil {
		return false
	}

	buf := protocol.NewBuffer(4)
	buf.WriteU32(mode)
	_ = s.write(uint16(protocol.OpcodeMSG_SET_DUNGEON_DIFFICULTY), buf.Bytes(), true)
	return true
}

// handleSetRaidDifficulty processes MSG_SET_RAID_DIFFICULTY (0x4EB).
// Reference: WorldSession::HandleSetRaidDifficultyOpcode (MiscHandler.cpp:1323).
func (s *session) handleSetRaidDifficulty(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	mode, err := r.ReadU32()
	if err != nil {
		return false
	}

	buf := protocol.NewBuffer(4)
	buf.WriteU32(mode)
	_ = s.write(uint16(protocol.OpcodeMSG_SET_RAID_DIFFICULTY), buf.Bytes(), true)
	return true
}

// handleInstanceLockResponse processes CMSG_INSTANCE_LOCK_RESPONSE (0x13F).
// Reference: WorldSession::HandleInstanceLockResponse (MiscHandler.cpp:1449).
func (s *session) handleInstanceLockResponse(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 1 {
		return true
	}
	accept := payload[0]
	if accept == 0 && s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		var homeMap, homeZone uint32
		var homeX, homeY, homeZ float32
		if err := cdb.QueryRowContext(ctx, "SELECT mapId, zoneId, posX, posY, posZ FROM character_homebind WHERE guid = ?", s.playerGUID).Scan(&homeMap, &homeZone, &homeX, &homeY, &homeZ); err == nil {
			s.player.Map = homeMap
			s.player.Zone = homeZone
			s.player.X = homeX
			s.player.Y = homeY
			s.player.Z = homeZ
			s.sendPlayerUpdate()
		}
	}
	return true
}

// handleSetSavedInstanceExtend processes CMSG_SET_SAVED_INSTANCE_EXTEND (0x292).
// Reference: WorldSession::HandleSetSavedInstanceExtend (MiscHandler.cpp:1455).
func (s *session) handleSetSavedInstanceExtend(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 6 {
		return true
	}
	r := protocol.NewReader(payload)
	mapID, _ := r.ReadU32()
	difficulty, _ := r.ReadU8()
	extend, _ := r.ReadU8()

	if s.server != nil && s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		cdb := s.server.CharactersStore.DB
		_, _ = cdb.ExecContext(ctx, "UPDATE character_instance SET extended = ? WHERE guid = ? AND instance IN (SELECT id FROM instance WHERE map = ? AND difficulty = ?)", extend, s.playerGUID, mapID, difficulty)
	}
	return true
}

const (
	groupUpdateFlagStatus    uint32 = 0x00000001
	groupUpdateFlagCurHP     uint32 = 0x00000002
	groupUpdateFlagMaxHP     uint32 = 0x00000004
	groupUpdateFlagPowerType uint32 = 0x00000008
	groupUpdateFlagCurPower  uint32 = 0x00000010
	groupUpdateFlagMaxPower  uint32 = 0x00000020
	groupUpdateFlagLevel     uint32 = 0x00000040
	groupUpdateFlagZone      uint32 = 0x00000080
	groupUpdateFlagPosition  uint32 = 0x00000100
)

const (
	memberStatusOnline uint16 = 0x0001
	memberStatusPvP    uint16 = 0x0002
	memberStatusDead   uint16 = 0x0004
	memberStatusGhost  uint16 = 0x0008
	memberStatusAFK    uint16 = 0x0020
	memberStatusDND    uint16 = 0x0040
)

// handleRequestPartyMemberStats processes CMSG_REQUEST_PARTY_MEMBER_STATS (0x27F).
// Reference: WorldSession::HandleRequestPartyMemberStatsOpcode (GroupHandler.cpp:320),
// and GroupHandler::SendPartyMemberStats (GroupHandler.cpp:752).
func (s *session) handleRequestPartyMemberStats(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	targetGUID, _ := r.ReadU64()

	if s.server != nil {
		targetSess := s.server.findSessionByGUID(targetGUID)
		if targetSess != nil && targetSess.player != nil {
			tp := targetSess.player
			mask := groupUpdateFlagStatus | groupUpdateFlagCurHP | groupUpdateFlagMaxHP |
				groupUpdateFlagPowerType | groupUpdateFlagCurPower | groupUpdateFlagMaxPower |
				groupUpdateFlagLevel | groupUpdateFlagZone | groupUpdateFlagPosition

			var status uint16 = memberStatusOnline
			if tp.Health == 0 {
				if tp.PlayerFlags&playerFlagGhost != 0 {
					status |= memberStatusGhost
				} else {
					status |= memberStatusDead
				}
			}
			if tp.PlayerFlags&playerFlagAFK != 0 {
				status |= memberStatusAFK
			}
			if tp.PlayerFlags&playerFlagDND != 0 {
				status |= memberStatusDND
			}

			powerType := classPowerType(tp.Class)
			curPower := uint16(0)
			maxPower := uint16(0)
			if int(powerType) < len(tp.Powers) {
				curPower = uint16(tp.Powers[powerType])
				maxPower = uint16(tp.MaxPowers[powerType])
			}

			buf := protocol.NewBuffer(64)
			buf.WritePackedGUID(targetGUID)
			buf.WriteU32(mask)
			buf.WriteU16(status)
			buf.WriteU32(tp.Health)
			buf.WriteU32(tp.MaxHealth)
			buf.WriteU8(powerType)
			buf.WriteU16(curPower)
			buf.WriteU16(maxPower)
			buf.WriteU16(uint16(tp.Level))
			buf.WriteU16(uint16(tp.Zone))
			buf.WriteU16(uint16(tp.X))
			buf.WriteU16(uint16(tp.Y))
			_ = s.write(uint16(protocol.OpcodeSMSG_PARTY_MEMBER_STATS), buf.Bytes(), true)
		}
	}
	return true
}
