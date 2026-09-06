package world

import (
	"context"
	"math"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

// Death lifecycle conversion of the reference chain:
//   - Player::KillPlayer (Player.cpp:4770) triggered by lethal creature damage
//   - Player::Update 6 minute auto-release timer (Player.cpp:1303-1312)
//   - WorldSession::HandleRepopRequest (MiscHandler.cpp:61)
//   - Player::BuildPlayerRepop (Player.cpp:4627)
//   - Player::RepopAtGraveyard (Player.cpp:5109)
//   - ObjectMgr::GetClosestGraveyard / GetDefaultGraveyard (ObjectMgr.cpp:6849/6864)

const (
	// playerFieldByteReleaseTimer is PLAYER_FIELD_BYTE_RELEASE_TIMER within
	// PLAYER_FIELD_BYTES byte 0 (Player.h:417); set while the client shows the
	// auto release spirit countdown.
	playerFieldByteReleaseTimer uint32 = 0x00000008
	unitFieldPlayerFieldBytes          = 1197 // PLAYER_FIELD_BYTES = UNIT_END + 0x0419

	deathExpireStepSeconds = 5 * 60 // DEATH_EXPIRE_STEP (Player.cpp:174)
	maxDeathCount          = 3      // MAX_DEATH_COUNT (Player.cpp:175)

	autoRepopDelay = 6 * time.Minute // KillPlayer m_deathTimer (Player.cpp:4786)

	corpseReclaimRadius = 39.0 // CORPSE_RECLAIM_RADIUS (Corpse.h:35)

	corpseTypeBones uint32 = 0
	corpseTypePvE   uint32 = 1 // CORPSE_RESURRECTABLE_PVE

	teamAlliance uint32 = 469
	teamHorde    uint32 = 67

	defaultGraveyardAlliance uint32 = 4  // Westfall (ObjectMgr.cpp:6855)
	defaultGraveyardHorde    uint32 = 10 // Crossroads (ObjectMgr.cpp:6853)
)

// copseReclaimDelay mirrors the static table in Player.cpp:177.
var copseReclaimDelay = [maxDeathCount]uint32{30, 60, 120}

// corpseReclaimDelaySeconds mirrors Player::GetCorpseReclaimDelay: PvE deaths
// with Death.CorpseReclaimDelay.PvE disabled return 0; PvP deaths with the PvP
// option disabled still use the first table entry. The death count is derived
// from deathExpireTime the same way the reference derives it.
func (s *session) corpseReclaimDelaySeconds(pvp bool) uint32 {
	if pvp {
		if !s.server.Config.DeathCorpseReclaimDelayPvP {
			return copseReclaimDelay[0]
		}
	} else if !s.server.Config.DeathCorpseReclaimDelayPvE {
		return 0
	}
	now := time.Now().Unix()
	count := uint64(0)
	if deathExpire := s.deathExpireTime; deathExpire > 0 && now < deathExpire-1 {
		count = uint64(deathExpire-1-now) / deathExpireStepSeconds
	}
	if count >= maxDeathCount {
		count = maxDeathCount - 1
	}
	return copseReclaimDelay[count]
}

// updateCorpseReclaimDelay mirrors Player::UpdateCorpseReclaimDelay: each death
// within the deathExpireTime window pushes the expire time one further step
// into the future, capped at MAX_DEATH_COUNT steps.
func (s *session) updateCorpseReclaimDelay(pvp bool) {
	if pvp && !s.server.Config.DeathCorpseReclaimDelayPvP {
		return
	}
	if !pvp && !s.server.Config.DeathCorpseReclaimDelayPvE {
		return
	}
	now := time.Now().Unix()
	if s.deathExpireTime > now {
		count := uint64(s.deathExpireTime-now)/deathExpireStepSeconds + 1
		if count < maxDeathCount {
			s.deathExpireTime = now + int64(count+1)*deathExpireStepSeconds
		} else {
			s.deathExpireTime = now + maxDeathCount*deathExpireStepSeconds
		}
	} else {
		s.deathExpireTime = now + deathExpireStepSeconds
	}
}

// sendCorpseReclaimDelay mirrors Player::SendCorpseReclaimDelay: one u32
// remaining time in milliseconds.
func (s *session) sendCorpseReclaimDelay(delay uint32) {
	packet := protocol.NewBuffer(4)
	packet.WriteU32(delay * 1000)
	_ = s.write(uint16(protocol.OpcodeSMSG_CORPSE_RECLAIM_DELAY), packet.Bytes(), true)
}

// killPlayer mirrors Player::KillPlayer for the lethal-damage call site: root
// the corpse in place, keep health at zero, raise the release timer flag on
// non-instance maps (the Go server has no instance maps), start the 6 minute
// auto-release timer, and notify the client of the corpse reclaim delay.
func (s *session) killPlayer(ctx context.Context) {
	if s.player == nil || s.player.Health > 0 {
		return
	}
	if s.playerLoaded && s.player.PlayerFieldBytes&playerFieldByteReleaseTimer == 0 {
		s.player.PlayerFieldBytes |= playerFieldByteReleaseTimer
	}
	s.deathTimer = time.Now().Add(autoRepopDelay)
	if s.server != nil {
		s.server.handleWSGPlayerDeath(s)
		s.server.handleEOTSPlayerDeath(s)
		s.server.handleAVPlayerDeath(s)
		s.server.handleSAPlayerDeath(s)
		s.server.handleICPlayerDeath(s)
		s.server.handleArenaPlayerDeath(s)
	}
	s.clearActiveAuras()
	s.clearDiminishings()
	s.stopMirrorTimers()
	s.sendForcedMovement(uint16(protocol.OpcodeSMSG_FORCE_MOVE_ROOT))
	s.sendPlayerUpdate()
	pvp := false
	s.updateCorpseReclaimDelay(pvp)
	s.sendCorpseReclaimDelay(s.corpseReclaimDelaySeconds(pvp))
	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE characters SET death_expire_time = ? WHERE guid = ?", s.deathExpireTime, s.playerGUID)
	}
	s.durabilityLossAll(ctx, 0.10, false)
	_ = s.write(uint16(protocol.OpcodeSMSG_DURABILITY_DAMAGE_DEATH), []byte{}, true)
	s.debug("player killed", "account", s.accountName, "guid", s.playerGUID)
}

