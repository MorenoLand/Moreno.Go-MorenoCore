package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildNearbyCreatureUpdates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, map INTEGER NOT NULL, phaseMask INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL, orientation REAL NOT NULL, modelid INTEGER NOT NULL, npcflag INTEGER NOT NULL, unit_flags INTEGER NOT NULL, dynamicflags INTEGER NOT NULL, curhealth INTEGER NOT NULL, curmana INTEGER NOT NULL)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, modelid1 INTEGER NOT NULL, faction INTEGER NOT NULL, npcflag INTEGER NOT NULL, unit_flags INTEGER NOT NULL, dynamicflags INTEGER NOT NULL, maxlevel INTEGER NOT NULL, scale REAL NOT NULL, speed_walk REAL NOT NULL, speed_run REAL NOT NULL, BaseAttackTime INTEGER NOT NULL, RangeAttackTime INTEGER NOT NULL, flags_extra INTEGER NOT NULL DEFAULT 0)",
		"INSERT INTO creature_template VALUES (68, 3167, 11, 1, 32768, 0, 80, 1, 1, 1.14286, 2000, 2000, 0)",
		"INSERT INTO creature VALUES (79859, 68, 0, 1, -8958.4, 509.049, 96.5968, 0.70014, 0, 0, 0, 0, 100, 0)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}, Config: config.Default()}
	packet, count, err := server.buildNearbyCreatureUpdates(context.Background(), playerState{Map: 0, X: -8977.78, Y: 519.564, Z: 68})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || packet == nil {
		t.Fatalf("count=%d packet=%v", count, packet != nil)
	}
	payload := packet.Payload.Bytes()
	if packet.Opcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		payload, err = protocol.DecompressUpdatePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	reader := protocol.NewReader(payload)
	if blocks, err := reader.ReadU32(); err != nil || blocks != 1 {
		t.Fatalf("blocks=%d err=%v", blocks, err)
	}
	if updateType, err := reader.ReadU8(); err != nil || updateType != protocol.UpdateCreateObject2 {
		t.Fatalf("update type=%d err=%v", updateType, err)
	}
	expectedGUID := uint64(79859) | uint64(68)<<24 | uint64(0xF130)<<48
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != expectedGUID {
		t.Fatalf("guid=%x err=%v", guid, err)
	}
	if objectType, err := reader.ReadU8(); err != nil || objectType != 3 {
		t.Fatalf("object type=%d err=%v", objectType, err)
	}
	if flags, err := reader.ReadU16(); err != nil || flags != creatureUpdateFlags {
		t.Fatalf("update flags=%x err=%v", flags, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU16(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 4; index++ {
		if _, err := reader.ReadF32(); err != nil {
			t.Fatal(err)
		}
	}
	if fallTime, err := reader.ReadU32(); err != nil || fallTime != 0 {
		t.Fatalf("fall time=%d err=%v", fallTime, err)
	}
	for index := 0; index < 9; index++ {
		if _, err := reader.ReadF32(); err != nil {
			t.Fatal(err)
		}
	}
	if maskBlocks, err := reader.ReadU8(); err != nil || maskBlocks != 5 {
		t.Fatalf("mask blocks=%d err=%v", maskBlocks, err)
	}
	mask := make([]uint32, 5)
	for index := range mask {
		if mask[index], err = reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	values := make(map[int]uint32)
	for index := 0; index < creatureValuesCount; index++ {
		if mask[index/32]&(1<<uint(index%32)) == 0 {
			continue
		}
		value, readErr := reader.ReadU32()
		if readErr != nil {
			t.Fatal(readErr)
		}
		values[index] = value
	}
	if values[objectFieldEntry] != 68 {
		t.Fatalf("creature entry=%d", values[objectFieldEntry])
	}
}

