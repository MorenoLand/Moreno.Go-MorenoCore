package world

import (
	"context"
	"database/sql"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestPetNameQueryAndRename(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE character_pet (
			id INTEGER PRIMARY KEY,
			entry INTEGER NOT NULL DEFAULT 0,
			owner INTEGER NOT NULL DEFAULT 0,
			modelid INTEGER DEFAULT 0,
			CreatedBySpell INTEGER NOT NULL DEFAULT 0,
			PetType INTEGER NOT NULL DEFAULT 0,
			level INTEGER NOT NULL DEFAULT 1,
			exp INTEGER NOT NULL DEFAULT 0,
			Reactstate INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT 'Pet',
			renamed INTEGER NOT NULL DEFAULT 0,
			slot INTEGER NOT NULL DEFAULT 0,
			curhealth INTEGER NOT NULL DEFAULT 1,
			curmana INTEGER NOT NULL DEFAULT 0,
			curhappiness INTEGER NOT NULL DEFAULT 0,
			savetime INTEGER NOT NULL DEFAULT 0,
			abdata TEXT
		)`,
		"INSERT INTO character_pet (id, entry, owner, name, renamed, savetime) VALUES (42, 100, 1, 'Fluffy', 0, 1000)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, Config: config.Default()}
	sess := &session{server: srv, conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// 1. Query pet name
	qBuf := protocol.NewBuffer(12)
	qBuf.WriteU32(42) // petNumber
	qBuf.WriteU64(1)  // petGUID

	go func() {
		sess.handlePetNameQuery(ctx, qBuf.Bytes())
	}()

	op, data, err := readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_PET_NAME_QUERY_RESPONSE) {
		t.Fatalf("expected SMSG_PET_NAME_QUERY_RESPONSE, got op=%x err=%v", op, err)
	}
	r := protocol.NewReader(data)
	num, _ := r.ReadU32()
	name, _ := r.ReadCString()
	if num != 42 || name != "Fluffy" {
		t.Fatalf("expected pet 42 named 'Fluffy', got %d '%s'", num, name)
	}

	// 2. Rename pet
	rBuf := protocol.NewBuffer(32)
	rBuf.WriteU64(42) // petGUID with ID 42
	rBuf.WriteCString("Rex")
	if !sess.handlePetRename(ctx, rBuf.Bytes()) {
		t.Fatal("handlePetRename failed")
	}

	// Verify name updated in DB
	var dbName string
	var renamed int
	_ = db.QueryRow("SELECT name, renamed FROM character_pet WHERE id = 42").Scan(&dbName, &renamed)
	if dbName != "Rex" || renamed != 1 {
		t.Fatalf("expected pet renamed to 'Rex' (renamed=1), got '%s' (%d)", dbName, renamed)
	}
}

func TestRequestPetInfoWithActivePet(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE character_pet (
			id INTEGER PRIMARY KEY,
			entry INTEGER NOT NULL DEFAULT 0,
			owner INTEGER NOT NULL DEFAULT 0,
			modelid INTEGER DEFAULT 0,
			CreatedBySpell INTEGER NOT NULL DEFAULT 0,
			PetType INTEGER NOT NULL DEFAULT 0,
			level INTEGER NOT NULL DEFAULT 1,
			exp INTEGER NOT NULL DEFAULT 0,
			Reactstate INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT 'Pet',
			renamed INTEGER NOT NULL DEFAULT 0,
			slot INTEGER NOT NULL DEFAULT 0,
			curhealth INTEGER NOT NULL DEFAULT 1,
			curmana INTEGER NOT NULL DEFAULT 0,
			curhappiness INTEGER NOT NULL DEFAULT 0,
			savetime INTEGER NOT NULL DEFAULT 0,
			abdata TEXT
		)`,
		`CREATE TABLE pet_spell (
			guid INTEGER NOT NULL,
			spell INTEGER NOT NULL,
			active INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (guid, spell)
		)`,
		"INSERT INTO character_pet (id, entry, owner, modelid, level, name, slot) VALUES (42, 100, 1, 50, 10, 'Fluffy', 0)",
		"INSERT INTO pet_spell (guid, spell, active) VALUES (42, 16827, 1)", // Claw, autocast
		"INSERT INTO pet_spell (guid, spell, active) VALUES (42, 17253, 0)", // Bite, manual
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, Config: config.Default()}
	sess := &session{server: srv, conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	go func() {
		sess.handleRequestPetInfo(ctx, nil)
	}()

	op, data, err := readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_PET_SPELLS) {
		t.Fatalf("expected SMSG_PET_SPELLS (0x%x), got op=0x%x err=%v", protocol.OpcodeSMSG_PET_SPELLS, op, err)
	}

	r := protocol.NewReader(data)
	petGUID, err := r.ReadU64()
	if err != nil {
		t.Fatalf("failed reading petGUID: %v", err)
	}
	expectedGUID := uint64(42) | (uint64(0xF140) << 48)
	if petGUID != expectedGUID {
		t.Fatalf("expected petGUID %x, got %x", expectedGUID, petGUID)
	}

	family, _ := r.ReadU16()
	duration, _ := r.ReadU32()
	react, _ := r.ReadU8()
	cmd, _ := r.ReadU8()
	flags, _ := r.ReadU16()

	if family != 0 || duration != 0 || react != 1 || cmd != 1 || flags != 0 {
		t.Fatalf("unexpected header values: family=%d dur=%d react=%d cmd=%d flags=%d", family, duration, react, cmd, flags)
	}

	// 10 action bar slots
	expectedSlots := []uint32{
		0x07000002,             // Attack
		0x07000001,             // Follow
		0x07000000,             // Stay
		16827 | (0xC1 << 24),   // Spell 16827 (autocast)
		17253 | (0x81 << 24),   // Spell 17253 (manual)
		0,                      // Empty
		0,                      // Empty
		0x06000002,             // Aggressive
		0x06000001,             // Defensive
		0x06000000,             // Passive
	}

	for i, exp := range expectedSlots {
		slotVal, _ := r.ReadU32()
		if slotVal != exp {
			t.Fatalf("slot %d: expected 0x%08X, got 0x%08X", i, exp, slotVal)
		}
	}
}