// sendForcedMovement sends one of the forced movement packets used by
// Player::SetMovement (SMSG_FORCE_MOVE_ROOT, SMSG_FORCE_MOVE_UNROOT,
// SMSG_MOVE_WATER_WALK, SMSG_MOVE_LAND_WALK): packed GUID plus a zero
// movement counter.
func (s *session) sendForcedMovement(opcode uint16) {
	packet := protocol.NewBuffer(12)
	packet.WritePackedGUID(s.playerGUID)
	packet.WriteU32(0)
	_ = s.write(opcode, packet.Bytes(), true)
}

// buildPlayerRepop mirrors Player::BuildPlayerRepop: announce the repop with
// SMSG_PRE_RESURRECT, record the corpse at the death location, convert the
// body to a ghost with one health point, switch to water walking, unroot, and
// send the corpse reclaim delay. The reference ghost auras 8326 (Ghost) and
// buildCorpseCreateBlock constructs an SMSG_UPDATE_OBJECT block creating a visible
// Corpse object in the world, matching TrinityCore Corpse::Create and Corpse::BuildValuesUpdate.
func buildCorpseCreateBlock(corpseGUID, ownerGUID uint64, displayID uint32, posX, posY, posZ, orientation float32, isBones bool) []byte {
	values := make([]uint32, 36)
	values[0] = uint32(corpseGUID)
	values[1] = uint32(corpseGUID >> 32)
	values[2] = 0x81 // TYPEMASK_OBJECT (0x01) | TYPEMASK_CORPSE (0x80)
	values[3] = 0
	values[4] = math.Float32bits(1.0)
	values[6] = uint32(ownerGUID)
	values[7] = uint32(ownerGUID >> 32)
	values[10] = displayID
	if isBones {
		values[33] = 0x01 // CORPSE_FLAG_BONES
	}

	mask := protocol.NewUpdateMask(len(values))
	for idx, val := range values {
		if val != 0 {
			_ = mask.Set(idx)
		}
	}
	_ = mask.Set(1) // GUID high

	block := protocol.NewBuffer(128)
	block.WriteU8(protocol.UpdateCreateObject2)
	block.WritePackedGUID(corpseGUID)
	block.WriteU8(7)      // TYPEID_CORPSE
	block.WriteU16(0x0050) // UPDATEFLAG_STATIONARY_POSITION (0x0040) | UPDATEFLAG_LOWGUID (0x0010)
	block.WriteU32(uint32(corpseGUID & 0xFFFFFFFF))
	block.WriteF32(posX)
	block.WriteF32(posY)
	block.WriteF32(posZ)
	block.WriteF32(orientation)

	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for i := 0; i < len(values); i++ {
		if mask.Has(i) {
			block.WriteU32(values[i])
		}
	}
	return block.Bytes()
}

func (s *session) spawnCorpseObject(displayID uint32) {
	if s.player == nil {
		return
	}
	corpseGUID := s.playerGUID | (uint64(0xF101) << 48)
	block := buildCorpseCreateBlock(corpseGUID, s.playerGUID, displayID, s.player.X, s.player.Y, s.player.Z, s.player.Orientation, false)
	updates := protocol.NewUpdateData()
	updates.AddUpdateBlock(block)
	packet, err := updates.BuildPacket(0)
	if err == nil && packet != nil {
		_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
		if s.server != nil {
			s.server.broadcastToNearby(packet.Opcode, packet.Payload.Bytes(), s)
		}
	}
}

func (s *session) despawnCorpseObject() {
	corpseGUID := s.playerGUID | (uint64(0xF101) << 48)
	updates := protocol.NewUpdateData()
	updates.AddOutOfRangeGUID(corpseGUID)
	packet, err := updates.BuildPacket(0)
	if err == nil && packet != nil {
		_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
		if s.server != nil {
			s.server.broadcastToNearby(packet.Opcode, packet.Payload.Bytes(), s)
		}
	}
}

