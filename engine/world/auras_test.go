package world

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

type capturedPacket struct {
	opcode uint16
	data   []byte
}

func drainPackets(conn net.Conn, ch chan<- capturedPacket, stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
			_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			op, data, err := readServerFrame(conn, nil)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}
			ch <- capturedPacket{opcode: op, data: data}
		}
	}
}

func TestMagicSpellHitResult_Formulas(t *testing.T) {
	// 1. Level diff < 3: modHitChance = 96 - leveldif
	// Equal level: caster 80, target 80 -> diff 0 -> hit chance 96%
	// Equal level: caster 70, target 72 -> diff 2 -> hit chance 94%
	// Level diff >= 3: modHitChance = 94 - (leveldif-2)*lchance
	// Target is creature (lchance = 11): caster 70, target 73 -> diff 3 -> 94 - 1*11 = 83%
	// Target is player (lchance = 7): caster 70, target 73 -> diff 3 -> 94 - 1*7 = 87%
	// Target is 11 levels higher: caster 70, target 81 -> diff 11 -> 94 - 9*11 = -5 -> clamped to 1%
	// Target is lower level: caster 80, target 70 -> diff -10 -> 96 - (-10) = 106 -> clamped to 100%

	// We test that when modHitChance is 100, magicSpellHitResult NEVER misses
	for i := 0; i < 200; i++ {
		res := magicSpellHitResult(80, 1, false)
		if res != protocol.SpellMissNone {
			t.Fatalf("expected 100%% hit chance for level 80 vs 1, got miss: %d", res)
		}
	}

	// We test that when target is much higher level, magicSpellHitResult can miss
	missCount := 0
	total := 1000
	for i := 0; i < total; i++ {
		res := magicSpellHitResult(1, 80, false)
		if res == protocol.SpellMissMiss {
			missCount++
		}
	}
	// Target 80 vs Level 1 has modHitChance = 1% -> ~99% miss rate
	if missCount < 900 {
		t.Fatalf("expected high miss count for level 1 vs 80, got %d / %d", missCount, total)
	}
}

