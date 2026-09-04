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
		"INSERT INTO character_pet (id, entry, owner, modelid, level, name, slot, Reactstate) VALUES (42, 100, 1, 50, 10, 'Fluffy', 0, 1)",
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

func TestPetActionAndSetAction(t *testing.T) {
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
		"INSERT INTO character_pet (id, entry, owner, modelid, level, name, slot, Reactstate) VALUES (42, 100, 1, 50, 10, 'Fluffy', 0, 1)",
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

	// 1. Test Pet Attack Action emits SMSG_AI_REACTION
	petGUID := uint64(42) | (uint64(0xF140) << 48)
	targetGUID := uint64(12345)
	actBuf := protocol.NewBuffer(20)
	actBuf.WriteU64(petGUID)
	actBuf.WriteU32(0x07000002) // COMMAND_ATTACK
	actBuf.WriteU64(targetGUID)

	go func() {
		sess.handlePetAction(ctx, actBuf.Bytes())
	}()

	op, data, err := readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_AI_REACTION) {
		t.Fatalf("expected SMSG_AI_REACTION, got op=0x%x err=%v", op, err)
	}
	r := protocol.NewReader(data)
	g, _ := r.ReadU64()
	reactType, _ := r.ReadU32()
	if g != petGUID || reactType != 2 {
		t.Fatalf("expected ai reaction for pet %x type 2, got %x type %d", petGUID, g, reactType)
	}

	// 2. Test Pet Reaction Action updates Reactstate in DB
	reactBuf := protocol.NewBuffer(12)
	reactBuf.WriteU64(petGUID)
	reactBuf.WriteU32(0x06000002) // REACT_AGGRESSIVE = 2
	if !sess.handlePetAction(ctx, reactBuf.Bytes()) {
		t.Fatal("handlePetAction failed for reaction")
	}
	var newReact int
	_ = db.QueryRow("SELECT Reactstate FROM character_pet WHERE id = 42").Scan(&newReact)
	if newReact != 2 {
		t.Fatalf("expected Reactstate 2 in DB, got %d", newReact)
	}

	// 3. Test Pet Set Action saves abdata in DB
	setBuf := protocol.NewBuffer(16)
	setBuf.WriteU64(petGUID)
	setBuf.WriteU32(3)          // slot 3
	setBuf.WriteU32(0x07000000) // set slot 3 to Stay
	if !sess.handlePetSetAction(ctx, setBuf.Bytes()) {
		t.Fatal("handlePetSetAction failed")
	}

	var abdata string
	_ = db.QueryRow("SELECT abdata FROM character_pet WHERE id = 42").Scan(&abdata)
	if abdata == "" {
		t.Fatal("expected abdata to be saved in DB")
	}

	// 4. Test handleRequestPetInfo uses the saved abdata
	go func() {
		sess.handleRequestPetInfo(ctx, nil)
	}()

	op, data, err = readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_PET_SPELLS) {
		t.Fatalf("expected SMSG_PET_SPELLS, got op=0x%x err=%v", op, err)
	}
	r = protocol.NewReader(data)
	_, _ = r.ReadU64() // petGUID
	_, _ = r.ReadU16() // family
	_, _ = r.ReadU32() // duration
	react, _ := r.ReadU8()
	if react != 2 { // Should now be Aggressive (2)
		t.Fatalf("expected reactState 2 from DB, got %d", react)
	}
	_, _ = r.ReadU8()  // cmd
	_, _ = r.ReadU16() // flags

	// Read slots 0..3
	s0, _ := r.ReadU32()
	s1, _ := r.ReadU32()
	s2, _ := r.ReadU32()
	s3, _ := r.ReadU32()
	if s3 != 0x07000000 {
		t.Fatalf("expected slot 3 to be 0x07000000 (Stay), got 0x%08X (s0=%x s1=%x s2=%x)", s3, s0, s1, s2)
	}
}


