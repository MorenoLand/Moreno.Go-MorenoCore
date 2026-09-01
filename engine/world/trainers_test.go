package world

import (
	"context"
	"database/sql"
	"testing"

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
