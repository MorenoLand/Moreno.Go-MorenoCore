package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildCreatureQueryResponse(t *testing.T) {
	data := creatureQueryData{Entry: 68, Name: "Stormwind Guard", Subname: "", IconName: "", Flags: 0x10, Type: 7, Family: 0, Rank: 1, KillCredits: [creatureKillCredits]uint32{1, 2}, Models: [creatureModels]uint32{3167, 0, 0, 0}, Health: 1.5, Mana: 2.5, Leader: true, QuestItems: [creatureQuestItems]uint32{11, 12}, MovementID: 42}
	reader := protocol.NewReader(buildCreatureQueryResponse(data, true))
	if entry, err := reader.ReadU32(); err != nil || entry != data.Entry {
		t.Fatalf("entry=%d err=%v", entry, err)
	}
	if name, err := reader.ReadCString(); err != nil || name != data.Name {
		t.Fatalf("name=%q err=%v", name, err)
	}
	for index := 0; index < 3; index++ {
		if value, err := reader.ReadU8(); err != nil || value != 0 {
			t.Fatalf("empty name %d=%d err=%v", index, value, err)
		}
	}
	for _, expected := range []string{data.Subname, data.IconName} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("string=%q err=%v", value, err)
		}
	}
	for _, expected := range []uint32{data.Flags, data.Type, data.Family, data.Rank, 1, 2, 3167, 0, 0, 0} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadF32(); err != nil || value != data.Health {
		t.Fatalf("health=%f err=%v", value, err)
	}
	if value, err := reader.ReadF32(); err != nil || value != data.Mana {
		t.Fatalf("mana=%f err=%v", value, err)
	}
	if leader, err := reader.ReadU8(); err != nil || leader != 1 {
		t.Fatalf("leader=%d err=%v", leader, err)
	}
	for index := 0; index < creatureQuestItems; index++ {
		value, err := reader.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
		if value != data.QuestItems[index] {
			t.Fatalf("quest item %d=%d", index, value)
		}
	}
	if movementID, err := reader.ReadU32(); err != nil || movementID != data.MovementID {
		t.Fatalf("movement id=%d err=%v", movementID, err)
	}
}

func TestBuildUnknownCreatureQueryResponse(t *testing.T) {
	data := buildCreatureQueryResponse(creatureQueryData{Entry: 68}, false)
	if len(data) != 4 || protocol.NewReader(data) == nil {
		t.Fatalf("response=%x", data)
	}
	reader := protocol.NewReader(data)
	if entry, err := reader.ReadU32(); err != nil || entry != 0x80000044 {
		t.Fatalf("entry=%x err=%v", entry, err)
	}
}

func TestBuildGameObjectQueryResponse(t *testing.T) {
	data := gameObjectQueryData{Entry: 9001, Type: 3, DisplayID: 1234, Name: "Test Chest", IconName: "", CastBar: "Collecting", Unknown: "", Size: 1.25, QuestItems: [gameObjectQuestItems]uint32{7, 8}}
	for index := range data.Data {
		data.Data[index] = uint32(index)
	}
	reader := protocol.NewReader(buildGameObjectQueryResponse(data, true))
	for _, expected := range []uint32{data.Entry, data.Type, data.DisplayID} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	for _, expected := range []string{data.Name, "", "", "", data.IconName, data.CastBar, data.Unknown} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("string=%q expected=%q err=%v", value, expected, err)
		}
	}
	for index := range data.Data {
		if value, err := reader.ReadU32(); err != nil || value != data.Data[index] {
			t.Fatalf("data %d=%d expected=%d err=%v", index, value, data.Data[index], err)
		}
	}
	if size, err := reader.ReadF32(); err != nil || size != data.Size {
		t.Fatalf("size=%f err=%v", size, err)
	}
	for index := range data.QuestItems {
		if value, err := reader.ReadU32(); err != nil || value != data.QuestItems[index] {
			t.Fatalf("quest item %d=%d expected=%d err=%v", index, value, data.QuestItems[index], err)
		}
	}
}
