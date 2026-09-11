package world

import (
	"context"
	"database/sql"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func TestContinentTransportLoadsRouteAndMoves(t *testing.T) {
	dbcDir := t.TempDir()
	const fieldCount = 9
	records := make([]byte, fieldCount*4*2)
	writeNode := func(index int, id, path, node, mapID uint32, x, y float32) {
		offset := index * fieldCount * 4
		values := make([]uint32, fieldCount)
		values[0], values[1], values[2], values[3] = id, path, node, mapID
		values[4], values[5] = math.Float32bits(x), math.Float32bits(y)
		for field, value := range values {
			binary.LittleEndian.PutUint32(records[offset+field*4:], value)
		}
	}
	writeNode(0, 1, 10, 0, 0, 0, 0)
	writeNode(1, 2, 10, 1, 0, 10, 0)
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 2)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dbcDir, "TaxiPathNode.dbc"), append(header, append(records, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		`CREATE TABLE transports (guid INTEGER PRIMARY KEY, entry INTEGER NOT NULL, name TEXT NOT NULL DEFAULT '', ScriptName TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE gameobject_template (entry INTEGER PRIMARY KEY, type INTEGER NOT NULL, name TEXT NOT NULL, data0 INTEGER NOT NULL, data1 INTEGER NOT NULL, displayId INTEGER NOT NULL, size REAL NOT NULL)`,
		`INSERT INTO transports VALUES (7, 9000, 'Test Ferry', '')`,
		`INSERT INTO gameobject_template VALUES (9000, 15, 'Test Ferry', 10, 10, 1234, 1)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}, Data: wotlk.NewStore(dbcDir), Config: config.Default(), transports: make(map[uint32]*continentTransport), sessions: make(map[*session]struct{})}
	server.loadContinentTransports(context.Background())
	transport := server.transports[7]
	if transport == nil || len(transport.Points) != 2 || transport.Spawn.Type != GameObjectTypeMOTransport {
		t.Fatalf("transport=%+v", transport)
	}
	server.updateContinentTransports(transport.LastUpdate.Add(500 * time.Millisecond))
	if transport.Spawn.X < 4.9 || transport.Spawn.X > 5.1 {
		t.Fatalf("transport x=%v, want midpoint", transport.Spawn.X)
	}
	create := buildGameObjectUpdate(transport.Spawn)
	reader := protocol.NewReader(create)
	updateType, err := reader.ReadU8()
	if err != nil || updateType != protocol.UpdateCreateObject2 {
		t.Fatalf("create update type=%d err=%v", updateType, err)
	}
	if _, err := reader.ReadPackedGUID(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	flags, err := reader.ReadU16()
	if err != nil || flags != transportGameObjectUpdateFlags {
		t.Fatalf("transport flags=%x err=%v", flags, err)
	}
	movement := buildGameObjectMovementUpdate(transport.Spawn)
	if len(movement) == 0 || movement[0] != protocol.UpdateMovement {
		t.Fatalf("movement update=%x", movement)
	}
}