func TestPeriodicAura_DamageTickOnCreature(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	creatureGUID := uint64(101) | (uint64(500) << 24) | (uint64(0xF130) << 48)

	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			creatureGUID: {
				GUID:      creatureGUID,
				Health:    500,
				MaxHealth: 500,
				Level:     1,
			},
		},
		activeCreatureAuras: make(map[uint64]map[uint32]*activeAura),
		creatureAuras:       make(map[uint64]map[uint32]struct{}),
		sessions:            make(map[*session]struct{}),
	}

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1001,
		player: &playerState{
			GUID:      1001,
			Level:     80, // Level 80 ensures 100% spell hit chance
			Health:    1000,
			MaxHealth: 1000,
		},
	}
	srv.sessions[sess] = struct{}{}

	pktChan := make(chan capturedPacket, 100)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	dotSpell := wotlk.Spell{
		ID:            12345,
		DurationIndex: 0,
		SchoolMask:    32, // Shadow
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:          6,  // SPELL_EFFECT_APPLY_AURA
				Aura:            3,  // SPELL_AURA_PERIODIC_DAMAGE
				BasePoints:      19, // Amount = BasePoints + 1 = 20
				AuraPeriod:      20, // 20ms period
				ImplicitTargetA: 6,  // TARGET_UNIT_TARGET_ENEMY
			},
		},
	}

	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: creatureGUID,
	}

	ctx := context.Background()
	sess.finishSpellCast(ctx, 1, 12345, dotSpell, target)

	// Wait for at least 2 ticks (~60ms)
	time.Sleep(60 * time.Millisecond)

	// Verify creature took damage
	srv.motionMu.Lock()
	creatureHealth := srv.creatureMotion[creatureGUID].Health
	threatMgr := srv.creatureMotion[creatureGUID].ThreatMgr
	srv.motionMu.Unlock()

	if creatureHealth >= 500 {
		t.Fatalf("expected creature health to decrease below 500, got %d", creatureHealth)
	}
	if threatMgr == nil || threatMgr.GetThreat(1001) == 0 {
		t.Fatalf("expected creature threat to be tracked for caster 1001")
	}

	// Verify packets received: SMSG_AURA_UPDATE and SMSG_PERIODICAURALOG
	hasAuraUpdate := false
	hasPeriodicLog := false
	for len(pktChan) > 0 {
		p := <-pktChan
		if p.opcode == uint16(protocol.OpcodeSMSG_AURA_UPDATE) {
			hasAuraUpdate = true
		}
		if p.opcode == uint16(protocol.OpcodeSMSG_PERIODICAURALOG) {
			hasPeriodicLog = true
			r := protocol.NewReader(p.data)
			tgt, err := r.ReadPackedGUID()
			if err != nil || tgt != creatureGUID {
				t.Fatalf("unexpected target in periodic log: %d, err: %v", tgt, err)
			}
			caster, err := r.ReadPackedGUID()
			if err != nil || caster != 1001 {
				t.Fatalf("unexpected caster in periodic log: %d, err: %v", caster, err)
			}
			sid, _ := r.ReadU32()
			if sid != 12345 {
				t.Fatalf("expected spellID 12345, got %d", sid)
			}
			cnt, _ := r.ReadU32()
			if cnt != 1 {
				t.Fatalf("expected count 1, got %d", cnt)
			}
			atype, _ := r.ReadU32()
			if atype != 3 {
				t.Fatalf("expected auraType 3 (damage), got %d", atype)
			}
			dmg, _ := r.ReadU32()
			if dmg != 20 {
				t.Fatalf("expected damage 20, got %d", dmg)
			}
		}
	}

	if !hasAuraUpdate {
		t.Errorf("expected SMSG_AURA_UPDATE packet")
	}
	if !hasPeriodicLog {
		t.Errorf("expected SMSG_PERIODICAURALOG packet")
	}

	// Wait for expiration (duration = AuraPeriod * 5 = 100ms)
	time.Sleep(80 * time.Millisecond)

	srv.auraMu.Lock()
	activeCount := len(srv.activeCreatureAuras[creatureGUID])
	srv.auraMu.Unlock()

	if activeCount != 0 {
		t.Fatalf("expected 0 active creature auras after expiration, got %d", activeCount)
	}
}

func TestPeriodicAura_DamageKillCreature(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	creatureGUID := uint64(202) | (uint64(500) << 24) | (uint64(0xF130) << 48)

	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			creatureGUID: {
				GUID:      creatureGUID,
				Health:    15, // Less than DoT tick (20 dmg)
				MaxHealth: 100,
				Level:     1,
			},
		},
		activeCreatureAuras: make(map[uint64]map[uint32]*activeAura),
		creatureAuras:       make(map[uint64]map[uint32]struct{}),
		sessions:            make(map[*session]struct{}),
	}

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   2001,
		player: &playerState{
			GUID:      2001,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
		},
	}
	srv.sessions[sess] = struct{}{}

	pktChan := make(chan capturedPacket, 100)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	dotSpell := wotlk.Spell{
		ID:         54321,
		SchoolMask: 32,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:          6,
				Aura:            3,
				BasePoints:      19, // 20 damage
				AuraPeriod:      20, // 20ms period
				ImplicitTargetA: 6,
			},
		},
	}

	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: creatureGUID,
	}

	sess.finishSpellCast(context.Background(), 1, 54321, dotSpell, target)

	// Wait for tick and kill processing
	time.Sleep(50 * time.Millisecond)

	srv.motionMu.Lock()
	motion := srv.creatureMotion[creatureGUID]
	srv.motionMu.Unlock()

	if motion.Health != 0 {
		t.Fatalf("expected creature health to be 0 (killed), got %d", motion.Health)
	}

	// Verify creature auras stopped and cleared
	srv.auraMu.Lock()
	activeCount := len(srv.activeCreatureAuras[creatureGUID])
	srv.auraMu.Unlock()

	if activeCount != 0 {
		t.Fatalf("expected creature auras to be cleaned up on death, got %d", activeCount)
	}
}

