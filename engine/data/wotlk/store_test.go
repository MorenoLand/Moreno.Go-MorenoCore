package wotlk

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestPlayableRaceAndMountSpeedRules(t *testing.T) {
	if !IsPlayableRace(Race{Alliance: 0}) || IsPlayableRace(Race{Alliance: 2}) || IsPlayableRace(Race{Flags: 1}) {
		t.Fatal("playable race flags are incorrect")
	}
	spell := Spell{Effects: [3]SpellEffect{{BasePoints: 309, Aura: MountedFlightSpeedAura}, {BasePoints: 279, Aura: MountedFlightSpeedAura}, {BasePoints: 149}}}
	if !HasMountedFlightSpeed(spell, 310) || !HasMountedFlightSpeed(spell, 280) || HasMountedFlightSpeed(spell, 150) {
		t.Fatal("mount speed detection is incorrect")
	}
}

func TestAreaTableLoading(t *testing.T) {
	dbcDir := t.TempDir()
	const fieldCount = 36
	record := make([]uint32, fieldCount)
	record[0] = 4197
	record[1] = 571
	record[2] = 0
	record[3] = 12
	record[4] = AreaFlagWintergrasp2
	recordBytes := make([]byte, fieldCount*4)
	for i, val := range record {
		binary.LittleEndian.PutUint32(recordBytes[i*4:(i+1)*4], val)
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 1)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dbcDir, "AreaTable.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dbcDir)
	area, found, err := store.Area(4197)
	if err != nil || !found {
		t.Fatalf("expected area found, err: %v", err)
	}
	if area.ContinentID != 571 || area.Flags&AreaFlagWintergrasp2 == 0 {
		t.Fatalf("unexpected area: %+v", area)
	}
	_, found2, err := store.Area(9999)
	if err != nil || found2 {
		t.Fatalf("unexpected non-existent area found: %v", found2)
	}
}

func TestTalentLoading(t *testing.T) {
	dbcDir := t.TempDir()
	const fieldCount = 23
	record := make([]uint32, fieldCount)
	record[0] = 123
	record[1] = 10
	record[2] = 1
	record[3] = 2
	record[4] = 1001
	record[5] = 1002
	record[6] = 1003
	record[7] = 1004
	record[8] = 1005
	record[13] = 50
	record[16] = 3

	recordBytes := make([]byte, fieldCount*4)
	for i, val := range record {
		binary.LittleEndian.PutUint32(recordBytes[i*4:(i+1)*4], val)
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 1)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dbcDir, "Talent.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dbcDir)
	tal, found, err := store.Talent(123)
	if err != nil || !found {
		t.Fatalf("expected talent found, err: %v", err)
	}
	if tal.TabID != 10 || tal.TierID != 1 || tal.ColumnIndex != 2 || tal.SpellRank[0] != 1001 || tal.SpellRank[4] != 1005 || tal.PrereqTalent != 50 || tal.PrereqRank != 3 {
		t.Fatalf("unexpected talent: %+v", tal)
	}
	_, found2, err := store.Talent(9999)
	if err != nil || found2 {
		t.Fatalf("unexpected non-existent talent found: %v", found2)
	}
}

func TestMapLoading(t *testing.T) {
	dbcDir := t.TempDir()
	const fieldCount = 66
	record := make([]uint32, fieldCount)
	record[0] = 33 // Shadowfang Keep
	record[2] = 1  // InstanceType = MAP_INSTANCE
	record[59] = 0 // CorpseMapID = Eastern Kingdoms (0)

	recordBytes := make([]byte, fieldCount*4)
	for i, val := range record {
		binary.LittleEndian.PutUint32(recordBytes[i*4:(i+1)*4], val)
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 1)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dbcDir, "Map.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dbcDir)
	m, found, err := store.Map(33)
	if err != nil || !found {
		t.Fatalf("expected map 33 found, err: %v", err)
	}
	if !m.IsDungeon() || !m.IsNonRaidDungeon() || m.IsRaid() {
		t.Fatalf("expected dungeon map 33, got %+v", m)
	}
	if m.CorpseMapID != 0 {
		t.Fatalf("expected corpseMapID 0, got %d", m.CorpseMapID)
	}
}

