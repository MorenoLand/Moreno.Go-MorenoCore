package world

import (
	"context"
	"database/sql"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildNPCTextUpdate(t *testing.T) {
	options := defaultNPCTextOptions()
	options[0] = npcTextOption{Probability: 0.5, Text0: "male", Text1: "female", Language: 2, Emotes: [npcTextEmotes]npcTextEmote{{Delay: 3, ID: 4}, {Delay: 5, ID: 6}, {Delay: 7, ID: 8}}}
	reader := protocol.NewReader(buildNPCTextUpdate(780, options))
	if value, err := reader.ReadU32(); err != nil || value != 780 {
		t.Fatalf("text=%d err=%v", value, err)
	}
	if value, err := reader.ReadF32(); err != nil || value != 0.5 {
		t.Fatalf("probability=%f err=%v", value, err)
	}
	for _, expected := range []string{"male", "female"} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != 2 {
		t.Fatalf("language=%d err=%v", value, err)
	}
	for _, expected := range []uint32{3, 4, 5, 6, 7, 8} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("emote=%d expected=%d err=%v", value, expected, err)
		}
	}
	for index := 1; index < npcTextOptions; index++ {
		if probability, err := reader.ReadF32(); err != nil || probability != 0 {
			t.Fatalf("default probability=%f err=%v", probability, err)
		}
		for _, expected := range []string{"Greetings $N", "Greetings $N"} {
			if value, err := reader.ReadCString(); err != nil || value != expected {
				t.Fatalf("default text=%q expected=%q err=%v", value, expected, err)
			}
		}
		for emote := 0; emote < 1+npcTextEmotes*2; emote++ {
			if _, err := reader.ReadU32(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if reader.Remaining() != 0 {
		t.Fatalf("remaining=%d", reader.Remaining())
	}
}

func TestHandleNPCTextQueryLoadsDatabaseRow(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	columns := []string{"ID INTEGER PRIMARY KEY"}
	values := []any{780}
	placeholders := []string{"?"}
	for index := 0; index < npcTextOptions; index++ {
		suffix := strconv.Itoa(index)
		columns = append(columns, "text0_"+suffix+" TEXT", "text1_"+suffix+" TEXT", "lang"+suffix+" INTEGER", "Probability"+suffix+" REAL")
		if index == 0 {
			values = append(values, "male", "female", 2, 0.5)
		} else {
			values = append(values, "", "", 0, 0)
		}
		placeholders = append(placeholders, "?", "?", "?", "?")
		for emote := 0; emote < npcTextEmotes; emote++ {
			columns = append(columns, "EmoteDelay"+suffix+"_"+strconv.Itoa(emote)+" INTEGER", "Emote"+suffix+"_"+strconv.Itoa(emote)+" INTEGER")
			values = append(values, 0, 0)
			placeholders = append(placeholders, "?", "?")
		}
	}
	if _, err := db.Exec("CREATE TABLE npc_text (" + strings.Join(columns, ", ") + ")"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO npc_text VALUES ("+strings.Join(placeholders, ", ")+")", values...); err != nil {
		t.Fatal(err)
	}
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}}
	state := &session{server: server, conn: serverConn, playerLoaded: true, accountName: "TEST"}
	payload := protocol.NewBuffer(12)
	payload.WriteU32(780)
	payload.WriteU64(123)
	done := make(chan bool, 1)
	go func() { done <- state.handleNpcTextQuery(context.Background(), payload.Bytes()) }()
	opcode, response, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !<-done || opcode != uint16(protocol.OpcodeSMSG_NPC_TEXT_UPDATE) {
		t.Fatalf("result=%v opcode=%x", state.accountName, opcode)
	}
	reader := protocol.NewReader(response)
	if value, err := reader.ReadU32(); err != nil || value != 780 {
		t.Fatalf("text=%d err=%v", value, err)
	}
	if value, err := reader.ReadF32(); err != nil || value != 0.5 {
		t.Fatalf("probability=%f err=%v", value, err)
	}
	for _, expected := range []string{"male", "female"} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != 2 {
		t.Fatalf("language=%d err=%v", value, err)
	}
	for index := 0; index < npcTextEmotes*2; index++ {
		if _, err := reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if reader.Remaining() == 0 {
		t.Fatal("missing remaining NPC text options")
	}
}
