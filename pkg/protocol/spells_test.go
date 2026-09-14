package protocol

import "testing"

func TestSpellTargetDataRoundTrip(t *testing.T) {
	target := SpellTargetData{Flags: SpellTargetFlagUnit | SpellTargetFlagSourceLocation | SpellTargetFlagDestLocation | SpellTargetFlagString, UnitGUID: 0xF130000000000007, Source: SpellTargetLocation{Transport: 4, X: 1.5, Y: 2.5, Z: 3.5}, Destination: SpellTargetLocation{X: 4.5, Y: 5.5, Z: 6.5}, StringTarget: "target"}
	packet := NewBuffer(64)
	writeSpellTargetData(packet, target)
	decoded, err := ReadSpellTargetData(NewReader(packet.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if decoded != target {
		t.Fatalf("decoded=%+v target=%+v", decoded, target)
	}
}

func TestBuildSpellGoIncludesHitAndMissLists(t *testing.T) {
	target := SpellTargetData{Flags: SpellTargetFlagUnit, UnitGUID: 7}
	data := BuildSpellGo(26, 26, 1, 123, SpellCastFlagVisualChain, 456, []uint64{7}, []SpellMissStatus{{TargetGUID: 8, Reason: SpellMissReflect, ReflectStatus: 3}}, target)
	reader := NewReader(data)
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != 26 {
		t.Fatalf("caster=%x err=%v", guid, err)
	}
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != 26 {
		t.Fatalf("unit=%x err=%v", guid, err)
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 123 {
		t.Fatalf("spell=%d err=%v", value, err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU32(); err != nil {
		t.Fatal(err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("hits=%d err=%v", value, err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 7 {
		t.Fatalf("hit=%d err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 1 {
		t.Fatalf("misses=%d err=%v", value, err)
	}
	if value, err := reader.ReadU64(); err != nil || value != 8 {
		t.Fatalf("miss target=%d err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != SpellMissReflect {
		t.Fatalf("miss reason=%d err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 3 {
		t.Fatalf("reflect=%d err=%v", value, err)
	}
	decoded, err := ReadSpellTargetData(reader)
	if err != nil || decoded != target {
		t.Fatalf("target=%+v err=%v", decoded, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("visual chain 1=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 0 {
		t.Fatalf("visual chain 2=%d err=%v", value, err)
	}
}

func TestBuildSpellFailure(t *testing.T) {
	data := BuildSpellFailure(2, 123, 4)
	reader := NewReader(data)
	if value, err := reader.ReadU8(); err != nil || value != 2 {
		t.Fatalf("cast id=%d err=%v", value, err)
	}
	if value, err := reader.ReadU32(); err != nil || value != 123 {
		t.Fatalf("spell=%d err=%v", value, err)
	}
	if value, err := reader.ReadU8(); err != nil || value != 4 {
		t.Fatalf("result=%d err=%v", value, err)
	}
}

func TestBuildAuraUpdateAllPreservesReferenceRecords(t *testing.T) {
	data := BuildAuraUpdateAll(7, []AuraUpdateRecord{{CasterGUID: 7, Slot: 2, SpellID: 123, Positive: true, MaxDurationMs: 60000, DurationMs: 30000, CasterLevel: 20, StackCount: 2}, {CasterGUID: 8, Slot: 5, SpellID: 456, Positive: false, CasterLevel: 30, StackCount: 1}})
	reader := NewReader(data)
	if guid, err := reader.ReadPackedGUID(); err != nil || guid != 7 {
		t.Fatalf("target=%d err=%v", guid, err)
	}
	if slot, err := reader.ReadU8(); err != nil || slot != 2 {
		t.Fatalf("first slot=%d err=%v", slot, err)
	}
	if spell, err := reader.ReadU32(); err != nil || spell != 123 {
		t.Fatalf("first spell=%d err=%v", spell, err)
	}
	flags, err := reader.ReadU8()
	if err != nil || flags&AuraFlagCaster == 0 || flags&AuraFlagDuration == 0 || flags&AuraFlagPositive == 0 {
		t.Fatalf("first flags=%x err=%v", flags, err)
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if stack, err := reader.ReadU8(); err != nil || stack != 2 {
		t.Fatalf("first stack=%d err=%v", stack, err)
	}
	if max, err := reader.ReadU32(); err != nil || max != 60000 {
		t.Fatalf("first max=%d err=%v", max, err)
	}
	if remaining, err := reader.ReadU32(); err != nil || remaining != 30000 {
		t.Fatalf("first remaining=%d err=%v", remaining, err)
	}
	if slot, err := reader.ReadU8(); err != nil || slot != 5 {
		t.Fatalf("second slot=%d err=%v", slot, err)
	}
	if spell, err := reader.ReadU32(); err != nil || spell != 456 {
		t.Fatalf("second spell=%d err=%v", spell, err)
	}
	flags, err = reader.ReadU8()
	if err != nil || flags&AuraFlagCaster != 0 || flags&AuraFlagNegative == 0 {
		t.Fatalf("second flags=%x err=%v", flags, err)
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadU8(); err != nil {
		t.Fatal(err)
	}
	if caster, err := reader.ReadPackedGUID(); err != nil || caster != 8 {
		t.Fatalf("second caster=%d err=%v", caster, err)
	}
}
