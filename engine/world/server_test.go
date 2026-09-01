package world

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/crypto"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestAuthSessionAndPing(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		"CREATE TABLE account (id INTEGER PRIMARY KEY, username TEXT NOT NULL, session_key_auth BLOB, last_ip TEXT, locked INTEGER, lock_country TEXT, os TEXT, online INTEGER NOT NULL DEFAULT 0)",
		"CREATE TABLE account_banned (id INTEGER NOT NULL, bandate INTEGER NOT NULL, unbandate INTEGER NOT NULL, active INTEGER NOT NULL)",
		"CREATE TABLE character_banned (guid INTEGER NOT NULL, active INTEGER NOT NULL)",
		"CREATE TABLE character_pet (owner INTEGER NOT NULL, slot INTEGER NOT NULL, entry INTEGER, modelid INTEGER, level INTEGER)",
		"CREATE TABLE character_spell (guid INTEGER NOT NULL, spell INTEGER NOT NULL, active INTEGER NOT NULL, disabled INTEGER NOT NULL)",
		"CREATE TABLE guild_member (guid INTEGER NOT NULL, guildid INTEGER NOT NULL)",
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, account INTEGER NOT NULL, name TEXT NOT NULL, race INTEGER NOT NULL, class INTEGER NOT NULL, gender INTEGER NOT NULL, skin INTEGER NOT NULL, face INTEGER NOT NULL, hairStyle INTEGER NOT NULL, hairColor INTEGER NOT NULL, facialStyle INTEGER NOT NULL, level INTEGER NOT NULL, zone INTEGER NOT NULL, map INTEGER NOT NULL, position_x REAL NOT NULL, position_y REAL NOT NULL, position_z REAL NOT NULL, orientation REAL NOT NULL, playerFlags INTEGER NOT NULL, extra_flags INTEGER NOT NULL DEFAULT 0, at_login INTEGER NOT NULL, equipmentCache TEXT, deleteInfos_Name TEXT, online INTEGER NOT NULL DEFAULT 0)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	key := bytes.Repeat([]byte{0x42}, crypto.SRP6SessionKeyLength)
	if _, err := db.Exec("INSERT INTO account (id, username, session_key_auth, last_ip, locked, lock_country, os) VALUES (7, 'test', ?, '127.0.0.1', 0, '00', 'Win')", key); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO characters (guid, account, name, race, class, gender, skin, face, hairStyle, hairColor, facialStyle, level, zone, map, position_x, position_y, position_z, orientation, playerFlags, at_login, equipmentCache) VALUES (99, 7, 'Tester', 1, 1, 0, 0, 0, 0, 0, 0, 1, 12, 0, 1.5, 2.5, 3.5, 0.5, 0, 32, '')"); err != nil {
		t.Fatal(err)
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	stores := &database.Set{Auth: store, Characters: store, World: store}
	server := NewServer(stores, slog.New(slog.NewTextHandler(io.Discard, nil)), 1)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.Handle(context.Background(), serverConn)
	challengeHeader := make([]byte, 4)
	if _, err := io.ReadFull(clientConn, challengeHeader); err != nil {
		t.Fatal(err)
	}
	challengeSize := int(binary.BigEndian.Uint16(challengeHeader[:2])) - 2
	challengePayload := make([]byte, challengeSize)
	if _, err := io.ReadFull(clientConn, challengePayload); err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(challengeHeader[2:]) != opcodeAuthChallenge || len(challengePayload) != 40 {
		t.Fatalf("challenge header=%x payload=%d", challengeHeader, len(challengePayload))
	}
	authSeed := challengePayload[4:8]
	localChallenge := []byte{1, 2, 3, 4}
	h := sha1.New()
	_, _ = h.Write([]byte("TEST"))
	_, _ = h.Write(make([]byte, 4))
	_, _ = h.Write(localChallenge)
	_, _ = h.Write(authSeed)
	_, _ = h.Write(key)
	payload := protocol.NewBuffer(96)
	payload.WriteU32(12340)
	payload.WriteU32(0)
	payload.WriteCString("TEST")
	payload.WriteU32(0)
	payload.Write(localChallenge)
	payload.WriteU32(0)
	payload.WriteU32(0)
	payload.WriteU32(1)
	payload.WriteU64(0)
	payload.Write(h.Sum(nil))
	if err := writeClientFrame(clientConn, opcodeAuthSession, payload.Bytes(), nil); err != nil {
		t.Fatal(err)
	}
	clientCrypt, err := crypto.NewClientAuthCrypt(key)
	if err != nil {
		t.Fatal(err)
	}
	responseOpcode, responsePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if responseOpcode != opcodeAuthResponse || !bytes.Equal(responsePayload, []byte{authOK}) {
		t.Fatalf("auth response opcode=%x payload=%x", responseOpcode, responsePayload)
	}
	ping := protocol.NewBuffer(8)
	ping.WriteU32(123)
	ping.WriteU32(45)
	if err := writeClientFrame(clientConn, opcodePing, ping.Bytes(), clientCrypt); err != nil {
		t.Fatal(err)
	}
	pongOpcode, pongPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if pongOpcode != opcodePong || !bytes.Equal(pongPayload, []byte{123, 0, 0, 0}) {
		t.Fatalf("pong opcode=%x payload=%x", pongOpcode, pongPayload)
	}
	if err := writeClientFrame(clientConn, uint32(protocol.OpcodeCMSG_CHAR_ENUM), nil, clientCrypt); err != nil {
		t.Fatal(err)
	}
	charOpcode, charPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if charOpcode != uint16(protocol.OpcodeSMSG_CHAR_ENUM) {
		t.Fatalf("character opcode=%x", charOpcode)
	}
	characters := protocol.NewReader(charPayload)
	count, err := characters.ReadU8()
	if err != nil || count != 1 {
		t.Fatalf("character count=%d err=%v", count, err)
	}
	guid, err := characters.ReadU64()
	if err != nil || guid != 99 {
		t.Fatalf("character guid=%d err=%v", guid, err)
	}
	if name, err := characters.ReadCString(); err != nil || name != "Tester" {
		t.Fatalf("character name=%q err=%v", name, err)
	}
	loginPayload := protocol.NewBuffer(8)
	loginPayload.WriteU64(99)
	if err := writeClientFrame(clientConn, uint32(protocol.OpcodeCMSG_PLAYER_LOGIN), loginPayload.Bytes(), clientCrypt); err != nil {
		t.Fatal(err)
	}
	verifyOpcode, verifyPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if verifyOpcode != uint16(protocol.OpcodeSMSG_LOGIN_VERIFY_WORLD) || len(verifyPayload) != 20 {
		t.Fatalf("verify world opcode=%x payload=%d", verifyOpcode, len(verifyPayload))
	}
	accountDataOpcode, accountDataPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if accountDataOpcode != uint16(protocol.OpcodeSMSG_ACCOUNT_DATA_TIMES) || len(accountDataPayload) != 29 {
		t.Fatalf("account data opcode=%x payload=%d", accountDataOpcode, len(accountDataPayload))
	}
	featureOpcode, featurePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if featureOpcode != uint16(protocol.OpcodeSMSG_FEATURE_SYSTEM_STATUS) || !bytes.Equal(featurePayload, []byte{2, 0}) {
		t.Fatalf("feature opcode=%x payload=%x", featureOpcode, featurePayload)
	}
	motdOpcode, motdPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if motdOpcode != uint16(protocol.OpcodeSMSG_MOTD) || len(motdPayload) < 5 {
		t.Fatalf("motd opcode=%x payload=%d", motdOpcode, len(motdPayload))
	}
	danceOpcode, dancePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if danceOpcode != uint16(protocol.OpcodeSMSG_LEARNED_DANCE_MOVES) || len(dancePayload) != 8 {
		t.Fatalf("dance opcode=%x payload=%d", danceOpcode, len(dancePayload))
	}
	instanceOpcode, instancePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if instanceOpcode != uint16(protocol.OpcodeSMSG_INSTANCE_DIFFICULTY) || len(instancePayload) != 8 {
		t.Fatalf("instance difficulty opcode=%x payload=%d", instanceOpcode, len(instancePayload))
	}
	initialSpellsOpcode, initialSpellsPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if initialSpellsOpcode != uint16(protocol.OpcodeSMSG_INITIAL_SPELLS) || len(initialSpellsPayload) != 5 {
		t.Fatalf("initial spells opcode=%x payload=%d", initialSpellsOpcode, len(initialSpellsPayload))
	}
	unlearnOpcode, unlearnPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if unlearnOpcode != uint16(protocol.OpcodeSMSG_SEND_UNLEARN_SPELLS) || len(unlearnPayload) != 4 {
		t.Fatalf("unlearn opcode=%x payload=%d", unlearnOpcode, len(unlearnPayload))
	}
	actionOpcode, actionPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if actionOpcode != uint16(protocol.OpcodeSMSG_ACTION_BUTTONS) || len(actionPayload) != 577 {
		t.Fatalf("action opcode=%x payload=%d", actionOpcode, len(actionPayload))
	}
	tutOpcode, tutPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if tutOpcode != uint16(protocol.OpcodeSMSG_TUTORIAL_FLAGS) || len(tutPayload) != 32 {
		t.Fatalf("tutorial opcode=%x payload=%d", tutOpcode, len(tutPayload))
	}
	timeOpcode, timePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if timeOpcode != uint16(protocol.OpcodeSMSG_LOGIN_SET_TIME_SPEED) || len(timePayload) != 12 {
		t.Fatalf("time opcode=%x payload=%d", timeOpcode, len(timePayload))
	}
	chatOpcode, chatPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if chatOpcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) || len(chatPayload) == 0 {
		t.Fatalf("chat opcode=%x payload=%d", chatOpcode, len(chatPayload))
	}
	updateOpcode, updatePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if updateOpcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		updatePayload, err = protocol.DecompressUpdatePayload(updatePayload)
		if err != nil {
			t.Fatal(err)
		}
	} else if updateOpcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) {
		t.Fatalf("update opcode=%x", updateOpcode)
	}
	updates := protocol.NewReader(updatePayload)
	if blocks, err := updates.ReadU32(); err != nil || blocks != 1 {
		t.Fatalf("update blocks=%d err=%v", blocks, err)
	}
	if updateType, err := updates.ReadU8(); err != nil || updateType != protocol.UpdateCreateObject2 {
		t.Fatalf("update type=%d err=%v", updateType, err)
	}
	if updateGUID, err := updates.ReadPackedGUID(); err != nil || updateGUID != 99 {
		t.Fatalf("update guid=%d err=%v", updateGUID, err)
	}
	worldStateOpcode, worldStatePayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if worldStateOpcode != uint16(protocol.OpcodeSMSG_INIT_WORLD_STATES) || len(worldStatePayload) < 14 {
		t.Fatalf("world states opcode=%x payload=%d", worldStateOpcode, len(worldStatePayload))
	}
	timeSyncOpcode, timeSyncPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if timeSyncOpcode != uint16(protocol.OpcodeSMSG_TIME_SYNC_REQ) || len(timeSyncPayload) != 4 {
		t.Fatalf("time sync opcode=%x payload=%d", timeSyncOpcode, len(timeSyncPayload))
	}
	if err := writeClientFrame(clientConn, uint32(protocol.OpcodeCMSG_TIME_SYNC_RESP), []byte{0, 0, 0, 0, 0, 0, 0, 0}, clientCrypt); err != nil {
		t.Fatal(err)
	}
	if err := writeClientFrame(clientConn, uint32(protocol.OpcodeCMSG_LOGOUT_REQUEST), nil, clientCrypt); err != nil {
		t.Fatal(err)
	}
	logoutOpcode, logoutPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if logoutOpcode != uint16(protocol.OpcodeSMSG_LOGOUT_RESPONSE) || len(logoutPayload) != 5 || binary.LittleEndian.Uint32(logoutPayload[:4]) != 0 || logoutPayload[4] != 0 {
		t.Fatalf("logout response opcode=%x payload=%x", logoutOpcode, logoutPayload)
	}
	if err := writeClientFrame(clientConn, uint32(protocol.OpcodeCMSG_LOGOUT_CANCEL), nil, clientCrypt); err != nil {
		t.Fatal(err)
	}
	cancelOpcode, cancelPayload, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if cancelOpcode != uint16(protocol.OpcodeSMSG_LOGOUT_CANCEL_ACK) || len(cancelPayload) != 0 {
		t.Fatalf("logout cancel opcode=%x payload=%x", cancelOpcode, cancelPayload)
	}
	deletePayload := protocol.NewBuffer(8)
	deletePayload.WriteU64(99)
	if err := writeClientFrame(clientConn, uint32(protocol.OpcodeCMSG_CHAR_DELETE), deletePayload.Bytes(), clientCrypt); err != nil {
		t.Fatal(err)
	}
	deleteOpcode, deleteResponse, err := readServerFrame(clientConn, clientCrypt)
	if err != nil {
		t.Fatal(err)
	}
	if deleteOpcode != uint16(protocol.OpcodeSMSG_CHAR_DELETE) || !bytes.Equal(deleteResponse, []byte{71}) {
		t.Fatalf("delete opcode=%x response=%x", deleteOpcode, deleteResponse)
	}
}

func writeClientFrame(w io.Writer, opcode uint32, payload []byte, crypt interface{ EncryptSend([]byte) error }) error {
	size := len(payload) + 4
	header := make([]byte, 6)
	binary.BigEndian.PutUint16(header[:2], uint16(size))
	binary.LittleEndian.PutUint32(header[2:], opcode)
	if crypt != nil {
		if err := crypt.EncryptSend(header); err != nil {
			return err
		}
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}

func readServerFrame(r io.Reader, crypt interface{ DecryptRecv([]byte) error }) (uint16, []byte, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	if crypt != nil {
		if err := crypt.DecryptRecv(header); err != nil {
			return 0, nil, err
		}
	}
	size := int(binary.BigEndian.Uint16(header[:2]))
	payload := make([]byte, size-2)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return binary.LittleEndian.Uint16(header[2:]), payload, nil
}
