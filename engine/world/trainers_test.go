package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/data/wotlk"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestTrainerListingAndLearning(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_spell (guid INTEGER, spell INTEGER, active INTEGER, disabled INTEGER, PRIMARY KEY (guid, spell))",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, trainer_id INTEGER, trainer_spell INTEGER)",
		"CREATE TABLE creature_default_trainer (CreatureId INTEGER PRIMARY KEY, TrainerId INTEGER)",
		"CREATE TABLE trainer (Id INTEGER PRIMARY KEY, Type INTEGER, Requirement INTEGER, Greeting TEXT)",
		"CREATE TABLE trainer_spell (TrainerId INTEGER, SpellId INTEGER, MoneyCost INTEGER, ReqSkillLine INTEGER, ReqSkillRank INTEGER, ReqAbility1 INTEGER, ReqAbility2 INTEGER, ReqAbility3 INTEGER, ReqLevel INTEGER, PRIMARY KEY (TrainerId, SpellId))",
		"CREATE TABLE npc_trainer (ID INTEGER, SpellID INTEGER, MoneyCost INTEGER, ReqSkill INTEGER, ReqSkillValue INTEGER, ReqLevel INTEGER)",
		"INSERT INTO characters VALUES (1, 500, '')",
		"INSERT INTO creature_template VALUES (202, 0, 0)",
		"INSERT INTO creature_default_trainer VALUES (202, 50)",
		"INSERT INTO trainer VALUES (50, 0, 0, 'Ready to learn?')",
		"INSERT INTO trainer_spell VALUES (50, 133, 100, 0, 0, 0, 0, 0, 1)", // Fireball
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Level: 5, Money: 500}}

	trainerGUID := creatureWorldGUID(1, 202)
	payload := protocol.NewBuffer(8)
	payload.WriteU64(trainerGUID)
	if !sess.handleTrainerList(context.Background(), payload.Bytes()) {
		t.Fatal("handleTrainerList failed")
	}

	buyBuf := protocol.NewBuffer(12)
	buyBuf.WriteU64(trainerGUID)
	buyBuf.WriteU32(133)
	if !sess.handleTrainerBuySpell(context.Background(), buyBuf.Bytes()) {
		t.Fatal("handleTrainerBuySpell failed")
	}
	if sess.player.Money != 400 {
		t.Fatalf("expected 400 money, got %d", sess.player.Money)
	}
	if !sess.hasLearnedSpell(133) {
		t.Fatal("expected spell 133 to be learned")
	}
}

func TestTrainerClassFallback(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_spell (guid INTEGER, spell INTEGER, active INTEGER, disabled INTEGER, PRIMARY KEY (guid, spell))",
		"CREATE TABLE creature (guid INTEGER PRIMARY KEY, id INTEGER)",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, trainer_id INTEGER, trainer_spell INTEGER)",
		"CREATE TABLE creature_default_trainer (CreatureId INTEGER PRIMARY KEY, TrainerId INTEGER)",
		"CREATE TABLE trainer (Id INTEGER PRIMARY KEY, Type INTEGER, Requirement INTEGER, Greeting TEXT)",
		"CREATE TABLE trainer_spell (TrainerId INTEGER, SpellId INTEGER, MoneyCost INTEGER, ReqSkillLine INTEGER, ReqSkillRank INTEGER, ReqAbility1 INTEGER, ReqAbility2 INTEGER, ReqAbility3 INTEGER, ReqLevel INTEGER, PRIMARY KEY (TrainerId, SpellId))",
		"CREATE TABLE npc_trainer (ID INTEGER, SpellID INTEGER, MoneyCost INTEGER, ReqSkill INTEGER, ReqSkillValue INTEGER, ReqLevel INTEGER)",
		"INSERT INTO characters VALUES (1, 1000, '')",
		"INSERT INTO creature_template VALUES (303, 0, 0)",                 // No default trainer row
		"INSERT INTO trainer VALUES (60, 0, 1, 'Warrior training ready!')", // Class trainer for class 1 (Warrior)
		"INSERT INTO trainer_spell VALUES (60, 78, 50, 0, 0, 0, 0, 0, 1)",  // Heroic Strike
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Class: 1, Level: 5, Money: 1000}}

	trainerGUID := creatureWorldGUID(1, 303)
	payload := protocol.NewBuffer(8)
	payload.WriteU64(trainerGUID)
	if !sess.handleTrainerList(context.Background(), payload.Bytes()) {
		t.Fatal("handleTrainerList failed with class fallback")
	}

	buyBuf := protocol.NewBuffer(12)
	buyBuf.WriteU64(trainerGUID)
	buyBuf.WriteU32(78)
	if !sess.handleTrainerBuySpell(context.Background(), buyBuf.Bytes()) {
		t.Fatal("handleTrainerBuySpell failed with class fallback")
	}
	if !sess.hasLearnedSpell(78) {
		t.Fatal("expected spell 78 to be learned via class trainer fallback")
	}
}