// buildPlayerRepop converts the dead player into a ghost, persists the corpse record,
// spawns the physical Corpse object into the world grid, and applies ghost visual auras.
func (s *session) buildPlayerRepop(ctx context.Context) {
	if s.player == nil {
		return
	}
	preResurrect := protocol.NewBuffer(12)
	preResurrect.WritePackedGUID(s.playerGUID)
	_ = s.write(uint16(protocol.OpcodeSMSG_PRE_RESURRECT), preResurrect.Bytes(), true)

	displayID := uint32(0)
	if s.server.Data != nil {
		if race, found, err := s.server.Data.Race(uint32(s.player.Race)); err == nil && found {
			displayID = race.MaleDisplayID
			if s.player.Gender != 0 {
				displayID = race.FemaleDisplayID
			}
		}
	}

	if s.server.CharactersStore != nil && s.server.CharactersStore.DB != nil {
		// Reference Corpse::SaveToDB deletes any previous record first.
		_, _ = s.server.CharactersStore.ExecStatement(ctx, "CHAR_DEL_CORPSE", s.playerGUID)
		_, _ = s.server.CharactersStore.ExecStatement(ctx, "CHAR_INS_CORPSE",
			s.playerGUID, s.player.X, s.player.Y, s.player.Z, s.player.Orientation, s.player.Map,
			displayID, s.player.Equipment, uint32(s.player.StandState), uint32(s.player.SheathState)<<8,
			s.player.GuildID, 0, 0, time.Now().Unix(), corpseTypePvE, 0, 1)
	}

	s.player.PlayerFlags |= playerFlagGhost
	s.player.PlayerFieldBytes &^= playerFieldByteReleaseTimer
	s.player.Health = 1
	s.deathTimer = time.Time{}
	if s.player.Race == 4 { // RACE_NIGHTELF
		s.applyAura(20584) // Wisp Spirit
	} else {
		s.applyAura(8326) // Ghost
	}
	s.spawnCorpseObject(displayID)
	s.sendPlayerUpdate()
	s.sendForcedMovement(uint16(protocol.OpcodeSMSG_MOVE_WATER_WALK))
	s.sendForcedMovement(uint16(protocol.OpcodeSMSG_FORCE_MOVE_UNROOT))
	s.sendCorpseReclaimDelay(s.corpseReclaimDelaySeconds(false))
}

// repopAtGraveyard mirrors Player::RepopAtGraveyard: locate the graveyard
// linked to the ghost zone, teleport there, point the client corpse map at
// the graveyard, and in battlegrounds automatically queue into the spirit wave.
func (s *session) repopAtGraveyard(ctx context.Context) {
	if s.player == nil {
		return
	}
	grave, ok := s.server.closestGraveyard(ctx, s.player.X, s.player.Y, s.player.Z, s.player.Map, s.player.Zone, playerTeam(s.player.Race))
	if ok {
		s.teleportTo(grave.MapID, grave.X, grave.Y, grave.Z, s.player.Orientation)
		packet := protocol.NewBuffer(16)
		packet.WriteU32(grave.MapID)
		packet.WriteF32(grave.X)
		packet.WriteF32(grave.Y)
		packet.WriteF32(grave.Z)
		_ = s.write(uint16(protocol.OpcodeSMSG_DEATH_RELEASE_LOC), packet.Bytes(), true)
	} else {
		s.debug("no graveyard found, staying at current location", "account", s.accountName, "guid", s.playerGUID)
	}

	// In battlegrounds, automatically queue for wave resurrection
	switch s.player.Map {
	case 30, 489, 529, 566, 607, 628:
		if s.server != nil {
			s.server.spiritWaveMu.Lock()
			if s.server.spiritReviveQueue == nil {
				s.server.spiritReviveQueue = make(map[uint64]uint64)
			}
			s.server.spiritReviveQueue[s.playerGUID] = 0
			elapsed := time.Since(s.server.lastSpiritWave)
			s.server.spiritWaveMu.Unlock()

			s.applyAura(2584) // SPELL_WAITING_FOR_RESURRECT
			timeLeftMs := uint32(30000)
			if elapsed < 30*time.Second {
				timeLeftMs = uint32((30*time.Second - elapsed).Milliseconds())
			}
			s.sendAreaSpiritHealerTime(0, timeLeftMs)
		}
	}
}

// playerTeam maps a race to the graveyard faction domain: TEAM_ALLIANCE (469),
// TEAM_HORDE (67), or 0 when the race has no team assignment.
func playerTeam(race uint8) uint32 {
	if isAllianceRace(race) {
		return teamAlliance
	}
	switch race {
	case 2, 5, 6, 8, 10:
		return teamHorde
	}
	return 0
}

