//go:build ignore

package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildQuestQueryResponse(t *testing.T) {
	data := questQueryData{ID: 77, Method: 1, Level: -2, MinLevel: 3, SortID: -4, Type: 5, SuggestedGroupNum: 6, AllowableRaces: 0x12345678, RequiredFactionID: [2]uint32{7, 8}, RequiredFactionValue: [2]int32{-9, 10}, RewardNextQuest: 11, RewardXPDifficulty: 12, RewardMoney: 13, RewardBonusMoney: 14, RewardDisplaySpell: 15, RewardSpell: -16, RewardHonor: 17, RewardKillHonor: 1.25, StartItem: 18, Flags: 0x12345078, RewardTitleID: 19, RequiredPlayerKills: 20, RewardTalents: 21, RewardArenaPoints: -22, POIContinent: 23, POIX: 24.5, POIY: -25.5, POIPriority: 26, Title: "Title", Objectives: "Objectives", Details: "Details", AreaDescription: "Area", CompletedText: "Completed", RequiredNpcOrGo: [4]int32{-27, 28, 0, -30}, RequiredNpcOrGoCount: [4]uint32{31, 32, 33, 34}, ItemDrop: [4]uint32{35, 36, 37, 38}, RequiredItemID: [6]uint32{39, 40, 41, 42, 43, 44}, RequiredItemCount: [6]uint32{45, 46, 47, 48, 49, 50}, ObjectiveText: [4]string{"One", "Two", "Three", "Four"}}
	for index := range data.RewardItems {
		data.RewardItems[index] = questQueryItem{ID: uint32(51 + index*2), Quantity: uint32(52 + index*2)}
	}
	for index := range data.ChoiceItems {
		data.ChoiceItems[index] = questQueryItem{ID: uint32(59 + index*2), Quantity: uint32(60 + index*2)}
	}
	for index := range data.RewardFactionID {
		data.RewardFactionID[index], data.RewardFactionValue[index], data.RewardFactionOverride[index] = uint32(71+index), int32(76+index), int32(-81-index)
	}
	reader := protocol.NewReader(buildQuestQueryResponse(data))
	readU32 := func(expected uint32) {
		t.Helper()
		value, err := reader.ReadU32()
		if err != nil || value != expected {
			t.Fatalf("u32=%d expected=%d err=%v", value, expected, err)
		}
	}
	readI32 := func(expected int32) {
		t.Helper()
		value, err := reader.ReadI32()
		if err != nil || value != expected {
			t.Fatalf("i32=%d expected=%d err=%v", value, expected, err)
		}
	}
	readF32 := func(expected float32) {
		t.Helper()
		value, err := reader.ReadF32()
		if err != nil || value != expected {
			t.Fatalf("f32=%f expected=%f err=%v", value, expected, err)
		}
	}
	readText := func(expected string) {
		t.Helper()
		value, err := reader.ReadCString()
		if err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	for _, expected := range []uint32{data.ID, data.Method} {
		readU32(expected)
	}
	readI32(data.Level)
	readU32(data.MinLevel)
	readI32(data.SortID)
	for _, expected := range []uint32{data.Type, data.SuggestedGroupNum, data.RequiredFactionID[0]} {
		readU32(expected)
	}
	readI32(data.RequiredFactionValue[0])
	readU32(data.RequiredFactionID[1])
	readI32(data.RequiredFactionValue[1])
	for _, expected := range []uint32{data.RewardNextQuest, data.RewardXPDifficulty, data.RewardMoney, data.RewardBonusMoney, data.RewardDisplaySpell} {
		readU32(expected)
	}
	readI32(data.RewardSpell)
	for _, expected := range []uint32{data.RewardHonor} {
		readU32(expected)
	}
	readF32(data.RewardKillHonor)
	for _, expected := range []uint32{data.StartItem, data.Flags & 0xFFFF, data.RewardTitleID, data.RequiredPlayerKills, data.RewardTalents} {
		readU32(expected)
	}
	readI32(data.RewardArenaPoints)
	readU32(0)
	for _, item := range data.RewardItems {
		readU32(item.ID)
		readU32(item.Quantity)
	}
	for _, item := range data.ChoiceItems {
		readU32(item.ID)
		readU32(item.Quantity)
	}
	for _, value := range data.RewardFactionID {
		readU32(value)
	}
	for _, value := range data.RewardFactionValue {
		readI32(value)
	}
	for _, value := range data.RewardFactionOverride {
		readI32(value)
	}
	readU32(data.POIContinent)
	readF32(data.POIX)
	readF32(data.POIY)
	readU32(data.POIPriority)
	for _, expected := range []string{data.Title, data.Objectives, data.Details, data.AreaDescription, data.CompletedText} {
		readText(expected)
	}
	for index, value := range data.RequiredNpcOrGo {
		expected := uint32(value)
		if value < 0 {
			expected = uint32(-value) | 0x80000000
		}
		readU32(expected)
		readU32(data.RequiredNpcOrGoCount[index])
		readU32(data.ItemDrop[index])
		readU32(0)
	}
	for index, value := range data.RequiredItemID {
		readU32(value)
		readU32(data.RequiredItemCount[index])
	}
	for _, value := range data.ObjectiveText {
		readText(value)
	}
	if reader.Remaining() != 0 {
		t.Fatalf("remaining=%d", reader.Remaining())
	}
}

func TestBuildQuestQueryResponseHidesRewards(t *testing.T) {
	data := questQueryData{Flags: questHiddenRewardsFlag, RewardMoney: 55, RewardItems: [questRewardsCount]questQueryItem{{ID: 56, Quantity: 57}}, ChoiceItems: [questChoicesCount]questQueryItem{{ID: 58, Quantity: 59}}}
	reader := protocol.NewReader(buildQuestQueryResponse(data))
	for index := 0; index < 13; index++ {
		if _, err := reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("hidden money=%d err=%v", value, err)
	}
	for index := 0; index < 12; index++ {
		if _, err := reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	for index := 0; index < questRewardsCount+questChoicesCount; index++ {
		if value, err := reader.ReadU32(); err != nil || value != 0 {
			t.Fatalf("hidden reward field=%d err=%v", value, err)
		}
		if value, err := reader.ReadU32(); err != nil || value != 0 {
			t.Fatalf("hidden reward quantity=%d err=%v", value, err)
		}
	}
}

