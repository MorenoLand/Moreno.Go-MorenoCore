package world

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type playerPos struct {
	Map    uint32
	X      float32
	Y      float32
	Z      float32
	GUID   uint64
	Race   uint8
	Level  uint8
	IsGM   bool
	IsDead bool
	Sess   *session
}

// creatureMotion tracks live server-side creature movement state, the role
// TrinityCore's MotionMaster fills: home position (for random wander around
// spawn), current position, waypoint path/point and the next move deadline.
type creatureMotion struct {
	GUID     uint64
	Entry    uint32
	Map      uint32
	HomeX    float32
	HomeY    float32
	HomeZ    float32
	X        float32
	Y        float32
	Z        float32
	Speed    float32 // yd/s walk speed used for wander
	RunSpeed float32 // yd/s run speed used for pursuit
	MoveType uint32  // 1 random, 2 waypoint
	Wander   float64

	Faction uint32
	Level   uint32

	TargetGUID uint64
	InCombat   bool
	LastAttack time.Time

	PathID  uint32
	Points  []waypointPoint
	NextIdx int

	Moving    bool
	MoveEnds  time.Time
	WaitUntil time.Time
	Refreshed time.Time
}

type waypointPoint struct {
	X           float32
	Y           float32
	Z           float32
	Orientation float32
	Delay       uint32
}

// motionTTL bounds how long idle state survives between nearby sweeps so a
// creature nobody observes stops consuming memory.
const motionTTL = 10 * time.Minute

func (s *Server) motionFor(ctx context.Context, guid, entry, mapID uint32, x, y, z float32, moveType uint32, wander float64, walkSpeed float32) *creatureMotion {
	s.motionMu.Lock()
	defer s.motionMu.Unlock()
	if s.creatureMotion == nil {
		s.creatureMotion = make(map[uint64]*creatureMotion)
	}
	key := creatureWorldGUID(guid, entry)
	motion := s.creatureMotion[key]
	if motion == nil || motion.Entry != entry {
		motion = &creatureMotion{
			GUID:     key,
			Entry:    entry,
			Map:      mapID,
			HomeX:    x, HomeY: y, HomeZ: z,
			X: x, Y: y, Z: z,
			Speed:    walkSpeed,
			RunSpeed: 7.0,
			MoveType: moveType,
			Wander:   wander,
		}
		if walkSpeed <= 0 {
			motion.Speed = 2.5
		}
		if moveType == 2 {
			motion.PathID = s.loadCreaturePathID(ctx, guid, entry)
			motion.Points = s.loadWaypoints(ctx, motion.PathID)
		}
		s.creatureMotion[key] = motion
	}
	motion.Refreshed = time.Now()
	return motion
}

func (s *Server) triggerCreatureAggro(ctx context.Context, creatureGUID, playerGUID uint64) {
	s.motionMu.Lock()
	defer s.motionMu.Unlock()
	if s.creatureMotion == nil {
		s.creatureMotion = make(map[uint64]*creatureMotion)
	}
	motion := s.creatureMotion[creatureGUID]
	if motion == nil && s.WorldStore != nil && s.WorldStore.DB != nil {
		guid := uint32(creatureGUID & 0x00FFFFFF)
		entry := uint32((creatureGUID >> 24) & 0x00FFFFFF)
		var x, y, z float64
		var mapID, faction, level int64
		if err := s.WorldStore.DB.QueryRowContext(ctx, `SELECT c.map, c.position_x, c.position_y, c.position_z,
			COALESCE(t.faction, 0), COALESCE(t.maxlevel, 1)
			FROM creature AS c
			JOIN creature_template AS t ON t.entry = c.id
			WHERE c.guid = ?`, guid).Scan(&mapID, &x, &y, &z, &faction, &level); err == nil {
			motion = &creatureMotion{
				GUID:     creatureGUID,
				Entry:    entry,
				Map:      uint32(mapID),
				HomeX:    float32(x), HomeY: float32(y), HomeZ: float32(z),
				X: float32(x), Y: float32(y), Z: float32(z),
				Speed:    2.5,
				RunSpeed: 7.0,
				Faction:  uint32(faction),
				Level:    uint32(level),
			}
			s.creatureMotion[creatureGUID] = motion
		}
	}
	if motion != nil {
		motion.TargetGUID = playerGUID
		motion.InCombat = true
		motion.Moving = false
	}
}

func (s *Server) loadCreaturePathID(ctx context.Context, guid, entry uint32) uint32 {
	var pathID int64
	if err := s.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(NULLIF(ca.path_id, 0), NULLIF(cta.path_id, 0), ?) FROM creature AS c LEFT JOIN creature_addon AS ca ON ca.guid = c.guid LEFT JOIN creature_template_addon AS cta ON cta.entry = c.id WHERE c.guid = ?", guid, guid).Scan(&pathID); err != nil {
		return guid
	}
	if pathID == 0 {
		return guid
	}
	return uint32(pathID)
}

