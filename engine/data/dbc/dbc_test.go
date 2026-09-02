package dbc

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParseAndReadWDBC(t *testing.T) {
	data := make([]byte, 20+8+7)
	copy(data, []byte("WDBC"))
	binary.LittleEndian.PutUint32(data[4:8], 1)
	binary.LittleEndian.PutUint32(data[8:12], 2)
	binary.LittleEndian.PutUint32(data[12:16], 8)
	binary.LittleEndian.PutUint32(data[16:20], 7)
	binary.LittleEndian.PutUint32(data[20:24], 42)
	binary.LittleEndian.PutUint32(data[24:28], 1)
	copy(data[28:], []byte("Goblin\x00"))
	file, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	record, err := file.Record(0)
	if err != nil {
		t.Fatal(err)
	}
	if value, err := record.Uint32(0); err != nil || value != 42 {
		t.Fatalf("id=%d err=%v", value, err)
	}
	if value, err := record.String(1); err != nil || value != "oblin" {
		t.Fatalf("name=%q err=%v", value, err)
	}
	if _, err := record.Float32(2); err == nil {
		t.Fatal("expected out-of-range field")
	}
	if math.Float32frombits(0) != 0 {
		t.Fatal("unexpected float conversion")
	}
}

func TestParseRejectsBadFiles(t *testing.T) {
	if _, err := Parse([]byte("bad")); err == nil {
		t.Fatal("expected invalid header")
	}
	data := make([]byte, 20)
	copy(data, []byte("WDBC"))
	binary.LittleEndian.PutUint32(data[8:12], 1)
	binary.LittleEndian.PutUint32(data[12:16], 8)
	if _, err := Parse(data); err == nil {
		t.Fatal("expected invalid record size")
	}
}
