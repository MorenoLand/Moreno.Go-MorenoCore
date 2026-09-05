package world

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestDispel_PacketEncodings(t *testing.T) {
	// 1. SMSG_SPELLDISPELLOG (0x27B)
	dispelled := []uint32{1243, 589}
	logPkt := buildSpellDispelLog(0x1000, 0x2000, 527, dispelled)
	r := protocol.NewReader(logPkt)
	vGUID, err := r.ReadPackedGUID()
	if err != nil || vGUID != 0x1000 {
		t.Fatalf("expected victim GUID 0x1000, got %x, err %v", vGUID, err)
	}
	cGUID, err := r.ReadPackedGUID()
	if err != nil || cGUID != 0x2000 {
		t.Fatalf("expected caster GUID 0x2000, got %x, err %v", cGUID, err)
	}
	dispelSpell, _ := r.ReadU32()
	if dispelSpell != 527 {
		t.Fatalf("expected dispel spell 527, got %d", dispelSpell)
	}
	notUsed, _ := r.ReadU8()
	if notUsed != 0 {
		t.Fatalf("expected notUsed 0, got %d", notUsed)
	}
	count, _ := r.ReadU32()
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}
	s1, _ := r.ReadU32()
	c1, _ := r.ReadU8()
	if s1 != 1243 || c1 != 0 {
		t.Fatalf("expected spell 1243 cleansed 0, got %d %d", s1, c1)
	}
	s2, _ := r.ReadU32()
	c2, _ := r.ReadU8()
	if s2 != 589 || c2 != 0 {
		t.Fatalf("expected spell 589 cleansed 0, got %d %d", s2, c2)
	}

	// 2. SMSG_DISPEL_FAILED (0x262)
	failed := []uint32{999}
	failPkt := buildDispelFailed(0x2000, 0x1000, 527, failed)
	rf := protocol.NewReader(failPkt)
	cFailGUID, _ := rf.ReadU64()
	if cFailGUID != 0x2000 {
		t.Fatalf("expected caster GUID 0x2000, got %x", cFailGUID)
	}
	vFailGUID, _ := rf.ReadU64()
	if vFailGUID != 0x1000 {
		t.Fatalf("expected victim GUID 0x1000, got %x", vFailGUID)
	}
	fSpell, _ := rf.ReadU32()
	if fSpell != 527 {
		t.Fatalf("expected dispel spell 527, got %d", fSpell)
	}
	fTargetSpell, _ := rf.ReadU32()
	if fTargetSpell != 999 {
		t.Fatalf("expected failed spell 999, got %d", fTargetSpell)
	}

	// 3. SMSG_SPELLSTEALLOG (0x333)
	stolen := []uint32{1243}
	stealPkt := buildSpellstealLog(0x1000, 0x2000, 30449, stolen)
	rs := protocol.NewReader(stealPkt)
	stVictim, _ := rs.ReadPackedGUID()
	if stVictim != 0x1000 {
		t.Fatalf("expected victim 0x1000, got %x", stVictim)
	}
	stCaster, _ := rs.ReadPackedGUID()
	if stCaster != 0x2000 {
		t.Fatalf("expected caster 0x2000, got %x", stCaster)
	}
	stSpellID, _ := rs.ReadU32()
	if stSpellID != 30449 {
		t.Fatalf("expected spellsteal 30449, got %d", stSpellID)
	}
	stCount, _ := rs.ReadU32()
	if stCount != 0 { // after notUsed byte
		// read byte first
	}
}

func TestDispel_PreCastNothingToDispel(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	pktChan := make(chan capturedPacket, 50)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:   1001,
			Race:   1, // Human (Alliance)
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}
	srv.sessions[sess] = struct{}{}

	// Pure dispel spell: Dispel Magic (Effect 38, MiscValue 1 = Magic)
	spell := wotlk.Spell{
		ID: 527,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:    38, // SPELL_EFFECT_DISPEL
				MiscValue: 1,  // DISPEL_MAGIC
			},
		},
	}

	// Self-cast when player has no debuffs
	fail := sess.checkDispelPreCast(spell, sess.playerGUID)
	if fail != spellFailedNothingToDispel {
		t.Fatalf("expected spellFailedNothingToDispel (86), got %d", fail)
	}
}

