package world

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/scripting"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestGuildChatUsesReferenceRankRights(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE guild_member (guildid INTEGER, guid INTEGER, rank INTEGER)",
		"CREATE TABLE guild_rank (guildid INTEGER, rid INTEGER, rights INTEGER)",
		"CREATE TABLE character_social (guid INTEGER, friend INTEGER, flags INTEGER)",
		"INSERT INTO guild_member VALUES (4, 10, 1), (4, 11, 2)",
		"INSERT INTO guild_rank VALUES (4, 1, 66), (4, 2, 65)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	server := &Server{CharactersStore: &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}}
	source := &session{server: server, playerGUID: 10, player: &playerState{GuildID: 4}}
	if !source.guildChatSpeakAllowed(false) || source.guildChatSpeakAllowed(true) {
		t.Fatal("rank 1 guild/officer speaking rights were not enforced")
	}
	target := &session{server: server, playerGUID: 11, player: &playerState{GuildID: 4}}
	if !server.guildChatListenAllowed(target, false) || server.guildChatListenAllowed(target, true) {
		t.Fatal("rank 2 guild/officer listening rights were not enforced")
	}
	if _, err := db.Exec("INSERT INTO character_social VALUES (11, 10, 2)"); err != nil {
		t.Fatal(err)
	}
	if !server.chatIgnoredBy(11, 10) {
		t.Fatal("guild recipient ignore state was not enforced")
	}
}

func TestLuaChatHookCanCancelMessage(t *testing.T) {
	runtime := scripting.NewRuntime(scripting.Config{Enabled: true})
	if err := runtime.LoadString(`RegisterPlayerEvent(18, function(event, player, msg) if msg == "blocked" then return false end end)`); err != nil {
		t.Fatal(err)
	}
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Features: &Features{Scripts: runtime}, sessions: make(map[*session]struct{})}
	state := &session{server: server, accountName: "TEST", playerGUID: 99, playerLoaded: true, player: &playerState{GUID: 99, Name: "Tester", Map: 0}, auras: make(map[uint32]struct{})}
	for _, message := range []struct {
		text      string
		cancelled bool
	}{
		{text: "blocked", cancelled: true},
		{text: "allowed", cancelled: false},
	} {
		payload := protocol.NewBuffer(32)
		payload.WriteU32(chatSay)
		payload.WriteU32(1)
		payload.WriteCString(message.text)
		if !state.handleMessageChat(context.Background(), payload.Bytes()) {
			t.Fatalf("message %q closed the session", message.text)
		}
	}
}

func TestChatWithoutScriptingRuntimeDoesNotPanic(t *testing.T) {
	server := &Server{sessions: make(map[*session]struct{})}
	state := &session{server: server, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Name: "Tester"}}
	payload := protocol.NewBuffer(16)
	payload.WriteU32(chatSay)
	payload.WriteU32(languageUniversal)
	payload.WriteCString("hello")
	if !state.handleMessageChat(context.Background(), payload.Bytes()) {
		t.Fatal("chat without scripting runtime should remain handled")
	}
}

