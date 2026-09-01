package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildQuestGiverDetails(t *testing.T) {
	data := questDetailData{ID: 100, Title: "A Test Quest", Details: "Details", Objectives: "Objectives", Flags: 7, SuggestedGroupNum: 3, RewardMoney: 4, RewardXPDifficulty: 5, RewardBonusMoney: 6, RewardDisplaySpell: 7, RewardSpell: -8, RewardHonor: 9, RewardKillHonor: 1.5, RewardTitleID: 10, RewardTalents: 11, RewardArenaPoints: -12, ChoiceItems: []questRewardItem{{ID: 20, Quantity: 2, DisplayID: 200}}, RewardItems: []questRewardItem{{ID: 21, Quantity: 3, DisplayID: 201}}, DescEmotes: [questDetailEmotes]questDescEmote{{Type: 30, Delay: 31}}}
	reader := protocol.NewReader(buildQuestGiverDetails(data, 0x1122, 0))
	for _, expected := range []uint64{0x1122, 0} {
		if value, err := reader.ReadU64(); err != nil || value != expected {
			t.Fatalf("guid=%x expected=%x err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != data.ID {
		t.Fatalf("quest=%d err=%v", value, err)
	}
	for _, expected := range []string{data.Title, data.Details, data.Objectives} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("auto launched=%d err=%v", value, err)
	}
	for _, expected := range []uint32{data.Flags, data.SuggestedGroupNum} {
		value, err := reader.ReadU32()
		if err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU8(); err != nil || value != 0 {
		t.Fatalf("start cheat=%d err=%v", value, err)
	}
	for _, expected := range []uint32{1, 20, 2, 200, 1, 21, 3, 201, data.RewardMoney, data.RewardXPDifficulty, data.RewardHonor} {
		value, err := reader.ReadU32()
		if err != nil || value != expected {
			t.Fatalf("reward value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadF32(); err != nil || value != data.RewardKillHonor {
		t.Fatalf("honor=%f err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != data.RewardDisplaySpell {
		t.Fatalf("display spell=%d err=%v", value, err)
	}
	if value, err := reader.ReadI32(); err != nil || value != data.RewardSpell {
		t.Fatalf("reward spell=%d err=%v", value, err)
	}
	for _, expected := range []uint32{data.RewardTitleID, data.RewardTalents} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("reward value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadI32(); err != nil || value != data.RewardArenaPoints {
		t.Fatalf("arena=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("faction flags=%d err=%v", value, err)
	}
	for index := 0; index < questRewardFactions*3; index++ {
		if _, err := reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if value, err := reader.ReadI32(); err != nil || value != questDetailEmotes {
		t.Fatalf("emote count=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 30 {
		t.Fatalf("emote type=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 31 {
		t.Fatalf("emote delay=%d err=%v", value, err)
	}
}