// closestGraveyard mirrors ObjectMgr::GetClosestGraveyard with the zone taken
// from the persisted player zone instead of the reference map-data zone
// lookup: graveyard_zone rows linked to the zone, filtered by faction, nearest
// same-map entry by 3D distance, first other-map entry as fallback, and the
// default Westfall/Crossroads graveyard when the zone has no links.
func (s *Server) closestGraveyard(ctx context.Context, x, y, z float32, mapID, zoneID, team uint32) (wotlk.WorldSafeLoc, bool) {
	if s.WorldStore == nil || s.WorldStore.DB == nil || s.Data == nil {
		return wotlk.WorldSafeLoc{}, false
	}
	defaultFor := func() (wotlk.WorldSafeLoc, bool) {
		id := uint32(0)
		switch team {
		case teamAlliance:
			id = defaultGraveyardAlliance
		case teamHorde:
			id = defaultGraveyardHorde
		default:
			return wotlk.WorldSafeLoc{}, false
		}
		loc, found, err := s.Data.WorldSafeLoc(id)
		if err != nil {
			s.debug("default graveyard lookup failed", "error", err)
			return wotlk.WorldSafeLoc{}, false
		}
		return loc, found
	}
	rows, err := s.WorldStore.DB.QueryContext(ctx, "SELECT ID, Faction FROM graveyard_zone WHERE GhostZone = ?", zoneID)
	if err != nil {
		s.debug("graveyard zone query failed", "error", err)
		return defaultFor()
	}
	defer rows.Close()
	type candidate struct {
		loc  wotlk.WorldSafeLoc
		dist float32
	}
	var nearest *candidate
	var farLoc *wotlk.WorldSafeLoc
	for rows.Next() {
		var safeLocID, faction uint32
		if err := rows.Scan(&safeLocID, &faction); err != nil {
			continue
		}
		if faction != 0 && team != 0 && faction != team {
			continue
		}
		loc, found, err := s.Data.WorldSafeLoc(safeLocID)
		if err != nil || !found {
			continue
		}
		if loc.MapID == mapID {
			dx := loc.X - x
			dy := loc.Y - y
			dz := loc.Z - z
			dist := dx*dx + dy*dy + dz*dz
			if nearest == nil || dist < nearest.dist {
				nearest = &candidate{loc: loc, dist: dist}
			}
		} else if farLoc == nil {
			far := loc
			farLoc = &far
		}
	}
	if err := rows.Err(); err != nil {
		s.debug("graveyard zone rows failed", "error", err)
	}
	if nearest != nil {
		return nearest.loc, true
	}
	if farLoc != nil {
		return *farLoc, true
	}
	return defaultFor()
}

// handleRepopRequest mirrors WorldSession::HandleRepopRequest: alive players
// and players that are already ghosts are ignored, otherwise the repop flow
// (corpse creation, ghost conversion) runs followed by the graveyard teleport.
// The reference SPELL_AURA_PREVENT_RESURRECTION guard has no aura system yet.
// The payload carries one bool (CheckInstance) which the reference also reads
// but does not act on.
func (s *session) handleRepopRequest(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	if _, err := reader.ReadU8(); err != nil {
		return false
	}
	if !s.playerLoaded || s.player == nil {
		return true
	}
	if s.player.Health > 0 || s.player.PlayerFlags&playerFlagGhost != 0 || s.hasAura(58549) {
		return true
	}
	s.buildPlayerRepop(ctx)
	s.repopAtGraveyard(ctx)
	return true
}

// updatePlayerDeathTimers runs the reference Player::Update auto-release:
// after six minutes a dead player that has not released is converted to a
// ghost and teleported to the graveyard. The reference skips this on
// instanceable maps and under SPELL_AURA_PREVENT_RESURRECTION; the Go server
// has no instance maps or aura system.
func (s *Server) updatePlayerDeathTimers(ctx context.Context, now time.Time) {
	s.sessionsMu.RLock()
	var due []*session
	for sess := range s.sessions {
		if !sess.playerLoaded || sess.player == nil {
			continue
		}
		if sess.player.Health == 0 && sess.player.PlayerFlags&playerFlagGhost == 0 && !sess.deathTimer.IsZero() && !now.Before(sess.deathTimer) {
			due = append(due, sess)
		}
	}
	s.sessionsMu.RUnlock()
	for _, sess := range due {
		sess.buildPlayerRepop(ctx)
		sess.repopAtGraveyard(ctx)
	}
}

