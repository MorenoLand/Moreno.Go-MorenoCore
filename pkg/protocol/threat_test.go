package protocol

import "testing"

func TestThreatPackets(t *testing.T) {
	creatureGUID := uint64(100500)
	p1GUID := uint64(10)
	p2GUID := uint64(20)

	// 1. Threat Clear
	clearBytes := BuildThreatClear(creatureGUID)
	r := NewReader(clearBytes)
	guid, err := r.ReadPackedGUID()
	if err != nil || guid != creatureGUID {
		t.Fatalf("expected creatureGUID %d, got %d (err: %v)", creatureGUID, guid, err)
	}

	// 2. Threat Remove
	removeBytes := BuildThreatRemove(creatureGUID, p1GUID)
	r = NewReader(removeBytes)
	guid, err = r.ReadPackedGUID()
	if err != nil || guid != creatureGUID {
		t.Fatalf("expected creatureGUID %d, got %d", creatureGUID, guid)
	}
	victim, err := r.ReadPackedGUID()
	if err != nil || victim != p1GUID {
		t.Fatalf("expected victimGUID %d, got %d", p1GUID, victim)
	}

	// 3. Threat Update
	list := []ThreatEntry{
		{VictimGUID: p1GUID, Threat: 50000},
		{VictimGUID: p2GUID, Threat: 25000},
	}
	updateBytes := BuildThreatUpdate(creatureGUID, list)
	r = NewReader(updateBytes)
	guid, err = r.ReadPackedGUID()
	if err != nil || guid != creatureGUID {
		t.Fatalf("expected creatureGUID %d, got %d", creatureGUID, guid)
	}
	count, err := r.ReadU32()
	if err != nil || count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
	v1, _ := r.ReadPackedGUID()
	t1, _ := r.ReadU32()
	v2, _ := r.ReadPackedGUID()
	t2, _ := r.ReadU32()
	if v1 != p1GUID || t1 != 50000 || v2 != p2GUID || t2 != 25000 {
		t.Fatalf("unexpected list entries: %d:%d, %d:%d", v1, t1, v2, t2)
	}

	// 4. Highest Threat Update
	highestBytes := BuildHighestThreatUpdate(creatureGUID, p1GUID, list)
	r = NewReader(highestBytes)
	guid, _ = r.ReadPackedGUID()
	highest, _ := r.ReadPackedGUID()
	if guid != creatureGUID || highest != p1GUID {
		t.Fatalf("expected creature %d highest %d, got %d, %d", creatureGUID, p1GUID, guid, highest)
	}
	count, _ = r.ReadU32()
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
}

func TestBuildMonsterChatMessage(t *testing.T) {
	data := BuildMonsterChatMessage(14, 0, 500, "Edwin VanCleef", "None may challenge the Brotherhood!")
	r := NewReader(data)
	chatType, _ := r.ReadU8()
	if chatType != 14 {
		t.Fatalf("expected chatType 14, got %d", chatType)
	}
	lang, _ := r.ReadU32()
	if lang != 0 {
		t.Fatalf("expected lang 0, got %d", lang)
	}
	senderGUID, _ := r.ReadU64()
	if senderGUID != 500 {
		t.Fatalf("expected senderGUID 500, got %d", senderGUID)
	}
	flags, _ := r.ReadU32()
	if flags != 0 {
		t.Fatalf("expected flags 0, got %d", flags)
	}
	nameLen, _ := r.ReadU32()
	if nameLen != uint32(len("Edwin VanCleef")+1) {
		t.Fatalf("expected nameLen %d, got %d", len("Edwin VanCleef")+1, nameLen)
	}
	name, _ := r.ReadCString()
	if name != "Edwin VanCleef" {
		t.Fatalf("expected name 'Edwin VanCleef', got %q", name)
	}
	receiverGUID, _ := r.ReadU64()
	if receiverGUID != 0 {
		t.Fatalf("expected receiverGUID 0, got %d", receiverGUID)
	}
	msgLen, _ := r.ReadU32()
	if msgLen != uint32(len("None may challenge the Brotherhood!")+1) {
		t.Fatalf("expected msgLen %d, got %d", len("None may challenge the Brotherhood!")+1, msgLen)
	}
	msg, _ := r.ReadCString()
	if msg != "None may challenge the Brotherhood!" {
		t.Fatalf("expected msg 'None may challenge the Brotherhood!', got %q", msg)
	}
}
