package world

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	LFGRoleTank   uint8 = 0x02
	LFGRoleHealer uint8 = 0x04
	LFGRoleDamage uint8 = 0x08

	LFGStateNone   uint8 = 0
	LFGStateQueued uint8 = 2

	LFGUpdateJoinQueue        uint8 = 5
	LFGUpdateRemovedFromQueue uint8 = 7
	LFGUpdateStatus           uint8 = 14

	LFGJoinOK             uint32 = 0
	LFGJoinNotMeetReqs    uint32 = 5
	LFGJoinInvalidDungeon uint32 = 11
)

type LFGQueueEntry struct {
	GUID     uint64
	Roles    uint8
	Dungeons []uint32
	Comment  string
	QueuedAt time.Time
	State    uint8
}

type LFGManager struct {
	mu              sync.RWMutex
	enabled         bool
	solo            bool
	queue           map[uint64]LFGQueueEntry
	validateDungeon func(uint32) bool
}

func NewLFGManager(enabled bool) *LFGManager {
	return &LFGManager{enabled: enabled, queue: make(map[uint64]LFGQueueEntry)}
}

func (m *LFGManager) SetDungeonValidator(validate func(uint32) bool) {
	m.mu.Lock()
	m.validateDungeon = validate
	m.mu.Unlock()
}

func (m *LFGManager) OnLogin() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.enabled && !m.solo {
		m.solo = true
	}
}

func (m *LFGManager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

func (m *LFGManager) Solo() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.solo
}

func (m *LFGManager) ToggleSolo() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.solo = !m.solo
	return m.solo
}

func (m *LFGManager) RequiresFullGroup(players, groupSize int) bool {
	if players < 0 || groupSize < 0 {
		return true
	}
	return !m.Solo() && players != groupSize
}

func (m *LFGManager) Join(guid uint64, roles uint8, dungeons []uint32, comment string) (uint32, LFGQueueEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled || len(dungeons) == 0 {
		return LFGJoinNotMeetReqs, LFGQueueEntry{GUID: guid, Roles: roles, State: LFGStateNone}
	}
	roles &= LFGRoleTank | LFGRoleHealer | LFGRoleDamage
	if roles == 0 {
		return LFGJoinNotMeetReqs, LFGQueueEntry{GUID: guid, Roles: roles, State: LFGStateNone}
	}
	filtered := append([]uint32(nil), dungeons...)
	if m.validateDungeon != nil {
		valid := filtered[:0]
		for _, dungeon := range filtered {
			if m.validateDungeon(dungeon) {
				valid = append(valid, dungeon)
			}
		}
		filtered = valid
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i] < filtered[j] })
	filtered = uniqueUint32(filtered)
	if len(filtered) == 0 {
		return LFGJoinInvalidDungeon, LFGQueueEntry{GUID: guid, Roles: roles, State: LFGStateNone}
	}
	entry := LFGQueueEntry{GUID: guid, Roles: roles, Dungeons: filtered, Comment: comment, QueuedAt: time.Now(), State: LFGStateQueued}
	m.queue[guid] = entry
	return LFGJoinOK, cloneLFGQueueEntry(entry)
}

func (m *LFGManager) Leave(guid uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.queue[guid]; !ok {
		return false
	}
	delete(m.queue, guid)
	return true
}

func (m *LFGManager) Status(guid uint64) (LFGQueueEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.queue[guid]
	if !ok {
		return LFGQueueEntry{GUID: guid, State: LFGStateNone}, false
	}
	return cloneLFGQueueEntry(entry), true
}

