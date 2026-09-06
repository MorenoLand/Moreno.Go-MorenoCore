package world

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestSpellReflection_HarmfulSpellReflectedToCaster(t *testing.T) {
	sConnCaster, cConnCaster := net.Pipe()
	defer sConnCaster.Close()
	defer cConnCaster.Close()

	sConnTarget, cConnTarget := net.Pipe()
	defer sConnTarget.Close()
	defer cConnTarget.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	caster := &session{
		server:       srv,
		conn:         sConnCaster,
		playerGUID:   10,
		playerLoaded: true,
		player:       &playerState{GUID: 10, Level: 80, Health: 10000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
	}

	warrior := &session{
		server:       srv,
		conn:         sConnTarget,
		playerGUID:   20,
		playerLoaded: true,
		player:       &playerState{GUID: 20, Level: 80, Health: 10000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
	}

	srv.sessions[caster] = struct{}{}
	srv.sessions[warrior] = struct{}{}

	// Give warrior Spell Reflection buff (23920)
	warrior.castMu.Lock()
	warrior.auras[23920] = struct{}{}
	warrior.auraSlots[23920] = 0
	warrior.activeAuras[23920] = &activeAura{
		SpellID:    23920,
		TargetGUID: 20,
		AuraType:   spellAuraReflectSpells, // 63
	}
	warrior.castMu.Unlock()

	go func() {
		for {
			if _, _, err := readServerFrame(cConnTarget, nil); err != nil {
				return
			}
		}
	}()

	var casterOpcodes []uint16
	var casterLock sync.Mutex
	doneRead := make(chan struct{})
	go func() {
		defer close(doneRead)
		for {
			_ = cConnCaster.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
			op, _, err := readServerFrame(cConnCaster, nil)
			if err != nil {
				return
			}
			casterLock.Lock()
			casterOpcodes = append(casterOpcodes, op)
			casterLock.Unlock()
		}
	}()

	// Caster casts offensive spell (Frostbolt: effect 2 = damage, SchoolMask = 16) targeting warrior
	frostbolt := wotlk.Spell{
		ID:         116,
		SchoolMask: 16,
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2, BasePoints: 999}, // ~1000 damage
		},
	}
	targetData := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: warrior.playerGUID,
	}

	caster.finishSpellCast(context.Background(), 1, 116, frostbolt, targetData)

	// 1. Verify warrior's Spell Reflection buff was consumed
	if warrior.hasAura(23920) {
		t.Fatalf("expected warrior's Spell Reflection (23920) to be consumed on reflect")
	}

	// 2. Verify caster took the reflected damage
	if caster.player.Health >= 10000 {
		t.Fatalf("expected caster to take reflected damage, health is %d", caster.player.Health)
	}

	// 3. Verify warrior took NO damage
	if warrior.player.Health != 10000 {
		t.Fatalf("expected warrior to take 0 damage, health is %d", warrior.player.Health)
	}

	time.Sleep(50 * time.Millisecond)
	_ = cConnCaster.Close()
	<-doneRead

	casterLock.Lock()
	hasSpellGo := false
	for _, op := range casterOpcodes {
		if op == uint16(protocol.OpcodeSMSG_SPELL_GO) {
			hasSpellGo = true
			break
		}
	}
	casterLock.Unlock()

	if !hasSpellGo {
		t.Fatalf("expected SMSG_SPELL_GO sent to caster with reflect miss status")
	}
}