func TestPeriodicAura_HealTickOnPlayer(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		sessions: make(map[*session]struct{}),
	}

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   3001,
		player: &playerState{
			GUID:      3001,
			Level:     80,
			Health:    50,
			MaxHealth: 200,
		},
	}
	srv.sessions[sess] = struct{}{}

	pktChan := make(chan capturedPacket, 100)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	hotSpell := wotlk.Spell{
		ID: 77777,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:          6,  // APPLY_AURA
				Aura:            8,  // SPELL_AURA_PERIODIC_HEAL
				BasePoints:      24, // 25 heal
				AuraPeriod:      20, // 20ms period
				ImplicitTargetA: 1,  // TARGET_UNIT_CASTER
			},
		},
	}

	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: 3001,
	}

	sess.finishSpellCast(context.Background(), 1, 77777, hotSpell, target)

	// Wait for ticks
	time.Sleep(60 * time.Millisecond)

	if sess.player.Health <= 50 {
		t.Fatalf("expected player health to increase from HoT, got %d", sess.player.Health)
	}

	// Verify periodic heal log was emitted
	hasPeriodicHeal := false
	for len(pktChan) > 0 {
		p := <-pktChan
		if p.opcode == uint16(protocol.OpcodeSMSG_PERIODICAURALOG) {
			hasPeriodicHeal = true
			r := protocol.NewReader(p.data)
			tgt, _ := r.ReadPackedGUID()
			caster, _ := r.ReadPackedGUID()
			sid, _ := r.ReadU32()
			cnt, _ := r.ReadU32()
			atype, _ := r.ReadU32()
			heal, _ := r.ReadU32()
			if tgt != 3001 || caster != 3001 || sid != 77777 || cnt != 1 || atype != 8 || heal != 25 {
				t.Fatalf("unexpected heal packet contents: tgt=%d caster=%d sid=%d cnt=%d atype=%d heal=%d", tgt, caster, sid, cnt, atype, heal)
			}
		}
	}

	if !hasPeriodicHeal {
		t.Errorf("expected SMSG_PERIODICAURALOG for HoT")
	}

	// Wait for expiration
	time.Sleep(80 * time.Millisecond)

	sess.castMu.Lock()
	activeCount := len(sess.activeAuras)
	sess.castMu.Unlock()

	if activeCount != 0 {
		t.Fatalf("expected 0 active player auras after expiration, got %d", activeCount)
	}
}

func TestSpellCrit_NonMeleeDamageLogPacket(t *testing.T) {
	// Verify layout matches TrinityCore Unit::SendSpellNonMeleeDamageLog
	targetGUID := uint64(555)
	attackerGUID := uint64(666)
	spellID := uint32(133)
	damage := uint32(150)
	overkill := uint32(0)
	schoolMask := uint8(4) // Fire
	hitInfo := uint32(0x02) // SPELL_HIT_TYPE_CRIT

	logBytes := buildSpellNonMeleeDamageLog(targetGUID, attackerGUID, spellID, damage, overkill, schoolMask, 0, 0, hitInfo)
	r := protocol.NewReader(logBytes)

	tgt, err := r.ReadPackedGUID()
	if err != nil || tgt != targetGUID {
		t.Fatalf("targetGUID mismatch: %d err: %v", tgt, err)
	}
	atk, err := r.ReadPackedGUID()
	if err != nil || atk != attackerGUID {
		t.Fatalf("attackerGUID mismatch: %d err: %v", atk, err)
	}
	sid, _ := r.ReadU32()
	if sid != spellID {
		t.Fatalf("spellID mismatch: %d", sid)
	}
	dmg, _ := r.ReadU32()
	if dmg != damage {
		t.Fatalf("damage mismatch: %d", dmg)
	}
	okill, _ := r.ReadU32()
	if okill != overkill {
		t.Fatalf("overkill mismatch: %d", okill)
	}
	smask, _ := r.ReadU8()
	if smask != schoolMask {
		t.Fatalf("schoolMask mismatch: %d", smask)
	}
	absorb, _ := r.ReadU32()
	if absorb != 0 {
		t.Fatalf("absorb mismatch: %d", absorb)
	}
	resist, _ := r.ReadU32()
	if resist != 0 {
		t.Fatalf("resist mismatch: %d", resist)
	}
	periodic, _ := r.ReadU8()
	if periodic != 0 {
		t.Fatalf("periodic mismatch: %d", periodic)
	}
	unused, _ := r.ReadU8()
	if unused != 0 {
		t.Fatalf("unused mismatch: %d", unused)
	}
	blocked, _ := r.ReadU32()
	if blocked != 0 {
		t.Fatalf("blocked mismatch: %d", blocked)
	}
	hinfo, _ := r.ReadU32()
	if hinfo != hitInfo {
		t.Fatalf("hitInfo mismatch: expected 0x02, got 0x%x", hinfo)
	}
}

