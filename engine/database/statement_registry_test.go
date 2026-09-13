package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestGeneratedStatementRegistry(t *testing.T) {
	registry, err := NewStatementRegistry(AllStatements())
	if err != nil {
		t.Fatal(err)
	}
	if registry.Len() != 612 {
		t.Fatalf("statement count: %d", registry.Len())
	}
	for _, definition := range AllStatements() {
		if got, ok := registry.Get(definition.ID); !ok || got.SQL != definition.SQL || got.Async != definition.Async {
			t.Fatalf("missing or changed %s", definition.ID)
		}
	}
	if query, err := StatementSQL("LOGIN_SEL_LOGONCHALLENGE", BackendSQLite); err != nil || query == "" || strings.Contains(query, "UNIX_TIMESTAMP") {
		t.Fatalf("SQLite statement: %q %v", query, err)
	}
}

func TestSQLiteDialectOverridesAvoidMySQLOnlySyntax(t *testing.T) {
	ids := []StatementID{
		"LOGIN_DEL_EXPIRED_IP_BANS", "LOGIN_UPD_EXPIRED_ACCOUNT_BANS", "LOGIN_SEL_IP_BANNED_BY_IP",
		"LOGIN_SEL_ACCOUNT_BANNED_BY_FILTER", "LOGIN_INS_ACCOUNT", "CHAR_SEL_CHAR_CREATE_INFO",
		"CHAR_DEL_CHARACTER_BAN", "CHAR_SEL_GUID_BY_NAME_FILTER", "CHAR_SEL_CHARACTER_SPELLCOOLDOWNS",
		"CHAR_INS_AUCTION_BIDDERS", "CHAR_INS_CHAR_QUESTSTATUS_REWARDED", "CHAR_SEL_PET_SPELL_COOLDOWN",
		"CHAR_INS_GUILD_BANK_RIGHT", "CHAR_INS_GUILD_MEMBER_WITHDRAW", "CHAR_UPD_CHANNEL", "CHAR_UPD_CHANNEL_USAGE",
		"CHAR_DEL_OLD_CHANNELS", "CHAR_INS_GM_SURVEY", "CHAR_UPD_DELETE_INFO", "CHAR_SEL_CHAR_DEL_INFO_BY_NAME", "CHAR_INS_DESERTER_TRACK",
		"LOGIN_INS_ALDL_IP_LOGGING", "LOGIN_INS_FACL_IP_LOGGING", "LOGIN_INS_CHAR_IP_LOGGING", "LOGIN_INS_FALP_IP_LOGGING", "LOGIN_INS_ACCOUNT_MUTE",
		"LOGIN_INS_RBAC_ACCOUNT_PERMISSION", "CHAR_INS_CHARACTER_BAN",
		"CHAR_INS_PVPSTATS_BATTLEGROUND", "CHAR_SEL_PVPSTATS_FACTIONS_OVERALL", "CHAR_INS_QUEST_TRACK", "CHAR_UPD_QUEST_TRACK_COMPLETE_TIME", "CHAR_UPD_QUEST_TRACK_ABANDON_TIME",
	}
	for _, id := range ids {
		query, err := StatementSQL(id, BackendSQLite)
		if err != nil {
			t.Fatal(err)
		}
		upper := strings.ToUpper(query)
		for _, forbidden := range []string{"UNIX_TIMESTAMP", "CONCAT(", "INSERT IGNORE", "LIMIT 0,"} {
			if strings.Contains(upper, forbidden) {
				t.Fatalf("%s retained MySQL syntax %q: %s", id, forbidden, query)
			}
		}
	}
}