// updateSpiritHealerResurrectWaves pulses every 30 seconds to resurrect ghosts
// queued at spirit guides / battleground spirit healers.
// Reference: Battleground::HandleTrigger / Battleground.cpp:310-340.
func (s *Server) updateSpiritHealerResurrectWaves(ctx context.Context, now time.Time) {
	s.spiritWaveMu.Lock()
	if s.lastSpiritWave.IsZero() {
		s.lastSpiritWave = now
	}
	if now.Sub(s.lastSpiritWave) < 30*time.Second {
		s.spiritWaveMu.Unlock()
		return
	}
	s.lastSpiritWave = now
	if s.spiritReviveQueue == nil || len(s.spiritReviveQueue) == 0 {
		s.spiritWaveMu.Unlock()
		return
	}
	queued := make(map[uint64]uint64, len(s.spiritReviveQueue))
	for pGUID, sGUID := range s.spiritReviveQueue {
		queued[pGUID] = sGUID
	}
	s.spiritReviveQueue = make(map[uint64]uint64)
	s.spiritWaveMu.Unlock()

	for playerGUID, spiritGUID := range queued {
		sess := s.findSessionByGUID(playerGUID)
		if sess != nil && sess.playerLoaded && sess.player != nil && sess.player.PlayerFlags&playerFlagGhost != 0 {
			sess.resurrectPlayer(ctx, 1.0)
			sess.removeAura(2584) // SPELL_WAITING_FOR_RESURRECT
			sess.applyAura(22012) // SPELL_SPIRIT_HEAL_MANA
			sess.spawnCorpseBones(ctx)
			sess.debug("spirit wave resurrected ghost", "account", sess.accountName, "guid", playerGUID, "spirit", spiritGUID)
		}
	}
}

// spawnCorpseBones converts an existing corpse record into bones, mirroring
// Player::SpawnCorpseBones for the resurrect-at-graveyard path.
func (s *session) spawnCorpseBones(ctx context.Context) {
	if s.server == nil || s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return
	}
	_, _ = s.server.CharactersStore.DB.ExecContext(ctx, "UPDATE corpse SET corpseType = ?, time = ? WHERE guid = ?", corpseTypeBones, time.Now().Unix(), s.playerGUID)
	s.despawnCorpseObject()
	if s.player != nil {
		corpseGUID := s.playerGUID | (uint64(0xF101) << 48)
		block := buildCorpseCreateBlock(corpseGUID, s.playerGUID, 0, s.player.X, s.player.Y, s.player.Z, s.player.Orientation, true)
		updates := protocol.NewUpdateData()
		updates.AddUpdateBlock(block)
		packet, err := updates.BuildPacket(0)
		if err == nil && packet != nil {
			_ = s.write(packet.Opcode, packet.Payload.Bytes(), true)
			if s.server != nil {
				s.server.broadcastToNearby(packet.Opcode, packet.Payload.Bytes(), s)
			}
		}
	}
}

// resurrectPlayer mirrors Player::ResurrectPlayer for the core state: clear
// the ghost flag and death timer, restore land walking and control, and point
// the corpse map at an invalid map id. When restorePercent is positive the
// reference health and power restoration applies (half of maximum health and
// mana, zero rage, half energy).
func (s *session) resurrectPlayer(ctx context.Context, restorePercent float32) {
	if s.player == nil {
		return
	}
	packet := protocol.NewBuffer(16)
	packet.WriteU32(0xFFFFFFFF)
	packet.WriteF32(0)
	packet.WriteF32(0)
	packet.WriteF32(0)
	_ = s.write(uint16(protocol.OpcodeSMSG_DEATH_RELEASE_LOC), packet.Bytes(), true)
	s.player.PlayerFlags &^= playerFlagGhost
	s.player.PlayerFieldBytes &^= playerFieldByteReleaseTimer
	s.deathTimer = time.Time{}
	s.removeAura(8326)
	s.removeAura(20584)
	s.despawnCorpseObject()
	if restorePercent > 0 {
		s.player.Health = uint32(float32(s.player.MaxHealth) * restorePercent)
		s.player.Powers[0] = uint32(float32(s.player.MaxPowers[0]) * restorePercent) // mana
		s.player.Powers[1] = 0                                                       // rage
		s.player.Powers[3] = uint32(float32(s.player.MaxPowers[3]) * restorePercent) // energy
	}
	s.sendPlayerUpdate()
	s.sendForcedMovement(uint16(protocol.OpcodeSMSG_MOVE_LAND_WALK))
	s.sendForcedMovement(uint16(protocol.OpcodeSMSG_FORCE_MOVE_UNROOT))
}

// resurrectionData mirrors Player::_resurrectionData (ResurrectionData):
// the caster, the caster location for the teleport, and the restored health
// and mana values carried by the resurrect spell effect.
type resurrectionData struct {
	GUID    uint64
	MapID   uint32
	X, Y, Z float32
	Health  uint32
	Mana    uint32
}

// setResurrectRequestData mirrors Player::SetResurrectRequestData. The
// reference asserts that no request is outstanding; the caller is expected to
// check first, so an overwrite here is logged and refused.
func (s *session) setResurrectRequestData(casterGUID uint64, mapID uint32, x, y, z float32, health, mana uint32) {
	if s.resurrection != nil {
		s.debug("resurrect request overwritten", "account", s.accountName, "guid", s.playerGUID)
		return
	}
	s.resurrection = &resurrectionData{GUID: casterGUID, MapID: mapID, X: x, Y: y, Z: z, Health: health, Mana: mana}
}