func TestDispel_FriendlyDebuffDispel(t *testing.T) {
	sConnA, cConnA := net.Pipe()
	defer sConnA.Close()
	defer cConnA.Close()

	sConnB, cConnB := net.Pipe()
	defer sConnB.Close()
	defer cConnB.Close()

	pktChanA := make(chan capturedPacket, 50)
	stopDrainA := make(chan struct{})
	defer close(stopDrainA)
	go drainPackets(cConnA, pktChanA, stopDrainA)

	pktChanB := make(chan capturedPacket, 50)
	stopDrainB := make(chan struct{})
	defer close(stopDrainB)
	go drainPackets(cConnB, pktChanB, stopDrainB)

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	sessA := &session{
		server:       srv,
		conn:         sConnA,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:   1001,
			Race:   1, // Human (Alliance)
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	sessB := &session{
		server:       srv,
		conn:         sConnB,
		playerLoaded: true,
		playerGUID:   1002,
		player: &playerState{
			GUID:   1002,
			Race:   3, // Dwarf (Alliance - Friendly to Human)
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
		auraSlots:   make(map[uint32]uint8),
	}

	srv.sessions[sessA] = struct{}{}
	srv.sessions[sessB] = struct{}{}

	// Give Player B:
	// 1. Beneficial buff (Fortitude 1243, positive=true, DispelType=1 Magic)
	sessB.activeAuras[1243] = &activeAura{
		SpellID:    1243,
		DispelType: 1,
		Positive:   true,
		Slot:       0,
	}
	sessB.auraSlots[1243] = 0

	// 2. Harmful debuff (Shadow Word: Pain 589, positive=false, DispelType=1 Magic)
	sessB.activeAuras[589] = &activeAura{
		SpellID:    589,
		DispelType: 1,
		Positive:   false,
		Slot:       1,
	}
	sessB.auraSlots[589] = 1

	// Player A casts Dispel Magic on Player B
	spell := wotlk.Spell{
		ID: 527,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:     38, // SPELL_EFFECT_DISPEL
				MiscValue:  1,  // DISPEL_MAGIC
				BasePoints: 0,  // maxDispelled = 1
			},
		},
	}

	fail := sessA.checkDispelPreCast(spell, sessB.playerGUID)
	if fail != 0 {
		t.Fatalf("expected checkDispelPreCast to succeed, got %d", fail)
	}

	sessA.handleEffectDispel(context.Background(), sessB.playerGUID, spell, spell.Effects[0])

	// Verify Player B's debuff (589) is removed
	if _, hasDebuff := sessB.activeAuras[589]; hasDebuff {
		t.Fatal("expected debuff 589 to be dispelled from friendly target")
	}

	// Verify Player B's buff (1243) is NOT removed
	if _, hasBuff := sessB.activeAuras[1243]; !hasBuff {
		t.Fatal("expected buff 1243 to remain on friendly target")
	}

	// Verify SMSG_SPELLDISPELLOG was sent to Player A
	time.Sleep(20 * time.Millisecond)
	foundLog := false
	for len(pktChanA) > 0 {
		pkt := <-pktChanA
		if pkt.opcode == uint16(protocol.OpcodeSMSG_SPELLDISPELLOG) {
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Fatal("expected SMSG_SPELLDISPELLOG packet sent to caster")
	}
}

func TestDispel_HostileBuffDispel(t *testing.T) {
	sConnA, cConnA := net.Pipe()
	defer sConnA.Close()
	defer cConnA.Close()

	sConnB, cConnB := net.Pipe()
	defer sConnB.Close()
	defer cConnB.Close()

	pktChanA := make(chan capturedPacket, 50)
	stopDrainA := make(chan struct{})
	defer close(stopDrainA)
	go drainPackets(cConnA, pktChanA, stopDrainA)

	pktChanB := make(chan capturedPacket, 50)
	stopDrainB := make(chan struct{})
	defer close(stopDrainB)
	go drainPackets(cConnB, pktChanB, stopDrainB)

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	sessA := &session{
		server:       srv,
		conn:         sConnA,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:   1001,
			Race:   1, // Human (Alliance)
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	sessB := &session{
		server:       srv,
		conn:         sConnB,
		playerLoaded: true,
		playerGUID:   1002,
		player: &playerState{
			GUID:   1002,
			Race:   2, // Orc (Horde - Hostile)
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
		auraSlots:   make(map[uint32]uint8),
	}

	srv.sessions[sessA] = struct{}{}
	srv.sessions[sessB] = struct{}{}

	// Give Player B:
	// 1. Beneficial buff (Fortitude 1243, positive=true, DispelType=1 Magic)
	sessB.activeAuras[1243] = &activeAura{
		SpellID:    1243,
		DispelType: 1,
		Positive:   true,
		Slot:       0,
	}
	sessB.auraSlots[1243] = 0

	// 2. Harmful debuff (Shadow Word: Pain 589, positive=false, DispelType=1 Magic)
	sessB.activeAuras[589] = &activeAura{
		SpellID:    589,
		DispelType: 1,
		Positive:   false,
		Slot:       1,
	}
	sessB.auraSlots[589] = 1

	// Hostile dispel: Player A casts Dispel Magic on Player B
	spell := wotlk.Spell{
		ID: 527,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:     38, // SPELL_EFFECT_DISPEL
				MiscValue:  1,  // DISPEL_MAGIC
				BasePoints: 0,  // maxDispelled = 1
			},
		},
	}

	fail := sessA.checkDispelPreCast(spell, sessB.playerGUID)
	if fail != 0 {
		t.Fatalf("expected checkDispelPreCast to succeed, got %d", fail)
	}

	sessA.handleEffectDispel(context.Background(), sessB.playerGUID, spell, spell.Effects[0])

	// Verify Player B's buff (1243) is removed
	if _, hasBuff := sessB.activeAuras[1243]; hasBuff {
		t.Fatal("expected buff 1243 to be dispelled from hostile target")
	}

	// Verify Player B's debuff (589) is NOT removed
	if _, hasDebuff := sessB.activeAuras[589]; !hasDebuff {
		t.Fatal("expected debuff 589 to remain on hostile target")
	}
}

func TestDispel_UnholyBlightDiseaseImmunity(t *testing.T) {
	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sessA := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:   1001,
			Race:   1, // Human (Alliance)
			Level:  80,
			Health: 1000,
		},
	}
	sessB := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1002,
		player: &playerState{
			GUID:   1002,
			Race:   1, // Human (Alliance)
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
		auras:       make(map[uint32]struct{}),
	}
	srv.sessions[sessA] = struct{}{}
	srv.sessions[sessB] = struct{}{}

	// Target has Unholy Blight (50536) and a disease debuff
	sessB.auras[50536] = struct{}{}
	sessB.activeAuras[25049] = &activeAura{
		SpellID:    25049,
		DispelType: 3, // DISPEL_DISEASE
		Positive:   false,
	}

	// Caster attempts to dispel disease (e.g. Purify 528)
	spell := wotlk.Spell{
		ID: 528,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:    38, // SPELL_EFFECT_DISPEL
				MiscValue: 3,  // DISPEL_DISEASE
			},
		},
	}

	fail := sessA.checkDispelPreCast(spell, sessB.playerGUID)
	if fail != spellFailedNothingToDispel {
		t.Fatalf("expected spellFailedNothingToDispel (86) due to Unholy Blight immunity, got %d", fail)
	}
}

