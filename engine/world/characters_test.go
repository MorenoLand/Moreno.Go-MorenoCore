package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestCharacterCreatePersistsReferenceDefaults(t *testing.T) {
	root, err := packageRoot()
	if err != nil {
		t.Fatal(err)
	}
	stores := makeMemoryStores(t, root)
	if _, err := stores.World.DB.Exec("INSERT INTO playercreateinfo (race, class, map, zone, position_x, position_y, position_z, orientation) VALUES (1, 1, 0, 12, 1.5, 2.5, 3.5, 0.5)"); err != nil {
		t.Fatal(err)
	}
	if _, err := stores.World.DB.Exec("INSERT INTO player_classlevelstats (class, level, basehp, basemana) VALUES (1, 1, 20, 0)"); err != nil {
		t.Fatal(err)
	}
	state := &session{server: NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1), accountID: 7, legitimate: make(map[uint64]struct{})}
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	state.conn = serverConn
	payload := protocol.NewBuffer(16)
	payload.WriteCString("Newhero")
	for _, value := range []uint8{1, 1, 0, 0, 0, 0, 0, 0, 0} {
		payload.WriteU8(value)
	}
	done := make(chan bool, 1)
	go func() { done <- state.handleCharCreate(context.Background(), payload.Bytes()) }()
	opcode, response, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !<-done || opcode != uint16(protocol.OpcodeSMSG_CHAR_CREATE) || len(response) != 1 || response[0] != charCreateSuccess {
		t.Fatalf("opcode=%x response=%x", opcode, response)
	}
	var count int
	if err := stores.Characters.DB.QueryRow("SELECT COUNT(*) FROM characters WHERE account = 7 AND name = 'Newhero'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("characters=%d", count)
	}
	var health int
	if err := stores.Characters.DB.QueryRow("SELECT health FROM characters WHERE account = 7 AND name = 'Newhero'").Scan(&health); err != nil {
		t.Fatal(err)
	}
	if health != 20 {
		t.Fatalf("created character health=%d", health)
	}
	var realmCount int
	if err := stores.Auth.DB.QueryRow("SELECT numchars FROM realmcharacters WHERE acctid = 7 AND realmid = 1").Scan(&realmCount); err != nil {
		t.Fatal(err)
	}
	if realmCount != 1 {
		t.Fatalf("realm characters=%d", realmCount)
	}
}

func TestCompleteCinematicPersists(t *testing.T) {
	root, err := packageRoot()
	if err != nil {
		t.Fatal(err)
	}
	stores := makeMemoryStores(t, root)
	if _, err := stores.Characters.DB.Exec("INSERT INTO characters (guid, account, name, race, class, gender, level, map, position_x, position_y, position_z, orientation, taximask, cinematic) VALUES (9, 7, 'Cine', 1, 1, 0, 1, 0, 0, 0, 0, 0, '', 0)"); err != nil {
		t.Fatal(err)
	}
	server := NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	state := &session{server: server, accountID: 7, playerGUID: 9, playerLoaded: true, player: &playerState{GUID: 9}}
	if !state.handleCompleteCinematic(context.Background()) {
		t.Fatal("handleCompleteCinematic failed")
	}
	var cinematic int
	if err := stores.Characters.DB.QueryRow("SELECT cinematic FROM characters WHERE guid = 9").Scan(&cinematic); err != nil {
		t.Fatal(err)
	}
	if cinematic != 1 || state.player.Cinematic != 1 {
		t.Fatalf("cinematic=%d state=%d", cinematic, state.player.Cinematic)
	}
}

func TestTutorialFlags(t *testing.T) {
	root, err := packageRoot()
	if err != nil {
		t.Fatal(err)
	}
	stores := makeMemoryStores(t, root)
	server := NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	state := &session{server: server, accountID: 7, authed: true}

	state.loadTutorials(context.Background())
	if state.tutorials != [8]uint32{} {
		t.Fatalf("initial tutorials=%v", state.tutorials)
	}

	flagBuf := protocol.NewBuffer(4)
	flagBuf.WriteU32(22) // bit 22 of tutorial 0 (chat tutorial)
	if !state.handleTutorialFlag(context.Background(), flagBuf.Bytes()) {
		t.Fatal("handleTutorialFlag failed")
	}
	if state.tutorials[0] != (1 << 22) {
		t.Fatalf("tutorials[0]=%x", state.tutorials[0])
	}

	state.loadTutorials(context.Background())
	if state.tutorials[0] != (1 << 22) {
		t.Fatalf("persisted tutorials[0]=%x", state.tutorials[0])
	}

	if !state.handleTutorialClear(context.Background()) {
		t.Fatal("handleTutorialClear failed")
	}
	for i, v := range state.tutorials {
		if v != 0xFFFFFFFF {
			t.Fatalf("tutorials[%d]=%x", i, v)
		}
	}

	if !state.handleTutorialReset(context.Background()) {
		t.Fatal("handleTutorialReset failed")
	}
	for i, v := range state.tutorials {
		if v != 0 {
			t.Fatalf("tutorials[%d]=%x", i, v)
		}
	}
}

