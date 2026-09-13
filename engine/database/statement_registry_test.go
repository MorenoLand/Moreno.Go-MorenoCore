package database

import (
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
