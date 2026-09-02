package world

import (
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildChannelNotify(t *testing.T) {
	reader := protocol.NewReader(buildChannelNotify(channelYouJoinedNotice, "General", &channelNotifyChannel{Flags: 0x18, ID: 1}))
	if value, err := reader.ReadU8(); err != nil || value != channelYouJoinedNotice {
		t.Fatalf("notice=%d err=%v", value, err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "General" {
		t.Fatalf("name=%q err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0x18 {
		t.Fatalf("flags=%x err=%v", value, err)
	}
	for _, expected := range []uint32{1, 0} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	left := protocol.NewReader(buildChannelNotify(channelYouLeftNotice, "General", &channelNotifyChannel{Flags: 0x18, ID: 1}))
	if _, err := left.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if _, err := left.ReadCString(); err != nil {
		t.Fatal(err)
	}
	if value, err := left.ReadU32(); err != nil || value != 1 {
		t.Fatalf("left id=%d err=%v", value, err)
	}
	if value, err := left.ReadU8(); err != nil || value != 1 {
		t.Fatalf("left constant=%d err=%v", value, err)
	}
}

func TestTradeChannelKeepsCityRestrictionForLocalizedNames(t *testing.T) {
	if channelFlags(2, "Trade - Dun Morogh")&channelFlagCity == 0 {
		t.Fatal("trade channel lost its city-only flag")
	}
	if channelFlags(2, "Trade - Stormwind")&channelFlagCity == 0 {
		t.Fatal("localized trade channel lost its city-only flag")
	}
}

func TestHandleJoinAndLeaveChannel(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{}
	state := &session{server: server, conn: serverConn, playerLoaded: true, player: &playerState{GUID: 26}, playerGUID: 26, accountName: "TEST"}
	join := protocol.NewBuffer(32)
	join.WriteU32(1)
	join.WriteU8(0)
	join.WriteU8(0)
	join.WriteCString("General")
	join.WriteCString("")
	joined := make(chan bool, 1)
	go func() { joined <- state.handleJoinChannel(join.Bytes()) }()
	opcode, response, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !<-joined || opcode != uint16(protocol.OpcodeSMSG_CHANNEL_NOTIFY) {
		t.Fatalf("joined=%v opcode=%x", state.channels, opcode)
	}
	reader := protocol.NewReader(response)
	if notice, err := reader.ReadU8(); err != nil || notice != channelYouJoinedNotice {
		t.Fatalf("notice=%d err=%v", notice, err)
	}
	if name, err := reader.ReadCString(); err != nil || name != "General" {
		t.Fatalf("name=%q err=%v", name, err)
	}
	if flags, err := reader.ReadU8(); err != nil || flags != 0x18 {
		t.Fatalf("flags=%x err=%v", flags, err)
	}
	if id, err := reader.ReadU32(); err != nil || id != 1 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	leave := protocol.NewBuffer(16)
	leave.WriteU32(1)
	leave.WriteCString("General")
	left := make(chan bool, 1)
	go func() { left <- state.handleLeaveChannel(leave.Bytes()) }()
	opcode, response, err = readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !<-left || opcode != uint16(protocol.OpcodeSMSG_CHANNEL_NOTIFY) {
		t.Fatalf("left=%v opcode=%x", state.channels, opcode)
	}
	reader = protocol.NewReader(response)
	if notice, err := reader.ReadU8(); err != nil || notice != channelYouLeftNotice {
		t.Fatalf("notice=%d err=%v", notice, err)
	}
	if _, err := reader.ReadCString(); err != nil {
		t.Fatal(err)
	}
	if id, err := reader.ReadU32(); err != nil || id != 1 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if constant, err := reader.ReadU8(); err != nil || constant != 1 {
		t.Fatalf("constant=%d err=%v", constant, err)
	}
	if _, ok := server.channels["general"]; ok {
		t.Fatal("channel was not removed")
	}
}

func TestHandleChannelList(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	server := &Server{channels: map[string]*worldChannel{"general": {ID: 1, Name: "General", Flags: 0x18, Members: map[*session]struct{}{}}}}
	state := &session{server: server, conn: serverConn, playerLoaded: true, player: &playerState{GUID: 26}, playerGUID: 26}
	server.channels["general"].Members[state] = struct{}{}
	payload := protocol.NewBuffer(16)
	payload.WriteCString("General")
	done := make(chan bool, 1)
	go func() { done <- state.handleChannelList(payload.Bytes()) }()
	opcode, response, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !<-done || opcode != uint16(protocol.OpcodeSMSG_CHANNEL_LIST) {
		t.Fatalf("result=%v opcode=%x", state.playerLoaded, opcode)
	}
	reader := protocol.NewReader(response)
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("type=%d err=%v", value, err)
	}
	if value, err := reader.ReadCString(); err != nil || value != "General" {
		t.Fatalf("name=%q err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0x18 {
		t.Fatalf("flags=%x err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 1 {
		t.Fatalf("count=%d err=%v", value, err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 26 {
		t.Fatalf("guid=%d err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("member flags=%d err=%v", value, err)
	}
}

