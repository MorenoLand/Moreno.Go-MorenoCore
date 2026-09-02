package world

import (
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBuildQuestGiverRequestItems(t *testing.T) {
	view := questRewardView{Detail: questDetailData{ID: 9, Title: "Quest", Flags: 10, SuggestedGroupNum: 11}, RequestText: "Bring it", RequiredItems: []questRewardItem{{ID: 12, Quantity: 13, DisplayID: 14}}}
	reader := protocol.NewReader(buildQuestGiverRequestItems(view, 15, 16, true, false))
	for _, expected := range []uint64{15} {
		if value, err := reader.ReadU64(); err != nil || value != expected {
			t.Fatalf("giver=%d err=%v", value, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != view.Detail.ID {
		t.Fatalf("quest=%d err=%v", value, err)
	}
	for _, expected := range []string{view.Detail.Title, view.RequestText} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	for _, expected := range []uint32{0, 16, 0, view.Detail.Flags, view.Detail.SuggestedGroupNum, 0, 1, 12, 13, 14, 3, 4, 8, 16} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
}

func TestBuildQuestGiverOfferReward(t *testing.T) {
	view := questRewardView{Detail: questDetailData{ID: 20, Title: "Quest", Flags: 21, SuggestedGroupNum: 22, ChoiceItems: []questRewardItem{{ID: 23, Quantity: 24, DisplayID: 25}}, RewardItems: []questRewardItem{{ID: 26, Quantity: 27, DisplayID: 28}}, RewardMoney: 29, RewardXPDifficulty: 30, RewardHonor: 31, RewardKillHonor: 1.5, RewardDisplaySpell: 32, RewardSpell: -33, RewardTitleID: 34, RewardTalents: 35, RewardArenaPoints: -36, RewardFactionID: [questRewardFactions]uint32{37}, RewardFactionValue: [questRewardFactions]int32{-42}, RewardFactionOverride: [questRewardFactions]int32{-47}}, RewardText: "Reward", OfferEmotes: []questDescEmote{{Type: 38, Delay: 39}}}
	reader := protocol.NewReader(buildQuestGiverOfferReward(view, 40, true))
	if value, err := reader.ReadU64(); err != nil || value != 40 {
		t.Fatalf("giver=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != view.Detail.ID {
		t.Fatalf("quest=%d err=%v", value, err)
	}
	for _, expected := range []string{view.Detail.Title, view.RewardText} {
		if value, err := reader.ReadCString(); err != nil || value != expected {
			t.Fatalf("text=%q expected=%q err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("auto=%d err=%v", value, err)
	}
	for _, expected := range []uint32{view.Detail.Flags, view.Detail.SuggestedGroupNum, 1, 39, 38, 1, 23, 24, 25, 1, 26, 27, 28, view.Detail.RewardMoney, view.Detail.RewardXPDifficulty, view.Detail.RewardHonor} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadF32(); err != nil || value != view.Detail.RewardKillHonor {
		t.Fatalf("honor=%f err=%v", value, err)
	}
	for _, expected := range []int32{int32(view.Detail.RewardDisplaySpell), view.Detail.RewardSpell, int32(view.Detail.RewardTitleID), int32(view.Detail.RewardTalents), view.Detail.RewardArenaPoints} {
		if value, err := reader.ReadI32(); err != nil || value != expected {
			t.Fatalf("signed=%d expected=%d err=%v", value, expected, err)
		}
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("faction flags=%d err=%v", value, err)
	}
	for index := 0; index < questRewardFactions; index++ {
		if value, err := reader.ReadU32(); err != nil || value != view.Detail.RewardFactionID[index] {
			t.Fatalf("faction id=%d err=%v", value, err)
		}
	}
	for index := 0; index < questRewardFactions; index++ {
		if value, err := reader.ReadI32(); err != nil || value != view.Detail.RewardFactionValue[index] {
			t.Fatalf("faction value=%d err=%v", value, err)
		}
	}
	for index := 0; index < questRewardFactions; index++ {
		if value, err := reader.ReadI32(); err != nil || value != view.Detail.RewardFactionOverride[index] {
			t.Fatalf("faction override=%d err=%v", value, err)
		}
	}
	if reader.Remaining() != 0 {
		t.Fatalf("remaining=%d", reader.Remaining())
	}
}

func TestBuildQuestRewardPackets(t *testing.T) {
	reader := protocol.NewReader(buildQuestRewardComplete(1, 2, 3, 4, 5, 6))
	for expected := uint32(1); expected <= 6; expected++ {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("value=%d expected=%d err=%v", value, expected, err)
		}
	}
	if reader.Remaining() != 0 {
		t.Fatalf("remaining=%d", reader.Remaining())
	}
	reader = protocol.NewReader(buildItemPushResult(7, 0, 23, 8, 9, 10, false))
	if value, err := reader.ReadU64(); err != nil || value != 7 {
		t.Fatalf("guid=%d err=%v", value, err)
	}
	for _, expected := range []uint32{1, 0, 0} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("header=%d expected=%d err=%v", value, expected, err)
		}
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []uint32{23, 8, 0, 0, 9, 10} {
		if value, err := reader.ReadU32(); err != nil || value != expected {
			t.Fatalf("item=%d expected=%d err=%v", value, expected, err)
		}
	}
}

