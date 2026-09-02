//go:build ignore

package world

import (
	"context"
	"net"
	"testing"
	"time"

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

