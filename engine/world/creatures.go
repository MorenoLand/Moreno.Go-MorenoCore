package world

import (
	"context"
	"math"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	creatureTypeMask      uint32 = 0x00000009
	creatureUpdateFlags   uint16 = 0x0060
	creatureValuesCount          = 148
	unitFieldDynamicFlags        = 79
	unitFieldNPCFlags            = 82
)

type creatureSpawn struct {
	GUID         uint32
	Entry        uint32
	Map          uint32
	X            float32
	Y            float32
	Z            float32
	Orientation  float32
	Model        uint32
	Faction      uint32
	NPCFlags     uint32
	UnitFlags    uint32
	DynamicFlags uint32
	Level        uint32
	Health       uint32
	Mana         uint32
	Scale        float32
	WalkSpeed    float32
	RunSpeed     float32
	AttackTime   uint32
	RangedAttack uint32
}

func (s *Server) buildNearbyCreatureUpdates(ctx context.Context, state playerState) (*protocol.Packet, int, error) {
	distance := float64(s.Config.VisibilityDistanceContinents)
	if distance <= 0 {
		return nil, 0, nil
	}
	rows, err := s.WorldStore.DB.QueryContext(ctx, "SELECT c.guid, c.id, c.map, c.position_x, c.position_y, c.position_z, c.orientation, COALESCE(NULLIF(c.modelid, 0), t.modelid1), t.faction, t.npcflag, t.unit_flags, t.dynamicflags, t.maxlevel, c.curhealth, c.curmana, t.scale, t.speed_walk, t.speed_run, t.BaseAttackTime, t.RangeAttackTime FROM creature AS c JOIN creature_template AS t ON t.entry = c.id WHERE c.map = ? AND c.position_x BETWEEN ? AND ? AND c.position_y BETWEEN ? AND ? AND (c.phaseMask & 1) <> 0 ORDER BY c.guid", state.Map, float64(state.X)-distance, float64(state.X)+distance, float64(state.Y)-distance, float64(state.Y)+distance)
	if err != nil {
		if missingTable(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	defer rows.Close()
	updates := protocol.NewUpdateData()
	count := 0
	for rows.Next() {
		var guid, entry, mapID, model, faction, npcFlags, unitFlags, dynamicFlags, level, health, mana, attackTime, rangedAttack int64
		var x, y, z, orientation, scale, walkSpeed, runSpeed float64
		if err := rows.Scan(&guid, &entry, &mapID, &x, &y, &z, &orientation, &model, &faction, &npcFlags, &unitFlags, &dynamicFlags, &level, &health, &mana, &scale, &walkSpeed, &runSpeed, &attackTime, &rangedAttack); err != nil {
			return nil, count, err
		}
		if math.Hypot(x-float64(state.X), y-float64(state.Y)) > distance || !validMovementPosition(float32(x), float32(y), float32(z), float32(orientation)) {
			continue
		}
		spawn := creatureSpawn{GUID: uint32(guid), Entry: uint32(entry), Map: uint32(mapID), X: float32(x), Y: float32(y), Z: float32(z), Orientation: float32(orientation), Model: uint32(model), Faction: uint32(faction), NPCFlags: uint32(npcFlags), UnitFlags: uint32(unitFlags), DynamicFlags: uint32(dynamicFlags), Level: uint32(level), Health: uint32(health), Mana: uint32(mana), Scale: float32(scale), WalkSpeed: float32(walkSpeed), RunSpeed: float32(runSpeed), AttackTime: uint32(attackTime), RangedAttack: uint32(rangedAttack)}
		updates.AddUpdateBlock(buildCreatureUpdate(spawn))
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, count, err
	}
	if count == 0 {
		return nil, 0, nil
	}
	packet, err := updates.BuildPacket(0)
	return packet, count, err
}

func buildCreatureUpdate(spawn creatureSpawn) []byte {
	values := make([]uint32, creatureValuesCount)
	rawGUID := creatureWorldGUID(spawn.GUID, spawn.Entry)
	values[0] = uint32(rawGUID)
	values[1] = uint32(rawGUID >> 32)
	values[2] = creatureTypeMask
	values[objectFieldEntry] = spawn.Entry
	values[unitFieldHealth] = maxUint32(spawn.Health, 1)
	values[unitFieldLevel] = maxUint32(spawn.Level, 1)
	values[unitFieldFaction] = spawn.Faction
	values[unitFieldFlags] = spawn.UnitFlags
	values[unitFieldDynamicFlags] = spawn.DynamicFlags
	values[unitFieldNPCFlags] = spawn.NPCFlags
	values[unitFieldAttackTime] = maxUint32(spawn.AttackTime, 2000)
	values[unitFieldAttackTimeOffhand] = maxUint32(spawn.AttackTime, 2000)
	values[unitFieldBoundingRadius] = math.Float32bits(0.306349)
	values[unitFieldCombatReach] = math.Float32bits(1.5)
	values[unitFieldDisplayID] = spawn.Model
	values[unitFieldNativeDisplayID] = spawn.Model
	values[unitFieldMaxHealth] = maxUint32(spawn.Health, 1)
	values[unitFieldHealth+1] = spawn.Mana
	values[unitFieldMaxPower1] = spawn.Mana
	if spawn.Scale > 0 {
		values[objectFieldScale] = math.Float32bits(spawn.Scale)
	}
	mask := protocol.NewUpdateMask(len(values))
	for index, value := range values {
		if value != 0 {
			_ = mask.Set(index)
		}
	}
	block := protocol.NewBuffer(256)
	block.WriteU8(protocol.UpdateCreateObject2)
	block.WritePackedGUID(rawGUID)
	block.WriteU8(3)
	block.WriteU16(creatureUpdateFlags)
	block.WriteU32(0)
	block.WriteU16(0)
	block.WriteU32(uint32(time.Now().UnixMilli()))
	block.WriteF32(spawn.X)
	block.WriteF32(spawn.Y)
	block.WriteF32(spawn.Z)
	block.WriteF32(spawn.Orientation)
	block.WriteU32(0)
	for _, speed := range []float32{2.5 * spawn.WalkSpeed, 7 * spawn.RunSpeed, 4.5 * spawn.RunSpeed, 4.722222 * spawn.WalkSpeed, 2.5 * spawn.WalkSpeed, 7 * spawn.RunSpeed, 4.5 * spawn.RunSpeed, 3.141594, 3.14} {
		block.WriteF32(speed)
	}
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for index, value := range values {
		if mask.Has(index) {
			block.WriteU32(value)
		}
	}
	return block.Bytes()
}

func creatureWorldGUID(guid, entry uint32) uint64 {
	return uint64(guid) | uint64(entry)<<24 | uint64(0xF130)<<48
}

func maxUint32(value, fallback uint32) uint32 {
	if value == 0 {
		return fallback
	}
	return value
}
