package world

import (
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestDynamicSpellObjectCreateAndDespawn(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{sessions: make(map[*session]struct{}), dynamicSpellObjects: make(map[uint64]*dynamicSpellObjectState)}
	sess := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, player: &playerState{GUID: 1, Map: 0}}
	server.sessions[sess] = struct{}{}
	object := &dynamicSpellObjectState{GUID: dynamicSpellGUID(1), CasterGUID: 1, SpellID: 5740, Map: 0, X: 10, Y: 20, Z: 30, Radius: 8}
	go server.spawnDynamicSpellObject(object, 20*time.Millisecond)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("create opcode=%x", opcode)
	}
	if opcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		payload, err = protocol.DecompressUpdatePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	reader := protocol.NewReader(payload)
	if blocks, err := reader.ReadU32(); err != nil || blocks != 1 {
		t.Fatalf("create blocks=%d err=%v", blocks, err)
	}
	if updateType, err := reader.ReadU8(); err != nil || updateType != protocol.UpdateCreateObject2 {
		t.Fatalf("create update type=%d err=%v", updateType, err)
	}
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != object.GUID {
		t.Fatalf("create guid=%x err=%v", guid, err)
	}
	if objectType, err := reader.ReadU8(); err != nil || objectType != 6 {
		t.Fatalf("create object type=%d err=%v", objectType, err)
	}
	if flags, err := reader.ReadU16(); err != nil || flags != dynamicObjectFlags {
		t.Fatalf("create flags=%x err=%v", flags, err)
	}
	if transport, err := reader.ReadPackedGUID(); err != nil || transport != 0 {
		t.Fatalf("create transport=%x err=%v", transport, err)
	}
	for index, expected := range []float32{10, 20, 30, 10, 20, 30, 0, 0} {
		value, err := reader.ReadF32()
		if err != nil || value != expected {
			t.Fatalf("create position[%d]=%f err=%v want=%f", index, value, err, expected)
		}
	}
	opcode, payload, err = readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_DESTROY_OBJECT) || len(payload) != 9 {
		t.Fatalf("destroy opcode=%x payload=%x", opcode, payload)
	}
	reader = protocol.NewReader(payload)
	guid, err := reader.ReadU64()
	if err != nil || guid != object.GUID {
		t.Fatalf("destroy guid=%x err=%v", guid, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("destroy death flag=%d err=%v", value, err)
	}
	server.objectsMu.Lock()
	_, exists := server.dynamicSpellObjects[object.GUID]
	server.objectsMu.Unlock()
	if exists {
		t.Fatal("dynamic spell object remained after despawn")
	}
}

func TestStreamDynamicSpellObjectsToEnteringPlayer(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{Config: config.Default(), sessions: make(map[*session]struct{}), dynamicSpellObjects: make(map[uint64]*dynamicSpellObjectState)}
	object := &dynamicSpellObjectState{GUID: dynamicSpellGUID(2), CasterGUID: 1, SpellID: 2120, Map: 0, X: 10, Y: 20, Z: 30, Radius: 8}
	server.dynamicSpellObjects[object.GUID] = object
	sess := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, player: &playerState{GUID: 2, Map: 0, X: 11, Y: 20, Z: 30}}
	go sess.streamDynamicSpellObjects()
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("stream opcode=%x payload=%d", opcode, len(payload))
	}
}
