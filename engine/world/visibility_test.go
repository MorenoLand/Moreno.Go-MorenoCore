package world

import (
	"context"
	"database/sql"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func TestDestroyHiddenCreaturesInRange(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE creature (guid INTEGER, id INTEGER, map INTEGER, position_x REAL, position_y REAL, phaseMask INTEGER)",
		"CREATE TABLE creature_template (entry INTEGER, npcflag INTEGER, flags_extra INTEGER)",
		"INSERT INTO creature VALUES (7, 68, 0, 10, 20, 0)",
		"INSERT INTO creature_template VALUES (68, 0, 1024)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	sess := &session{server: &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}, Config: config.Default()}, conn: serverConn, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Map: 0, X: 10, Y: 20, Z: 0}}
	done := make(chan struct{})
	go func() {
		sess.destroyHiddenCreaturesInRange(context.Background(), *sess.player)
		close(done)
	}()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_DESTROY_OBJECT) || len(payload) != 9 {
		t.Fatalf("opcode=%x payload=%x", opcode, payload)
	}
	reader := protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil || guid != creatureWorldGUID(7, 68) {
		t.Fatalf("guid=%x err=%v", guid, err)
	}
}