// sendResurrectRequest mirrors Spell::SendResurrectRequest: raw caster GUID,
// length-prefixed caster name (empty for player casters, the client resolves
// those by GUID), the spirit healer resurrection sickness flag, and the flag
// overriding the corpse reclaim delay for spells that ignore the timer.
func (s *session) sendResurrectRequest(casterGUID uint64, name string, spiritHealer, ignoreReclaimTimer bool) {
	packet := protocol.NewBuffer(24 + len(name))
	packet.WriteU64(casterGUID)
	packet.WriteU32(uint32(len(name)) + 1)
	packet.WriteString(name)
	packet.WriteU8(boolByte(spiritHealer))
	packet.WriteU8(boolByte(ignoreReclaimTimer))
	_ = s.write(uint16(protocol.OpcodeSMSG_RESURRECT_REQUEST), packet.Bytes(), true)
}

func boolByte(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}

// handleResurrectResponse mirrors WorldSession::HandleResurrectResponse:
// alive players ignore the packet, a zero response clears the pending request,
// and an accepted response must match the stored resurrecter before the stored
// health, mana, and location are applied.
func (s *session) handleResurrectResponse(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	resurrecter, err := reader.ReadU64()
	if err != nil {
		return false
	}
	response, err := reader.ReadU8()
	if err != nil {
		return false
	}
	if !s.playerLoaded || s.player == nil {
		return true
	}
	// Reference IsAlive() is death-state based: ghosts and corpses are not alive.
	if s.player.PlayerFlags&playerFlagGhost == 0 && s.player.Health > 0 {
		return true
	}
	if response == 0 {
		s.resurrection = nil
		return true
	}
	if s.resurrection == nil || s.resurrection.GUID != resurrecter {
		return true
	}
	data := *s.resurrection
	// Reference teleports to the caster location before resurrecting so the
	// player does not revive into nearby creatures at the corpse; the delayed
	// teleport retry path has no Go equivalent because teleportTo is sync.
	if data.MapID != s.player.Map || data.X != s.player.X || data.Y != s.player.Y || data.Z != s.player.Z {
		s.teleportTo(data.MapID, data.X, data.Y, data.Z, s.player.Orientation)
	}
	s.resurrectPlayer(ctx, 0)
	s.player.Health = data.Health
	s.player.Powers[0] = data.Mana
	s.player.Powers[1] = 0 // rage
	if s.player.MaxPowers[3] > 0 {
		s.player.Powers[3] = s.player.MaxPowers[3] // full energy
	}
	s.resurrection = nil
	s.spawnCorpseBones(ctx)
	s.sendPlayerUpdate()
	return true
}

// corpseRecord is one row of the characters.corpse table as written by
// buildPlayerRepop.
type corpseRecord struct {
	MapID       uint32
	X, Y, Z     float32
	Orientation float32
	CorpseType  uint32
	GhostTime   int64
}

// loadCorpse mirrors Player::GetCorpse: the resurrectable corpse of this
// player (corpseType PvE or PvP); bones (type 0) are not returned.
func (s *session) loadCorpse(ctx context.Context) (corpseRecord, bool) {
	if s.server.CharactersStore == nil || s.server.CharactersStore.DB == nil {
		return corpseRecord{}, false
	}
	row := s.server.CharactersStore.DB.QueryRowContext(ctx, "SELECT mapId, posX, posY, posZ, orientation, corpseType, time FROM corpse WHERE guid = ? AND corpseType <> ?", s.playerGUID, corpseTypeBones)
	var corpse corpseRecord
	if err := row.Scan(&corpse.MapID, &corpse.X, &corpse.Y, &corpse.Z, &corpse.Orientation, &corpse.CorpseType, &corpse.GhostTime); err != nil {
		return corpseRecord{}, false
	}
	return corpse, true
}

// handleReclaimCorpse mirrors WorldSession::HandleReclaimCorpse: a ghost in
// range of its own resurrectable corpse after the reclaim delay elapses is
// resurrected at half health and the corpse is turned into bones. The arena
// guard has no Go arena system yet.
func (s *session) handleReclaimCorpse(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	_, err := reader.ReadU64()
	if err != nil {
		reader = protocol.NewReader(payload)
		if _, err = reader.ReadPackedGUID(); err != nil {
			return false
		}
	}
	if !s.playerLoaded || s.player == nil {
		return true
	}
	// Reference: IsAlive() or not-yet-released. The ghost flag distinguishes
	// the dead body (no flag) from the released ghost, matching the reference
	// death-state machine; health alone cannot (ghosts carry one health point).
	if s.player.PlayerFlags&playerFlagGhost == 0 {
		return true
	}
	corpse, ok := s.loadCorpse(ctx)
	if !ok {
		return true
	}
	// prevent resurrect before the reclaim delay after body release finished
	if corpse.GhostTime+int64(s.corpseReclaimDelaySeconds(corpse.CorpseType != corpseTypePvE)) > time.Now().Unix() {
		return true
	}
	if corpse.MapID != s.player.Map || distance3D(s.player.X, s.player.Y, s.player.Z, corpse.X, corpse.Y, corpse.Z) > corpseReclaimRadius {
		return true
	}
	s.resurrectPlayer(ctx, 0.5)
	s.spawnCorpseBones(ctx)
	return true
}

