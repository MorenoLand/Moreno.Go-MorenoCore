package world

import (
	"context"
	"database/sql"
	"testing"

	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/config"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/engine/database"
	"github.com/MorenoLand/Moreno.Go-MorenoCore/pkg/protocol"
)

func TestChannelAndCharServiceHandlers(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, gender INTEGER, skin INTEGER, face INTEGER, hairStyle INTEGER, hairColor INTEGER, facialStyle INTEGER, race INTEGER, class INTEGER, level INTEGER)",
		"INSERT INTO characters VALUES (1, 'OldName', 0, 1, 2, 3, 4, 5, 1, 1, 80)",
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	store := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: db}
	srv := &Server{CharactersStore: store, WorldStore: store, Config: config.Default()}
	sess := &session{server: srv, playerGUID: 1, playerLoaded: true, player: &playerState{GUID: 1, Name: "Hero"}}
	ctx := context.Background()

	// 1. Channel tests
	if !sess.handleChannelPassword(ctx, nil) ||
		!sess.handleChannelSetOwner(ctx, nil) ||
		!sess.handleChannelOwner(ctx, nil) ||
		!sess.handleChannelModerator(ctx, nil) ||
		!sess.handleChannelUnmoderator(ctx, nil) ||
		!sess.handleChannelMute(ctx, nil) ||
		!sess.handleChannelUnmute(ctx, nil) ||
		!sess.handleChannelInvite(ctx, nil) ||
		!sess.handleChannelKick(ctx, nil) ||
		!sess.handleChannelBan(ctx, nil) ||
		!sess.handleChannelUnban(ctx, nil) ||
		!sess.handleChannelAnnouncements(ctx, nil) ||
		!sess.handleChannelVoiceOn(ctx, nil) ||
		!sess.handleDeclineChannelInvite(ctx, nil) {
		t.Fatal("channel handler failed")
	}

	// 2. Character Rename
	renBuf := protocol.NewBuffer(16)
	renBuf.WriteU64(1)
	renBuf.WriteCString("NewName")
	if !sess.handleCharRename(ctx, renBuf.Bytes()) {
		t.Fatal("handleCharRename failed")
	}
	var name string
	_ = db.QueryRowContext(ctx, "SELECT name FROM characters WHERE guid = 1").Scan(&name)
	if name != "NewName" {
		t.Fatalf("expected NewName, got %s", name)
	}

	// 3. Character Customize
	custBuf := protocol.NewBuffer(24)
	custBuf.WriteU64(1)
	custBuf.WriteCString("CustomHero")
	custBuf.WriteU8(1) // gender
	custBuf.WriteU8(2) // skin
	custBuf.WriteU8(3) // face
	custBuf.WriteU8(4) // hairStyle
	custBuf.WriteU8(5) // hairColor
	custBuf.WriteU8(6) // facialHair
	if !sess.handleCharCustomize(ctx, custBuf.Bytes()) {
		t.Fatal("handleCharCustomize failed")
	}
	var gender int
	_ = db.QueryRowContext(ctx, "SELECT gender FROM characters WHERE guid = 1").Scan(&gender)
	if gender != 1 {
		t.Fatalf("expected gender 1, got %d", gender)
	}

	// 4. Character Race Change (Human 1 -> Dwarf 3, same team)
	raceBuf := protocol.NewBuffer(25)
	raceBuf.WriteU64(1)
	raceBuf.WriteCString("RaceHero")
	raceBuf.WriteU8(1) // gender
	raceBuf.WriteU8(2) // skin
	raceBuf.WriteU8(3) // face
	raceBuf.WriteU8(4) // hairStyle
	raceBuf.WriteU8(5) // hairColor
	raceBuf.WriteU8(6) // facialHair
	raceBuf.WriteU8(3) // Dwarf
	if !sess.handleCharRaceChange(ctx, raceBuf.Bytes()) {
		t.Fatal("handleCharRaceChange failed")
	}
	var race int
	_ = db.QueryRowContext(ctx, "SELECT race FROM characters WHERE guid = 1").Scan(&race)
	if race != 3 {
		t.Fatalf("expected race 3, got %d", race)
	}

	// 5. Character Faction Change (Dwarf 3 -> Orc 2, opposite team)
	factionBuf := protocol.NewBuffer(25)
	factionBuf.WriteU64(1)
	factionBuf.WriteCString("FactionHero")
	factionBuf.WriteU8(1) // gender
	factionBuf.WriteU8(2) // skin
	factionBuf.WriteU8(3) // face
	factionBuf.WriteU8(4) // hairStyle
	factionBuf.WriteU8(5) // hairColor
	factionBuf.WriteU8(6) // facialHair
	factionBuf.WriteU8(2) // Orc
	if !sess.handleCharFactionChange(ctx, factionBuf.Bytes()) {
		t.Fatal("handleCharFactionChange failed")
	}
	_ = db.QueryRowContext(ctx, "SELECT race FROM characters WHERE guid = 1").Scan(&race)
	if race != 2 {
		t.Fatalf("expected race 2, got %d", race)
	}

	// 5. Complete movie
	if !sess.handleCompleteMovie(ctx, nil) {
		t.Fatal("handleCompleteMovie failed")
	}

	// 6. Complain
	if !sess.handleComplain(ctx, nil) {
		t.Fatal("handleComplain failed")
	}
}

