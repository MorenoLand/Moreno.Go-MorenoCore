package world

import (
	"context"
	"database/sql"
	"net"
	"testing"
	"time"

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

	// Read SMSG_PLAY_SOUND (1455: Spell Learn Chime)
	opSound1, dataSound1, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opSound1 != uint16(protocol.OpcodeSMSG_PLAY_SOUND) {
		t.Fatalf("expected SMSG_PLAY_SOUND (0x2D2), got 0x%04X", opSound1)
	}
	snd1, _ := protocol.NewReader(dataSound1).ReadU32()
	if snd1 != 1455 {
		t.Fatalf("expected SoundKit 1455, got %d", snd1)
	}

	// Read SMSG_PLAY_SOUND (618: Spellbook Open)
	opSound2, dataSound2, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opSound2 != uint16(protocol.OpcodeSMSG_PLAY_SOUND) {
		t.Fatalf("expected SMSG_PLAY_SOUND (0x2D2), got 0x%04X", opSound2)
	}
	snd2, _ := protocol.NewReader(dataSound2).ReadU32()
	if snd2 != 618 {
		t.Fatalf("expected SoundKit 618, got %d", snd2)
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

	// Read SMSG_TRAINER_LIST (from s.sendTrainerList refresh)
	opList, _, err := readServerFrame(cConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if opList != uint16(protocol.OpcodeSMSG_TRAINER_LIST) {
		t.Fatalf("expected SMSG_TRAINER_LIST (0x1B0), got 0x%04X", opList)
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

	// 3. SMSG_PLAY_SOUND (1455)
	op, data, _ := readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_PLAY_SOUND) {
		t.Fatalf("expected SMSG_PLAY_SOUND, got 0x%04X", op)
	}
	snd, _ := protocol.NewReader(data).ReadU32()
	if snd != 1455 {
		t.Fatalf("expected sound 1455, got %d", snd)
	}

	// 4. SMSG_PLAY_SOUND (618)
	op, _, _ = readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_PLAY_SOUND) {
		t.Fatalf("expected SMSG_PLAY_SOUND, got 0x%04X", op)
	}

	// 5. SMSG_SUPERCEDED_SPELL (old: 133, new: 143)
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

	// 6. SMSG_LEARNED_SPELL (143)
	op, learnData, _ := readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_LEARNED_SPELL) {
		t.Fatalf("expected SMSG_LEARNED_SPELL, got 0x%04X", op)
	}
	spLearned, _ := protocol.NewReader(learnData).ReadU32()
	if spLearned != 143 {
		t.Fatalf("expected learned 143, got %d", spLearned)
	}

	// 7. SMSG_TRAINER_BUY_SUCCEEDED
	op, _, _ = readServerFrame(cConn, nil)
	if op != uint16(protocol.OpcodeSMSG_TRAINER_BUY_SUCCEEDED) {
		t.Fatalf("expected SMSG_TRAINER_BUY_SUCCEEDED, got 0x%04X", op)
	}

	// 8. SMSG_UPDATE_OBJECT (from sendPlayerUpdate)
	_, _, _ = readServerFrame(cConn, nil)

	// 9. SMSG_TRAINER_LIST (from sendTrainerList refresh)
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
