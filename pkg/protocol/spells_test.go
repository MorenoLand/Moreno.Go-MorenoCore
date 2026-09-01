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