func TestSQLiteDialectOverridesExecuteGuildAndChannelUpserts(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &Store{Name: "characters", Backend: BackendSQLite, DB: db}
	for _, statement := range []string{
		"CREATE TABLE guild_bank_right (guildid INTEGER, TabId INTEGER, rid INTEGER, gbright INTEGER, SlotPerDay INTEGER, PRIMARY KEY (guildid, TabId, rid))",
		"CREATE TABLE guild_member_withdraw (guid INTEGER PRIMARY KEY, tab0 INTEGER, tab1 INTEGER, tab2 INTEGER, tab3 INTEGER, tab4 INTEGER, tab5 INTEGER, money INTEGER)",
		"CREATE TABLE channels (name TEXT, team INTEGER, announce INTEGER, ownership INTEGER, password TEXT, bannedList TEXT, lastUsed INTEGER, PRIMARY KEY (name, team))",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if _, err := store.ExecStatement(ctx, "CHAR_INS_GUILD_BANK_RIGHT", 1, 2, 3, 4, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecStatement(ctx, "CHAR_INS_GUILD_BANK_RIGHT", 1, 2, 3, 8, 9); err != nil {
		t.Fatal(err)
	}
	var right, slots int
	if err := db.QueryRow("SELECT gbright, SlotPerDay FROM guild_bank_right WHERE guildid = 1 AND TabId = 2 AND rid = 3").Scan(&right, &slots); err != nil || right != 8 || slots != 9 {
		t.Fatalf("guild bank right=%d slots=%d err=%v", right, slots, err)
	}
	if _, err := store.ExecStatement(ctx, "CHAR_INS_GUILD_MEMBER_WITHDRAW", 4, 1, 2, 3, 4, 5, 6, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecStatement(ctx, "CHAR_INS_GUILD_MEMBER_WITHDRAW", 4, 8, 9, 10, 11, 12, 13, 14); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT tab0, tab5, money FROM guild_member_withdraw WHERE guid = 4").Scan(&right, &slots, &slots); err != nil || right != 8 || slots != 14 {
		t.Fatalf("guild withdraw tab0=%d money=%d err=%v", right, slots, err)
	}
	if _, err := store.ExecStatement(ctx, "CHAR_UPD_CHANNEL", "General", 1, 1, 1, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecStatement(ctx, "CHAR_UPD_CHANNEL", "General", 1, 0, 0, "secret", "7"); err != nil {
		t.Fatal(err)
	}
	var announce, ownership int
	var password string
	if err := db.QueryRow("SELECT announce, ownership, password FROM channels WHERE name = 'General' AND team = 1").Scan(&announce, &ownership, &password); err != nil {
		t.Fatal(err)
	}
	if announce != 0 || ownership != 0 || password != "secret" {
		t.Fatalf("channel announce=%d ownership=%d password=%q", announce, ownership, password)
	}
}

func TestSQLiteDialectOverridesExecuteRBACAndCharacterBanStatements(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &Store{Name: "auth", Backend: BackendSQLite, DB: db}
	for _, statement := range []string{
		"CREATE TABLE rbac_account_permissions (accountId INTEGER, permissionId INTEGER, granted INTEGER, realmId INTEGER, PRIMARY KEY (accountId, permissionId, realmId))",
		"CREATE TABLE character_banned (guid INTEGER, bandate INTEGER, unbandate INTEGER, bannedby TEXT, banreason TEXT, active INTEGER)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if _, err := store.ExecStatement(ctx, "LOGIN_INS_RBAC_ACCOUNT_PERMISSION", 7, 100, 1, -1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecStatement(ctx, "LOGIN_INS_RBAC_ACCOUNT_PERMISSION", 7, 100, 0, -1); err != nil {
		t.Fatal(err)
	}
	var granted int
	if err := db.QueryRow("SELECT granted FROM rbac_account_permissions WHERE accountId = 7 AND permissionId = 100 AND realmId = -1").Scan(&granted); err != nil || granted != 0 {
		t.Fatalf("rbac granted=%d err=%v", granted, err)
	}
	if _, err := store.ExecStatement(ctx, "CHAR_INS_CHARACTER_BAN", 99, 60, "test", "fixture"); err != nil {
		t.Fatal(err)
	}
	var bandate, unbandate int64
	var bannedBy, reason string
	if err := db.QueryRow("SELECT bandate, unbandate, bannedby, banreason FROM character_banned WHERE guid = 99").Scan(&bandate, &unbandate, &bannedBy, &reason); err != nil {
		t.Fatal(err)
	}
	if bandate <= 0 || unbandate-bandate != 60 || bannedBy != "test" || reason != "fixture" {
		t.Fatalf("ban bandate=%d unbandate=%d bannedby=%q reason=%q", bandate, unbandate, bannedBy, reason)
	}
}

func TestAllGeneratedStatementsResolveForConfiguredDialects(t *testing.T) {
	backends := []Backend{BackendSQLite, BackendMySQL, BackendMariaDB}
	for _, definition := range AllStatements() {
		for _, backend := range backends {
			query, err := StatementSQL(definition.ID, backend)
			if err != nil {
				t.Fatalf("statement=%s backend=%s err=%v", definition.ID, backend, err)
			}
			if strings.TrimSpace(query) == "" {
				t.Fatalf("statement=%s backend=%s returned empty SQL", definition.ID, backend)
			}
		}
	}
}

func TestSQLiteOverridesCoverKnownMySQLSpecificStatements(t *testing.T) {
	for _, definition := range AllStatements() {
		upper := strings.ToUpper(definition.SQL)
		needsOverride := strings.Contains(upper, "UNIX_TIMESTAMP") || strings.Contains(upper, "CONCAT(") || strings.Contains(upper, "ON DUPLICATE KEY") || strings.Contains(upper, "INSERT IGNORE") || strings.Contains(upper, "LIMIT 0,") || strings.Contains(upper, "DATEDIFF(") || strings.Contains(upper, "DELETE CB FROM")
		if needsOverride {
			if _, ok := sqliteStatementOverrides[definition.ID]; !ok {
				t.Fatalf("statement %s contains dialect-specific SQL without explicit SQLite override", definition.ID)
			}
		}
	}
}
