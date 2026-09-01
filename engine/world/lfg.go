package world

import (
	"sort"
	"sync"
	"time"
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
	mu      sync.RWMutex
	enabled bool
	solo    bool
	queue   map[uint64]LFGQueueEntry
}

func NewLFGManager(enabled bool) *LFGManager {
	return &LFGManager{enabled: enabled, queue: make(map[uint64]LFGQueueEntry)}
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