func TestTrainerVisualSoundAndSkillLearning(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_spell (guid INTEGER, spell INTEGER, active INTEGER, disabled INTEGER, PRIMARY KEY (guid, spell))",
		"CREATE TABLE character_skills (guid INTEGER, skill INTEGER, value INTEGER, max INTEGER, PRIMARY KEY (guid, skill))",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, trainer_id INTEGER, trainer_spell INTEGER)",
		"CREATE TABLE creature_default_trainer (CreatureId INTEGER PRIMARY KEY, TrainerId INTEGER)",
		"CREATE TABLE trainer (Id INTEGER PRIMARY KEY, Type INTEGER, Requirement INTEGER, Greeting TEXT)",
		"CREATE TABLE trainer_spell (TrainerId INTEGER, SpellId INTEGER, MoneyCost INTEGER, ReqSkillLine INTEGER, ReqSkillRank INTEGER, ReqAbility1 INTEGER, ReqAbility2 INTEGER, ReqAbility3 INTEGER, ReqLevel INTEGER, PRIMARY KEY (TrainerId, SpellId))",
		"CREATE TABLE spell_learn_spell (entry INTEGER, SpellID INTEGER, Active INTEGER, PRIMARY KEY(entry, SpellID))",
		"INSERT INTO characters VALUES (1, 5000, '')",
		"INSERT INTO creature_template VALUES (501, 0, 0)",
		"INSERT INTO creature_default_trainer VALUES (501, 99)",
		"INSERT INTO trainer VALUES (99, 0, 0, 'Welcome apprentice!')",
		// Spell 2575 teaches Mining (Skill 186). Also in spell_learn_spell teaches 2580 (Find Minerals)
		"INSERT INTO trainer_spell VALUES (99, 2575, 100, 0, 0, 0, 0, 0, 1)",
		"INSERT INTO spell_learn_spell VALUES (2575, 2580, 1)",
		// Spell 2576 requires Mining rank 50
		"INSERT INTO trainer_spell VALUES (99, 2576, 200, 186, 50, 0, 0, 0, 10)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		AuthStore:       store,
		CharactersStore: store,
		WorldStore:      store,
		sessions:        make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:   1,
			Level:  10,
			Money:  5000,
			Skills: []playerSkill{},
		},
	}
	srv.sessions[sess] = struct{}{}

	trainerGUID := creatureWorldGUID(1, 501)

	// 1. Try buying Spell 2576 which requires Mining rank 50 -> Should FAIL with reason 2 (NotEnoughSkill)
	failBuf := protocol.NewBuffer(12)
	failBuf.WriteU64(trainerGUID)
	failBuf.WriteU32(2576)

	doneFail := make(chan struct{})
	go func() {
		sess.handleTrainerBuySpell(context.Background(), failBuf.Bytes())
		close(doneFail)
	}()

	_ = cConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	opFail, dataFail, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-doneFail
	if opFail != uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_FAILED (0x1B2), got 0x%04X", opFail)
	}
	rf := protocol.NewReader(dataFail)
	_, _ = rf.ReadU64()
	_, _ = rf.ReadU32()
	reason, _ := rf.ReadU32()
	if reason != 2 {
		t.Fatalf("expected fail reason 2 (NotEnoughSkill), got %d", reason)
	}

	// 2. Buy Spell 2575 (Apprentice Mining) -> Should succeed and emit:
	// - SMSG_PLAY_SPELL_VISUAL (0x1F3) on trainer with kit 179
	// - SMSG_PLAY_SPELL_IMPACT (0x1F7) on player with kit 362
	// - SMSG_LEARNED_SPELL (0x12B) for 2575
	// - SMSG_LEARNED_SPELL (0x12B) for 2580 (from spell_learn_spell)
	// - SMSG_TRAINER_BUY_SUCCEEDED (0x1B1)
	buyBuf := protocol.NewBuffer(12)
	buyBuf.WriteU64(trainerGUID)
	buyBuf.WriteU32(2575)

	doneBuy := make(chan struct{})
	go func() {
		sess.handleTrainerBuySpell(context.Background(), buyBuf.Bytes())
		close(doneBuy)
	}()

	// Read SMSG_PLAY_SPELL_VISUAL
	opVisual, dataVisual, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opVisual != uint16(protocol.OpcodeSMSG_PLAY_SPELL_VISUAL) {
		t.Fatalf("expected SMSG_PLAY_SPELL_VISUAL (0x1F3), got 0x%04X", opVisual)
	}
	rv := protocol.NewReader(dataVisual)
	vTarget, _ := rv.ReadU64()
	vKit, _ := rv.ReadU32()
	if vTarget != trainerGUID || vKit != 179 {
		t.Fatalf("expected visual on trainer %d with kit 179, got target=%d kit=%d", trainerGUID, vTarget, vKit)
	}

	// Read SMSG_PLAY_SPELL_IMPACT
	opImpact, dataImpact, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opImpact != uint16(protocol.OpcodeSMSG_PLAY_SPELL_IMPACT) {
		t.Fatalf("expected SMSG_PLAY_SPELL_IMPACT (0x1F7), got 0x%04X", opImpact)
	}
	ri := protocol.NewReader(dataImpact)
	iTarget, _ := ri.ReadU64()
	iKit, _ := ri.ReadU32()
	if iTarget != 1 || iKit != 362 {
		t.Fatalf("expected impact on player 1 with kit 362, got target=%d kit=%d", iTarget, iKit)
	}


	// Read SMSG_LEARNED_SPELL (2575)
	opLearn1, dataLearn1, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opLearn1 != uint16(protocol.OpcodeSMSG_LEARNED_SPELL) {
		t.Fatalf("expected SMSG_LEARNED_SPELL (0x12B), got 0x%04X", opLearn1)
	}
	sp1, _ := protocol.NewReader(dataLearn1).ReadU32()
	if sp1 != 2575 {
		t.Fatalf("expected learned spell 2575, got %d", sp1)
	}

	// Read SMSG_LEARNED_SPELL (2580 from spell_learn_spell)
	opLearn2, dataLearn2, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opLearn2 != uint16(protocol.OpcodeSMSG_LEARNED_SPELL) {
		t.Fatalf("expected SMSG_LEARNED_SPELL for dependent spell, got 0x%04X", opLearn2)
	}
	sp2, _ := protocol.NewReader(dataLearn2).ReadU32()
	if sp2 != 2580 {
		t.Fatalf("expected learned spell 2580, got %d", sp2)
	}

	// Read SMSG_TRAINER_BUY_SUCCEEDED
	opSuccess, _, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opSuccess != uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_SUCCEEDED (0x1B1), got 0x%04X", opSuccess)
	}

	// Read SMSG_UPDATE_OBJECT or SMSG_COMPRESSED_UPDATE_OBJECT (from s.sendPlayerUpdate)
	opUpdate, _, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opUpdate != uint16(protocol.OpcodeSMSG_UPDATE_OBJECT) && opUpdate != uint16(protocol.OpcodeSMSG_COMPRESSED_UPDATE_OBJECT) {
		t.Fatalf("expected SMSG_UPDATE_OBJECT or SMSG_COMPRESSED_UPDATE_OBJECT, got 0x%04X", opUpdate)
	}

	<-doneBuy

	// Verify player has both spells
	if !sess.hasLearnedSpell(2575) || !sess.hasLearnedSpell(2580) {
		t.Fatal("expected both 2575 and 2580 to be learned")
	}

	// Verify player has skill line updated
	sess.setOrUpdateSkill(context.Background(), 186, 75)
	if val := sess.getSkillValue(186); val != 1 {
		t.Fatalf("expected Mining skill value 1, got %d", val)
	}
}