func (s *Server) loadWaypoints(ctx context.Context, pathID uint32) []waypointPoint {
	query := "SELECT position_x, position_y, position_z, orientation, delay FROM waypoint_data WHERE id = ? ORDER BY point"
	rows, err := s.WorldStore.DB.QueryContext(ctx, query, pathID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var points []waypointPoint
	for rows.Next() {
		var p waypointPoint
		var x, y, z, orientation float64
		var delay int64
		if err := rows.Scan(&x, &y, &z, &orientation, &delay); err != nil {
			continue
		}
		p.X, p.Y, p.Z, p.Orientation, p.Delay = float32(x), float32(y), float32(z), float32(orientation), uint32(delay)
		points = append(points, p)
	}
	return points
}

// updateActiveCreatures drives wander/patrol/combat motion for creatures near
// online players, mirroring RandomMovementGenerator, WaypointMovementGenerator,
// and TargetedMovementGenerator behaviour.
func (s *Server) updateActiveCreatures(ctx context.Context) {
	if s.WorldStore == nil || s.WorldStore.DB == nil {
		return
	}
	var players []playerPos
	s.sessionsMu.RLock()
	for sess := range s.sessions {
		if sess.playerLoaded && sess.player != nil {
			isGM := (sess.player.ExtraFlags&0x00000001 != 0) || sess.gmChat
			isDead := sess.player.Health == 0
			players = append(players, playerPos{
				Map:    sess.player.Map,
				X:      sess.player.X,
				Y:      sess.player.Y,
				Z:      sess.player.Z,
				GUID:   sess.playerGUID,
				Race:   sess.player.Race,
				Level:  sess.player.Level,
				IsGM:   isGM,
				IsDead: isDead,
				Sess:   sess,
			})
		}
	}
	s.sessionsMu.RUnlock()
	if len(players) == 0 {
		return
	}
	now := time.Now()
	s.pruneCreatureMotion(now)
	distance := float64(s.Config.VisibilityDistanceContinents)
	if distance <= 0 {
		distance = 100.0
	}
	query := `SELECT c.guid, c.id, c.position_x, c.position_y, c.position_z, c.MovementType, c.wander_distance,
		COALESCE(NULLIF(t.speed_walk, 0), 2.5), COALESCE(NULLIF(t.speed_run, 0), 7.0),
		COALESCE(t.faction, 0), COALESCE(t.maxlevel, 1), COALESCE(c.curhealth, 100)
		FROM creature AS c
		JOIN creature_template AS t ON t.entry = c.id
		WHERE c.map = ? AND c.position_x BETWEEN ? AND ? AND c.position_y BETWEEN ? AND ? AND (c.phaseMask = 0 OR (c.phaseMask & 1) <> 0)`
	seenCreatures := make(map[uint32]struct{})
	for _, p := range players {
		rows, err := s.WorldStore.DB.QueryContext(ctx, query, p.Map, float64(p.X)-distance, float64(p.X)+distance, float64(p.Y)-distance, float64(p.Y)+distance)
		if err != nil {
			continue
		}
		for rows.Next() {
			var guid, entry, moveType, faction, level, curHealth int64
			var x, y, z, wander, walkSpeed, runSpeed float64
			if err := rows.Scan(&guid, &entry, &x, &y, &z, &moveType, &wander, &walkSpeed, &runSpeed, &faction, &level, &curHealth); err != nil {
				continue
			}
			if _, dup := seenCreatures[uint32(guid)]; dup {
				continue
			}
			seenCreatures[uint32(guid)] = struct{}{}
			if curHealth <= 0 {
				continue
			}
			motion := s.motionFor(ctx, uint32(guid), uint32(entry), p.Map, float32(x), float32(y), float32(z), uint32(moveType), wander, float32(walkSpeed))
			motion.Faction = uint32(faction)
			motion.Level = uint32(level)
			if runSpeed > 0 {
				motion.RunSpeed = float32(runSpeed)
			}
			s.stepCreatureMotion(ctx, motion, players, now)
		}
		rows.Close()
	}
}

// stepCreatureMotion advances one creature: handles combat pursuit/attacks,
// finishes in-flight moves, honors waypoint delays, or wanders randomly.
func (s *Server) stepCreatureMotion(ctx context.Context, motion *creatureMotion, players []playerPos, now time.Time) {
	// 1. If currently in combat with a target:
	if motion.InCombat && motion.TargetGUID != 0 {
		var target *playerPos
		for i := range players {
			if players[i].GUID == motion.TargetGUID && players[i].Map == motion.Map {
				target = &players[i]
				break
			}
		}
		// If target left map, logged out, dead, or turned on GM mode: drop combat
		if target == nil || target.IsDead || target.IsGM {
			motion.InCombat = false
			motion.TargetGUID = 0
			motion.Moving = false
			return
		}
		dist := float32(math.Hypot(float64(target.X-motion.X), float64(target.Y-motion.Y)))
		if dist > 45.0 {
			// Evade / drop combat if player ran too far
			motion.InCombat = false
			motion.TargetGUID = 0
			motion.Moving = false
			return
		}
		if dist > 3.0 {
			// Pursue player: move towards target at run speed
			if !motion.Moving || now.After(motion.MoveEnds) {
				duration := uint32((dist / motion.RunSpeed) * 1000)
				if duration < 300 {
					duration = 300
				}
				s.broadcastMonsterMove(motion.Map, motion.GUID, motion.X, motion.Y, motion.Z, target.X, target.Y, target.Z, duration)
				motion.X, motion.Y, motion.Z = target.X, target.Y, target.Z
				motion.Moving = true
				motion.MoveEnds = now.Add(time.Duration(duration) * time.Millisecond)
			}
			return
		}
		// In melee range (<= 3.0 yards): attack player
		motion.Moving = false
		if now.Sub(motion.LastAttack) >= 2*time.Second {
			damage := uint32(10 + int(motion.Level)*2)
			overkill := uint32(0)
			if damage >= target.Sess.player.Health {
				overkill = damage - target.Sess.player.Health
				target.Sess.player.Health = 0
			} else {
				target.Sess.player.Health -= damage
			}
			_ = target.Sess.write(uint16(protocol.OpcodeSMSG_ATTACKERSTATEUPDATE), buildAttackerStateUpdate(motion.GUID, target.GUID, damage, overkill), true)
			target.Sess.sendPlayerUpdate()
			motion.LastAttack = now
		}
		return
	}

	// 2. Check for nearby hostile aggro
	for _, p := range players {
		if p.Map != motion.Map || p.IsGM || p.IsDead {
			continue
		}
		dist := float32(math.Hypot(float64(p.X-motion.X), float64(p.Y-motion.Y)))
		aggroDist := float32(15.0)
		if isHostileFaction(motion.Faction, p.Race) && dist <= aggroDist {
			motion.InCombat = true
			motion.TargetGUID = p.GUID
			motion.Moving = false
			_ = p.Sess.write(uint16(protocol.OpcodeSMSG_ATTACK_START), buildAttackStart(motion.GUID, p.GUID), true)
			return
		}
	}

	// 3. Normal wandering or waypoint patrolling
	if motion.Moving {
		if now.Before(motion.MoveEnds) {
			return
		}
		motion.Moving = false
		motion.WaitUntil = motion.MoveEnds
	}
	if now.Before(motion.WaitUntil) {
		return
	}
	if motion.MoveType == 2 && len(motion.Points) == 0 {
		return
	}
	var destX, destY, destZ float32
	var speed float32
	var wait time.Duration
	if motion.MoveType == 2 {
		point := motion.Points[motion.NextIdx]
		destX, destY, destZ = point.X, point.Y, point.Z
		speed = motion.Speed * 2.0 // WaypointMovementGenerator defaults to run speed
		if point.Delay > 0 {
			wait = time.Duration(point.Delay) * time.Second
		}
		motion.NextIdx = (motion.NextIdx + 1) % len(motion.Points)
	} else {
		angle := rand.Float64() * 2 * math.Pi
		dist := rand.Float64() * motion.Wander
		destX = float32(float64(motion.HomeX) + dist*math.Cos(angle))
		destY = float32(float64(motion.HomeY) + dist*math.Sin(angle))
		destZ = motion.HomeZ
		speed = motion.Speed
		wait = time.Duration(1+rand.Intn(9)) * time.Second
	}
	moveDist := math.Hypot(float64(destX-motion.X), float64(destY-motion.Y))
	if moveDist < 0.5 {
		return
	}
	duration := uint32((moveDist / float64(speed)) * 1000)
	if duration < 250 {
		duration = 250
	}
	s.broadcastMonsterMove(motion.Map, motion.GUID, motion.X, motion.Y, motion.Z, destX, destY, destZ, duration)
	motion.X, motion.Y, motion.Z = destX, destY, destZ
	motion.Moving = true
	motion.MoveEnds = now.Add(time.Duration(duration) * time.Millisecond)
	motion.WaitUntil = motion.MoveEnds.Add(wait)
}

func isHostileFaction(creatureFaction uint32, playerRace uint8) bool {
	if creatureFaction == 0 || creatureFaction == 35 || creatureFaction == 7 || creatureFaction == 8 || creatureFaction == 114 || creatureFaction == 120 || creatureFaction == 534 {
		return false
	}
	switch creatureFaction {
	case 14, 16, 17, 38, 48, 91, 100, 101, 102, 103, 104, 105, 106, 117, 168, 188, 189, 214, 254:
		return true
	}
	isAlliance := isAllianceRace(playerRace)
	switch creatureFaction {
	case 1, 3, 4, 11, 12, 55, 57, 72, 115:
		return !isAlliance
	case 2, 5, 6, 8, 10, 29, 67, 68, 76, 116:
		return isAlliance
	}
	return false
}

func isAllianceRace(race uint8) bool {
	switch race {
	case 1, 3, 4, 7, 11:
		return true
	default:
		return false
	}
}

func (s *Server) pruneCreatureMotion(now time.Time) {
	s.motionMu.Lock()
	defer s.motionMu.Unlock()
	for key, motion := range s.creatureMotion {
		if now.Sub(motion.Refreshed) > motionTTL {
			delete(s.creatureMotion, key)
		}
	}
}