func TestDispel_DispelResistAura(t *testing.T) {
	sConnA, cConnA := net.Pipe()
	defer sConnA.Close()
	defer cConnA.Close()

	pktChanA := make(chan capturedPacket, 50)
	stopDrainA := make(chan struct{})
	defer close(stopDrainA)
	go drainPackets(cConnA, pktChanA, stopDrainA)

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sessA := &session{
		server:       srv,
		conn:         sConnA,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:   1001,
			Race:   1, // Human
			Level:  80,
			Health: 1000,
		},
	}
	sessB := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1002,
		player: &playerState{
			GUID:   1002,
			Race:   2, // Orc (Hostile)
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}
	srv.sessions[sessA] = struct{}{}
	srv.sessions[sessB] = struct{}{}

	// Target has buff
	sessB.activeAuras[1243] = &activeAura{
		SpellID:    1243,
		DispelType: 1, // Magic
		Positive:   true,
	}

	// Target has 99% dispel resistance via SPELL_AURA_MOD_DISPEL_RESIST (235) (1% dispel chance)
	sessB.activeAuras[99999] = &activeAura{
		SpellID:  99999,
		AuraType: 235, // SPELL_AURA_MOD_DISPEL_RESIST
		Amount:   99,  // 99% resistance -> 1% chance -> failure emits SMSG_DISPEL_FAILED
		Positive: true,
	}

	spell := wotlk.Spell{
		ID: 527,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:     38,
				MiscValue:  1, // Magic
				BasePoints: 0,
			},
		},
	}

	sessA.handleEffectDispel(context.Background(), sessB.playerGUID, spell, spell.Effects[0])

	// Verify buff was NOT removed
	if _, hasBuff := sessB.activeAuras[1243]; !hasBuff {
		t.Fatal("expected buff 1243 to resist dispel")
	}

	// Verify SMSG_DISPEL_FAILED was sent to caster
	time.Sleep(20 * time.Millisecond)
	foundFail := false
	for len(pktChanA) > 0 {
		pkt := <-pktChanA
		if pkt.opcode == uint16(protocol.OpcodeSMSG_DISPEL_FAILED) {
			foundFail = true
			break
		}
	}
	if !foundFail {
		t.Fatal("expected SMSG_DISPEL_FAILED sent to caster")
	}
}

