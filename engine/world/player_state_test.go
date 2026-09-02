package world

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestPlayerCreateMask(t *testing.T) {
	if playerCreateMask(3) != 0x04 || playerCreateMask(11) != 0x400 || playerCreateMask(0) != 0 {
		t.Fatalf("masks are incorrect")
	}
}

func TestBuildInitialReputations(t *testing.T) {
	payload := buildInitialReputations(playerState{Reputations: []playerReputation{{ListID: 72, Standing: 42999, Flags: 1}}})
	reader := protocol.NewReader(payload)
	count, err := reader.ReadU32()
	if err != nil || count != 128 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	for index := uint32(0); index < count; index++ {
		flags, flagsErr := reader.ReadU8()
		standing, standingErr := reader.ReadU32()
		if flagsErr != nil || standingErr != nil {
			t.Fatalf("entry %d flags=%v standing=%v", index, flagsErr, standingErr)
		}
		if index == 72 && (flags != 1 || standing != 42999) {
			t.Fatalf("stormwind entry flags=%d standing=%d", flags, standing)
		}
	}
}

func TestBuildBagCreateBlockIncludesContentsAndContainer(t *testing.T) {
	bagGUID := uint64(0x4000000000000019)
	itemGUID := uint64(0x4000000000000020)
	block := buildItemCreateBlockForLocation(bagGUID, 1725, 1, 26, 26, 4, map[uint32]uint64{0: itemGUID})
	reader := protocol.NewReader(block)
	if value, err := reader.ReadU8(); err != nil || value != protocol.UpdateCreateObject2 {
		t.Fatalf("update type=%d err=%v", value, err)
	}
	if value, err := reader.ReadPackedGUID(); err != nil || value != bagGUID {
		t.Fatalf("bag guid=%x err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("object type=%d err=%v", value, err)
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	maskBlocks, err := reader.ReadU8()
	if err != nil {
		t.Fatal(err)
	}
	mask := make([]uint32, maskBlocks)
	for index := range mask {
		if mask[index], err = reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	values := make(map[int]uint32)
	for index := 0; index < 76; index++ {
		if mask[index/32]&(1<<uint(index%32)) == 0 {
			continue
		}
		value, readErr := reader.ReadU32()
		if readErr != nil {
			t.Fatal(readErr)
		}
		values[index] = value
	}
	if values[8] != 26 || values[9] != 0 || values[64] != 4 || values[66] != uint32(itemGUID) || values[67] != uint32(itemGUID>>32) {
		t.Fatalf("bag fields=%x/%x/%d/%x/%x", values[8], values[9], values[64], values[66], values[67])
	}
}

func TestBuildPlayerUpdateKeepsMovementAndUpdateMaskAligned(t *testing.T) {
	server := &Server{}
	packet, err := server.buildPlayerUpdate(playerState{GUID: 26, Level: 21, Map: 0, X: 1, Y: 2, Z: 3, Orientation: 4})
	if err != nil {
		t.Fatal(err)
	}
	payload := packet.Payload.Bytes()
	if packet.Opcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		payload, err = protocol.DecompressUpdatePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	reader := protocol.NewReader(payload)
	if blocks, err := reader.ReadU32(); err != nil || blocks != 1 {
		t.Fatalf("blocks=%d err=%v", blocks, err)
	}
	if updateType, err := reader.ReadU8(); err != nil || updateType != protocol.UpdateCreateObject2 {
		t.Fatalf("update type=%d err=%v", updateType, err)
	}
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != 26 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
	if objectType, err := reader.ReadU8(); err != nil || objectType != 4 {
		t.Fatalf("object type=%d err=%v", objectType, err)
	}
	if flags, err := reader.ReadU16(); err != nil || flags != 0x0061 {
		t.Fatalf("update flags=%x err=%v", flags, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU16(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 4; index++ {
		if _, err := reader.ReadF32(); err != nil {
			t.Fatal(err)
		}
	}
	if fallTime, err := reader.ReadU32(); err != nil || fallTime != 0 {
		t.Fatalf("fall time=%d err=%v", fallTime, err)
	}
	for index := 0; index < 9; index++ {
		if _, err := reader.ReadF32(); err != nil {
			t.Fatal(err)
		}
	}
	maskBlocks, err := reader.ReadU8()
	if err != nil || maskBlocks != 42 {
		t.Fatalf("mask blocks=%d err=%v", maskBlocks, err)
	}
	mask := make([]uint32, maskBlocks)
	for index := range mask {
		if mask[index], err = reader.ReadU32(); err != nil {
			t.Fatal(err)
		}
	}
	if mask[0]&0x3 != 0x3 {
		t.Fatalf("object guid mask=%x", mask[0])
	}
	values := make(map[int]uint32)
	for index := 0; index < playerValuesCount; index++ {
		if mask[index/32]&(1<<uint(index%32)) == 0 {
			continue
		}
		value, readErr := reader.ReadU32()
		if readErr != nil {
			t.Fatal(readErr)
		}
		values[index] = value
	}
	if values[0] != 26 || values[1] != 0 || values[2] != 0x19 {
		t.Fatalf("object values=%x/%x/%x", values[0], values[1], values[2])
	}
}

func TestBuildPlayerUpdateCharacterSheetFields(t *testing.T) {
	server := &Server{}
	state := playerState{
		GUID:      100,
		Race:      1,
		Class:     1,
		Level:     20,
		Health:    500,
		MaxHealth: 500,
		Powers:    [7]uint32{400},
		MaxPowers: [7]uint32{400},
		Talents:   map[uint32]uint8{1: 2, 2: 1}, // rank 2 (3 pts) + rank 1 (2 pts) = 5 pts
	}
	packet, err := server.buildPlayerUpdate(state)
	if err != nil {
		t.Fatal(err)
	}
	payload := packet.Payload.Bytes()
	if packet.Opcode == uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		payload, err = protocol.DecompressUpdatePayload(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	reader := protocol.NewReader(payload)
	_, _ = reader.ReadU32() // blocks
	_, _ = reader.ReadU8()  // updateType
	_, _ = reader.ReadPackedGUID()
	_, _ = reader.ReadU8()  // obj type
	_, _ = reader.ReadU16() // flags
	_, _ = reader.ReadU32()
	_, _ = reader.ReadU16()
	_, _ = reader.ReadU32()
	for i := 0; i < 4; i++ {
		_, _ = reader.ReadF32()
	}
	_, _ = reader.ReadU32() // fall time
	for i := 0; i < 9; i++ {
		_, _ = reader.ReadF32()
	}
	maskBlocks, _ := reader.ReadU8()
	mask := make([]uint32, maskBlocks)
	for i := range mask {
		mask[i], _ = reader.ReadU32()
	}
	values := make(map[int]uint32)
	for i := 0; i < playerValuesCount; i++ {
		if mask[i/32]&(1<<uint(i%32)) == 0 {
			continue
		}
		val, err := reader.ReadU32()
		if err != nil {
			t.Fatal(err)
		}
		values[i] = val
	}

	// Attack speeds and damages
	if values[unitFieldRangedAttackTime] != 2000 {
		t.Errorf("ranged attack time expected 2000, got %d", values[unitFieldRangedAttackTime])
	}
	if values[unitModCastSpeed] != math.Float32bits(1.0) {
		t.Errorf("cast speed expected 1.0, got %f", math.Float32frombits(values[unitModCastSpeed]))
	}
	if values[playerFieldModDamageDonePct] != math.Float32bits(1.0) {
		t.Errorf("damage done pct expected 1.0, got %f", math.Float32frombits(values[playerFieldModDamageDonePct]))
	}

	// Base stats (20 + 20*2 = 60)
	if values[unitFieldStat0] != 60 || values[unitFieldStat1] != 60 {
		t.Errorf("expected stat0=60, got %d", values[unitFieldStat0])
	}
	if values[unitFieldResistances] != 120 {
		t.Errorf("expected armor=120, got %d", values[unitFieldResistances])
	}

	// Free talent points: level 20 has (20 - 9) = 11 total. Spent = (2+1) + (1+1) = 5. Free = 6.
	if values[playerCharacterPoints1] != 6 {
		t.Errorf("expected free talent points 6, got %d", values[playerCharacterPoints1])
	}
	if values[playerCharacterPoints2] != 5 {
		t.Errorf("expected spent talent points 5, got %d", values[playerCharacterPoints2])
	}

	// UNIT_FIELD_BYTES_0: Race, Class, Gender, PowerType (prevents client 0x007F69D1 crash)
	bytes0 := values[unitFieldBytes0]
	raceByte := uint8(bytes0 & 0xFF)
	classByte := uint8((bytes0 >> 8) & 0xFF)
	genderByte := uint8((bytes0 >> 16) & 0xFF)
	powerTypeByte := uint8((bytes0 >> 24) & 0xFF)
	if raceByte != 1 || classByte != 1 || genderByte != 0 || powerTypeByte != 1 {
		t.Errorf("expected bytes0=(race 1, class 1, gender 0, power 1), got race %d, class %d, gender %d, power %d", raceByte, classByte, genderByte, powerTypeByte)
	}

	// PLAYER_NEXT_LEVEL_XP: Must match xpCurve so client displays EXP bar
	if values[unitFieldNextLevelXP] != xpCurve[state.Level] {
		t.Errorf("expected unitFieldNextLevelXP %d, got %d", xpCurve[state.Level], values[unitFieldNextLevelXP])
	}
}

func TestIsAllowedClassSkill(t *testing.T) {
	// Warlock (class 9) must NOT have Plate (293), Leather (414), or Mage Frost (6)
	if isAllowedClassSkill(9, 293) {
		t.Error("warlock should not be allowed Plate Mail")
	}
	if isAllowedClassSkill(9, 414) {
		t.Error("warlock should not be allowed Leather")
	}
	if isAllowedClassSkill(9, 6) {
		t.Error("warlock should not be allowed Mage Frost")
	}
	// Warlock MUST be allowed Cloth (415) and Warlock Demo (354)
	if !isAllowedClassSkill(9, 415) {
		t.Error("warlock should be allowed Cloth")
	}
	if !isAllowedClassSkill(9, 354) {
		t.Error("warlock should be allowed Warlock Demo")
	}
	// Warrior (class 1) must be allowed Plate (293) and Leather (414)
	if !isAllowedClassSkill(1, 293) {
		t.Error("warrior should be allowed Plate Mail")
	}
	if !isAllowedClassSkill(1, 414) {
		t.Error("warrior should be allowed Leather")
	}
}

