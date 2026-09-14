package world

import (
	"math"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

const (
	dynamicObjectTypeMask uint32 = 0x0041
	dynamicObjectHighGUID uint64 = 0xF100
	dynamicObjectTypeArea uint8  = 1
	dynamicObjectFlags           = uint16(0x0150)
)

type dynamicSpellObjectState struct {
	GUID, CasterGUID, SpellID uint64
	Map                       uint32
	X, Y, Z, Orientation      float32
	Radius                    float32
	CastTime                  uint32
	DespawnTimer              *time.Timer
}

func dynamicSpellGUID(low uint32) uint64 { return (dynamicObjectHighGUID << 48) | uint64(low) }

func (s *Server) nextDynamicSpellLowGUID() uint32 {
	s.objectsMu.Lock()
	defer s.objectsMu.Unlock()
	s.nextDynamicSpellGUID++
	if s.nextDynamicSpellGUID == 0 {
		s.nextDynamicSpellGUID = 1
	}
	return s.nextDynamicSpellGUID
}

func buildDynamicSpellObjectUpdate(object *dynamicSpellObjectState) []byte {
	values := make([]uint32, 12)
	values[0] = uint32(object.GUID)
	values[1] = uint32(object.GUID >> 32)
	values[2] = dynamicObjectTypeMask
	values[3] = uint32(object.SpellID)
	values[4] = math.Float32bits(1)
	values[6] = uint32(object.CasterGUID)
	values[7] = uint32(object.CasterGUID >> 32)
	values[8] = uint32(dynamicObjectTypeArea)
	values[9] = uint32(object.SpellID)
	values[10] = math.Float32bits(object.Radius)
	values[11] = object.CastTime
	mask := protocol.NewUpdateMask(len(values))
	for index, value := range values {
		if value != 0 {
			_ = mask.Set(index)
		}
	}
	block := protocol.NewBuffer(128)
	block.WriteU8(protocol.UpdateCreateObject2)
	block.WritePackedGUID(object.GUID)
	block.WriteU8(6)
	block.WriteU16(dynamicObjectFlags)
	block.WriteU8(0)
	block.WriteF32(object.X)
	block.WriteF32(object.Y)
	block.WriteF32(object.Z)
	block.WriteF32(object.Orientation)
	block.WriteU32(uint32(object.GUID))
	block.WriteU8(uint8(mask.BlockCount()))
	mask.AppendTo(block)
	for index, value := range values {
		if mask.Has(index) {
			block.WriteU32(value)
		}
	}
	return block.Bytes()
}

func (s *Server) spawnDynamicSpellObject(object *dynamicSpellObjectState, duration time.Duration) {
	if s == nil || object == nil || duration <= 0 {
		return
	}
	s.objectsMu.Lock()
	if s.dynamicSpellObjects == nil {
		s.dynamicSpellObjects = make(map[uint64]*dynamicSpellObjectState)
	}
	s.dynamicSpellObjects[object.GUID] = object
	object.DespawnTimer = time.AfterFunc(duration, func() { s.despawnDynamicSpellObject(object.GUID) })
	s.objectsMu.Unlock()
	updates := protocol.NewUpdateData()
	updates.AddUpdateBlock(buildDynamicSpellObjectUpdate(object))
	if packet, err := updates.BuildPacket(0); err == nil && packet != nil {
		s.broadcastToMap(object.Map, packet.Opcode, packet.Payload.Bytes())
	}
}

func (s *Server) despawnDynamicSpellObject(guid uint64) {
	if s == nil || guid == 0 {
		return
	}
	s.objectsMu.Lock()
	object, ok := s.dynamicSpellObjects[guid]
	if ok && object != nil && object.DespawnTimer != nil {
		object.DespawnTimer.Stop()
	}
	if ok {
		delete(s.dynamicSpellObjects, guid)
	}
	var mapID uint32
	if object != nil {
		mapID = object.Map
	}
	s.objectsMu.Unlock()
	if !ok || object == nil {
		return
	}
	packet := protocol.NewBuffer(9)
	packet.WriteU64(object.GUID)
	packet.WriteU8(0)
	s.broadcastToMap(mapID, uint16(protocol.OpcodeSMSG_DESTROY_OBJECT), packet.Bytes())
}