func uniqueUint32(values []uint32) []uint32 {
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func cloneLFGQueueEntry(entry LFGQueueEntry) LFGQueueEntry {
	entry.Dungeons = append([]uint32(nil), entry.Dungeons...)
	return entry
}

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

// handleLfdPartyLockInfoRequest processes CMSG_LFD_PARTY_LOCK_INFO_REQUEST (0x371).
// Reference: WorldSession::HandleLfdPartyLockInfoRequestOpcode (LFGHandler.cpp:115).
func (s *session) handleLfdPartyLockInfoRequest(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(1)
	buf.WriteU8(0) // count = 0 players
	_ = s.write(uint16(protocol.OpcodeSMSG_LFG_PARTY_INFO), buf.Bytes(), true)
	return true
}

// handleLfdPlayerLockInfoRequest processes CMSG_LFD_PLAYER_LOCK_INFO_REQUEST (0x36E).
// Reference: WorldSession::HandleLfgPlayerLockInfoRequestOpcode (LFGHandler.cpp:154).
func (s *session) handleLfdPlayerLockInfoRequest(ctx context.Context, payload []byte) bool {
	buf := protocol.NewBuffer(5)
	buf.WriteU8(0)  // Random Dungeon count (uint8, LFGHandler.cpp:173)
	buf.WriteU32(0) // Locked Dungeon count (uint32, LFGHandler.cpp:33)
	_ = s.write(uint16(protocol.OpcodeSMSG_LFG_PLAYER_INFO), buf.Bytes(), true)
	return true
}

// handleLfgProposalResult processes CMSG_LFG_PROPOSAL_RESULT (0x362).
// Reference: WorldSession::HandleLfgProposalResultOpcode (LFGHandler.cpp:68).
func (s *session) handleLfgProposalResult(ctx context.Context, payload []byte) bool {
	if len(payload) < 5 {
		return true
	}
	r := protocol.NewReader(payload)
	proposalID, _ := r.ReadU32()
	accept, _ := r.ReadU8()
	s.debug("lfg proposal result", "account", s.accountName, "proposal", proposalID, "accept", accept)
	return true
}

// handleLfgSetBootVote processes CMSG_LFG_SET_BOOT_VOTE (0x367).
// Reference: WorldSession::HandleLfgSetBootVoteOpcode (LFGHandler.cpp:80).
func (s *session) handleLfgSetBootVote(ctx context.Context, payload []byte) bool {
	if len(payload) < 1 {
		return true
	}
	agree := payload[0]
	s.debug("lfg boot vote", "account", s.accountName, "agree", agree)
	return true
}

// handleLfgSetRoles processes CMSG_LFG_SET_ROLES (0x35E).
// Reference: WorldSession::HandleLfgSetRolesOpcode (LFGHandler.cpp:52).
func (s *session) handleLfgSetRoles(ctx context.Context, payload []byte) bool {
	if len(payload) < 1 {
		return true
	}
	roles := payload[0]
	if s.server != nil && s.server.Features != nil && s.server.Features.LFG != nil {
		lfg := s.server.Features.LFG
		lfg.mu.Lock()
		if entry, ok := lfg.queue[s.playerGUID]; ok {
			entry.Roles = roles & (LFGRoleTank | LFGRoleHealer | LFGRoleDamage)
			lfg.queue[s.playerGUID] = entry
		}
		lfg.mu.Unlock()
	}
	s.debug("lfg set roles", "account", s.accountName, "roles", roles)

	// SMSG_LFG_ROLE_CHOSEN (0x2BB): guid (8), ready (1), roles (4)
	packet := protocol.NewBuffer(13)
	packet.WriteU64(s.playerGUID)
	packet.WriteU8(1)
	packet.WriteU32(uint32(roles))
	if s.groupID != 0 && s.server != nil {
		s.server.broadcastToGroup(s.groupID, uint16(protocol.OpcodeSMSG_LFG_ROLE_CHOSEN), packet.Bytes())
	} else {
		_ = s.write(uint16(protocol.OpcodeSMSG_LFG_ROLE_CHOSEN), packet.Bytes(), true)
	}
	return true
}

// handleLfgTeleport processes CMSG_LFG_TELEPORT (0x369).
// Reference: WorldSession::HandleLfgTeleportOpcode (LFGHandler.cpp:125).
func (s *session) handleLfgTeleport(ctx context.Context, payload []byte) bool {
	if len(payload) < 1 {
		return true
	}
	out := payload[0]
	s.debug("lfg teleport", "account", s.accountName, "out", out)
	// SMSG_LFG_TELEPORT_DENIED (0x200): error (4)
	packet := protocol.NewBuffer(4)
	packet.WriteU32(4) // LFG_TELEPORT_NOT_IN_LFG_GROUP
	_ = s.write(uint16(protocol.OpcodeSMSG_LFG_TELEPORT_DENIED), packet.Bytes(), true)
	return true
}

// handleSearchLfgJoin processes CMSG_SEARCH_LFG_JOIN (0x35C).
// Reference: WorldSession::HandleSearchLfgJoinOpcode (LFGHandler.cpp:145).
func (s *session) handleSearchLfgJoin(ctx context.Context, payload []byte) bool {
	if len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	dungeonID, _ := r.ReadU32()
	s.debug("search lfg join", "account", s.accountName, "dungeon", dungeonID)
	return true
}

// handleSearchLfgLeave processes CMSG_SEARCH_LFG_LEAVE (0x35D).
// Reference: WorldSession::HandleSearchLfgLeaveOpcode (LFGHandler.cpp:160).
func (s *session) handleSearchLfgLeave(ctx context.Context, payload []byte) bool {
	if len(payload) < 4 {
		return true
	}
	r := protocol.NewReader(payload)
	dungeonID, _ := r.ReadU32()
	s.debug("search lfg leave", "account", s.accountName, "dungeon", dungeonID)
	return true
}

// handleSetLfgComment processes CMSG_SET_LFG_COMMENT (0x368).
// Reference: WorldSession::HandleLfgSetCommentOpcode (LFGHandler.cpp:60).
func (s *session) handleSetLfgComment(ctx context.Context, payload []byte) bool {
	r := protocol.NewReader(payload)
	comment, _ := r.ReadCString()
	if s.server != nil && s.server.Features != nil && s.server.Features.LFG != nil {
		lfg := s.server.Features.LFG
		lfg.mu.Lock()
		if entry, ok := lfg.queue[s.playerGUID]; ok {
			entry.Comment = comment
			lfg.queue[s.playerGUID] = entry
		}
		lfg.mu.Unlock()
	}
	s.debug("lfg set comment", "account", s.accountName, "comment", comment)
	return true
}
