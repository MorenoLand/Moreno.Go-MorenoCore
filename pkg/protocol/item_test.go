package protocol

import "testing"

func TestBuildItemEnchantTimeUpdate(t *testing.T) {
	reader := NewReader(BuildItemEnchantTimeUpdate(77, 0x4000000000000008, 3, 42))
	itemGUID, err := reader.ReadU64()
	if err != nil || itemGUID != 0x4000000000000008 {
		t.Fatalf("item guid=%x err=%v", itemGUID, err)
	}
	slot, err := reader.ReadU32()
	if err != nil || slot != 3 {
		t.Fatalf("slot=%d err=%v", slot, err)
	}
	duration, err := reader.ReadU32()
	if err != nil || duration != 42 {
		t.Fatalf("duration=%d err=%v", duration, err)
	}
	playerGUID, err := reader.ReadU64()
	if err != nil || playerGUID != 77 {
		t.Fatalf("player guid=%x err=%v", playerGUID, err)
	}
}

func TestBuildItemTimeUpdate(t *testing.T) {
	reader := NewReader(BuildItemTimeUpdate(0x4000000000000008, 120))
	guid, err := reader.ReadU64()
	if err != nil || guid != 0x4000000000000008 {
		t.Fatalf("guid=%x err=%v", guid, err)
	}
	duration, err := reader.ReadU32()
	if err != nil || duration != 120 {
		t.Fatalf("duration=%d err=%v", duration, err)
	}
}