func TestTrainerBuySpellSupercededAndCastableParity(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_spell (guid INTEGER, spell INTEGER, active INTEGER, disabled INTEGER, PRIMARY KEY (guid, spell))",
		"CREATE TABLE creature_default_trainer (CreatureId INTEGER PRIMARY KEY, TrainerId INTEGER)",
		"CREATE TABLE trainer (Id INTEGER PRIMARY KEY, Type INTEGER, Requirement INTEGER, Greeting TEXT)",
		"CREATE TABLE trainer_spell (TrainerId INTEGER, SpellId INTEGER, MoneyCost INTEGER, ReqSkillLine INTEGER, ReqSkillRank INTEGER, ReqAbility1 INTEGER, ReqAbility2 INTEGER, ReqAbility3 INTEGER, ReqLevel INTEGER, PRIMARY KEY (TrainerId, SpellId))",
		"CREATE TABLE spell_ranks (first_spell_id INTEGER, spell_id INTEGER, rank INTEGER, PRIMARY KEY (first_spell_id, spell_id))",
		"CREATE TABLE spell_learn_spell (entry INTEGER, SpellID INTEGER, PRIMARY KEY (entry, SpellID))",
		"INSERT INTO characters VALUES (1, 1000, '')",
		"INSERT INTO creature_default_trainer VALUES (303, 75)",
		"INSERT INTO trainer VALUES (75, 0, 0, 'Welcome')",
		// Fireball rank 1 = 133, rank 2 = 143 (ReqAbility1 = 133)
		"INSERT INTO trainer_spell VALUES (75, 143, 100, 0, 0, 133, 0, 0, 6)",
		"INSERT INTO spell_ranks VALUES (133, 133, 1)",
		"INSERT INTO spell_ranks VALUES (133, 143, 2)",
		// Character initially knows rank 1 (133)
		"INSERT INTO character_spell VALUES (1, 133, 1, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{AuthStore: store, CharactersStore: store, WorldStore: store}
	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:  1,
			Level: 10,
			Money: 1000,
			Spells: []learnedSpell{
				{ID: 133, Active: true, Disabled: false},
			},
		},
	}

	trainerGUID := creatureWorldGUID(1, 303)
	buyBuf := protocol.NewBuffer(12)
	buyBuf.WriteU64(trainerGUID)
	buyBuf.WriteU32(143) // Buy Fireball Rank 2

	doneBuy := make(chan struct{})
	go func() {
		sess.handleTrainerBuySpell(context.Background(), buyBuf.Bytes())
		close(doneBuy)
	}()

	// 1. SMSG_PLAY_SPELL_VISUAL
	op, _, _ := readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_PLAY_SPELL_VISUAL) {
		t.Fatalf("expected SMSG_PLAY_SPELL_VISUAL, got 0x%04X", op)
	}

	// 2. SMSG_PLAY_SPELL_IMPACT
	op, _, _ = readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_PLAY_SPELL_IMPACT) {
		t.Fatalf("expected SMSG_PLAY_SPELL_IMPACT, got 0x%04X", op)
	}

	// 3. SMSG_SUPERCEDED_SPELL (old: 133, new: 143)
	op, superData, _ := readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_SUPERCEDED_SPELL) {
		t.Fatalf("expected SMSG_SUPERCEDED_SPELL (0x12C), got 0x%04X", op)
	}
	rSup := protocol.NewReader(superData)
	oldSp, _ := rSup.ReadU32()
	newSp, _ := rSup.ReadU32()
	if oldSp != 133 || newSp != 143 {
		t.Fatalf("expected superceded 133 -> 143, got %d -> %d", oldSp, newSp)
	}

	// 5. SMSG_LEARNED_SPELL (143)
	op, learnData, _ := readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_LEARNED_SPELL) {
		t.Fatalf("expected SMSG_LEARNED_SPELL, got 0x%04X", op)
	}
	spLearned, _ := protocol.NewReader(learnData).ReadU32()
	if spLearned != 143 {
		t.Fatalf("expected learned 143, got %d", spLearned)
	}

	// 6. SMSG_TRAINER_BUY_SUCCEEDED
	op, _, _ = readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_SUCCEEDED, got 0x%04X", op)
	}

	// 7. SMSG_UPDATE_OBJECT (from sendPlayerUpdate)
	_, _, _ = readServerFrame(cConn, nil)

	<-doneBuy

	// Verify old rank was deactivated
	var oldActive int
	_ = db.QueryRow("SELECT active FROM character_spell WHERE guid = 1 AND spell = 133").Scan(&oldActive)
	if oldActive != 0 {
		t.Fatalf("expected old rank 133 to be deactivated, got active=%d", oldActive)
	}

	// Verify new rank is active
	var newActive int
	_ = db.QueryRow("SELECT active FROM character_spell WHERE guid = 1 AND spell = 143").Scan(&newActive)
	if newActive != 1 {
		t.Fatalf("expected new rank 143 to be active, got active=%d", newActive)
	}
}

