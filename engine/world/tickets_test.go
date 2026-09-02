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

func TestGMTicketSystemStatus(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{conn: serverConn, authed: true, playerLoaded: true, playerGUID: 1}

	done := make(chan struct{})
	go func() {
		if !sess.handleGMTicketSystemStatus(context.Background(), nil) {
			t.Error("handleGMTicketSystemStatus returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_GMTICKET_SYSTEMSTATUS) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	status, err := r.ReadU32()
	if err != nil || status != 1 {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestGMTicketGetTicket(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{conn: serverConn, authed: true, playerLoaded: true, playerGUID: 1}

	done := make(chan struct{})
	go func() {
		if !sess.handleGMTicketGetTicket(context.Background(), nil) {
			t.Error("handleGMTicketGetTicket returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_GMTICKET_GETTICKET) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	status, err := r.ReadU32()
	if err != nil || status != 10 {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestGMTicketCreateAndUpdateAndDelete(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{conn: serverConn, authed: true, playerLoaded: true, playerGUID: 1}

	// 1. Create
	createBuf := protocol.NewBuffer(64)
	createBuf.WriteU32(0) // mapId
	createBuf.WriteF32(1.0)
	createBuf.WriteF32(2.0)
	createBuf.WriteF32(3.0)
	createBuf.WriteCString("Need help with quest")

	done := make(chan struct{})
	go func() {
		if !sess.handleGMTicketCreate(context.Background(), createBuf.Bytes()) {
			t.Error("handleGMTicketCreate returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_GMTICKET_CREATE) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	res, err := r.ReadU32()
	if err != nil || res != 1 {
		t.Fatalf("res=%d err=%v", res, err)
	}

	// 2. Update
	updateBuf := protocol.NewBuffer(64)
	updateBuf.WriteCString("Updated ticket message")

	done2 := make(chan struct{})
	go func() {
		if !sess.handleGMTicketUpdate(context.Background(), updateBuf.Bytes()) {
			t.Error("handleGMTicketUpdate returned false")
		}
		close(done2)
	}()

	opcode2, data2, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done2
	if opcode2 != uint16(protocol.OpcodeSMSG_GMTICKET_UPDATETEXT) {
		t.Fatalf("unexpected opcode %x", opcode2)
	}
	r2 := protocol.NewReader(data2)
	res2, err := r2.ReadU32()
	if err != nil || res2 != 1 {
		t.Fatalf("res=%d err=%v", res2, err)
	}

	// 3. Delete
	done3 := make(chan struct{})
	go func() {
		if !sess.handleGMTicketDelete(context.Background(), nil) {
			t.Error("handleGMTicketDelete returned false")
		}
		close(done3)
	}()

	opcode3, data3, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done3
	if opcode3 != uint16(protocol.OpcodeSMSG_GMTICKET_DELETETICKET) {
		t.Fatalf("unexpected opcode %x", opcode3)
	}
	r3 := protocol.NewReader(data3)
	res3, err := r3.ReadU32()
	if err != nil || res3 != 9 {
		t.Fatalf("res=%d err=%v", res3, err)
	}
}

func TestGMResponseResolve(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	sess := &session{conn: serverConn, authed: true, playerLoaded: true, playerGUID: 1}

	done := make(chan struct{})
	go func() {
		if !sess.handleGMResponseResolve(context.Background(), nil) {
			t.Error("handleGMResponseResolve returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op1, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	op2, data2, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if op1 != uint16(protocol.OpcodeSMSG_GMRESPONSE_STATUS_UPDATE) {
		t.Fatalf("unexpected op1 %x", op1)
	}
	if op2 != uint16(protocol.OpcodeSMSG_GMTICKET_DELETETICKET) {
		t.Fatalf("unexpected op2 %x", op2)
	}
	r := protocol.NewReader(data2)
	res, _ := r.ReadU32()
	if res != 9 {
		t.Fatalf("expected 9, got %d", res)
	}
}

func TestGMSurveyAndReportLag(t *testing.T) {
	sess := &session{authed: true, playerLoaded: true, playerGUID: 1}

	surveyBuf := protocol.NewBuffer(4)
	surveyBuf.WriteU32(9)
	if !sess.handleGMSurveySubmit(context.Background(), surveyBuf.Bytes()) {
		t.Fatal("handleGMSurveySubmit failed")
	}

	lagBuf := protocol.NewBuffer(20)
	lagBuf.WriteU32(1)   // type
	lagBuf.WriteU32(571) // Northrend map
	lagBuf.WriteF32(100.0)
	lagBuf.WriteF32(200.0)
	lagBuf.WriteF32(300.0)
	if !sess.handleGMReportLag(context.Background(), lagBuf.Bytes()) {
		t.Fatal("handleGMReportLag failed")
	}
}

func TestGMTicketCRUDWithDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		`CREATE TABLE gm_ticket (
			id INTEGER PRIMARY KEY,
			type INTEGER NOT NULL DEFAULT 0,
			playerGuid INTEGER NOT NULL DEFAULT 0,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			createTime INTEGER NOT NULL DEFAULT 0,
			mapId INTEGER NOT NULL DEFAULT 0,
			posX REAL NOT NULL DEFAULT 0,
			posY REAL NOT NULL DEFAULT 0,
			posZ REAL NOT NULL DEFAULT 0,
			lastModifiedTime INTEGER NOT NULL DEFAULT 0,
			closedBy INTEGER NOT NULL DEFAULT 0,
			assignedTo INTEGER NOT NULL DEFAULT 0,
			comment TEXT NOT NULL DEFAULT '',
			response TEXT NOT NULL DEFAULT '',
			completed INTEGER NOT NULL DEFAULT 0,
			escalated INTEGER NOT NULL DEFAULT 0,
			viewed INTEGER NOT NULL DEFAULT 0,
			needMoreHelp INTEGER NOT NULL DEFAULT 0,
			resolvedBy INTEGER NOT NULL DEFAULT 0
		)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store}
	sess := &session{server: srv, conn: serverConn, authed: true, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// 1. Initial check - no ticket, status = 10
	go func() {
		sess.handleGMTicketGetTicket(ctx, nil)
	}()
	op, data, err := readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_GMTICKET_GETTICKET) {
		t.Fatalf("expected SMSG_GMTICKET_GETTICKET, got op=%x err=%v", op, err)
	}
	r := protocol.NewReader(data)
	st, _ := r.ReadU32()
	if st != 10 {
		t.Fatalf("expected status 10, got %d", st)
	}

	// 2. Create ticket
	cBuf := protocol.NewBuffer(64)
	cBuf.WriteU32(0) // mapId
	cBuf.WriteF32(0)
	cBuf.WriteF32(0)
	cBuf.WriteF32(0)
	cBuf.WriteCString("Stuck in rock")
	cBuf.WriteU32(1) // needResponse
	cBuf.WriteU8(0)  // needMoreHelp
	go func() {
		sess.handleGMTicketCreate(ctx, cBuf.Bytes())
	}()
	op, _, err = readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_GMTICKET_CREATE) {
		t.Fatalf("expected SMSG_GMTICKET_CREATE, got op=%x err=%v", op, err)
	}

	// 3. GetTicket now returns status 6 with "Stuck in rock"
	go func() {
		sess.handleGMTicketGetTicket(ctx, nil)
	}()
	op, data, err = readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_GMTICKET_GETTICKET) {
		t.Fatalf("expected SMSG_GMTICKET_GETTICKET, got op=%x err=%v", op, err)
	}
	r = protocol.NewReader(data)
	st, _ = r.ReadU32()
	if st != 6 {
		t.Fatalf("expected status 6 (has text), got %d", st)
	}
	tid, _ := r.ReadU32()
	if tid != 1 {
		t.Fatalf("expected ticket id 1, got %d", tid)
	}
	msg, _ := r.ReadCString()
	if msg != "Stuck in rock" {
		t.Fatalf("expected message 'Stuck in rock', got '%s'", msg)
	}

	// 4. Update ticket
	uBuf := protocol.NewBuffer(64)
	uBuf.WriteCString("Still stuck in rock")
	go func() {
		sess.handleGMTicketUpdate(ctx, uBuf.Bytes())
	}()
	op, _, err = readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_GMTICKET_UPDATETEXT) {
		t.Fatalf("expected SMSG_GMTICKET_UPDATETEXT, got op=%x err=%v", op, err)
	}

	// Verify updated
	var dbDesc string
	_ = db.QueryRow("SELECT description FROM gm_ticket WHERE id = 1").Scan(&dbDesc)
	if dbDesc != "Still stuck in rock" {
		t.Fatalf("expected updated desc in DB, got '%s'", dbDesc)
	}

	// 5. Delete ticket
	go func() {
		sess.handleGMTicketDelete(ctx, nil)
	}()
	op, _, err = readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_GMTICKET_DELETETICKET) {
		t.Fatalf("expected SMSG_GMTICKET_DELETETICKET, got op=%x err=%v", op, err)
	}

	// 6. GetTicket now returns status 10
	go func() {
		sess.handleGMTicketGetTicket(ctx, nil)
	}()
	op, data, err = readServerFrame(clientConn, nil)
	if err != nil || op != uint16(protocol.OpcodeSMSG_GMTICKET_GETTICKET) {
		t.Fatalf("expected SMSG_GMTICKET_GETTICKET, got op=%x err=%v", op, err)
	}
	r = protocol.NewReader(data)
	st, _ = r.ReadU32()
	if st != 10 {
		t.Fatalf("expected status 10 after delete, got %d", st)
	}
}