func TestFactionChangeConversionsParity(t *testing.T) {
	cdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer cdb.Close()
	cdb.SetMaxOpenConns(1)

	wdb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer wdb.Close()
	wdb.SetMaxOpenConns(1)

	// Set up character DB
	for _, stmt := range []string{
		"CREATE TABLE characters (guid INTEGER PRIMARY KEY, name TEXT, gender INTEGER, skin INTEGER, face INTEGER, hairStyle INTEGER, hairColor INTEGER, facialStyle INTEGER, race INTEGER, class INTEGER, level INTEGER, map INTEGER, zone INTEGER, position_x REAL, position_y REAL, position_z REAL, orientation REAL, at_login INTEGER, health INTEGER)",
		"CREATE TABLE character_homebind (guid INTEGER PRIMARY KEY, mapId INTEGER, zoneId INTEGER, posX REAL, posY REAL, posZ REAL)",
		"CREATE TABLE character_social (guid INTEGER, friend INTEGER)",
		"CREATE TABLE character_queststatus (guid INTEGER, quest INTEGER, status INTEGER)",
		"CREATE TABLE character_queststatus_rewarded (guid INTEGER, quest INTEGER, active INTEGER)",
		"CREATE TABLE character_spell (guid INTEGER, spell INTEGER, active INTEGER, disabled INTEGER)",
		"CREATE TABLE character_inventory (guid INTEGER, bag INTEGER, slot INTEGER, item INTEGER)",
		"CREATE TABLE item_instance (guid INTEGER PRIMARY KEY, itemEntry INTEGER, count INTEGER)",
		"CREATE TABLE character_reputation (guid INTEGER, faction INTEGER, standing INTEGER, flags INTEGER)",
		"CREATE TABLE character_skills (guid INTEGER, skill INTEGER, value INTEGER, max INTEGER)",
		"CREATE TABLE character_achievement (guid INTEGER, achievement INTEGER, date INTEGER)",
		// Seed Human (race 1, Alliance)
		"INSERT INTO characters VALUES (10, 'AlliHero', 0, 1, 2, 3, 4, 5, 1, 1, 80, 0, 1519, -8867.0, 673.0, 97.0, 0.0, 64, 100)",
		"INSERT INTO character_homebind VALUES (10, 0, 1519, -8867.0, 673.0, 97.0)",
		"INSERT INTO character_social VALUES (10, 999)",
		"INSERT INTO character_queststatus VALUES (10, 1001, 1)", // Active quest in progress
		"INSERT INTO character_queststatus_rewarded VALUES (10, 2001, 1)", // Alliance rewarded quest
		"INSERT INTO character_spell VALUES (10, 3001, 1, 0)", // Alliance spell/mount
		"INSERT INTO character_inventory VALUES (10, 0, 23, 5001)",
		"INSERT INTO item_instance VALUES (5001, 4001, 1)", // Alliance item
		"INSERT INTO character_reputation VALUES (10, 72, 3000, 0)", // Stormwind rep
		"INSERT INTO character_skills VALUES (10, 98, 300, 300)", // Common
		"INSERT INTO character_achievement VALUES (10, 501, 123456)", // Alliance achievement
	} {
		if _, err := cdb.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	// Set up world DB conversion tables
	for _, stmt := range []string{
		"CREATE TABLE player_factionchange_spells (alliance_id INTEGER, horde_id INTEGER, PRIMARY KEY (alliance_id, horde_id))",
		"CREATE TABLE player_factionchange_items (alliance_id INTEGER, horde_id INTEGER, race_A INTEGER, commentA TEXT, race_H INTEGER, commentH TEXT, PRIMARY KEY (alliance_id, horde_id))",
		"CREATE TABLE player_factionchange_quests (alliance_id INTEGER, horde_id INTEGER, PRIMARY KEY (alliance_id, horde_id))",
		"CREATE TABLE player_factionchange_achievement (alliance_id INTEGER, horde_id INTEGER, PRIMARY KEY (alliance_id, horde_id))",
		"CREATE TABLE player_factionchange_reputations (alliance_id INTEGER, horde_id INTEGER, PRIMARY KEY (alliance_id, horde_id))",
		// Insert conversion pairs
		"INSERT INTO player_factionchange_spells VALUES (3001, 3002)",
		"INSERT INTO player_factionchange_items VALUES (4001, 4002, 0, '', 0, '')",
		"INSERT INTO player_factionchange_quests VALUES (2001, 2002)",
		"INSERT INTO player_factionchange_achievement VALUES (501, 502)",
		"INSERT INTO player_factionchange_reputations VALUES (72, 76)",
	} {
		if _, err := wdb.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}

	cStore := &database.Store{Name: "characters", Backend: database.BackendSQLite, DB: cdb}
	wStore := &database.Store{Name: "world", Backend: database.BackendSQLite, DB: wdb}
	srv := &Server{CharactersStore: cStore, WorldStore: wStore, Config: config.Default()}
	sess := &session{server: srv, playerGUID: 10, playerLoaded: true, player: &playerState{GUID: 10, Name: "AlliHero", Race: 1}}
	ctx := context.Background()

	// 1. Same-faction rejection (Human 1 -> Dwarf 3 is same team: Alliance)
	rejectBuf := protocol.NewBuffer(25)
	rejectBuf.WriteU64(10)
	rejectBuf.WriteCString("AlliDwarf")
	rejectBuf.WriteU8(0) // gender
	rejectBuf.WriteU8(1) // skin
	rejectBuf.WriteU8(2) // face
	rejectBuf.WriteU8(3) // hairStyle
	rejectBuf.WriteU8(4) // hairColor
	rejectBuf.WriteU8(5) // facialHair
	rejectBuf.WriteU8(3) // Dwarf (same team)
	if !sess.handleCharFactionChange(ctx, rejectBuf.Bytes()) {
		t.Fatal("handleCharFactionChange rejected flow failed")
	}
	var currentRace int
	_ = cdb.QueryRowContext(ctx, "SELECT race FROM characters WHERE guid = 10").Scan(&currentRace)
	if currentRace != 1 {
		t.Fatalf("expected race still 1 after same-faction attempt, got %d", currentRace)
	}

	// 2. Faction change to Horde (Human 1 -> Orc 2)
	factionBuf := protocol.NewBuffer(25)
	factionBuf.WriteU64(10)
	factionBuf.WriteCString("HordeHero")
	factionBuf.WriteU8(0) // gender
	factionBuf.WriteU8(2) // skin
	factionBuf.WriteU8(3) // face
	factionBuf.WriteU8(4) // hairStyle
	factionBuf.WriteU8(5) // hairColor
	factionBuf.WriteU8(6) // facialHair
	factionBuf.WriteU8(2) // Orc
	if !sess.handleCharFactionChange(ctx, factionBuf.Bytes()) {
		t.Fatal("handleCharFactionChange failed")
	}

	// Verify race changed to Orc (2)
	_ = cdb.QueryRowContext(ctx, "SELECT race FROM characters WHERE guid = 10").Scan(&currentRace)
	if currentRace != 2 {
		t.Fatalf("expected race 2 (Orc), got %d", currentRace)
	}

	// Verify homebind reset to Orgrimmar (Map 1, Zone 1637)
	var homeMap, homeZone int
	_ = cdb.QueryRowContext(ctx, "SELECT mapId, zoneId FROM character_homebind WHERE guid = 10").Scan(&homeMap, &homeZone)
	if homeMap != 1 || homeZone != 1637 {
		t.Fatalf("expected homebind map 1 zone 1637, got map %d zone %d", homeMap, homeZone)
	}

	// Verify active quests wiped
	var activeQuestCount int
	_ = cdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM character_queststatus WHERE guid = 10").Scan(&activeQuestCount)
	if activeQuestCount != 0 {
		t.Fatalf("expected 0 active quests, got %d", activeQuestCount)
	}

	// Verify rewarded quest converted (2001 -> 2002)
	var rewardedQuest int
	_ = cdb.QueryRowContext(ctx, "SELECT quest FROM character_queststatus_rewarded WHERE guid = 10").Scan(&rewardedQuest)
	if rewardedQuest != 2002 {
		t.Fatalf("expected converted rewarded quest 2002, got %d", rewardedQuest)
	}

	// Verify spell converted (3001 -> 3002)
	var convertedSpell int
	_ = cdb.QueryRowContext(ctx, "SELECT spell FROM character_spell WHERE guid = 10").Scan(&convertedSpell)
	if convertedSpell != 3002 {
		t.Fatalf("expected converted spell 3002, got %d", convertedSpell)
	}

	// Verify item converted (4001 -> 4002)
	var convertedItem int
	_ = cdb.QueryRowContext(ctx, "SELECT itemEntry FROM item_instance WHERE guid = 5001").Scan(&convertedItem)
	if convertedItem != 4002 {
		t.Fatalf("expected converted item 4002, got %d", convertedItem)
	}

	// Verify reputation converted (Stormwind 72 -> Orgrimmar 76)
	var convertedRepFaction, standing int
	_ = cdb.QueryRowContext(ctx, "SELECT faction, standing FROM character_reputation WHERE guid = 10").Scan(&convertedRepFaction, &standing)
	if convertedRepFaction != 76 || standing != 3000 {
		t.Fatalf("expected faction 76 standing 3000, got %d standing %d", convertedRepFaction, standing)
	}

	// Verify achievement converted (501 -> 502)
	var convertedAchiev int
	_ = cdb.QueryRowContext(ctx, "SELECT achievement FROM character_achievement WHERE guid = 10").Scan(&convertedAchiev)
	if convertedAchiev != 502 {
		t.Fatalf("expected converted achievement 502, got %d", convertedAchiev)
	}

	// Verify friends list cleared
	var friendCount int
	_ = cdb.QueryRowContext(ctx, "SELECT COUNT(*) FROM character_social WHERE guid = 10").Scan(&friendCount)
	if friendCount != 0 {
		t.Fatalf("expected 0 friends after faction change, got %d", friendCount)
	}

	// Verify language skill switched to Orcish (109)
	var orcishSkill int
	_ = cdb.QueryRowContext(ctx, "SELECT skill FROM character_skills WHERE guid = 10 AND skill = 109").Scan(&orcishSkill)
	if orcishSkill != 109 {
		t.Fatalf("expected Orcish language skill 109, got %d", orcishSkill)
	}
}

