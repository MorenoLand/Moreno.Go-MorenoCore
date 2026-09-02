package wotlk

import (
	"encoding/binary"
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

