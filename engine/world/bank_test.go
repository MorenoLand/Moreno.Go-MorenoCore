package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestBankHandlers(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, equipmentCache TEXT)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"INSERT INTO characters VALUES (1, 50000, '')", // 5g
		"INSERT INTO character_inventory VALUES (1, 0, 23, 1001)", // item in backpack slot 23
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, WorldStore: store, Config: config.Default()}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 50000}}
	ctx := context.Background()

	// 1. Test handleBankerActivate
	actPayload := protocol.NewBuffer(8)
	actPayload.WriteU64(12345)
	if !sess.handleBankerActivate(ctx, actPayload.Bytes()) {
		t.Fatal("handleBankerActivate failed")
	}

	// 2. Test handleBuyBankSlot
	buyPayload := protocol.NewBuffer(8)
	buyPayload.WriteU64(12345)
	if !sess.handleBuyBankSlot(ctx, buyPayload.Bytes()) {
		t.Fatal("handleBuyBankSlot failed")
	}
	if sess.player.Money != 49000 { // 50000 - 1000 = 49000
		t.Fatalf("expected money 49000, got %d", sess.player.Money)
	}

	// 3. Test handleAutoBankItem (moves item from slot 23 to first bank slot 39)
	autoBankPayload := protocol.NewBuffer(2)
	autoBankPayload.WriteU8(0)
	autoBankPayload.WriteU8(23)
	if !sess.handleAutoBankItem(ctx, autoBankPayload.Bytes()) {
		t.Fatal("handleAutoBankItem failed")
	}

	var slot int
	err = db.QueryRowContext(ctx, "SELECT slot FROM character_inventory WHERE guid = 1 AND item = 1001").Scan(&slot)
	if err != nil || slot != 39 {
		t.Fatalf("expected item 1001 in bank slot 39, got %d (err: %v)", slot, err)
	}

	// 4. Test handleAutoStoreBankItem (moves item from bank slot 39 back to backpack)
	autoStorePayload := protocol.NewBuffer(2)
	autoStorePayload.WriteU8(0)
	autoStorePayload.WriteU8(39)
	if !sess.handleAutoStoreBankItem(ctx, autoStorePayload.Bytes()) {
		t.Fatal("handleAutoStoreBankItem failed")
	}

	err = db.QueryRowContext(ctx, "SELECT slot FROM character_inventory WHERE guid = 1 AND item = 1001").Scan(&slot)
	if err != nil || slot < 23 || slot > 38 {
		t.Fatalf("expected item 1001 in backpack (23..38), got %d (err: %v)", slot, err)
	}
}

func TestAreaTriggerAndBarberShopHandlers(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, money INTEGER, hairStyle INTEGER, hairColor INTEGER, facialStyle INTEGER, skin INTEGER)",
		"CREATE TABLE character_queststatus (guid INTEGER, quest INTEGER, status INTEGER)",
		"CREATE TABLE areatrigger_involvedrelation (id INTEGER PRIMARY KEY, quest INTEGER)",
		"CREATE TABLE areatrigger_tavern (id INTEGER PRIMARY KEY)",
		"CREATE TABLE areatrigger_teleport (id INTEGER PRIMARY KEY, target_map INTEGER, target_position_x REAL, target_position_y REAL, target_position_z REAL, target_orientation REAL)",
		"INSERT INTO characters VALUES (1, 1000, 1, 1, 1, 1)",
		"INSERT INTO areatrigger_tavern VALUES (501)",
		"INSERT INTO areatrigger_teleport VALUES (601, 1, 100.0, 200.0, 30.0, 1.57)",
		"INSERT INTO areatrigger_involvedrelation VALUES (701, 99)",
		"INSERT INTO character_queststatus VALUES (1, 99, 3)", // incomplete
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: db}
	srv := &Server{WorldStore: store, CharactersStore: store, Config: config.Default()}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Money: 1000}}
	ctx := context.Background()

	// 1. AreaTrigger quest completion
	atQuest := protocol.NewBuffer(4)
	atQuest.WriteU32(701)
	if !sess.handleAreaTrigger(ctx, atQuest.Bytes()) {
		t.Fatal("handleAreaTrigger quest failed")
	}
	var qStatus int
	_ = db.QueryRowContext(ctx, "SELECT status FROM character_queststatus WHERE guid = 1 AND quest = 99").Scan(&qStatus)
	if qStatus != questStatusComplete {
		t.Fatalf("expected questStatusComplete (1), got %d", qStatus)
	}

	// 2. AreaTrigger tavern rest
	atTavern := protocol.NewBuffer(4)
	atTavern.WriteU32(501)
	if !sess.handleAreaTrigger(ctx, atTavern.Bytes()) {
		t.Fatal("handleAreaTrigger tavern failed")
	}
	if sess.player.ExtraFlags&1 == 0 {
		t.Fatal("expected resting flag to be set")
	}

	// 3. AreaTrigger teleport
	atTeleport := protocol.NewBuffer(4)
	atTeleport.WriteU32(601)
	if !sess.handleAreaTrigger(ctx, atTeleport.Bytes()) {
		t.Fatal("handleAreaTrigger teleport failed")
	}
	if sess.player.Map != 1 || sess.player.X != 100.0 {
		t.Fatalf("teleport failed: map=%d x=%f", sess.player.Map, sess.player.X)
	}

	// 4. Alter Appearance (Barber Shop)
	barberPayload := protocol.NewBuffer(16)
	barberPayload.WriteU32(5)  // hair
	barberPayload.WriteU32(6)  // color
	barberPayload.WriteU32(7)  // facial hair
	barberPayload.WriteU32(8)  // skin color
	if !sess.handleAlterAppearance(ctx, barberPayload.Bytes()) {
		t.Fatal("handleAlterAppearance failed")
	}
	if sess.player.HairStyle != 5 || sess.player.HairColor != 6 || sess.player.FacialStyle != 7 || sess.player.Skin != 8 {
		t.Fatalf("barber shop styles not updated: %+v", sess.player)
	}
}