func TestImmunity_DivineShieldAndIceBlock(t *testing.T) {
	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	attacker := &session{
		server:       srv,
		playerGUID:   10,
		playerLoaded: true,
		player:       &playerState{GUID: 10, Level: 80, Health: 10000, MaxHealth: 10000, AttackTime: 2000},
	}

	paladin := &session{
		server:       srv,
		playerGUID:   20,
		playerLoaded: true,
		player:       &playerState{GUID: 20, Level: 80, Health: 10000, MaxHealth: 10000},
		auras:        make(map[uint32]struct{}),
		auraSlots:    make(map[uint32]uint8),
		activeAuras:  make(map[uint32]*activeAura),
	}

	srv.sessions[attacker] = struct{}{}
	srv.sessions[paladin] = struct{}{}

	// Give paladin Divine Shield (642)
	paladin.auras[642] = struct{}{}

	// 1. Verify damage immunity
	if !paladin.isImmuneToDamage(1) { // Physical
		t.Fatalf("expected Divine Shield to be immune to Physical damage")
	}
	if !paladin.isImmuneToDamage(16) { // Frost
		t.Fatalf("expected Divine Shield to be immune to Frost damage")
	}

	// 2. Verify spell immunity to harmful spells
	harmfulSpell := wotlk.Spell{
		ID: 116,
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2, BasePoints: 500},
		},
	}
	if !paladin.isImmuneToSpell(harmfulSpell) {
		t.Fatalf("expected Divine Shield to be immune to harmful spell")
	}

	// 3. Beneficial spell is NOT immune under Divine Shield (can be healed)
	healSpell := wotlk.Spell{
		ID: 2050, // Lesser Heal
		Effects: [3]wotlk.SpellEffect{
			{Effect: 10, BasePoints: 500},
		},
	}
	if paladin.isImmuneToSpell(healSpell) {
		t.Fatalf("expected Divine Shield NOT to be immune to healing spells")
	}

	// 4. Test Ice Block (45438)
	mage := &session{
		player: &playerState{GUID: 30, Level: 80},
		auras:  map[uint32]struct{}{45438: {}},
	}
	if !mage.isImmuneToDamage(1) || !mage.isImmuneToDamage(4) {
		t.Fatalf("expected Ice Block to grant total damage immunity")
	}
	if !mage.isImmuneToSpell(harmfulSpell) {
		t.Fatalf("expected Ice Block to grant spell immunity to harmful spells")
	}
}

func TestImmunity_BlessingOfProtection(t *testing.T) {
	bopTarget := &session{
		player: &playerState{GUID: 40, Level: 80},
		auras:  map[uint32]struct{}{1022: {}}, // Blessing of Protection (1022)
	}

	// 1. Physical damage is IMMUNE
	if !bopTarget.isImmuneToDamage(1) {
		t.Fatalf("expected BoP to grant physical damage immunity")
	}

	// 2. Magic damage (Fire = 4, Frost = 16) is NOT immune
	if bopTarget.isImmuneToDamage(4) {
		t.Fatalf("expected BoP NOT to grant Fire damage immunity")
	}
	if bopTarget.isImmuneToDamage(16) {
		t.Fatalf("expected BoP NOT to grant Frost damage immunity")
	}

	// 3. Physical harmful spell (e.g. Sinister Strike, Gouge) is IMMUNE
	physHarmful := wotlk.Spell{
		ID:         1776,
		SchoolMask: 1, // Physical
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2},
		},
	}
	if !bopTarget.isImmuneToSpell(physHarmful) {
		t.Fatalf("expected BoP to grant immunity to physical spells")
	}

	// 4. Magic harmful spell is NOT immune
	magicHarmful := wotlk.Spell{
		ID:         116,
		SchoolMask: 16, // Frost
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2},
		},
	}
	if bopTarget.isImmuneToSpell(magicHarmful) {
		t.Fatalf("expected BoP NOT to grant immunity to magic spells")
	}
}

func TestImmunity_CycloneBlocksAllSpells(t *testing.T) {
	cyclonedTarget := &session{
		player: &playerState{GUID: 50, Level: 80},
		auras:  map[uint32]struct{}{33786: {}}, // Cyclone (33786)
	}

	// 1. Total damage immunity
	if !cyclonedTarget.isImmuneToDamage(1) || !cyclonedTarget.isImmuneToDamage(16) {
		t.Fatalf("expected Cyclone to grant total damage immunity")
	}

	// 2. Harmful spell is IMMUNE
	harmful := wotlk.Spell{
		ID:      116,
		Effects: [3]wotlk.SpellEffect{{Effect: 2}},
	}
	if !cyclonedTarget.isImmuneToSpell(harmful) {
		t.Fatalf("expected Cyclone to block harmful spells")
	}

	// 3. Beneficial spell (Heal) is ALSO IMMUNE (cannot heal cycloned target)
	heal := wotlk.Spell{
		ID:      2050,
		Effects: [3]wotlk.SpellEffect{{Effect: 10}},
	}
	if !cyclonedTarget.isImmuneToSpell(heal) {
		t.Fatalf("expected Cyclone to block beneficial/healing spells")
	}
}
