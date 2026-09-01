package world

import (
	"context"
	"math"
	"math/rand"
	"time"
)

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
	MoveType uint32  // 1 random, 2 waypoint
	Wander   float64

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
			MoveType: moveType,
			Wander:   wander,
		}
		if walkSpeed <= 0 {
			motion.Speed = 2.5
		}
		if moveType == 2 {
			motion.PathID = s.loadCreaturePathID(ctx, guid)
			motion.Points = s.loadWaypoints(ctx, motion.PathID)
		}
		s.creatureMotion[key] = motion
	}
	motion.Refreshed = time.Now()
	return motion
}

func (s *Server) loadCreaturePathID(ctx context.Context, guid uint32) uint32 {
	var pathID int64
	if err := s.WorldStore.DB.QueryRowContext(ctx, "SELECT COALESCE(NULLIF(path_id, 0), ?) FROM creature_addon WHERE guid = ?", guid, guid).Scan(&pathID); err != nil {
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

// updateActiveCreatures drives wander/patrol motion for creatures near
// online players, mirroring RandomMovementGenerator and
// WaypointMovementGenerator behaviour at a coarse tick.
func (s *Server) updateActiveCreatures(ctx context.Context) {
	if s.WorldStore == nil || s.WorldStore.DB == nil {
		return
	}
	type playerPos struct {
		Map uint32
		X   float32
		Y   float32
	}
	var players []playerPos
	s.sessionsMu.RLock()
	for sess := range s.sessions {
		if sess.playerLoaded && sess.player != nil {
			players = append(players, playerPos{Map: sess.player.Map, X: sess.player.X, Y: sess.player.Y})
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
		COALESCE(NULLIF(t.speed_walk, 0), 2.5)
		FROM creature AS c
		JOIN creature_template AS t ON t.entry = c.id
		WHERE c.map = ? AND c.MovementType IN (1, 2) AND c.position_x BETWEEN ? AND ? AND c.position_y BETWEEN ? AND ? AND (c.phaseMask = 0 OR (c.phaseMask & 1) <> 0)`
	seen := make(map[uint32]struct{})
	for _, p := range players {
		if _, dup := seen[p.Map]; dup {
			continue
		}
		seen[p.Map] = struct{}{}
		rows, err := s.WorldStore.DB.QueryContext(ctx, query, p.Map, float64(p.X)-distance, float64(p.X)+distance, float64(p.Y)-distance, float64(p.Y)+distance)
		if err != nil {
			continue
		}
		for rows.Next() {
			var guid, entry, moveType int64
			var x, y, z, wander, walkSpeed float64
			if err := rows.Scan(&guid, &entry, &x, &y, &z, &moveType, &wander, &walkSpeed); err != nil {
				continue
			}
			motion := s.motionFor(ctx, uint32(guid), uint32(entry), p.Map, float32(x), float32(y), float32(z), uint32(moveType), wander, float32(walkSpeed))
			s.stepCreatureMotion(ctx, motion, now)
		}
		rows.Close()
	}
}

// stepCreatureMotion advances one creature: finishes an in-flight move,
// honors waypoint delays, then plans the next broadcast move.
func (s *Server) stepCreatureMotion(ctx context.Context, motion *creatureMotion, now time.Time) {
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
		// RandomMovementGenerator: pick a point inside wander_distance of the
		// home position, pause between 1 and 10 seconds like the reference.
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

func (s *Server) pruneCreatureMotion(now time.Time) {
	s.motionMu.Lock()
	defer s.motionMu.Unlock()
	for key, motion := range s.creatureMotion {
		if now.Sub(motion.Refreshed) > motionTTL {
			delete(s.creatureMotion, key)
		}
	}
}


