package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildItemQueryResponse(t *testing.T) {
	data := itemQueryData{Entry: 45, Class: 4, SubClass: 2, SoundOverrideSubclass: -1, Name: "Test Item", DisplayInfoID: 3265, Quality: 1, Flags: 2, Flags2: 4, BuyPrice: -3, SellPrice: 7, InventoryType: 4, AllowableClass: ^uint32(0), AllowableRace: ^uint32(0), StatsCount: 1, Stats: [itemStats]itemStatQueryData{{Type: 3, Value: -42}}, Damage: [itemDamages]itemDamageQueryData{{Min: 1, Max: 2, Type: 0}}, Spells: [itemSpells]itemSpellQueryData{{ID: 123, Trigger: 1, Charges: 4, Cooldown: -1, Category: 2, CategoryCooldown: -1}}}
	reader := protocol.NewReader(buildItemQueryResponse(data, true))
	for _, expected := range []uint32{data.Entry, data.Class, data.SubClass} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadI32(); err != nil || value != data.SoundOverrideSubclass {
		t.Fatalf("sound=%d err=%v", value, err)
	}
	if value, err := reader.ReadCString(); err != nil || value != data.Name {
		t.Fatalf("name=%q err=%v", value, err)
	}
	for index := 0; index < 3; index++ {
		if value, err := reader.ReadU8(); err != nil || value != 0 {
			t.Fatalf("empty name %d=%d err=%v", index, value, err)
		}
	}
	for index := 0; index < 21; index++ {
		if _, err := reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if statsCount, err := reader.ReadU32(); err != nil || statsCount != 1 {
		t.Fatalf("stats count=%d err=%v", statsCount, err)
	}
	if statType, err := reader.ReadU32(); err != nil || statType != 3 {
		t.Fatalf("stat type=%d err=%v", statType, err)
	}
	if statValue, err := reader.ReadI32(); err != nil || statValue != -42 {
		t.Fatalf("stat value=%d err=%v", statValue, err)
	}
}

func TestBuildUnknownItemQueryResponse(t *testing.T) {
	data := buildItemQueryResponse(itemQueryData{Entry: 45}, false)
	reader := protocol.NewReader(data)
	if entry, err := reader.ReadU32(); err != nil || entry != 0x8000002D || len(data) != 4 {
		t.Fatalf("entry=%x len=%d err=%v", entry, len(data), err)
	}
}
