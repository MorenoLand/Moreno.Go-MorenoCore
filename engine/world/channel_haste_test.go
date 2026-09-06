package world

import (
	"math"
	"net"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
)

func TestChannelHaste_DurationAndTickCompression(t *testing.T) {
	sConn, cConn := net.Pipe()
	defer sConn.Close()
	defer cConn.Close()

	srv := &Server{
		Data: wotlk.NewStore("../../data/dbc"),
	}

	sess := &session{
		server:       srv,
		conn:         sConn,
		playerLoaded: true,
		playerGUID:   4001,
		player: &playerState{
			GUID:          4001,
			Level:         80,
			Class:         5, // Priest
			CombatRatings: [25]uint32{CombatRatingHasteSpell: 1640}, // ~50% haste
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	go func() {
		for {
			if _, _, err := readServerFrame(cConn, nil); err != nil {
				return
			}
		}
	}()

	// Mind Flay (15407): duration 3000ms (DurationIndex 27), tick period 1000ms
	mindFlay := wotlk.Spell{
		ID:            15407,
		DurationIndex: 27, // 3000ms
		Effects: [3]wotlk.SpellEffect{
			{Effect: 6, Aura: 3, AuraPeriod: 1000},
		},
	}

	sess.startChannel(1, 15407, mindFlay, 500)

	sess.castMu.Lock()
	ch := sess.activeChannel
	sess.castMu.Unlock()

	if ch == nil {
		t.Fatalf("expected active channel to be created")
	}

	// 50% haste: 3000ms / 1.50 = 2000ms duration, 1000ms / 1.50 = 667ms period
	if ch.DurationMs != 2000 {
		t.Fatalf("expected compressed channel duration 2000ms at 50%% haste, got %d", ch.DurationMs)
	}
	if ch.PeriodMs != 667 {
		t.Fatalf("expected compressed tick period 667ms at 50%% haste, got %d", ch.PeriodMs)
	}
}

func TestSpellCritMultiplier_TalentsAndMetagem(t *testing.T) {
	sess := &session{
		player: &playerState{
			Level:  80,
			Spells: []learnedSpell{},
		},
		activeAuras: make(map[uint32]*activeAura),
	}

	shadowBolt := wotlk.Spell{ID: 686, SchoolMask: 32} // Shadow
	frostbolt := wotlk.Spell{ID: 116, SchoolMask: 16}   // Frost

	// 1. Base crit multiplier: 1.5x (150%)
	if mult := sess.getSpellCritMultiplier(shadowBolt); mult != 1.5 {
		t.Fatalf("expected base crit multiplier 1.5, got %f", mult)
	}

	// 2. Ruin talent (17959): gives +100% bonus (2.0x total) for Destruction spells (Shadow 32 / Fire 4)
	sess.player.Spells = append(sess.player.Spells, learnedSpell{ID: 17959, Active: true})
	if mult := sess.getSpellCritMultiplier(shadowBolt); mult != 2.0 {
		t.Fatalf("expected Ruin crit multiplier 2.0 for Shadow Bolt, got %f", mult)
	}
	// Frostbolt does not benefit from Ruin
	if mult := sess.getSpellCritMultiplier(frostbolt); mult != 1.5 {
		t.Fatalf("expected Frostbolt to remain at 1.5 base crit with Ruin, got %f", mult)
	}

	// 3. Add Chaotic Skyflare Diamond (26297: +3% crit damage)
	// Total with Ruin: 2.0 * 1.03 = 2.06x (206%)
	sess.activeAuras[26297] = &activeAura{
		SpellID:  26297,
		AuraType: 0,
	}
	if mult := sess.getSpellCritMultiplier(shadowBolt); math.Abs(mult-2.06) > 0.001 {
		t.Fatalf("expected Ruin + Chaotic Skyflare multiplier 2.06, got %f", mult)
	}
	// Frostbolt with Chaotic Skyflare only: 1.5 * 1.03 = 1.545x
	if mult := sess.getSpellCritMultiplier(frostbolt); math.Abs(mult-1.545) > 0.001 {
		t.Fatalf("expected Frostbolt + Chaotic Skyflare multiplier 1.545, got %f", mult)
	}

	// 4. Ice Shards talent (15058): gives +100% bonus for Frost spells
	sess.player.Spells = append(sess.player.Spells, learnedSpell{ID: 15058, Active: true})
	// Frostbolt with Ice Shards (2.0) and Chaotic Skyflare (1.03): 2.06
	if mult := sess.getSpellCritMultiplier(frostbolt); math.Abs(mult-2.06) > 0.001 {
		t.Fatalf("expected Frostbolt with Ice Shards + Chaotic Skyflare multiplier 2.06, got %f", mult)
	}
}