func TestClearActiveAuras(t *testing.T) {
	srv := &Server{
		activeCreatureAuras: make(map[uint64]map[uint32]*activeAura),
		creatureAuras:       make(map[uint64]map[uint32]struct{}),
	}
	sess := &session{
		server:      srv,
		activeAuras: make(map[uint32]*activeAura),
	}

	creatureGUID := uint64(999)
	srv.activeCreatureAuras[creatureGUID] = map[uint32]*activeAura{
		101: {SpellID: 101, Stopped: false},
	}
	srv.creatureAuras[creatureGUID] = map[uint32]struct{}{
		101: {},
	}

	sess.activeAuras[202] = &activeAura{SpellID: 202, Stopped: false}

	// Clear creature auras
	srv.clearCreatureAuras(creatureGUID)
	srv.auraMu.Lock()
	if len(srv.activeCreatureAuras[creatureGUID]) != 0 {
		t.Fatalf("expected activeCreatureAuras to be cleared")
	}
	if len(srv.creatureAuras[creatureGUID]) != 0 {
		t.Fatalf("expected creatureAuras to be cleared")
	}
	srv.auraMu.Unlock()

	// Clear session auras
	sess.clearActiveAuras()
	sess.castMu.Lock()
	if len(sess.activeAuras) != 0 {
		t.Fatalf("expected sess.activeAuras to be cleared")
	}
	sess.castMu.Unlock()
}