func TestHandleMessageChatBroadcastsSayToSender(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 99, player: &playerState{GUID: 99, Name: "Tester", Map: 0, Skills: []playerSkill{{Skill: 98, Value: 300, Max: 300}}}}
	server.sessions[state] = struct{}{}
	payload := protocol.NewBuffer(16)
	payload.WriteU32(chatSay)
	payload.WriteU32(7)
	payload.WriteCString("hello")
	done := make(chan bool, 1)
	go func() { done <- state.handleMessageChat(context.Background(), payload.Bytes()) }()
	opcode, response, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(response)
	if value, err := reader.ReadU8(); err != nil || value != chatSay {
		t.Fatalf("type=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != state.playerGUID {
		t.Fatalf("sender=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != state.playerGUID {
		t.Fatalf("receiver=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "hello" {
		t.Fatalf("message=%q err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("chat tag=%d err=%v", value, err)
	}
	if !<-done {
		t.Fatal("chat handler rejected a valid say packet")
	}
}

func TestChatLanguageSkillMappingMatchesReference(t *testing.T) {
	for _, test := range []struct {
		language uint32
		skill    uint16
		known    bool
	}{
		{language: 1, skill: 109, known: true},
		{language: 7, skill: 98, known: true},
		{language: 13, skill: 313, known: true},
		{language: 36, skill: 0, known: true},
		{language: 99, skill: 0, known: false},
	} {
		skill, known := languageSkill(test.language)
		if skill != test.skill || known != test.known {
			t.Fatalf("language=%d skill=%d known=%v, want skill=%d known=%v", test.language, skill, known, test.skill, test.known)
		}
	}
	state := &session{player: &playerState{Skills: []playerSkill{{Skill: 98, Value: 300, Max: 300}}}}
	if !state.hasLanguageSkill(98) || state.hasLanguageSkill(109) {
		t.Fatal("language skill ownership did not follow the loaded player skills")
	}
}

func TestMalformedChatDoesNotCloseSession(t *testing.T) {
	server := &Server{sessions: make(map[*session]struct{})}
	state := &session{server: server, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Name: "Tester"}}
	for _, payload := range [][]byte{{}, {1, 0, 0, 0}, {chatWhisper, 0, 0, 0, 0, 0, 0, 0, 'T'}} {
		if !state.handleMessageChat(context.Background(), payload) {
			t.Fatalf("malformed payload closed session: %x", payload)
		}
	}
}

func TestAddonLanguageRequiresReferenceMessageType(t *testing.T) {
	server := &Server{sessions: make(map[*session]struct{})}
	state := &session{server: server, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Name: "Tester"}}
	invalid := protocol.NewBuffer(16)
	invalid.WriteU32(chatSay)
	invalid.WriteU32(languageAddon)
	invalid.WriteCString("blocked")
	if !state.handleMessageChat(context.Background(), invalid.Bytes()) {
		t.Fatal("invalid addon chat closed the session")
	}
	valid := protocol.NewBuffer(16)
	valid.WriteU32(chatParty)
	valid.WriteU32(languageAddon)
	valid.WriteCString("allowed")
	if !state.handleMessageChat(context.Background(), valid.Bytes()) {
		t.Fatal("valid addon chat closed the session")
	}
}

func TestBroadcastSayUsesSenderReceiverGUID(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 99, player: &playerState{GUID: 99, Name: "Tester", Map: 0}}
	server.sessions[state] = struct{}{}
	done := make(chan struct{})
	go func() {
		server.broadcastChat(state, nil, chatSay, 1, "hello", "")
		close(done)
	}()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
		t.Fatalf("opcode=%x", opcode)
	}
	reader := protocol.NewReader(payload)
	if value, err := reader.ReadU8(); err != nil || value != chatSay {
		t.Fatalf("type=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("sender=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("receiver=%d err=%v", value, err)
	}
	<-done
}

func TestBroadcastWhisperUsesReferenceDirections(t *testing.T) {
	sourceServer, sourceClient := net.Pipe()
	targetServer, targetClient := net.Pipe()
	defer sourceServer.Close()
	defer sourceClient.Close()
	defer targetServer.Close()
	defer targetClient.Close()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	source := &session{server: server, conn: sourceServer, authed: true, playerLoaded: true, playerGUID: 101, player: &playerState{GUID: 101, Name: "Source", Map: 0}}
	target := &session{server: server, conn: targetServer, authed: true, playerLoaded: true, playerGUID: 202, player: &playerState{GUID: 202, Name: "Target", Map: 0}}
	server.sessions[source] = struct{}{}
	server.sessions[target] = struct{}{}
	result := make(chan struct {
		opcode  uint16
		payload []byte
		err     error
	}, 2)
	read := func(conn net.Conn) {
		opcode, payload, err := readServerFrame(conn, nil)
		result <- struct {
			opcode  uint16
			payload []byte
			err     error
		}{opcode, payload, err}
	}
	go read(sourceClient)
	go read(targetClient)
	go server.broadcastChat(source, target, chatWhisper, languageUniversal, "hello", "")
	packets := <-result
	if packets.err != nil {
		t.Fatal(packets.err)
	}
	other := <-result
	if other.err != nil {
		t.Fatal(other.err)
	}
	seenWhisper, seenInform := false, false
	for _, packet := range []struct {
		opcode  uint16
		payload []byte
	}{{packets.opcode, packets.payload}, {other.opcode, other.payload}} {
		if packet.opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
			t.Fatalf("opcode=%x", packet.opcode)
		}
		reader := protocol.NewReader(packet.payload)
		messageType, err := reader.ReadU8()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = reader.ReadU32()
		senderGUID, err := reader.ReadU64()
		if err != nil {
			t.Fatal(err)
		}
		_, _ = reader.ReadU32()
		receiverGUID, err := reader.ReadU64()
		if err != nil {
			t.Fatal(err)
		}
		switch messageType {
		case chatWhisper:
			seenWhisper = senderGUID == source.playerGUID && receiverGUID == source.playerGUID
		case chatWhisperInform:
			seenInform = senderGUID == target.playerGUID && receiverGUID == target.playerGUID
		}
	}
	if !seenWhisper || !seenInform {
		t.Fatalf("whisper directions missing whisper=%v inform=%v", seenWhisper, seenInform)
	}
}