func TestLogoutSitRootStunAndCancel(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	srv := &Server{}
	sess := &session{server: srv, conn: serverConn, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, StandState: 0, UnitFlags: 0}}
	ctx := context.Background()

	// Handle logout request
	go func() {
		sess.handleLogoutRequest(ctx)
	}()

	// 1. Read SMSG_LOGOUT_RESPONSE
	op1, _, err := readServerFrame(clientConn, nil)
	if err != nil || op1 != uint16(protocol.OpcodeSMSG_LOGOUT_RESPONSE) {
		t.Fatalf("expected SMSG_LOGOUT_RESPONSE, got op=%x err=%v", op1, err)
	}
	// 2. Read SMSG_FORCE_MOVE_ROOT
	op2, _, err := readServerFrame(clientConn, nil)
	if err != nil || op2 != uint16(protocol.OpcodeSMSG_FORCE_MOVE_ROOT) {
		t.Fatalf("expected SMSG_FORCE_MOVE_ROOT, got op=%x err=%v", op2, err)
	}
	// 3. Read SMSG_UPDATE_OBJECT / SMSG_COMPRESSED_UPDATE_OBJECT
	op3, _, err := readServerFrame(clientConn, nil)
	if err != nil || (op3 != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && op3 != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT)) {
		t.Fatalf("expected update object, got op=%x err=%v", op3, err)
	}

	// Verify player state is sitting and stunned
	if sess.player.StandState != 1 {
		t.Fatalf("expected player StandState=1 (sit), got %d", sess.player.StandState)
	}
	if sess.player.UnitFlags&unitFlagStunned == 0 {
		t.Fatalf("expected player UnitFlags to have UNIT_FLAG_STUNNED (%x), got %x", unitFlagStunned, sess.player.UnitFlags)
	}

	// Handle logout cancel
	go func() {
		sess.handleLogoutCancel()
	}()

	// 4. Read SMSG_FORCE_MOVE_UNROOT
	op4, _, err := readServerFrame(clientConn, nil)
	if err != nil || op4 != uint16(protocol.OpcodeSMSG_FORCE_MOVE_UNROOT) {
		t.Fatalf("expected SMSG_FORCE_MOVE_UNROOT, got op=%x err=%v", op4, err)
	}
	// 5. Read update object
	op5, _, err := readServerFrame(clientConn, nil)
	if err != nil || (op5 != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && op5 != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT)) {
		t.Fatalf("expected update object, got op=%x err=%v", op5, err)
	}
	// 6. Read SMSG_LOGOUT_CANCEL_ACK
	op6, _, err := readServerFrame(clientConn, nil)
	if err != nil || op6 != uint16(protocol.OpcodeSMSG_LOGOUT_CANCEL_ACK) {
		t.Fatalf("expected SMSG_LOGOUT_CANCEL_ACK, got op=%x err=%v", op6, err)
	}

	// Verify player state restored: standing and not stunned
	if sess.player.StandState != 0 {
		t.Fatalf("expected player StandState=0 (stand), got %d", sess.player.StandState)
	}
	if sess.player.UnitFlags&unitFlagStunned != 0 {
		t.Fatalf("expected player UnitFlags to clear UNIT_FLAG_STUNNED, got %x", sess.player.UnitFlags)
	}
}

func makeMemoryStores(t *testing.T, root string) *database.Set {
	t.Helper()
	open := func(name string) *database.Store {
		db, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		db.SetMaxOpenConns(1)
		path := filepath.Join(root, "sql", "sqlite", name+".sql")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range database.SplitSQL(string(data)) {
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		return &database.Store{Name: name, Backend: database.BackendSQLite, DB: db}
	}
	return &database.Set{Auth: open("auth"), Characters: open("characters"), World: open("world")}
}

func packageRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..")), nil
}
