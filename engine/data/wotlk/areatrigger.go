package wotlk

import (
	"math"
)

// AreaTriggerEntry mirrors TrinityCore AreaTriggerEntry (DBCStructure.h:244-254).
// DBC format: "niffffffff"
type AreaTriggerEntry struct {
	ID          uint32  // 0
	ContinentID uint32  // 1 (Map ID)
	X           float32 // 2
	Y           float32 // 3
	Z           float32 // 4
	Radius      float32 // 5
	BoxLength   float32 // 6
	BoxWidth    float32 // 7
	BoxHeight   float32 // 8
	BoxYaw      float32 // 9
}

// AreaTrigger decodes a record from AreaTrigger.dbc by ID.
func (s *Store) AreaTrigger(id uint32) (AreaTriggerEntry, bool, error) {
	file, err := s.File("AreaTrigger")
	if err != nil {
		return AreaTriggerEntry{}, false, err
	}
	record, ok := file.Find(id)
	if !ok {
		return AreaTriggerEntry{}, false, nil
	}

	entryID, _ := record.Uint32(0)
	continentID, _ := record.Uint32(1)
	x, _ := record.Float32(2)
	y, _ := record.Float32(3)
	z, _ := record.Float32(4)
	radius, _ := record.Float32(5)
	boxLength, _ := record.Float32(6)
	boxWidth, _ := record.Float32(7)
	boxHeight, _ := record.Float32(8)
	boxYaw, _ := record.Float32(9)

	return AreaTriggerEntry{
		ID:          entryID,
		ContinentID: continentID,
		X:           x,
		Y:           y,
		Z:           z,
		Radius:      radius,
		BoxLength:   boxLength,
		BoxWidth:    boxWidth,
		BoxHeight:   boxHeight,
		BoxYaw:      boxYaw,
	}, true, nil
}

// IsInAreaTriggerRadius tests whether position (px, py, pz) on mapID is inside the trigger volume.
// Reference: Player::IsInAreaTriggerRadius (Player.cpp:2417-2437) and Position::IsWithinBox (Position.cpp:86-112).
func (at *AreaTriggerEntry) IsInAreaTriggerRadius(mapID uint32, px, py, pz float32) bool {
	if at == nil || mapID != at.ContinentID {
		return false
	}

	if at.Radius > 0.0 {
		// Spherical trigger check
		dx := px - at.X
		dy := py - at.Y
		dz := pz - at.Z
		distSq := dx*dx + dy*dy + dz*dz
		return distSq <= at.Radius*at.Radius
	}

	// Oriented bounding box check
	// TrinityCore: rotate player point around center by (2*pi - BoxYaw)
	rotation := 2.0*math.Pi - float64(at.BoxYaw)
	sinVal := math.Sin(rotation)
	cosVal := math.Cos(rotation)

	boxDistX := float64(px - at.X)
	boxDistY := float64(py - at.Y)

	rotX := float32(float64(at.X) + boxDistX*cosVal - boxDistY*sinVal)
	rotY := float32(float64(at.Y) + boxDistY*cosVal + boxDistX*sinVal)

	dx := float32(math.Abs(float64(rotX - at.X)))
	dy := float32(math.Abs(float64(rotY - at.Y)))
	dz := float32(math.Abs(float64(pz - at.Z)))

	xradius := at.BoxLength / 2.0
	yradius := at.BoxWidth / 2.0
	zradius := at.BoxHeight / 2.0

	return dx <= xradius && dy <= yradius && dz <= zradius
}