func TestBroadcastGMChatIncludesChatTag(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, gmChat: true, playerGUID: 99, player: &playerState{GUID: 99, Name: "Tester", Map: 0, PlayerFlags: playerFlagGM}}
	server.sessions[state] = struct{}{}
	done := make(chan struct{})
	go func() {
		server.broadcastChat(state, nil, chatSay, 1, "hello", "")
		close(done)
	}()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_GM_MESSAGECHAT) && opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
		t.Fatalf("opcode=%x, expected SMSG_GM_MESSAGECHAT or SMSG_MESSAGECHAT", opcode)
	}
	reader := protocol.NewReader(payload)
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("sender=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if opcode == uint16(protocol.OpcodeSMSG_GM_MESSAGECHAT) {
		nameLen, err := reader.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
		if nameLen > 0 {
			if _, err := reader.ReadCString(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if value, err := reader.ReadU64(); err != nil || value != 99 {
		t.Fatalf("receiver=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "hello" {
		t.Fatalf("message=%q err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 4 {
		t.Fatalf("chat tag=%d err=%v", value, err)
	}
	<-done
}

func TestSecurityLevelDoesNotForceGMChatPacket(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	state := &session{server: server, conn: serverConn, authed: true, playerLoaded: true, security: 1, playerGUID: 99, player: &playerState{GUID: 99, Name: "Tester", Map: 0}}
	server.sessions[state] = struct{}{}
	done := make(chan struct{})
	go func() {
		server.broadcastChat(state, nil, chatSay, 1, "hello", "")
		close(done)
	}()
	opcode, payload, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
		t.Fatalf("opcode=%x, security level forced GM chat packet", opcode)
	}
	reader := protocol.NewReader(payload)
	for _, read := range []func() error{func() error { _, err := reader.ReadU8(); return err }, func() error { _, err := reader.ReadU32(); return err }, func() error { _, err := reader.ReadU64(); return err }, func() error { _, err := reader.ReadU32(); return err }, func() error { _, err := reader.ReadU64(); return err }, func() error { _, err := reader.ReadU32(); return err }, func() error { _, err := reader.ReadCString(); return err }} {
		if err := read(); err != nil {
			t.Fatal(err)
		}
	}
	if tag, err := reader.ReadU8(); err != nil || tag != 0 {
		t.Fatalf("chat tag=%d err=%v", tag, err)
	}
	<-done
}

func TestHandleChatIgnored(t *testing.T) {
	serverConn1, clientConn1 := net.Pipe()
	defer serverConn1.Close()
	defer clientConn1.Close()
	serverConn2, clientConn2 := net.Pipe()
	defer serverConn2.Close()
	defer clientConn2.Close()

	server := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), sessions: make(map[*session]struct{})}
	sender := &session{server: server, conn: serverConn1, authed: true, playerLoaded: true, playerGUID: 101, player: &playerState{GUID: 101, Name: "Ignorer", Map: 0}}
	target := &session{server: server, conn: serverConn2, authed: true, playerLoaded: true, playerGUID: 102, player: &playerState{GUID: 102, Name: "Spammer", Map: 0}}
	server.sessions[sender] = struct{}{}
	server.sessions[target] = struct{}{}

	payload := protocol.NewBuffer(9)
	payload.WriteU64(102) // target GUID
	payload.WriteU8(0)    // unk

	done := make(chan struct{})
	go func() {
		if !sender.handleChatIgnored(payload.Bytes()) {
			t.Error("handleChatIgnored returned false")
		}
		close(done)
	}()

	opcode, msgPayload, err := readServerFrame(clientConn2, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
		t.Fatalf("unexpected opcode: %x", opcode)
	}
	r := protocol.NewReader(msgPayload)
	msgType, err := r.ReadU8()
	if err != nil || msgType != chatIgnored {
		t.Fatalf("msgType=%d err=%v", msgType, err)
	}
	lang, err := r.ReadU32()
	if err != nil || lang != languageUniversal {
		t.Fatalf("lang=%d err=%v", lang, err)
	}
	senderGUID, err := r.ReadU64()
	if err != nil || senderGUID != 101 {
		t.Fatalf("senderGUID=%d err=%v", senderGUID, err)
	}
}