const (
	spellEffectResurrectNew = 113 // SPELL_EFFECT_RESURRECT_NEW (SharedDefines.h:924)
	npcFlagSpiritHealer     = 0x00004000
	npcFlagSpiritGuide      = 0x00008000
	npcFlagSpiritService    = npcFlagSpiritHealer | npcFlagSpiritGuide
)

// handleSelfRes mirrors WorldSession::HandleSelfResOpcode (SpellHandler.cpp:602):
// an empty opcode that casts the spell stored in PLAYER_SELF_RES_SPELL and
// clears the field. The stored spell's resurrect effect registers a resurrect
// request from the player to the player, which the client answers through
// CMSG_RESURRECT_RESPONSE, exactly like the reference EffectResurrectNew chain.
// The SPELL_AURA_PREVENT_RESURRECTION guard has no aura system yet.
func (s *session) handleSelfRes(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	spellID := s.player.SelfResSpell
	if spellID == 0 {
		return true
	}
	s.player.SelfResSpell = 0
	s.sendPlayerUpdate()
	if s.server.Data == nil {
		return true
	}
	spell, found, err := s.server.Data.Spell(spellID)
	if err != nil || !found {
		s.debug("self resurrect spell lookup failed", "account", s.accountName, "spell", spellID, "found", found, "error", err)
		return true
	}
	s.finishSpellCast(ctx, 0, spellID, spell, protocol.SpellTargetData{})
	return true
}

// applySelfResurrectEffect mirrors Spell::EffectResurrectNew (SpellEffects.cpp:246)
// for the self-cast case: a dead player gets a resurrect request from itself
// carrying the effect damage as health and MiscValue as mana.
func (s *session) applySelfResurrectEffect(spell wotlk.Spell) {
	if s.player == nil || s.player.Health > 0 {
		return
	}
	if s.resurrection != nil {
		return // already have one active request
	}
	for _, effect := range spell.Effects {
		if effect.Effect != spellEffectResurrectNew {
			continue
		}
		health := uint32(1)
		if effect.BasePoints >= 0 {
			health = uint32(effect.BasePoints + 1) // damage as computed by CalcValue
		}
		mana := uint32(0)
		if effect.MiscValue > 0 {
			mana = uint32(effect.MiscValue)
		}
		s.setResurrectRequestData(s.playerGUID, s.player.Map, s.player.X, s.player.Y, s.player.Z, health, mana)
		s.sendResurrectRequest(s.playerGUID, "", false, false)
		return
	}
}

// creatureIsSpiritService resolves the npcflag of a spawned creature and
// mirrors Unit::IsSpiritService (UNIT_NPC_FLAG_SPIRITHEALER | SPIRITGUIDE).
func (s *session) creatureIsSpiritService(ctx context.Context, guid uint64) bool {
	if s.server.WorldStore == nil || s.server.WorldStore.DB == nil || guid == 0 {
		return false
	}
	low := uint32(guid & 0x00FFFFFF)
	entry := uint32((guid >> 24) & 0x00FFFFFF)
	var npcFlag uint32
	var err error
	if entry != 0 {
		err = s.server.WorldStore.DB.QueryRowContext(ctx,
			"SELECT COALESCE(NULLIF(c.npcflag, 0), t.npcflag, 0) FROM creature_template t LEFT JOIN creature c ON c.guid = ? WHERE t.entry = ?", low, entry).Scan(&npcFlag)
	} else {
		err = s.server.WorldStore.DB.QueryRowContext(ctx,
			"SELECT COALESCE(NULLIF(c.npcflag, 0), t.npcflag, 0) FROM creature c JOIN creature_template t ON t.entry = c.id WHERE c.guid = ? OR c.guid = ?", low, guid).Scan(&npcFlag)
	}
	if err != nil {
		return false
	}
	return npcFlag&npcFlagSpiritService != 0
}

// sendAreaSpiritHealerTime mirrors BattlegroundMgr::SendAreaSpiritHealerQueryOpcode:
// send the time remaining until next spirit healer resurrection pulse.
func (s *session) sendAreaSpiritHealerTime(guid uint64, timeLeft uint32) {
	packet := protocol.NewBuffer(12)
	packet.WriteU64(guid)
	packet.WriteU32(timeLeft)
	_ = s.write(uint16(protocol.OpcodeSMSG_AREA_SPIRIT_HEALER_TIME), packet.Bytes(), true)
}

// handleAreaSpiritHealerQuery mirrors WorldSession::HandleAreaSpiritHealerQueryOpcode:
// Reference: BattlegroundMgr::SendAreaSpiritHealerQueryOpcode / BattlegroundHandler.cpp:80.
func (s *session) handleAreaSpiritHealerQuery(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	if !s.playerLoaded || s.player == nil {
		return true
	}
	if !s.creatureIsSpiritService(ctx, guid) {
		return true
	}
	now := time.Now()
	s.server.spiritWaveMu.Lock()
	elapsed := now.Sub(s.server.lastSpiritWave)
	s.server.spiritWaveMu.Unlock()
	timeLeftMs := uint32(30000)
	if elapsed < 30*time.Second {
		timeLeftMs = uint32((30*time.Second - elapsed).Milliseconds())
	}
	s.sendAreaSpiritHealerTime(guid, timeLeftMs)
	return true
}

