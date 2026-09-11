package world

import (
	"context"
	"database/sql"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
	_ "modernc.org/sqlite"
)

func TestHandleCancelMountAura(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1, MountDisplayID: 123}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelMountAura(nil) {
			t.Error("handleCancelMountAura returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	if player.MountDisplayID != 0 {
		t.Fatalf("expected MountDisplayID 0, got %d", player.MountDisplayID)
	}
}

func TestHandleCancelGrowthAura(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
		scale:        2.5,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelGrowthAura(nil) {
			t.Error("handleCancelGrowthAura returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	if s.scale != 1.0 {
		t.Fatalf("expected scale 1.0, got %f", s.scale)
	}
}

func TestHandleCancelAutoRepeatSpell(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 42}
	s := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   42,
		player:       player,
	}

	done := make(chan struct{})
	go func() {
		if !s.handleCancelAutoRepeatSpell(nil) {
			t.Error("handleCancelAutoRepeatSpell returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_CANCEL_AUTO_REPEAT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	guid, err := r.ReadPackedGUID()
	if err != nil || guid != 42 {
		t.Fatalf("guid=%d err=%v", guid, err)
	}
}

func TestHandleCancelTempEnchantment(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{GUID: 1}
	s := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
		player:       player,
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(16) // slot 16

	done := make(chan struct{})
	go func() {
		if !s.handleCancelTempEnchantment(context.Background(), payload.Bytes()) {
			t.Error("handleCancelTempEnchantment returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, _, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opcode != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
}

func TestHandleCorpseMapPositionQuery(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	s := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   1,
	}

	payload := protocol.NewBuffer(4)
	payload.WriteU32(0)

	done := make(chan struct{})
	go func() {
		if !s.handleCorpseMapPositionQuery(payload.Bytes()) {
			t.Error("handleCorpseMapPositionQuery returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opcode, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done
	if opcode != uint16(protocol.OpcodeSMSG_CORPSE_MAP_POSITION_QUERY_RESPONSE) {
		t.Fatalf("unexpected opcode %x", opcode)
	}
	r := protocol.NewReader(data)
	for i := 0; i < 4; i++ {
		val, err := r.ReadF32()
		if err != nil || val != 0.0 {
			t.Fatalf("coord[%d]=%f err=%v", i, val, err)
		}
	}
}

func TestMissileSpellTravelDelay(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{
		creatureMotion: map[uint64]*creatureMotion{
			100: {GUID: 100, Map: 0, X: 20, Y: 0, Z: 0, Health: 100},
		},
	}
	s := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Map: 0, X: 0, Y: 0, Z: 0, Level: 1, CombatRatings: [25]uint32{CombatRatingHitSpell: 1000}},
	}
	ctx := context.Background()

	// Drain frames in background
	go func() {
		for {
			if _, _, err := readServerFrame(clientConn, nil); err != nil {
				return
			}
		}
	}()

	spell := wotlk.Spell{
		ID:    686, // Shadow Bolt
		Speed: 20,  // 20 yards/sec -> 20 yards = 1000ms
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2, BasePoints: 24}, // 25 damage
		},
	}

	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: 100,
	}

	s.finishSpellCast(ctx, 1, 686, spell, target)

	// Immediately at t=5ms, target health must NOT be damaged yet
	time.Sleep(20 * time.Millisecond)
	srv.motionMu.Lock()
	healthEarly := srv.creatureMotion[100].Health
	srv.motionMu.Unlock()
	if healthEarly != 100 {
		t.Fatalf("expected target health 100 immediately upon cast release, got %d", healthEarly)
	}

	// After travel time (1000ms + margin), target health must be damaged
	time.Sleep(1100 * time.Millisecond)
	srv.motionMu.Lock()
	healthLate := srv.creatureMotion[100].Health
	srv.motionMu.Unlock()
	if healthLate >= 100 {
		t.Fatalf("expected target health < 100 after projectile arrival, got %d", healthLate)
	}
}

func TestCalculateSpellPowerCost_ManaCostPct(t *testing.T) {
	s := &session{
		player: &playerState{
			Powers:    [7]uint32{1000, 0, 0, 100, 0, 0, 0},
			MaxPowers: [7]uint32{1000, 0, 0, 100, 0, 0, 0},
		},
	}
	// Demon Armor Rank 1: PowerType = 0 (Mana), ManaCost = 0, ManaCostPct = 12%
	spell := wotlk.Spell{
		ID:          687,
		PowerType:   0,
		ManaCost:    0,
		ManaCostPct: 12,
	}
	cost := s.calculateSpellPowerCost(spell)
	// 12% of 1000 base mana = 120 mana
	if cost != 120 {
		t.Errorf("expected cost 120, got %d", cost)
	}

	// Flat cost spell: ManaCost = 50, ManaCostPct = 0
	flatSpell := wotlk.Spell{
		ID:        123,
		PowerType: 0,
		ManaCost:  50,
	}
	if s.calculateSpellPowerCost(flatSpell) != 50 {
		t.Errorf("expected flat cost 50, got %d", s.calculateSpellPowerCost(flatSpell))
	}
}

func TestSpellDamageUsesDBCBasePointsWithoutLevelFallback(t *testing.T) {
	const creatureGUID = uint64(100)
	srv := &Server{creatureMotion: map[uint64]*creatureMotion{creatureGUID: {GUID: creatureGUID, Health: 100, MaxHealth: 100, Level: 1}}, sessions: make(map[*session]struct{})}
	sess := &session{server: srv, playerLoaded: true, playerGUID: 1, player: &playerState{GUID: 1, Level: 80, CombatRatings: [25]uint32{CombatRatingHitSpell: 1000}}}
	spell := wotlk.Spell{ID: 123, Effects: [3]wotlk.SpellEffect{{Effect: 2, BasePoints: 0}}}
	sess.finishSpellCast(context.Background(), 1, spell.ID, spell, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnit, UnitGUID: creatureGUID})
	if got := srv.creatureMotion[creatureGUID].Health; got != 99 {
		t.Fatalf("health=%d, want 99 after one DBC damage point", got)
	}
}

func TestIsSelfCastOnly(t *testing.T) {
	// Demon Armor (all active effects target TARGET_UNIT_CASTER = 1)
	selfSpell := wotlk.Spell{
		ID: 687,
		Effects: [3]wotlk.SpellEffect{
			{Effect: 6, ImplicitTargetA: 1}, // SPELL_EFFECT_APPLY_AURA on CASTER
			{Effect: 0},
			{Effect: 0},
		},
	}
	if !isSelfCastOnly(selfSpell) {
		t.Error("expected selfSpell to be identified as self-cast only")
	}

	// Shadow Bolt (targets enemy: ImplicitTargetA = 6)
	enemySpell := wotlk.Spell{
		ID: 686,
		Effects: [3]wotlk.SpellEffect{
			{Effect: 2, ImplicitTargetA: 6}, // TARGET_UNIT_TARGET_ENEMY = 6
		},
	}
	if isSelfCastOnly(enemySpell) {
		t.Error("expected enemySpell to NOT be identified as self-cast only")
	}
}

func TestSpellCastPowerDeductionAndBroadcast(t *testing.T) {
	casterServerConn, casterClientConn := net.Pipe()
	defer casterServerConn.Close()
	defer casterClientConn.Close()

	observerServerConn, observerClientConn := net.Pipe()
	defer observerServerConn.Close()
	defer observerClientConn.Close()

	srv := &Server{sessions: make(map[*session]struct{})}

	casterSess := &session{
		conn:         casterServerConn,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   100,
		player:       &playerState{GUID: 100, Powers: [7]uint32{200}, MaxPowers: [7]uint32{200}, Map: 0, X: 10, Y: 10, Z: 0},
	}
	observerSess := &session{
		conn:         observerServerConn,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   200,
		player:       &playerState{GUID: 200, Map: 0, X: 12, Y: 10, Z: 0},
	}
	srv.sessions[casterSess] = struct{}{}
	srv.sessions[observerSess] = struct{}{}

	spell := wotlk.Spell{
		ID:        133,
		PowerType: 0,  // Mana
		ManaCost:  45, // costs 45 mana
	}
	target := protocol.SpellTargetData{
		Flags:    protocol.SpellTargetFlagUnit,
		UnitGUID: 100,
	}
	type observerFrame struct {
		opcode uint16
		err    error
	}
	observerFrameCh := make(chan observerFrame, 1)
	go func() {
		_ = observerClientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		op, _, err := readServerFrame(observerClientConn, nil)
		observerFrameCh <- observerFrame{opcode: op, err: err}
	}()

	done := make(chan struct{})
	go func() {
		casterSess.finishSpellCast(context.Background(), 1, 133, spell, target)
		close(done)
	}()

	// 1. Caster reads SMSG_UPDATE_OBJECT with power delta, then SMSG_SPELL_GO
	_ = casterClientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op1, _, err := readServerFrame(casterClientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if op1 != uint16(protocol.OpcodeSMSG_POWER_UPDATE) {
		t.Fatalf("expected power update, got 0x%x", op1)
	}
	op2, _, err := readServerFrame(casterClientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if op2 != uint16(protocol.OpcodeSMSG_SPELL_GO) {
		t.Fatalf("expected SMSG_SPELL_GO, got 0x%x", op2)
	}

	// 2. Observer receives broadcast of power update
	observer := <-observerFrameCh
	if observer.err != nil {
		t.Fatal(observer.err)
	}
	if observer.opcode != uint16(protocol.OpcodeSMSG_POWER_UPDATE) {
		t.Fatalf("expected observer to receive power update, got 0x%x", observer.opcode)
	}

	<-done

	if casterSess.player.Powers[0] != 200-45 {
		t.Fatalf("expected caster mana 155, got %d", casterSess.player.Powers[0])
	}
}

func TestCancelCastInterruptsSpellTimerAndSendsFailed(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	srv := &Server{sessions: make(map[*session]struct{})}
	sess := &session{
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		server:       srv,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Powers: [7]uint32{500}, MaxPowers: [7]uint32{500}},
	}

	spell := wotlk.Spell{
		ID:        133,
		PowerType: 0,
		ManaCost:  50,
	}
	target := protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnit, UnitGUID: 1}

	// Start a 1000ms cast
	sess.castMu.Lock()
	castState := &activeCastState{
		CastID:  1,
		SpellID: 133,
	}
	castState.Timer = time.AfterFunc(1000*time.Millisecond, func() {
		sess.castMu.Lock()
		if castState.Cancelled {
			sess.castMu.Unlock()
			return
		}
		sess.activeCast = nil
		sess.castMu.Unlock()
		sess.finishSpellCast(context.Background(), 1, 133, spell, target)
	})
	sess.activeCast = castState
	sess.castMu.Unlock()

	// Player cancels cast via CMSG_CANCEL_CAST
	cancelPayload := protocol.NewBuffer(5)
	cancelPayload.WriteU8(1)
	cancelPayload.WriteU32(133)

	done := make(chan struct{})
	go func() {
		if !sess.handleCancelCast(cancelPayload.Bytes()) {
			t.Error("handleCancelCast returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED (0x13A), got 0x%x", op)
	}
	r := protocol.NewReader(data)
	cid, _ := r.ReadU8()
	sid, _ := r.ReadU32()
	res, _ := r.ReadU8()
	if cid != 1 || sid != 133 || res != 24 { // 24 = SPELL_FAILED_INTERRUPTED
		t.Fatalf("unexpected fields: castID=%d spellID=%d result=%d (expected 24)", cid, sid, res)
	}

	// Verify activeCast is nil
	sess.castMu.Lock()
	if sess.activeCast != nil {
		t.Fatal("expected activeCast to be nil after cancel")
	}
	sess.castMu.Unlock()

	// Verify mana was untouched
	if sess.player.Powers[0] != 500 {
		t.Fatalf("expected mana 500, got %d", sess.player.Powers[0])
	}
}

func TestCastSpellRejectsReentryWhileCastIsActive(t *testing.T) {
	sess := &session{}
	sess.castMu.Lock()
	sess.activeCast = &activeCastState{CastID: 1, SpellID: 686}
	sess.castMu.Unlock()
	if !sess.castInProgress() {
		t.Fatal("expected an active cast to block spell re-entry")
	}
	sess.castMu.Lock()
	sess.activeCast.Cancelled = true
	sess.castMu.Unlock()
	if sess.castInProgress() {
		t.Fatal("expected a cancelled cast to stop blocking spell re-entry")
	}
}

func TestHarmfulSpellTriggersCreatureAggroWithoutDamageEffect(t *testing.T) {
	creatureGUID := creatureWorldGUID(7, 123)
	srv := &Server{
		sessions: make(map[*session]struct{}),
		creatureMotion: map[uint64]*creatureMotion{
			creatureGUID: {GUID: creatureGUID, Entry: 123, Map: 0, Health: 1000, MaxHealth: 1000, Level: 80},
		},
	}
	sess := &session{
		server:       srv,
		playerLoaded: true,
		playerGUID:   1,
		player:       &playerState{GUID: 1, Level: 80, Map: 0},
	}
	srv.sessions[sess] = struct{}{}
	spell := wotlk.Spell{ID: 99999, Effects: [3]wotlk.SpellEffect{{Effect: 1, ImplicitTargetA: 6}}}
	target := protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnit, UnitGUID: creatureGUID}
	sess.finishSpellCast(context.Background(), 1, spell.ID, spell, target)
	srv.motionMu.Lock()
	motion := srv.creatureMotion[creatureGUID]
	threat := motion.ThreatMgr.GetThreat(sess.playerGUID)
	inCombat := motion.InCombat
	targetGUID := motion.TargetGUID
	srv.motionMu.Unlock()
	if threat <= 0 || !inCombat || targetGUID != sess.playerGUID {
		t.Fatalf("expected hostile spell aggro, threat=%f inCombat=%v target=%d", threat, inCombat, targetGUID)
	}
}

func TestHandleTotemDestroyed(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	player := &playerState{
		GUID:       101,
		TotemSlots: [4]uint64{0, 9999, 0, 0},
	}
	sess := &session{
		server:       &Server{},
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   101,
		player:       player,
	}

	payload := []byte{1} // Slot 1 (Earth)
	done := make(chan struct{})
	go func() {
		if !sess.handleTotemDestroyed(context.Background(), payload) {
			t.Error("handleTotemDestroyed returned false")
		}
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	op, data, err := readServerFrame(clientConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-done

	if op != uint16(protocol.OpcodeSMSG_TOTEM_CREATED) {
		t.Fatalf("expected SMSG_TOTEM_CREATED (0x413), got 0x%x", op)
	}
	if len(data) != 17 {
		t.Fatalf("expected 17 bytes, got %d", len(data))
	}
	r := protocol.NewReader(data)
	slot, _ := r.ReadU8()
	totem, _ := r.ReadU64()
	duration, _ := r.ReadU32()
	spellID, _ := r.ReadU32()

	if slot != 1 || totem != 0 || duration != 0 || spellID != 0 {
		t.Fatalf("unexpected totem payload: slot=%d, totem=%d, dur=%d, spell=%d", slot, totem, duration, spellID)
	}
	if player.TotemSlots[1] != 0 {
		t.Fatalf("expected TotemSlots[1] to be cleared, got %d", player.TotemSlots[1])
	}
}

func TestHandleSpellClick(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE npc_spellclick_spells (
			npc_entry INTEGER NOT NULL,
			spell_id INTEGER NOT NULL,
			cast_flags INTEGER NOT NULL,
			user_type INTEGER NOT NULL
		);
		INSERT INTO npc_spellclick_spells VALUES (28389, 51592, 1, 0);
	`)
	if err != nil {
		t.Fatal(err)
	}

	player := &playerState{
		GUID:   101,
		Level:  80,
		Powers: [7]uint32{1000},
	}
	srv := &Server{
		WorldStore: &database.Store{DB: db},
	}
	sess := &session{
		server:       srv,
		conn:         serverConn,
		authed:       true,
		playerLoaded: true,
		playerGUID:   101,
		player:       player,
	}

	// NPC with entry 28389
	npcGUID := uint64(12) | (uint64(28389) << 24) | (uint64(0xF130) << 48)
	buf := protocol.NewBuffer(8)
	buf.WriteU64(npcGUID)

	go func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var b [1024]byte
		for {
			_, err := clientConn.Read(b[:])
			if err != nil {
				return
			}
		}
	}()

	if !sess.handleSpellClick(context.Background(), buf.Bytes()) {
		t.Fatal("handleSpellClick returned false")
	}
}

func TestHandleUpdateMissileTrajectory(t *testing.T) {
	sess := &session{
		playerLoaded: true,
		player:       &playerState{GUID: 101},
	}
	buf := protocol.NewBuffer(45)
	buf.WriteU64(101)
	buf.WriteU32(133)
	buf.WriteF32(1.5)
	buf.WriteF32(20.0)
	buf.WriteF32(100.0)
	buf.WriteF32(200.0)
	buf.WriteF32(300.0)
	buf.WriteF32(150.0)
	buf.WriteF32(250.0)
	buf.WriteF32(350.0)
	buf.WriteU8(0) // moveStop = 0

	if !sess.handleUpdateMissileTrajectory(context.Background(), buf.Bytes()) {
		t.Fatal("handleUpdateMissileTrajectory failed")
	}
}

func TestHandleUpdateProjectilePosition(t *testing.T) {
	sess := &session{
		playerLoaded: true,
		player:       &playerState{GUID: 101},
	}
	buf := protocol.NewBuffer(25)
	buf.WriteU64(101)
	buf.WriteU32(133)
	buf.WriteU8(1)
	buf.WriteF32(150.0)
	buf.WriteF32(250.0)
	buf.WriteF32(350.0)

	if !sess.handleUpdateProjectilePosition(context.Background(), buf.Bytes()) {
		t.Fatal("handleUpdateProjectilePosition failed")
	}
}

func TestSpellEquippedItemRequirements(t *testing.T) {
	ctx := context.Background()
	wdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.Close()

	_, err = wdb.Exec(`CREATE TABLE item_template (entry INTEGER PRIMARY KEY, class INTEGER, subclass INTEGER, InventoryType INTEGER)`)
	if err != nil {
		t.Fatal(err)
	}
	// Item 1001: Shield (class 4, subclass 6, invtype 14)
	// Item 1002: Offhand Frill/Orb (class 4, subclass 0, invtype 23)
	// Item 2001: 1H Sword (class 2, subclass 7, invtype 13)
	// Item 2002: Dagger (class 2, subclass 15, invtype 13)
	_, _ = wdb.Exec(`INSERT INTO item_template VALUES
		(1001, 4, 6, 14),
		(1002, 4, 0, 23),
		(2001, 2, 7, 13),
		(2002, 2, 15, 13)`)

	srv := &Server{
		WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: wdb},
	}

	// 1. Shield requirement test
	shieldSpell := wotlk.Spell{
		ID:                   23922,  // Shield Slam
		EquippedItemClass:    4,      // Armor
		EquippedItemSubClass: 1 << 6, // Shield
	}

	// Session with no equipment (all zeros)
	emptyEquip := "0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0"
	sess := &session{
		server:       srv,
		playerLoaded: true,
		player: &playerState{
			GUID:      1,
			Equipment: emptyEquip,
		},
	}

	failReason, ok := sess.checkSpellEquippedItemRequirements(ctx, shieldSpell)
	if ok || failReason != spellFailedEquippedItemClass {
		t.Fatalf("expected shield spell to fail with reason 29 when naked, got ok=%v reason=%d", ok, failReason)
	}

	// Equip non-shield in offhand slot 16 (index 16*2 = 32)
	equipSlots := make([]string, 38)
	for i := range equipSlots {
		equipSlots[i] = "0"
	}
	equipSlots[16*2] = "1002" // Offhand frill
	sess.player.Equipment = ""
	for i, s := range equipSlots {
		if i > 0 {
			sess.player.Equipment += " "
		}
		sess.player.Equipment += s
	}

	failReason, ok = sess.checkSpellEquippedItemRequirements(ctx, shieldSpell)
	if ok || failReason != spellFailedEquippedItemClass {
		t.Fatalf("expected shield spell to fail with non-shield offhand, got ok=%v reason=%d", ok, failReason)
	}

	// Equip shield 1001 in offhand slot 16
	equipSlots[16*2] = "1001"
	sess.player.Equipment = ""
	for i, s := range equipSlots {
		if i > 0 {
			sess.player.Equipment += " "
		}
		sess.player.Equipment += s
	}

	failReason, ok = sess.checkSpellEquippedItemRequirements(ctx, shieldSpell)
	if !ok {
		t.Fatalf("expected shield spell to succeed with shield equipped, got ok=%v reason=%d", ok, failReason)
	}

	// 2. Main hand Dagger requirement test (e.g. Ambush / Backstab)
	daggerSpell := wotlk.Spell{
		ID:                   8676,               // Ambush
		AttributesEx3:        spellAttr3MainHand, // Requires main hand weapon
		EquippedItemClass:    2,                  // Weapon
		EquippedItemSubClass: 1 << 15,            // Dagger
	}

	// Equip Sword 2001 in mainhand slot 15 (index 15*2 = 30)
	equipSlots[15*2] = "2001"
	sess.player.Equipment = ""
	for i, s := range equipSlots {
		if i > 0 {
			sess.player.Equipment += " "
		}
		sess.player.Equipment += s
	}

	failReason, ok = sess.checkSpellEquippedItemRequirements(ctx, daggerSpell)
	if ok || failReason != spellFailedEquippedItemClass {
		t.Fatalf("expected dagger spell to fail with sword in mainhand, got ok=%v reason=%d", ok, failReason)
	}

	// Equip Dagger 2002 in mainhand slot 15
	equipSlots[15*2] = "2002"
	sess.player.Equipment = ""
	for i, s := range equipSlots {
		if i > 0 {
			sess.player.Equipment += " "
		}
		sess.player.Equipment += s
	}

	failReason, ok = sess.checkSpellEquippedItemRequirements(ctx, daggerSpell)
	if !ok {
		t.Fatalf("expected dagger spell to succeed with dagger in mainhand, got ok=%v reason=%d", ok, failReason)
	}
}

func TestHandleCastSpell_EquippedItemCheckPacketFlow(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	wdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.Close()
	_, _ = wdb.Exec(`CREATE TABLE item_template (entry INTEGER PRIMARY KEY, class INTEGER, subclass INTEGER, InventoryType INTEGER)`)

	srv := &Server{
		sessions:   make(map[*session]struct{}),
		WorldStore: &database.Store{Name: "world", Backend: database.BackendSQLite, DB: wdb},
	}
	emptyEquip := "0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0"
	sess := &session{
		server:       srv,
		conn:         serverConn,
		playerLoaded: true,
		playerGUID:   1,
		player: &playerState{
			GUID:      1,
			Equipment: emptyEquip,
			Spells: []learnedSpell{
				{ID: 23922, Active: true},
			},
		},
	}
	srv.sessions[sess] = struct{}{}

	// Cast Shield Slam (23922) with empty equipment
	dbcDir := t.TempDir()
	const fieldCount = 234
	rec := make([]uint32, fieldCount)
	rec[0] = 23922
	rec[68] = 4      // EquippedItemClass = 4
	rec[69] = 1 << 6 // EquippedItemSubClass = Shield
	recBytes := make([]byte, fieldCount*4)
	for i, val := range rec {
		binary.LittleEndian.PutUint32(recBytes[i*4:(i+1)*4], val)
	}
	header := make([]byte, 20)
	copy(header, "WDBC")
	binary.LittleEndian.PutUint32(header[4:8], 1)
	binary.LittleEndian.PutUint32(header[8:12], fieldCount)
	binary.LittleEndian.PutUint32(header[12:16], fieldCount*4)
	binary.LittleEndian.PutUint32(header[16:20], 1)
	_ = os.WriteFile(filepath.Join(dbcDir, "Spell.dbc"), append(header, append(recBytes, 0)...), 0o644)
	srv.Data = wotlk.NewStore(dbcDir)

	payload := protocol.NewBuffer(32)
	payload.WriteU8(1)      // castID
	payload.WriteU32(23922) // spellID
	payload.WriteU8(0)      // castFlags
	protocol.WriteSpellTargetData(payload, protocol.SpellTargetData{Flags: protocol.SpellTargetFlagUnit, UnitGUID: 1})

	done := make(chan struct{})
	var op uint16
	var p []byte
	go func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(1 * time.Second))
		op, p, _ = readServerFrame(clientConn, nil)
		close(done)
	}()

	if !sess.handleCastSpell(context.Background(), payload.Bytes()) {
		t.Fatal("handleCastSpell failed")
	}
	<-done

	if op != uint16(protocol.OpcodeSMSG_CAST_FAILED) {
		t.Fatalf("expected SMSG_CAST_FAILED, got 0x%04X", op)
	}
	r := protocol.NewReader(p)
	_, _ = r.ReadU8() // castID
	spID, _ := r.ReadU32()
	reason, _ := r.ReadU8()
	if spID != 23922 || reason != spellFailedEquippedItemClass {
		t.Fatalf("expected spell 23922 failed with reason 29, got spell=%d reason=%d", spID, reason)
	}
}