func TestFinishSpellCast_SpellMiss(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	creatureGUID := uint64(777) | (uint64(500) << 24) | (uint64(0xF130) << 48)

	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			creatureGUID: {
				GUID:      creatureGUID,
				Health:    1000,
				MaxHealth: 1000,
				Level:     80, // Target is level 80
			},
		},
		activeCreatureAuras: make(map[uint64]map[uint32]*activeAura),
		creatureAuras:       make(map[uint64]map[uint32]struct{}),
		sessions:            make(map[*session]struct{}),
	}

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   4001,
		player: &playerState{
			GUID:      4001,
			Level:     1, // Level 1 vs 80 gives 99% miss chance
			Health:    1000,
			MaxHealth: 1000,
			Powers:    [7]uint32{200}, // 200 mana
		},
	}
	srv.sessions[sess] = struct{}{}

	pktChan := make(chan capturedPacket, 100)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	spell := wotlk.Spell{
		ID:        686,
		PowerType: 0,
		ManaCost:  50,
		Effects: [3]wotlk.SpellEffect{
			{
				Effect:          2, // SPELL_EFFECT_SCHOOL_DAMAGE
				BasePoints:      100,
				ImplicitTargetA: 6,
			},
		},
	}

	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: creatureGUID,
	}

	// Try casting until a miss occurs (level 1 vs 80 has 99% miss chance, virtually 1st try)
	for i := 0; i < 20; i++ {
		sess.player.Powers[0] = 200
		srv.creatureMotion[creatureGUID].Health = 1000
		sess.finishSpellCast(context.Background(), uint8(i+1), 686, spell, target)
		time.Sleep(20 * time.Millisecond)

		hasMiss := false
		for len(pktChan) > 0 {
			p := <-pktChan
			if p.opcode == uint16(protocol.OpcodeSMSG_SPELL_GO) {
				// Parse SMSG_SPELL_GO
				r := protocol.NewReader(p.data)
				_, _ = r.ReadPackedGUID() // caster
				_, _ = r.ReadPackedGUID() // casterUnit
				_, _ = r.ReadU8()         // castID
				_, _ = r.ReadU32()        // spellID
				_, _ = r.ReadU32()        // flags
				_, _ = r.ReadU32()        // timestamp
				hitCount, _ := r.ReadU8()
				for h := uint8(0); h < hitCount; h++ {
					_, _ = r.ReadU64()
				}
				missCount, _ := r.ReadU8()
				if missCount > 0 {
					missTgt, _ := r.ReadU64()
					reason, _ := r.ReadU8()
					if missTgt == creatureGUID && reason == protocol.SpellMissMiss {
						hasMiss = true
					}
				}
			}
		}

		if hasMiss {
			// Verify mana was deducted
			if sess.player.Powers[0] != 150 {
				t.Fatalf("expected mana 150 after miss, got %d", sess.player.Powers[0])
			}
			// Verify creature health untouched
			if srv.creatureMotion[creatureGUID].Health != 1000 {
				t.Fatalf("expected creature health untouched (1000), got %d", srv.creatureMotion[creatureGUID].Health)
			}
			return
		}
	}

	t.Fatalf("expected at least one spell miss in 20 attempts for level 1 vs 80")
}

func TestCalcMagicSpellResistance(t *testing.T) {
	// 1. Physical (1) and Holy (2) cannot be resisted
	res, rem := calcMagicSpellResistance(100, 1, 500, 80, 80)
	if res != 0 || rem != 100 {
		t.Fatalf("expected 0 physical resistance, got %d", res)
	}
	res, rem = calcMagicSpellResistance(100, 2, 500, 80, 80)
	if res != 0 || rem != 100 {
		t.Fatalf("expected 0 holy resistance, got %d", res)
	}

	// 2. Equal level with 0 resistance has 0 resist
	res, rem = calcMagicSpellResistance(100, 4, 0, 80, 80)
	if res != 0 || rem != 100 {
		t.Fatalf("expected 0 fire resistance for 0 rating, got %d", res)
	}

	// 3. Fire against 200 resistance at level 80 (avg resist ~33%)
	resistedTotal := uint32(0)
	for i := 0; i < 50; i++ {
		r, _ := calcMagicSpellResistance(100, 4, 200, 80, 80)
		resistedTotal += r
	}
	if resistedTotal == 0 {
		t.Fatalf("expected magic resistance to mitigate damage against 200 resistance")
	}
}

