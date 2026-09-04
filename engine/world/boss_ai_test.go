package world

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestThreatManagerTargetSwitchingAndBroadcastParity(t *testing.T) {
	c1, s1 := net.Pipe()
	defer c1.Close()
	defer s1.Close()

	c2, s2 := net.Pipe()
	defer c2.Close()
	defer s2.Close()

	srv := &Server{
		creatureMotion: make(map[uint64]*creatureMotion),
		sessions:       make(map[*session]struct{}),
	}

	p1 := &session{
		server:       srv,
		conn:         s1,
		playerGUID:   10,
		authed:       true,
		playerLoaded: true,
		player: &playerState{
			GUID:   10,
			Map:    0,
			X:      100.0,
			Y:      100.0,
			Z:      10.0,
			Health: 500,
		},
	}
	p2 := &session{
		server:       srv,
		conn:         s2,
		playerGUID:   20,
		authed:       true,
		playerLoaded: true,
		player: &playerState{
			GUID:   20,
			Map:    0,
			X:      102.0,
			Y:      100.0,
			Z:      10.0,
			Health: 500,
		},
	}
	srv.sessions[p1] = struct{}{}
	srv.sessions[p2] = struct{}{}

	p1Ops := make(chan uint16, 20)
	p2Ops := make(chan uint16, 20)
	go func() {
		for {
			op, _, err := readServerFrame(c1, nil)
			if err != nil {
				return
			}
			p1Ops <- op
		}
	}()
	go func() {
		for {
			op, _, err := readServerFrame(c2, nil)
			if err != nil {
				return
			}
			p2Ops <- op
		}
	}()

	creatureGUID := uint64(5000)
	motion := &creatureMotion{
		GUID:       creatureGUID,
		Entry:      100,
		Map:        0,
		HomeX:      100.0,
		HomeY:      100.0,
		HomeZ:      10.0,
		X:          100.0,
		Y:          100.0,
		Z:          10.0,
		Speed:      2.5,
		RunSpeed:   7.0,
		Health:     1000,
		MaxHealth:  1000,
		AttackTime: 2000,
		ThreatMgr:  NewThreatManager(creatureGUID),
	}
	srv.creatureMotion[creatureGUID] = motion

	ctx := context.Background()

	// 1. Initial aggro by p1 -> 100 threat
	srv.triggerCreatureAggro(ctx, creatureGUID, 10)
	if motion.TargetGUID != 10 {
		t.Fatalf("expected initial target p1 (10), got %d", motion.TargetGUID)
	}
	if motion.ThreatMgr.GetThreat(10) != 100.0 {
		t.Fatalf("expected 100 threat on p1, got %f", motion.ThreatMgr.GetThreat(10))
	}

	// 2. p2 attacks in melee range dealing 5 threat -> total 5 threat (less than 110% of 100) -> no switch
	switched, victim := motion.ThreatMgr.AddThreat(20, 5, true)
	if switched || victim != 10 {
		t.Fatalf("unexpected target switch on low threat: switched=%v victim=%d", switched, victim)
	}

	// 3. p2 deals 120 damage in melee range -> total 125 threat (> 110% of 100) -> switches to p2!
	switched, victim = motion.ThreatMgr.AddThreat(20, 120, true)
	if !switched || victim != 20 {
		t.Fatalf("expected target switch to p2: switched=%v victim=%d", switched, victim)
	}
	motion.TargetGUID = victim

	// Broadcast highest threat update
	entries := motion.ThreatMgr.SortedEntries()
	srv.broadcastHighestThreatUpdate(motion.Map, motion.GUID, victim, entries)

	select {
	case op := <-p1Ops:
		// Drain any initial packets until SMSG_HIGHEST_THREAT_UPDATE
		found := op == uint16(protocol.OpcodeSMSG_HIGHEST_THREAT_UPDATE)
		for !found {
			select {
			case nextOp := <-p1Ops:
				if nextOp == uint16(protocol.OpcodeSMSG_HIGHEST_THREAT_UPDATE) {
					found = true
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("timeout waiting for p1 SMSG_HIGHEST_THREAT_UPDATE")
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for p1 SMSG_HIGHEST_THREAT_UPDATE")
	}

	// 4. p2 runs > 45 yards away -> evades from p2, creature should switch back to p1 (the remaining target in threat table)!
	players := []playerPos{
		{
			GUID: 10,
			Map:  0,
			X:    100.0,
			Y:    100.0,
			Z:    10.0,
			Sess: p1,
		},
		{
			GUID: 20,
			Map:  0,
			X:    200.0, // 100 yards away
			Y:    100.0,
			Z:    10.0,
			Sess: p2,
		},
	}

	srv.stepCreatureMotion(ctx, motion, players, time.Now())

	// Verify creature switched target back to p1 instead of dropping combat!
	if motion.TargetGUID != 10 {
		t.Fatalf("expected creature to switch back to p1 after p2 evaded, got %d", motion.TargetGUID)
	}
	if !motion.InCombat {
		t.Fatalf("expected creature to still be in combat with p1")
	}
}

func TestEdwinVanCleefBossAIScriptParity(t *testing.T) {
	c1, s1 := net.Pipe()
	defer c1.Close()
	defer s1.Close()

	srv := &Server{
		creatureMotion:  make(map[uint64]*creatureMotion),
		sessions:        make(map[*session]struct{}),
		creatureTextMgr: newCreatureTextMgr(),
	}

	p1 := &session{
		server:       srv,
		conn:         s1,
		playerGUID:   10,
		authed:       true,
		playerLoaded: true,
		player: &playerState{
			GUID:   10,
			Map:    36, // Deadmines
			X:      -78.0,
			Y:      -820.0,
			Z:      40.0,
			Health: 1000,
		},
	}
	srv.sessions[p1] = struct{}{}

	// Populate mock creature_text for VanCleef
	srv.creatureTextMgr.texts[639] = map[uint8][]CreatureText{
		0: {{CreatureID: 639, GroupID: 0, Text: "None may challenge the Brotherhood!", Type: 14, Sound: 5780}},
		1: {{CreatureID: 639, GroupID: 1, Text: "Lapdogs, all of you!", Type: 14, Sound: 5782}},
		2: {{CreatureID: 639, GroupID: 2, Text: "%s calls more of his allies out of the shadows.", Type: 16}},
		3: {{CreatureID: 639, GroupID: 3, Text: "Fools! Our cause is righteous!", Type: 14, Sound: 5783}},
		4: {{CreatureID: 639, GroupID: 4, Text: "And stay down!", Type: 14, Sound: 5781}},
		5: {{CreatureID: 639, GroupID: 5, Text: "The Brotherhood shall prevail!", Type: 14, Sound: 5784}},
	}

	p1Ops := make(chan uint16, 50)
	go func() {
		for {
			op, _, err := readServerFrame(c1, nil)
			if err != nil {
				return
			}
			p1Ops <- op
		}
	}()

	vancleefGUID := uint64(0xF130000000000000) | uint64(639)<<24 | 1
	motion := &creatureMotion{
		GUID:       vancleefGUID,
		Entry:      639,
		Name:       "Edwin VanCleef",
		ScriptName: "boss_vancleef",
		Map:        36,
		HomeX:      -78.0,
		HomeY:      -820.0,
		HomeZ:      40.0,
		X:          -78.0,
		Y:          -820.0,
		Z:          40.0,
		Health:     1000,
		MaxHealth:  1000,
		AttackTime: 2000,
		ThreatMgr:  NewThreatManager(vancleefGUID),
	}
	motion.BossAI = newVanCleefAI(motion)
	srv.creatureMotion[vancleefGUID] = motion

	ctx := context.Background()

	// 1. Aggro -> Talk(0) "None may challenge the Brotherhood!" + Sound 5780
	motion.BossAI.OnAggro(ctx, srv, motion, 10)

	// Check p1 received SMSG_PLAY_SOUND (0x2D2) and SMSG_MESSAGECHAT (0x096)
	hasSound := false
	hasChat := false
	for i := 0; i < 2; i++ {
		select {
		case op := <-p1Ops:
			if op == uint16(protocol.OpcodeSMSG_PLAY_SOUND) {
				hasSound = true
			} else if op == uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
				hasChat = true
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for aggro sound/chat")
		}
	}
	if !hasSound || !hasChat {
		t.Fatalf("expected sound and chat on aggro: sound=%v chat=%v", hasSound, hasChat)
	}

	// 2. Health dropped to 600 (60%) -> triggers 66% quote "Lapdogs, all of you!"
	motion.Health = 600
	motion.BossAI.OnDamageTaken(ctx, srv, motion, 10, 400)

	hasSound = false
	hasChat = false
	for i := 0; i < 2; i++ {
		select {
		case op := <-p1Ops:
			if op == uint16(protocol.OpcodeSMSG_PLAY_SOUND) {
				hasSound = true
			} else if op == uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
				hasChat = true
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timeout waiting for 66% sound/chat")
		}
	}
	if !hasSound || !hasChat {
		t.Fatalf("expected sound and chat on 66%%%% HP: sound=%v chat=%v", hasSound, hasChat)
	}

	// 3. Health dropped to 450 (45%) -> triggers 50% allies call and summons 2 Defias Blackguards!
	motion.Health = 450
	motion.BossAI.OnDamageTaken(ctx, srv, motion, 10, 150)

	// Verify 2 summons were spawned into srv.creatureMotion
	blackguardCount := 0
	for _, m := range srv.creatureMotion {
		if m.Entry == 636 {
			blackguardCount++
		}
	}
	if blackguardCount != 2 {
		t.Fatalf("expected 2 summoned Defias Blackguards (entry 636), got %d", blackguardCount)
	}

	// 4. Test Evade: Cleans up summons and resets flags
	motion.BossAI.OnEvade(ctx, srv, motion)
	blackguardCount = 0
	for _, m := range srv.creatureMotion {
		if m.Entry == 636 {
			blackguardCount++
		}
	}
	if blackguardCount != 0 {
		t.Fatalf("expected 0 summons remaining after evade, got %d", blackguardCount)
	}
}

func TestMrSmiteBossAIScriptParity(t *testing.T) {
	c1, s1 := net.Pipe()
	defer c1.Close()
	defer s1.Close()

	srv := &Server{
		creatureMotion:  make(map[uint64]*creatureMotion),
		sessions:        make(map[*session]struct{}),
		creatureTextMgr: newCreatureTextMgr(),
	}

	p1 := &session{
		server:       srv,
		conn:         s1,
		playerGUID:   10,
		authed:       true,
		playerLoaded: true,
		player: &playerState{
			GUID:   10,
			Map:    36,
			X:      0,
			Y:      0,
			Z:      0,
			Health: 1000,
		},
	}
	srv.sessions[p1] = struct{}{}

	srv.creatureTextMgr.texts[646] = map[uint8][]CreatureText{
		0: {{CreatureID: 646, GroupID: 0, Text: "You there, check out that noise!", Type: 14, Sound: 5775}},
		2: {{CreatureID: 646, GroupID: 2, Text: "You landlubbers are tougher than I thought, I'll have to Improvise!", Type: 12, Sound: 5778}},
		3: {{CreatureID: 646, GroupID: 3, Text: "D'ah! Now you're making me angry!", Type: 12, Sound: 5779}},
	}

	p1Ops := make(chan uint16, 50)
	go func() {
		for {
			op, _, err := readServerFrame(c1, nil)
			if err != nil {
				return
			}
			p1Ops <- op
		}
	}()

	smiteGUID := uint64(0xF130000000000000) | uint64(646)<<24 | 2
	motion := &creatureMotion{
		GUID:       smiteGUID,
		Entry:      646,
		Name:       "Mr. Smite",
		ScriptName: "boss_mr_smite",
		Map:        36,
		Health:     1000,
		MaxHealth:  1000,
		AttackTime: 2000,
		TargetGUID: 10,
		InCombat:   true,
		ThreatMgr:  NewThreatManager(smiteGUID),
	}
	motion.BossAI = newMrSmiteAI(motion)
	srv.creatureMotion[smiteGUID] = motion

	ctx := context.Background()

	// 1. Health dropped to 600 (60%) -> triggers phase 1: Smite Stomp stun (6432) + Talk(2)
	motion.Health = 600
	motion.BossAI.OnDamageTaken(ctx, srv, motion, 10, 400)

	hasSpellGo := false
	hasChat := false
	for i := 0; i < 3; i++ {
		select {
		case op := <-p1Ops:
			if op == uint16(protocol.OpcodeSMSG_SPELL_GO) {
				hasSpellGo = true
			} else if op == uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
				hasChat = true
			}
		case <-time.After(500 * time.Millisecond):
			break
		}
	}
	if !hasSpellGo || !hasChat {
		t.Fatalf("expected Smite Stomp spell and chat on phase 1: spell=%v chat=%v", hasSpellGo, hasChat)
	}

	// 2. Health dropped to 300 (30%) -> triggers phase 2: Smite Stomp stun (6432) + Talk(3)
	motion.Health = 300
	motion.BossAI.OnDamageTaken(ctx, srv, motion, 10, 300)

	hasSpellGo = false
	hasChat = false
	for i := 0; i < 3; i++ {
		select {
		case op := <-p1Ops:
			if op == uint16(protocol.OpcodeSMSG_SPELL_GO) {
				hasSpellGo = true
			} else if op == uint16(protocol.OpcodeSMSG_MESSAGECHAT) {
				hasChat = true
			}
		case <-time.After(500 * time.Millisecond):
			break
		}
	}
	if !hasSpellGo || !hasChat {
		t.Fatalf("expected Smite Stomp spell and chat on phase 2: spell=%v chat=%v", hasSpellGo, hasChat)
	}
}