func TestVehicleAndVehicleSeatLoading(t *testing.T) {
	dbcDir := t.TempDir()

	// 1. Vehicle.dbc (40 fields)
	const vehFieldCount = 40
	vehRec := make([]uint32, vehFieldCount)
	vehRec[0] = 335 // Salvaged Chopper
	vehRec[1] = VehicleFlagNoStrafe | VehicleFlagFullSpeedTurning
	vehRec[6] = 3005 // seat 0
	vehRec[7] = 3004 // seat 1

	vehBytes := make([]byte, vehFieldCount*4)
	for i, val := range vehRec {
		binary.LittleEndian.PutUint32(vehBytes[i*4:(i+1)*4], val)
	}
	vehHeader := make([]byte, 20)
	copy(vehHeader, "WDBC")
	binary.LittleEndian.PutUint32(vehHeader[4:8], 1)
	binary.LittleEndian.PutUint32(vehHeader[8:12], vehFieldCount)
	binary.LittleEndian.PutUint32(vehHeader[12:16], vehFieldCount*4)
	binary.LittleEndian.PutUint32(vehHeader[16:20], 1)
	if err := os.WriteFile(filepath.Join(dbcDir, "Vehicle.dbc"), append(vehHeader, append(vehBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. VehicleSeat.dbc (58 fields)
	const seatFieldCount = 58
	seatRec := make([]uint32, seatFieldCount)
	seatRec[0] = 3005
	seatRec[1] = VehicleSeatFlagCanControl | VehicleSeatFlagCanEnterOrExit | VehicleSeatFlagCanSwitch
	seatRec[45] = VehicleSeatFlagBEjectable

	seatBytes := make([]byte, seatFieldCount*4)
	for i, val := range seatRec {
		binary.LittleEndian.PutUint32(seatBytes[i*4:(i+1)*4], val)
	}
	seatHeader := make([]byte, 20)
	copy(seatHeader, "WDBC")
	binary.LittleEndian.PutUint32(seatHeader[4:8], 1)
	binary.LittleEndian.PutUint32(seatHeader[8:12], seatFieldCount)
	binary.LittleEndian.PutUint32(seatHeader[12:16], seatFieldCount*4)
	binary.LittleEndian.PutUint32(seatHeader[16:20], 1)
	if err := os.WriteFile(filepath.Join(dbcDir, "VehicleSeat.dbc"), append(seatHeader, append(seatBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dbcDir)
	veh, found, err := store.Vehicle(335)
	if err != nil || !found {
		t.Fatalf("expected vehicle 335 found, err: %v", err)
	}
	if veh.ID != 335 || veh.SeatIDs[0] != 3005 || veh.SeatIDs[1] != 3004 {
		t.Fatalf("unexpected vehicle 335 data: %+v", veh)
	}

	seat, found, err := store.VehicleSeat(3005)
	if err != nil || !found {
		t.Fatalf("expected seat 3005 found, err: %v", err)
	}
	if !seat.CanControl() || !seat.CanEnterOrExit() || !seat.CanSwitchFromSeat() || !seat.IsEjectable() {
		t.Fatalf("unexpected seat 3005 flags: %+v", seat)
	}
}

func TestAreaTriggerLoadingAndRadiusCheck(t *testing.T) {
	dbcDir := t.TempDir()
	const fieldCount = 10

	// 1. Spherical trigger (ID 100, Map 0, X 100, Y 200, Z 30, Radius 15)
	rec1 := make([]uint32, fieldCount)
	rec1[0] = 100
	rec1[1] = 0
	rec1Bytes := make([]byte, fieldCount*4)
	binary.LittleEndian.PutUint32(rec1Bytes[0:4], 100)
	binary.LittleEndian.PutUint32(rec1Bytes[4:8], 0)
	binary.LittleEndian.PutUint32(rec1Bytes[8:12], math.Float32bits(100.0))
	binary.LittleEndian.PutUint32(rec1Bytes[12:16], math.Float32bits(200.0))
	binary.LittleEndian.PutUint32(rec1Bytes[16:20], math.Float32bits(30.0))
	binary.LittleEndian.PutUint32(rec1Bytes[20:24], math.Float32bits(15.0)) // Radius 15.0

	// 2. Box trigger (ID 200, Map 1, X 500, Y 600, Z 50, Radius 0, BoxLength 20, BoxWidth 30, BoxHeight 10, BoxYaw 0)
	rec2Bytes := make([]byte, fieldCount*4)
	binary.LittleEndian.PutUint32(rec2Bytes[0:4], 200)
	binary.LittleEndian.PutUint32(rec2Bytes[4:8], 1)
	binary.LittleEndian.PutUint32(rec2Bytes[8:12], math.Float32bits(500.0))
	binary.LittleEndian.PutUint32(rec2Bytes[12:16], math.Float32bits(600.0))
	binary.LittleEndian.PutUint32(rec2Bytes[16:20], math.Float32bits(50.0))
	binary.LittleEndian.PutUint32(rec2Bytes[20:24], math.Float32bits(0.0))  // Radius 0
	binary.LittleEndian.PutUint32(rec2Bytes[24:28], math.Float32bits(20.0)) // BoxLength
	binary.LittleEndian.PutUint32(rec2Bytes[28:32], math.Float32bits(30.0)) // BoxWidth
	binary.LittleEndian.PutUint32(rec2Bytes[32:36], math.Float32bits(10.0)) // BoxHeight
	binary.LittleEndian.PutUint32(rec2Bytes[36:40], math.Float32bits(0.0))  // BoxYaw

	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 2)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)

	allBytes := append(header, append(append(rec1Bytes, rec2Bytes...), 0)...)
	if err := os.WriteFile(filepath.Join(dbcDir, "AreaTrigger.dbc"), allBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dbcDir)

	// Test record 100 (spherical)
	at1, found, err := store.AreaTrigger(100)
	if err != nil || !found {
		t.Fatalf("expected trigger 100 found, err: %v", err)
	}
	if at1.ID != 100 || at1.ContinentID != 0 || at1.Radius != 15.0 {
		t.Fatalf("unexpected trigger 100 data: %+v", at1)
	}
	// Inside radius (dist ~10 <= 15)
	if !at1.IsInAreaTriggerRadius(0, 100.0, 210.0, 30.0) {
		t.Fatal("expected position inside sphere radius")
	}
	// Outside radius (dist 20 > 15)
	if at1.IsInAreaTriggerRadius(0, 100.0, 220.0, 30.0) {
		t.Fatal("expected position outside sphere radius")
	}
	// Wrong map
	if at1.IsInAreaTriggerRadius(1, 100.0, 205.0, 30.0) {
		t.Fatal("expected position on wrong map to be rejected")
	}

	// Test record 200 (box)
	at2, found, err := store.AreaTrigger(200)
	if err != nil || !found {
		t.Fatalf("expected trigger 200 found, err: %v", err)
	}
	// Inside box: center 500,600,50, dimensions 20x30x10 (half bounds: 10, 15, 5)
	if !at2.IsInAreaTriggerRadius(1, 505.0, 610.0, 52.0) {
		t.Fatal("expected position inside box bounds")
	}
	// Outside X bound (dx = 12 > 10)
	if at2.IsInAreaTriggerRadius(1, 512.0, 600.0, 50.0) {
		t.Fatal("expected position outside box X bound")
	}
	// Outside Y bound (dy = 18 > 15)
	if at2.IsInAreaTriggerRadius(1, 500.0, 618.0, 50.0) {
		t.Fatal("expected position outside box Y bound")
	}
	// Outside Z bound (dz = 7 > 5)
	if at2.IsInAreaTriggerRadius(1, 500.0, 600.0, 57.0) {
		t.Fatal("expected position outside box Z bound")
	}
}