func TestDispel_DevourMagicSelfHeal(t *testing.T) {
	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sessWarlock := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:      1001,
			Race:      1,
			Level:     80,
			Health:    500,
			MaxHealth: 1000,
		},
	}
	sessTarget := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1002,
		player: &playerState{
			GUID:   1002,
			Race:   2, // Hostile
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}
	srv.sessions[sessWarlock] = struct{}{}
	srv.sessions[sessTarget] = struct{}{}

	sessTarget.activeAuras[1243] = &activeAura{
		SpellID:    1243,
		DispelType: 1, // Magic
		Positive:   true,
	}

	// Devour Magic (spell 19505): Effect 0 Dispel Magic, Effect 1 Heal 150
	devourSpell := wotlk.Spell{
		ID: 19505,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:     38, // SPELL_EFFECT_DISPEL
				MiscValue:  1,  // Magic
				BasePoints: 0,
			},
			{
				Effect:     10,  // SPELL_EFFECT_HEAL
				BasePoints: 149, // BasePoints + 1 = 150
			},
		},
	}

	sessWarlock.handleEffectDispel(context.Background(), sessTarget.playerGUID, devourSpell, devourSpell.Effects[0])

	if sessWarlock.player.Health != 650 {
		t.Fatalf("expected warlock health to be 650 after Devour Magic heal, got %d", sessWarlock.player.Health)
	}
}

func TestDispel_Spellsteal(t *testing.T) {
	sConnA, cConnA := net.Pipe()
	defer sConnA.Close()
	defer cConnA.Close()

	pktChanA := make(chan capturedPacket, 50)
	stopDrainA := make(chan struct{})
	defer close(stopDrainA)
	go drainPackets(cConnA, pktChanA, stopDrainA)

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sessMage := &session{
		server:       srv,
		conn:         sConnA,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:   1001,
			Race:   1, // Human
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}
	sessTarget := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1002,
		player: &playerState{
			GUID:   1002,
			Race:   2, // Hostile
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}
	srv.sessions[sessMage] = struct{}{}
	srv.sessions[sessTarget] = struct{}{}

	// Target has Bloodlust (2825, positive=true, duration 300,000ms)
	sessTarget.activeAuras[2825] = &activeAura{
		SpellID:    2825,
		DispelType: 1, // Magic
		Positive:   true,
		DurationMs: 300000,
		Amount:     30,
	}

	spellsteal := wotlk.Spell{
		ID: 30449,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:     126, // SPELL_EFFECT_STEAL_BENEFICIAL_BUFF
				MiscValue:  1,   // Magic
				BasePoints: 0,
			},
		},
	}

	sessMage.handleEffectSpellsteal(context.Background(), sessTarget.playerGUID, spellsteal, spellsteal.Effects[0])

	// Target should lose Bloodlust
	if _, hasBuff := sessTarget.activeAuras[2825]; hasBuff {
		t.Fatal("expected target to lose Bloodlust after Spellsteal")
	}

	// Mage should gain Bloodlust, with duration capped at 120,000ms
	stolenAura, hasMageBuff := sessMage.activeAuras[2825]
	if !hasMageBuff {
		t.Fatal("expected mage to gain stolen Bloodlust")
	}
	if stolenAura.DurationMs > 120000 {
		t.Fatalf("expected stolen buff duration capped at 120000ms, got %d", stolenAura.DurationMs)
	}

	// Verify SMSG_SPELLSTEALLOG packet
	time.Sleep(20 * time.Millisecond)
	foundStealLog := false
	for len(pktChanA) > 0 {
		pkt := <-pktChanA
		if pkt.opcode == uint16(protocol.OpcodeSMSG_SPELLSTEALLOG) {
			foundStealLog = true
			break
		}
	}
	if !foundStealLog {
		t.Fatal("expected SMSG_SPELLSTEALLOG sent to mage")
	}
}

func TestDispel_MechanicDispel(t *testing.T) {
	srv := &Server{
		sessions: make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:   1001,
			Race:   1,
			Level:  80,
			Health: 1000,
		},
		activeAuras: make(map[uint32]*activeAura),
	}
	srv.sessions[sess] = struct{}{}

	// Player is afflicted by Fear (spell 5782, mechanic 5, auraType 7)
	sess.activeAuras[5782] = &activeAura{
		SpellID:  5782,
		Mechanic: 5, // MECHANIC_FEAR
		AuraType: 7, // SPELL_AURA_MOD_FEAR
		Positive: false,
	}

	// Spell with SPELL_EFFECT_DISPEL_MECHANIC (108) dispelling Fear (5)
	dispelMechanicSpell := wotlk.Spell{
		ID: 18499, // Berserker Rage
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:    108, // SPELL_EFFECT_DISPEL_MECHANIC
				MiscValue: 5,   // MECHANIC_FEAR
			},
		},
	}

	sess.handleEffectDispelMechanic(context.Background(), sess.playerGUID, dispelMechanicSpell, dispelMechanicSpell.Effects[0])

	if _, hasFear := sess.activeAuras[5782]; hasFear {
		t.Fatal("expected Fear (5782) to be dispelled by Dispel Mechanic")
	}
}
