package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestItemCreateBlockIncludesPersistedState(t *testing.T) {
	randomPropertyID := int32(-12)
	state := itemUpdateState{
		Duration:         77,
		SpellCharges:     [5]uint32{1, 2, 3, 4, 5},
		Flags:            0x21,
		RandomPropertyID: uint32(randomPropertyID),
		Durability:       9,
		MaxDurability:    100,
		DurabilityLoaded: true,
		CreatePlayedTime: 42,
	}
	state.Enchantments[0] = 123
	state.Enchantments[1] = 456
	state.Enchantments[2] = 789
	block := buildItemCreateBlockForLocationWithState(uint64(8)|uint64(0x4000)<<48, 6948, 2, 1, 1, 0, nil, state)
	r := protocol.NewReader(block)
	if value, err := r.ReadU8(); err != nil || value != protocol.UpdateCreateObject2 {
		t.Fatalf("update type=%d err=%v", value, err)
	}
	if _, err := r.ReadPackedGUID(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadU16(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadU32(); err != nil {
		t.Fatal(err)
	}
	maskBlocks, err := r.ReadU8()
	if err != nil {
		t.Fatal(err)
	}
	mask := make([]uint32, maskBlocks)
	for index := range mask {
		mask[index], err = r.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
	}
	values := make(map[int]uint32)
	for index := 0; index < 64; index++ {
		if mask[index/32]&(1<<uint(index%32)) == 0 {
			continue
		}
		values[index], err = r.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
	}
	want := map[int]uint32{14: 2, 15: 77, 16: 1, 17: 2, 18: 3, 19: 4, 20: 5, 21: 0x21, 22: 123, 23: 456, 24: 789, 59: uint32(randomPropertyID), 60: 9, 61: 100, 62: 42}
	for index, expected := range want {
		if values[index] != expected {
			t.Fatalf("item field %d=%d want %d", index, values[index], expected)
		}
	}
}

func TestSendInventoryItemsIncludesPersistedState(t *testing.T) {
	randomPropertyID := int32(-12)
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER, duration INTEGER, charges TEXT, flags INTEGER, enchantments TEXT, randomPropertyId INTEGER, playedTime INTEGER, durability INTEGER)",
		"CREATE TABLE item_template (entry INTEGER PRIMARY KEY, ContainerSlots INTEGER, MaxDurability INTEGER)",
		"INSERT INTO item_template VALUES (6948, 0, 100)",
		"INSERT INTO item_instance VALUES (8, 6948, 2, 77, '1 2 3 4 5', 33, '123 456 789', -12, 42, 9)",
		"INSERT INTO character_inventory VALUES (1, 0, 23, 8)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	client, serverConn := net.Pipe()
	defer client.Close()
	defer serverConn.Close()
	sess := &session{
		server:       &Server{CharactersStore: &database.Store{DB: db}, WorldStore: &database.Store{DB: db}},
		conn:         serverConn,
		playerGUID:   1,
		playerLoaded: true,
		player:       &playerState{GUID: 1},
	}
	packetCh := make(chan []byte, 1)
	go func() {
		opcode, payload, readErr := readServerFrame(client, nil)
		if readErr != nil {
			return
		}
		if opcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
			payload, readErr = protocol.DecompressUpdatePayload(payload)
			if readErr != nil {
				return
			}
		}
		packetCh <- payload
	}()
	if err := sess.sendInventoryItems(context.Background()); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	select {
	case payload = <-packetCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for inventory update")
	}
	r := protocol.NewReader(payload)
	blocks, err := r.ReadU32()
	if err != nil || blocks == 0 {
		t.Fatalf("blocks=%d err=%v", blocks, err)
	}
	if updateType, err := r.ReadU8(); err != nil || updateType != protocol.UpdateCreateObject2 {
		t.Fatalf("update type=%d err=%v", updateType, err)
	}
	if _, err := r.ReadPackedGUID(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadU16(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReadU32(); err != nil {
		t.Fatal(err)
	}
	maskBlocks, err := r.ReadU8()
	if err != nil {
		t.Fatal(err)
	}
	mask := make([]uint32, maskBlocks)
	for index := range mask {
		mask[index], err = r.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
	}
	values := make(map[int]uint32)
	for index := 0; index < 64; index++ {
		if mask[index/32]&(1<<uint(index%32)) == 0 {
			continue
		}
		values[index], err = r.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
	}
	for index, expected := range map[int]uint32{14: 2, 15: 77, 16: 1, 17: 2, 18: 3, 19: 4, 20: 5, 21: 33, 22: 123, 23: 456, 24: 789, 59: uint32(randomPropertyID), 60: 9, 61: 100, 62: 42} {
		if values[index] != expected {
			t.Fatalf("item field %d=%d want %d", index, values[index], expected)
		}
	}
}
