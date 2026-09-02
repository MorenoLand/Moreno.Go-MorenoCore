package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildNearbyGameObjectUpdates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE gameobject (guid INTEGER PRIMARY KEY, id INTEGER NOT NULL, map INTEGER NOT NULL, spawnMask INTEGER NOT NULL, phaseMask INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL, orientation REAL NOT NULL, rotation0 REAL NOT NULL, rotation1 REAL NOT NULL, rotation2 REAL NOT NULL, rotation3 REAL NOT NULL, state INTEGER NOT NULL, animprogress INTEGER NOT NULL)",
		"CREATE TABLE gameobject_template (entry INTEGER PRIMARY KEY, type INTEGER NOT NULL, displayId INTEGER NOT NULL, size REAL NOT NULL)",
		"CREATE TABLE gameobject_template_addon (entry INTEGER PRIMARY KEY, flags INTEGER NOT NULL, faction INTEGER NOT NULL, artkit0 INTEGER NOT NULL)",
		"CREATE TABLE gameobject_addon (guid INTEGER PRIMARY KEY, parent_rotation0 REAL NOT NULL, parent_rotation1 REAL NOT NULL, parent_rotation2 REAL NOT NULL, parent_rotation3 REAL NOT NULL)",
		"INSERT INTO gameobject VALUES (321, 9001, 0, 1, 1, -8978, 520, 68, 0.5, 0, 0, 0, 1, 0, 7)",
		"INSERT INTO gameobject_template VALUES (9001, 3, 1234, 1)",
		"INSERT INTO gameobject_template_addon VALUES (9001, 32, 35, 0)",
		"INSERT INTO gameobject_addon VALUES (321, 0, 0, 0, 1)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}, Config: config.Default()}
	packet, count, err := server.buildNearbyGameObjectUpdates(context.Background(), playerState{Map: 0, X: -8977.78, Y: 519.564, Z: 68})
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
	expectedGUID := uint64(321) | uint64(9001)<<24 | uint64(0xF110)<<48
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != expectedGUID {
		t.Fatalf("guid=%x err=%v", guid, err)
	}
	if objectType, err := reader.ReadU8(); err != nil || objectType != 5 {
		t.Fatalf("object type=%d err=%v", objectType, err)
	}
	if flags, err := reader.ReadU16(); err != nil || flags != gameObjectUpdateFlags {
		t.Fatalf("update flags=%x err=%v", flags, err)
	}
	if transport, err := reader.ReadPackedGUID(); err != nil || transport != 0 {
		t.Fatalf("transport=%x err=%v", transport, err)
	}
	for index := 0; index < 8; index++ {
		if _, err := reader.ReadF32(); err != nil {
			t.Fatal(err)
		}
	}
	if lowGUID, err := reader.ReadU32(); err != nil || lowGUID != 321 {
		t.Fatalf("low guid=%d err=%v", lowGUID, err)
	}
	if _, err := reader.ReadU64(); err != nil {
		t.Fatal(err)
	}
	maskBlocks, err := reader.ReadU8()
	if err != nil || maskBlocks != 1 {
		t.Fatalf("mask blocks=%d err=%v", maskBlocks, err)
	}
	mask, err := reader.ReadU32()
	if err != nil {
		t.Fatal(err)
	}
	if mask&(1<<uint(gameObjectDynamic)) == 0 || mask&(1<<uint(gameObjectDisplayID)) == 0 {
		t.Fatalf("mask=%x", mask)
	}
	values := make(map[int]uint32)
	for index := 0; index < gameObjectValuesCount; index++ {
		if mask&(1<<uint(index)) == 0 {
			continue
		}
		value, err := reader.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
		values[index] = value
	}
	if values[3] != 9001 || values[gameObjectDisplayID] != 1234 || values[gameObjectDynamic] != 0xFFFF0000 {
		t.Fatalf("values=%x", values)
	}
}

func TestGameObjectUseHandlers(t *testing.T) {
	s := &session{playerLoaded: true, player: &playerState{GUID: 1}}
	buf := protocol.NewBuffer(8)
	buf.WriteU64(12345)
	if !s.handleGameObjectUse(context.Background(), buf.Bytes()) {
		t.Fatal("handleGameObjectUse failed")
	}
	if !s.handleGameObjectReportUse(context.Background(), buf.Bytes()) {
		t.Fatal("handleGameObjectReportUse failed")
	}
}