func TestTrainerSpellFilteringByClassAndLevelAndChatNotice(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_spell (guid INTEGER, spell INTEGER, active INTEGER, disabled INTEGER, PRIMARY KEY (guid, spell))",
		"CREATE TABLE creature_template (entry INTEGER PRIMARY KEY, trainer_id INTEGER, trainer_spell INTEGER)",
		"CREATE TABLE creature_default_trainer (CreatureId INTEGER PRIMARY KEY, TrainerId INTEGER)",
		"CREATE TABLE trainer (Id INTEGER PRIMARY KEY, Type INTEGER, Requirement INTEGER, Greeting TEXT)",
		"CREATE TABLE trainer_spell (TrainerId INTEGER, SpellId INTEGER, MoneyCost INTEGER, ReqSkillLine INTEGER, ReqSkillRank INTEGER, ReqAbility1 INTEGER, ReqAbility2 INTEGER, ReqAbility3 INTEGER, ReqLevel INTEGER, PRIMARY KEY (TrainerId, SpellId))",
		"CREATE TABLE spell_ranks (first_spell_id INTEGER, spell_id INTEGER, rank INTEGER, PRIMARY KEY (first_spell_id, spell_id))",
		"CREATE TABLE spell_learn_spell (entry INTEGER, SpellID INTEGER, PRIMARY KEY (entry, SpellID))",
		// Warlock (Level 1, Class 9, Race 2) with 500 copper
		"INSERT INTO characters VALUES (1, 500, '')",
		// Trainer 32: Warlock trainer (Creature 459 - Drusilla)
		"INSERT INTO creature_template VALUES (459, 0, 0)",
		"INSERT INTO creature_default_trainer VALUES (459, 32)",
		"INSERT INTO trainer VALUES (32, 0, 9, 'Hello, warlock!')",
		// Trainer 16: Mage trainer (Creature 5880)
		"INSERT INTO creature_template VALUES (5880, 0, 0)",
		"INSERT INTO creature_default_trainer VALUES (5880, 16)",
		"INSERT INTO trainer VALUES (16, 0, 8, 'Hello, mage!')",
		// Warlock spells on Trainer 32
		"INSERT INTO trainer_spell VALUES (32, 688, 100, 0, 0, 0, 0, 0, 1)",  // Summon Imp (ReqLevel 1)
		"INSERT INTO trainer_spell VALUES (32, 348, 10, 0, 0, 0, 0, 0, 3)",    // Immolate Rank 1 (ReqLevel 3)
		"INSERT INTO trainer_spell VALUES (32, 695, 100, 0, 0, 686, 0, 0, 6)", // Shadow Bolt Rank 2 (ReqLevel 6)
		// Mage spell on Trainer 16
		"INSERT INTO trainer_spell VALUES (16, 118, 500, 0, 0, 0, 0, 0, 20)", // Polymorph
		// Spell chain for Shadow Bolt
		"INSERT INTO spell_ranks VALUES (686, 686, 1)",
		"INSERT INTO spell_ranks VALUES (686, 695, 2)",
		// Player knows Shadow Bolt Rank 1 initially
		"INSERT INTO character_spell VALUES (1, 686, 1, 0)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup failed on %s: %v", stmt, err)
		}
	}

	cConn, sConn := net.Pipe()
	defer cConn.Close()
	defer sConn.Close()

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{
		AuthStore:       store,
		CharactersStore: store,
		WorldStore:      store,
		sessions:        make(map[*session]struct{}),
	}
	sess := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:  1,
			Level: 1, // Level 1
			Class: 9, // Warlock
			Race:  2, // Orc
			Money: 500,
			Spells: []learnedSpell{
				{ID: 686, Active: true, Disabled: false}, // Knows Shadow Bolt Rank 1
			},
		},
	}
	srv.sessions[sess] = struct{}{}

	mageTrainerGUID := creatureWorldGUID(1, 5880)
	warlockTrainerGUID := creatureWorldGUID(2, 459)

	// 1. Cross-Class Trainer Check: Warlock talking to Mage Trainer 16
	// Trainer is invalid for player (Requirement 8 != Class 9)
	if sess.isTrainerValidForPlayer(0, 8) {
		t.Fatal("expected Mage trainer to be invalid for Warlock player")
	}

	// 2. Warlock Trainer Check: Valid for Warlock
	if !sess.isTrainerValidForPlayer(0, 9) {
		t.Fatal("expected Warlock trainer to be valid for Warlock player")
	}

	// 3. State calculation for Warlock spells on Level 1 player:
	// Spell 688 (ReqLevel 1, no prev rank): Should be Available (0)
	st688 := sess.getTrainerSpellState(688, 1, 0, 0, nil)
	if st688 != 0 {
		t.Fatalf("expected spell 688 state 0 (Available), got %d", st688)
	}

	// Spell 348 (ReqLevel 3, player is Level 1): Should be Unavailable (1)
	st348 := sess.getTrainerSpellState(348, 3, 0, 0, nil)
	if st348 != 1 {
		t.Fatalf("expected spell 348 state 1 (Unavailable due to level), got %d", st348)
	}

	// Spell 695 (ReqLevel 6, player is Level 1): Should be Unavailable (1)
	st695 := sess.getTrainerSpellState(695, 6, 0, 0, nil)
	if st695 != 1 {
		t.Fatalf("expected spell 695 state 1 (Unavailable due to level), got %d", st695)
	}

	// If player level was 6, but didn't know rank 1:
	sessNoRanks := &session{
		server:       srv,
		conn:         sConn,
		playerGUID:   1,
		playerLoaded: true,
		player: &playerState{
			GUID:   1,
			Level:  10,
			Class:  9,
			Spells: []learnedSpell{}, // Doesn't know rank 1 (686)
		},
	}
	stChainFail := sessNoRanks.getTrainerSpellState(695, 6, 0, 0, nil)
	if stChainFail != 1 {
		t.Fatalf("expected spell 695 state 1 (Unavailable due to missing prev rank), got %d", stChainFail)
	}

	// 4. Try to buy Spell 348 (ReqLevel 3) on Level 1 player -> Must fail with Reason 2 (NotEnoughSkill)
	failBuy348 := protocol.NewBuffer(12)
	failBuy348.WriteU64(warlockTrainerGUID)
	failBuy348.WriteU32(348)

	doneFail348 := make(chan struct{})
	go func() {
		sess.handleTrainerBuySpell(context.Background(), failBuy348.Bytes())
		close(doneFail348)
	}()

	opFail, dataFail, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-doneFail348
	if opFail != uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_FAILED, got 0x%04X", opFail)
	}
	rf := protocol.NewReader(dataFail)
	_, _ = rf.ReadU64()
	_, _ = rf.ReadU32()
	reason, _ := rf.ReadU32()
	if reason != 2 {
		t.Fatalf("expected fail reason 2 (NotEnoughSkill), got %d", reason)
	}
	if sess.hasLearnedSpell(348) {
		t.Fatal("level 1 warlock should not have learned spell 348 (req level 3)")
	}

	// 5. Try to buy Mage spell from Mage trainer -> Must fail
	failMage := protocol.NewBuffer(12)
	failMage.WriteU64(mageTrainerGUID)
	failMage.WriteU32(118)

	doneFailMage := make(chan struct{})
	go func() {
		sess.handleTrainerBuySpell(context.Background(), failMage.Bytes())
		close(doneFailMage)
	}()

	opMageFail, _, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-doneFailMage
	if opMageFail != uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_FAILED from invalid trainer, got 0x%04X", opMageFail)
	}

	// 6. Buy valid Spell 688 (Summon Imp, Level 1) -> Succeeds!
	buy688 := protocol.NewBuffer(12)
	buy688.WriteU64(warlockTrainerGUID)
	buy688.WriteU32(688)

	doneBuy := make(chan struct{})
	go func() {
		sess.handleTrainerBuySpell(context.Background(), buy688.Bytes())
		close(doneBuy)
	}()

	// 1: SMSG_PLAY_SPELL_VISUAL (179)
	opVis, _, _ := readServerFrame(cConn, nil)
	if opVis != uint16(protocol.OpcodeSMSG_PLAY_SPELL_VISUAL) {
		t.Fatalf("expected SMSG_PLAY_SPELL_VISUAL, got 0x%04X", opVis)
	}

	// 2: SMSG_PLAY_SPELL_IMPACT (362)
	opImp, _, _ := readServerFrame(cConn, nil)
	if opImp != uint16(protocol.OpcodeSMSG_PLAY_SPELL_IMPACT) {
		t.Fatalf("expected SMSG_PLAY_SPELL_IMPACT, got 0x%04X", opImp)
	}

	// 3: SMSG_LEARNED_SPELL (688) - No Sound 618!
	opLrn, dataLrn, _ := readServerFrame(cConn, nil)
	if opLrn != uint16(protocol.OpcodeSMSG_LEARNED_SPELL) {
		t.Fatalf("expected SMSG_LEARNED_SPELL (0x12B), got 0x%04X (Sound 618 was erroneously sent if 0x2D2)", opLrn)
	}
	spLrn, _ := protocol.NewReader(dataLrn).ReadU32()
	if spLrn != 688 {
		t.Fatalf("expected learned spell 688, got %d", spLrn)
	}

	// 5: SMSG_TRAINER_BUY_SUCCEEDED
	opSucc, _, _ := readServerFrame(cConn, nil)
	if opSucc != uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_SUCCEEDED, got 0x%04X", opSucc)
	}

	// 6: SMSG_UPDATE_OBJECT (money update)
	_, _, _ = readServerFrame(cConn, nil)

	<-doneBuy

	if sess.player.Money != 400 {
		t.Fatalf("expected 400 money, got %d", sess.player.Money)
	}
	if !sess.hasLearnedSpell(688) {
		t.Fatal("expected spell 688 to be learned")
	}

	// 7. Verify already learned spell state is Green / Known (2)
	stKnown := sess.getTrainerSpellState(688, 1, 0, 0, nil)
	if stKnown != 2 {
		t.Fatalf("expected known spell state 2, got %d", stKnown)
	}

	// 8. Try buying already known spell -> fails with reason 0 (AlreadyKnown)
	doneRebuy := make(chan struct{})
	go func() {
		sess.handleTrainerBuySpell(context.Background(), buy688.Bytes())
		close(doneRebuy)
	}()
	opRebuy, dataRebuy, _ := readServerFrame(cConn, nil)
	<-doneRebuy
	if opRebuy != uint16(protocol.OpcodeSMSG_TRAINER_BUY_FAILED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_FAILED, got 0x%04X", opRebuy)
	}
	rRebuy := protocol.NewReader(dataRebuy)
	_, _ = rRebuy.ReadU64()
	_, _ = rRebuy.ReadU32()
	rebuyReason, _ := rRebuy.ReadU32()
	if rebuyReason != 0 {
		t.Fatalf("expected reason 0 (AlreadyKnown), got %d", rebuyReason)
	}
}

