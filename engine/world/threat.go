package world

import (
	"sort"
	"sync"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// ThreatManager manages a creature's threat table and victim selection heuristics.
// Reference: TrinityCore ThreatManager.h / ThreatManager.cpp.
type ThreatManager struct {
	mu             sync.Mutex
	ownerGUID      uint64
	currentVictim  uint64
	entries        map[uint64]float32
	lastClientSync time.Time
}

// NewThreatManager initializes a ThreatManager for a creature.
func NewThreatManager(ownerGUID uint64) *ThreatManager {
	return &ThreatManager{
		ownerGUID: ownerGUID,
		entries:   make(map[uint64]float32),
	}
}

// AddThreat adds threat for a victim and evaluates target switching.
// Reference: TrinityCore ThreatManager::AddThreat / CompareThreatLessThan.
// In melee range (< 5yd), a new target requires 110% of current victim's threat.
// At range (>= 5yd), a new target requires 130% of current victim's threat.
func (tm *ThreatManager) AddThreat(victim uint64, amount float32, inMelee bool) (switched bool, newVictim uint64) {
	if victim == 0 || amount <= 0 {
		return false, tm.currentVictim
	}
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.entries[victim] += amount
	newThreat := tm.entries[victim]

	if tm.currentVictim == 0 || tm.currentVictim == victim {
		tm.currentVictim = victim
		return false, victim
	}

	currThreat := tm.entries[tm.currentVictim]
	threshold := currThreat * 1.30
	if inMelee {
		threshold = currThreat * 1.10
	}

	if newThreat > threshold {
		tm.currentVictim = victim
		return true, victim
	}
	return false, tm.currentVictim
}

// SetThreat sets absolute threat for a victim (e.g. taunt).
func (tm *ThreatManager) SetThreat(victim uint64, amount float32) (switched bool, newVictim uint64) {
	if victim == 0 {
		return false, tm.currentVictim
	}
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.entries[victim] = amount
	if tm.currentVictim == 0 || amount > tm.entries[tm.currentVictim] {
		tm.currentVictim = victim
		return true, victim
	}
	return false, tm.currentVictim
}

// RemoveThreat removes a victim from the threat table and re-evaluates top victim.
// Reference: TrinityCore ThreatManager::PurgeThreatListRef.
func (tm *ThreatManager) RemoveThreat(victim uint64) (switched bool, newVictim uint64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.entries, victim)
	if tm.currentVictim == victim {
		tm.currentVictim = 0
		var highestGUID uint64
		var highestThreat float32
		for guid, threat := range tm.entries {
			if threat > highestThreat {
				highestThreat = threat
				highestGUID = guid
			}
		}
		tm.currentVictim = highestGUID
		return true, tm.currentVictim
	}
	return false, tm.currentVictim
}

// ClearThreat wipes the threat table on evade or death.
// Reference: TrinityCore ThreatManager::ClearAllThreat.
func (tm *ThreatManager) ClearThreat() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.entries = make(map[uint64]float32)
	tm.currentVictim = 0
}

// GetCurrentVictim returns the current primary threat target.
func (tm *ThreatManager) GetCurrentVictim() uint64 {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.currentVictim
}

// GetThreat returns the threat value for a victim.
func (tm *ThreatManager) GetThreat(victim uint64) float32 {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return tm.entries[victim]
}

// IsEmpty returns true if there are no targets threatening the creature.
func (tm *ThreatManager) IsEmpty() bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return len(tm.entries) == 0
}

// SortedEntries returns protocol-ready ThreatEntry slice sorted descending by threat.
// Reference: TrinityCore ThreatManager::SendThreatListToClients.
func (tm *ThreatManager) SortedEntries() []protocol.ThreatEntry {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	list := make([]protocol.ThreatEntry, 0, len(tm.entries))
	for guid, threat := range tm.entries {
		list = append(list, protocol.ThreatEntry{
			VictimGUID: guid,
			Threat:     uint32(threat * 100),
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Threat > list[j].Threat
	})
	return list
}

// Broadcast helpers for Threat Packets
func (s *Server) broadcastThreatUpdate(mapID uint32, creatureGUID uint64, list []protocol.ThreatEntry) {
	if s == nil {
		return
	}
	payload := protocol.BuildThreatUpdate(creatureGUID, list)
	s.broadcastToNearby(uint16(protocol.OpcodeSMSG_THREAT_UPDATE), payload, nil)
}

func (s *Server) broadcastHighestThreatUpdate(mapID uint32, creatureGUID, highestGUID uint64, list []protocol.ThreatEntry) {
	if s == nil {
		return
	}
	payload := protocol.BuildHighestThreatUpdate(creatureGUID, highestGUID, list)
	s.broadcastToNearby(uint16(protocol.OpcodeSMSG_HIGHEST_THREAT_UPDATE), payload, nil)
}

func (s *Server) broadcastThreatRemove(mapID uint32, creatureGUID, victimGUID uint64) {
	if s == nil {
		return
	}
	payload := protocol.BuildThreatRemove(creatureGUID, victimGUID)
	s.broadcastToNearby(uint16(protocol.OpcodeSMSG_THREAT_REMOVE), payload, nil)
}

func (s *Server) broadcastThreatClear(mapID uint32, creatureGUID uint64) {
	if s == nil {
		return
	}
	payload := protocol.BuildThreatClear(creatureGUID)
	s.broadcastToNearby(uint16(protocol.OpcodeSMSG_THREAT_CLEAR), payload, nil)
}
