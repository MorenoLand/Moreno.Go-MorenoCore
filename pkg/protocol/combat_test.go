package protocol

import "testing"

func TestBuildAttackerStateUpdate_Normal(t *testing.T) {
	attacker := uint64(100)
	victim := uint64(200)
	damage := uint32(50)
	overkill := uint32(0)

	pkt := BuildAttackerStateUpdate(attacker, victim, damage, overkill, HitInfoAffectsVictim, VictimStateHit, 0)
	r := NewReader(pkt)

	hitInfo, err := r.ReadU32()
	if err != nil || hitInfo != HitInfoAffectsVictim {
		t.Fatalf("hitInfo mismatch: %d, err: %v", hitInfo, err)
	}
	atk, err := r.ReadPackedGUID()
	if err != nil || atk != attacker {
		t.Fatalf("attacker mismatch: %d, err: %v", atk, err)
	}
	vic, err := r.ReadPackedGUID()
	if err != nil || vic != victim {
		t.Fatalf("victim mismatch: %d, err: %v", vic, err)
	}
	dmg, _ := r.ReadU32()
	if dmg != damage {
		t.Fatalf("damage mismatch: %d", dmg)
	}
	okill, _ := r.ReadU32()
	if okill != overkill {
		t.Fatalf("overkill mismatch: %d", okill)
	}
	count, _ := r.ReadU8()
	if count != 1 {
		t.Fatalf("sub damage count mismatch: %d", count)
	}
	school, _ := r.ReadU32()
	if school != 1 {
		t.Fatalf("school mismatch: %d", school)
	}
	fSub, _ := r.ReadF32()
	if fSub != float32(damage) {
		t.Fatalf("float sub damage mismatch: %f", fSub)
	}
	uSub, _ := r.ReadU32()
	if uSub != damage {
		t.Fatalf("uint sub damage mismatch: %d", uSub)
	}
	tState, _ := r.ReadU8()
	if tState != VictimStateHit {
		t.Fatalf("targetState mismatch: %d", tState)
	}
	unk, _ := r.ReadU32()
	if unk != 0 {
		t.Fatalf("unknown mismatch: %d", unk)
	}
	spellID, _ := r.ReadU32()
	if spellID != 0 {
		t.Fatalf("spellID mismatch: %d", spellID)
	}
}

func TestBuildAttackerStateUpdate_Blocked(t *testing.T) {
	attacker := uint64(100)
	victim := uint64(200)
	damage := uint32(20)
	overkill := uint32(0)
	blocked := uint32(30)

	pkt := BuildAttackerStateUpdate(attacker, victim, damage, overkill, HitInfoAffectsVictim|HitInfoBlock, VictimStateHit, blocked)
	r := NewReader(pkt)

	hitInfo, _ := r.ReadU32()
	if hitInfo != HitInfoAffectsVictim|HitInfoBlock {
		t.Fatalf("hitInfo mismatch: 0x%x", hitInfo)
	}
	_, _ = r.ReadPackedGUID()
	_, _ = r.ReadPackedGUID()
	_, _ = r.ReadU32()
	_, _ = r.ReadU32()
	_, _ = r.ReadU8()
	_, _ = r.ReadU32()
	_, _ = r.ReadF32()
	_, _ = r.ReadU32()
	_, _ = r.ReadU8()
	_, _ = r.ReadU32()
	_, _ = r.ReadU32()
	blk, err := r.ReadU32()
	if err != nil || blk != blocked {
		t.Fatalf("blocked mismatch: %d, err: %v", blk, err)
	}
}