func TestTrainerSpellFitByClassAndRaceWithDBC(t *testing.T) {
	dbcStore := wotlk.NewStore("../../data/dbc")
	srv := &Server{
		Data: dbcStore,
	}
	// Warlock: Class 9, Race 2 (Orc)
	warlockSess := &session{
		server: srv,
		player: &playerState{
			Class: 9,
			Race:  2,
		},
	}
	// Mage: Class 8, Race 1 (Human)
	mageSess := &session{
		server: srv,
		player: &playerState{
			Class: 8,
			Race:  1,
		},
	}

	// Spell 172 (Corruption) is Warlock only
	if !warlockSess.isSpellFitByClassAndRace(172) {
		t.Fatal("expected Corruption (172) to fit Warlock")
	}
	if mageSess.isSpellFitByClassAndRace(172) {
		t.Fatal("expected Corruption (172) NOT to fit Mage")
	}

	// Spell 118 (Polymorph) is Mage only
	if !mageSess.isSpellFitByClassAndRace(118) {
		t.Fatal("expected Polymorph (118) to fit Mage")
	}
	if warlockSess.isSpellFitByClassAndRace(118) {
		t.Fatal("expected Polymorph (118) NOT to fit Warlock")
	}

	// Spell 196 (1H Axes) is NOT for Warlock or Mage (classMask 0x6f: Warrior, Paladin, Hunter, Rogue, Shaman, DK)
	if warlockSess.isSpellFitByClassAndRace(196) {
		t.Fatal("expected 1H Axes (196) NOT to fit Warlock")
	}
	if mageSess.isSpellFitByClassAndRace(196) {
		t.Fatal("expected 1H Axes (196) NOT to fit Mage")
	}

	// Spell 227 (Staves) fits both Mage and Warlock (classMask 0x5d5)
	if !warlockSess.isSpellFitByClassAndRace(227) {
		t.Fatal("expected Staves (227) to fit Warlock")
	}
	if !mageSess.isSpellFitByClassAndRace(227) {
		t.Fatal("expected Staves (227) to fit Mage")
	}
}