// handleAreaSpiritHealerQueue mirrors WorldSession::HandleAreaSpiritHealerQueueOpcode:
// Reference: Battleground::AddPlayerToResurrectQueue / Battleground.cpp:1240.
func (s *session) handleAreaSpiritHealerQueue(ctx context.Context, payload []byte) bool {
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil {
		return false
	}
	if !s.playerLoaded || s.player == nil {
		return true
	}
	if !s.creatureIsSpiritService(ctx, guid) {
		return true
	}
	s.server.spiritWaveMu.Lock()
	if s.server.spiritReviveQueue == nil {
		s.server.spiritReviveQueue = make(map[uint64]uint64)
	}
	s.server.spiritReviveQueue[s.playerGUID] = guid
	elapsed := time.Since(s.server.lastSpiritWave)
	s.server.spiritWaveMu.Unlock()

	s.applyAura(2584) // SPELL_WAITING_FOR_RESURRECT
	timeLeftMs := uint32(30000)
	if elapsed < 30*time.Second {
		timeLeftMs = uint32((30*time.Second - elapsed).Milliseconds())
	}
	s.sendAreaSpiritHealerTime(guid, timeLeftMs)
	return true
}

// handleHearthAndResurrect mirrors WorldSession::HandleHearthAndResurrect (MiscHandler.cpp:1505):
// if flying, ignore. Battlefield ask-to-leave has no Go battlefield system yet.
// If the player's area has AREA_FLAG_WINTERGRASP_2, repop the player (creating a corpse if needed),
// resurrect with 100% health and powers, and teleport to the homebind location.
func (s *session) handleHearthAndResurrect(ctx context.Context) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	if s.isInFlight() {
		return true
	}
	canHearthAndRes := false
	if s.server.Data != nil && s.player.Zone != 0 {
		area, found, err := s.server.Data.Area(s.player.Zone)
		if err == nil && found && area.Flags&wotlk.AreaFlagWintergrasp2 != 0 {
			canHearthAndRes = true
		}
	}
	if !canHearthAndRes {
		return true
	}
	s.buildPlayerRepop(ctx)
	s.resurrectPlayer(ctx, 1.0)
	destMap := s.player.HomebindMap
	destX := s.player.HomebindX
	destY := s.player.HomebindY
	destZ := s.player.HomebindZ
	if destMap == 0 && destX == 0 && destY == 0 && destZ == 0 {
		destMap = s.player.Map
		destX, destY, destZ = s.player.X, s.player.Y, s.player.Z
	}
	s.teleportTo(destMap, destX, destY, destZ, s.player.Orientation)
	return true
}

// handleSpiritHealerActivate processes CMSG_SPIRIT_HEALER_ACTIVATE (0x21C).
// Reference: WorldSession::HandleSpiritHealerActivateOpcode (MiscHandler.cpp:712)
// and WorldSession::SendSpiritResurrect (NPCHandler.cpp:219).
func (s *session) handleSpiritHealerActivate(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil || len(payload) < 8 {
		return true
	}
	r := protocol.NewReader(payload)
	guid, _ := r.ReadU64()
	if !s.creatureIsSpiritService(ctx, guid) {
		return true
	}
	s.resurrectPlayer(ctx, 0.5)
	s.durabilityLossAll(ctx, 0.25, true)
	if s.player.Level > 10 {
		// Characters level 1-10 have no sickness.
		// Characters level 11-19 suffer 1 minute per level above 10 (1-9 minutes).
		// Characters level 20+ suffer 10 minutes of sickness (TC Player::ResurrectPlayer:4740-4753).
		durationMinutes := s.player.Level - 10
		if durationMinutes > 10 {
			durationMinutes = 10
		}
		s.applyAuraWithDuration(15007, uint32(durationMinutes)*60*1000)
	}
	s.spawnCorpseBones(ctx)
	return true
}

// handleCorpseQuery processes MSG_CORPSE_QUERY (0x216).
// Reference: WorldSession::HandleCorpseQueryOpcode (QueryHandler.cpp:144).
func (s *session) handleCorpseQuery(ctx context.Context, payload []byte) bool {
	if !s.playerLoaded || s.player == nil {
		return true
	}
	corpse, ok := s.loadCorpse(ctx)
	if !ok {
		buf := protocol.NewBuffer(1)
		buf.WriteU8(0) // corpse not found
		_ = s.write(uint16(protocol.OpcodeMSG_CORPSE_QUERY), buf.Bytes(), true)
		return true
	}
	buf := protocol.NewBuffer(25)
	buf.WriteU8(1) // corpse found
	buf.WriteU32(corpse.MapID)
	buf.WriteF32(corpse.X)
	buf.WriteF32(corpse.Y)
	buf.WriteF32(corpse.Z)
	buf.WriteU32(corpse.MapID)
	buf.WriteU32(0) // unknown
	_ = s.write(uint16(protocol.OpcodeMSG_CORPSE_QUERY), buf.Bytes(), true)
	return true
}