func writeSpellWithSchoolDBC(t *testing.T, dir string, id, schoolMask uint32) {
	t.Helper()
	const fieldCount = 234
	record := make([]uint32, fieldCount)
	record[0] = id
	record[225] = schoolMask
	recordBytes := make([]byte, fieldCount*4)
	for i, val := range record {
		binary.LittleEndian.PutUint32(recordBytes[i*4:(i+1)*4], val)
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 1)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	if err := os.WriteFile(filepath.Join(dir, "Spell.dbc"), append(header, append(recordBytes, 0)...), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteSpellDamage_ResistanceMitigation(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	creatureGUID := uint64(888) | (uint64(500) << 24) | (uint64(0xF130) << 48)

	var resArray [7]uint32
	resArray[5] = 250 // Shadow resistance = 250

	dbcDir := t.TempDir()
	writeSpellWithSchoolDBC(t, dbcDir, 686, 32)

	srv := &Server{
		Data: wotlk.NewStore(dbcDir),
		creatureMotion: map[uint64]*creatureMotion{
			creatureGUID: {
				GUID:        creatureGUID,
				Health:      5000,
				MaxHealth:   5000,
				Level:       80,
				Resistances: resArray,
			},
		},
		activeCreatureAuras: make(map[uint64]map[uint32]*activeAura),
		creatureAuras:       make(map[uint64]map[uint32]struct{}),
		sessions:            make(map[*session]struct{}),
	}

	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   5001,
		player: &playerState{
			GUID:      5001,
			Level:     80,
			Health:    1000,
			MaxHealth: 1000,
		},
	}
	srv.sessions[sess] = struct{}{}

	pktChan := make(chan capturedPacket, 100)
	stopDrain := make(chan struct{})
	defer close(stopDrain)
	go drainPackets(clientConn, pktChan, stopDrain)

	// Cast Shadow damage spell (schoolMask = 32)
	sawResist := false
	for i := 0; i < 20; i++ {
		sess.executeSpellDamage(context.Background(), creatureGUID, 686, 200)
		time.Sleep(10 * time.Millisecond)

		for len(pktChan) > 0 {
			p := <-pktChan
			if p.opcode == uint16(protocol.OpcodeSMSG_SPELLNONMELEEDAMAGELOG) {
				r := protocol.NewReader(p.data)
				_, _ = r.ReadPackedGUID() // target
				_, _ = r.ReadPackedGUID() // attacker
				_, _ = r.ReadU32()        // spellID
				_, _ = r.ReadU32()        // damage
				_, _ = r.ReadU32()        // overkill
				_, _ = r.ReadU8()         // schoolMask
				_, _ = r.ReadU32()        // absorb
				resist, _ := r.ReadU32()  // resist
				if resist > 0 {
					sawResist = true
				}
			}
		}
		if sawResist {
			break
		}
	}

	if !sawResist {
		t.Fatalf("expected at least one spell resist against 250 shadow resistance")
	}
}

func TestCreatureEvade_ClearsAuras(t *testing.T) {
	creatureGUID := uint64(991) | (uint64(500) << 24) | (uint64(0xF130) << 48)
	now := time.Now()

	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			creatureGUID: {
				GUID:       creatureGUID,
				HomeX:      0,
				HomeY:      0,
				HomeZ:      0,
				X:          10,
				Y:          0,
				Z:          0,
				Health:     50,
				MaxHealth:  100,
				InCombat:   true,
				TargetGUID: 7001,
				RunSpeed:   7.0,
			},
		},
		activeCreatureAuras: map[uint64]map[uint32]*activeAura{
			creatureGUID: {
				12345: {SpellID: 12345, Stopped: false},
			},
		},
		creatureAuras: map[uint64]map[uint32]struct{}{
			creatureGUID: {
				12345: {},
			},
		},
		sessions: make(map[*session]struct{}),
	}

	// Player is far away (60 yards > 45.0 max leash)
	players := []playerPos{
		{
			GUID: 7001,
			Map:  0,
			X:    60,
			Y:    0,
			Z:    0,
		},
	}

	motion := srv.creatureMotion[creatureGUID]
	srv.stepCreatureMotion(context.Background(), motion, players, now)

	// Creature must be in evade: health restored to 100, InCombat = false, auras cleared
	if motion.Health != 100 {
		t.Fatalf("expected creature health restored to 100 on evade, got %d", motion.Health)
	}
	if motion.InCombat {
		t.Fatalf("expected creature InCombat false on evade")
	}

	srv.auraMu.Lock()
	auraCount := len(srv.activeCreatureAuras[creatureGUID])
	srv.auraMu.Unlock()

	if auraCount != 0 {
		t.Fatalf("expected 0 active auras after creature evade, got %d", auraCount)
	}
}


