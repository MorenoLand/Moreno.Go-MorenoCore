package world

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestBuildRealmSplit(t *testing.T) {
	packet, err := buildRealmSplit([]byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(packet) != 17 || binary.LittleEndian.Uint32(packet[:4]) != 0x04030201 || binary.LittleEndian.Uint32(packet[4:8]) != 0 {
		t.Fatalf("packet=%x", packet)
	}
	if string(packet[8:]) != "01/01/01\x00" {
		t.Fatalf("date=%q", packet[8:])
	}
}

func TestBuildAccountDataTimesUsesRequestedMask(t *testing.T) {
	packet := buildAccountDataTimes(time.Unix(123, 0), globalAccountDataMask)
	if len(packet) != 21 || binary.LittleEndian.Uint32(packet[9:13]) != 0 {
		t.Fatalf("packet=%x", packet)
	}
	if binary.LittleEndian.Uint32(packet[5:9]) != globalAccountDataMask {
		t.Fatalf("mask=%x", packet[5:9])
	}
}

func TestBuildAccountDataTimesIncludesSavedTimestamps(t *testing.T) {
	packet := buildAccountDataTimesWithTimestamps(time.Unix(123, 0), globalAccountDataMask, []uint32{11, 22, 33, 44, 55, 66, 77, 88})
	if binary.LittleEndian.Uint32(packet[9:13]) != 11 || binary.LittleEndian.Uint32(packet[13:17]) != 33 || binary.LittleEndian.Uint32(packet[17:21]) != 55 {
		t.Fatalf("timestamps=%x", packet)
	}
}
