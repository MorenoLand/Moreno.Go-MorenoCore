package database

import (
	"context"
	"database/sql"
	"fmt"
)

type StatementRegistry struct {
	definitions map[StatementID]StatementDefinition
}

func NewStatementRegistry(definitions []StatementDefinition) (*StatementRegistry, error) {
	registry := &StatementRegistry{definitions: make(map[StatementID]StatementDefinition, len(definitions))}
	for _, definition := range definitions {
		if definition.ID == "" || definition.SQL == "" {
			return nil, fmt.Errorf("statement definition is incomplete")
		}
		if _, exists := registry.definitions[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate statement %s", definition.ID)
		}
		registry.definitions[definition.ID] = definition
	}
	return registry, nil
}

func (r *StatementRegistry) Get(id StatementID) (StatementDefinition, bool) {
	if r == nil {
		return StatementDefinition{}, false
	}
	definition, ok := r.definitions[id]
	return definition, ok
}

func (r *StatementRegistry) Len() int {
	if r == nil {
		return 0
	}
	return len(r.definitions)
}

var sqliteStatementOverrides = map[StatementID]string{
	"LOGIN_DEL_EXPIRED_IP_BANS":          "DELETE FROM ip_banned WHERE unbandate <> bandate AND unbandate <= unixepoch()",
	"LOGIN_UPD_EXPIRED_ACCOUNT_BANS":     "UPDATE account_banned SET active = 0 WHERE active = 1 AND unbandate <> bandate AND unbandate <= unixepoch()",
	"LOGIN_SEL_IP_INFO":                  "SELECT unbandate > unixepoch() OR unbandate = bandate AS banned, NULL as country FROM ip_banned WHERE ip = ?",
	"LOGIN_SEL_IP_BANNED_ALL":            "SELECT ip, bandate, unbandate, bannedby, banreason FROM ip_banned WHERE (bandate = unbandate OR unbandate > unixepoch()) ORDER BY unbandate",
	"LOGIN_SEL_IP_BANNED_BY_IP":          "SELECT ip, bandate, unbandate, bannedby, banreason FROM ip_banned WHERE (bandate = unbandate OR unbandate > unixepoch()) AND ip LIKE '%' || ? || '%' ORDER BY unbandate",
	"LOGIN_SEL_ACCOUNT_BANNED_BY_FILTER": "SELECT account.id, username FROM account, account_banned WHERE account.id = account_banned.id AND active = 1 AND username LIKE '%' || ? || '%' GROUP BY account.id",
	"LOGIN_UPD_LOGONPROOF":               "UPDATE account SET session_key_auth = ?, last_ip = ?, last_login = CURRENT_TIMESTAMP, locale = ?, failed_logins = 0, os = ? WHERE UPPER(username) = UPPER(?)",
	"LOGIN_SEL_LOGONCHALLENGE":           "SELECT a.id, UPPER(a.username), a.locked, a.lock_country, a.last_ip, a.failed_logins, COALESCE(ab.unbandate > unixepoch() OR ab.unbandate = ab.bandate, 0), COALESCE(ab.unbandate = ab.bandate, 0), COALESCE(aa.SecurityLevel, 0), a.totp_secret, a.salt, a.verifier FROM account a LEFT JOIN account_access aa ON a.id = aa.AccountID LEFT JOIN account_banned ab ON ab.id = a.id AND ab.active = 1 WHERE UPPER(a.username) = UPPER(?)",
	"LOGIN_SEL_RECONNECTCHALLENGE":       "SELECT a.id, UPPER(a.username), a.locked, a.lock_country, a.last_ip, a.failed_logins, COALESCE(ab.unbandate > unixepoch() OR ab.unbandate = ab.bandate, 0), COALESCE(ab.unbandate = ab.bandate, 0), COALESCE(aa.SecurityLevel, 0), a.session_key_auth FROM account a LEFT JOIN account_access aa ON a.id = aa.AccountID LEFT JOIN account_banned ab ON ab.id = a.id AND ab.active = 1 WHERE UPPER(a.username) = UPPER(?) AND a.session_key_auth IS NOT NULL",
	"LOGIN_SEL_ACCOUNT_INFO_BY_NAME":     "SELECT a.id, a.session_key_auth, a.last_ip, a.locked, a.lock_country, a.expansion, a.mutetime, a.locale, a.recruiter, a.os, COALESCE(aa.SecurityLevel, 0), COALESCE(ab.unbandate > unixepoch() OR ab.unbandate = ab.bandate, 0), r.id FROM account a LEFT JOIN account_access aa ON a.id = aa.AccountID AND aa.RealmID IN (-1, ?) LEFT JOIN account_banned ab ON a.id = ab.id AND ab.active = 1 LEFT JOIN account r ON a.id = r.recruiter WHERE a.username = ? AND a.session_key_auth IS NOT NULL ORDER BY aa.RealmID DESC LIMIT 1",
	"LOGIN_SEL_REALM_CHARACTER_COUNTS":   "SELECT realmid, numchars FROM realmcharacters WHERE acctid = ?",
	"LOGIN_UPD_LAST_IP":                  "UPDATE account SET last_ip = ? WHERE UPPER(username) = UPPER(?)",
	"LOGIN_UPD_LAST_ATTEMPT_IP":          "UPDATE account SET last_attempt_ip = ? WHERE UPPER(username) = UPPER(?)",
	"LOGIN_UPD_FAILEDLOGINS":             "UPDATE account SET failed_logins = failed_logins + 1 WHERE UPPER(username) = UPPER(?)",
	"LOGIN_INS_IP_AUTO_BANNED":           "INSERT INTO ip_banned (ip, bandate, unbandate, bannedby, banreason) VALUES (?, unixepoch(), unixepoch()+?, 'Trinity Auth', 'Failed login autoban')",
	"LOGIN_INS_ACCOUNT_AUTO_BANNED":      "INSERT INTO account_banned (id, bandate, unbandate, bannedby, banreason, active) VALUES (?, unixepoch(), unixepoch()+?, 'Trinity Auth', 'Failed login autoban', 1)",
	"LOGIN_INS_IP_BANNED":                "INSERT INTO ip_banned (ip, bandate, unbandate, bannedby, banreason) VALUES (?, unixepoch(), unixepoch()+?, ?, ?)",
	"LOGIN_INS_ACCOUNT_BANNED":           "INSERT INTO account_banned (id, bandate, unbandate, bannedby, banreason, active) VALUES (?, unixepoch(), unixepoch()+?, ?, ?, 1)",
	"LOGIN_INS_ACCOUNT":                  "INSERT INTO account(username, salt, verifier, reg_mail, email, joindate) VALUES(?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
	"CHAR_DEL_EXPIRED_BANS":              "UPDATE character_banned SET active = 0 WHERE unbandate <= unixepoch() AND unbandate <> bandate",
	"CHAR_SEL_CHAR_CREATE_INFO":          "SELECT level, race, class FROM characters WHERE account = ? LIMIT ?",
	"CHAR_DEL_CHARACTER_BAN":             "DELETE FROM character_banned WHERE guid IN (SELECT guid FROM characters WHERE account = ?)",
	"CHAR_SEL_GUID_BY_NAME_FILTER":       "SELECT guid, name FROM characters WHERE name LIKE '%' || ? || '%'",
	"CHAR_SEL_CHARACTER_SPELLCOOLDOWNS":  "SELECT spell, item, time, categoryId, categoryEnd FROM character_spell_cooldown WHERE guid = ? AND time > unixepoch()",
	"CHAR_INS_AUCTION_BIDDERS":           "INSERT OR IGNORE INTO auctionbidders (id, bidderguid) VALUES (?, ?)",
	"CHAR_INS_CHAR_QUESTSTATUS_REWARDED": "INSERT OR IGNORE INTO character_queststatus_rewarded (guid, quest, active) VALUES (?, ?, 1)",
	"CHAR_SEL_PET_SPELL_COOLDOWN":        "SELECT spell, time, categoryId, categoryEnd FROM pet_spell_cooldown WHERE guid = ? AND time > unixepoch()",
	"CHAR_SEL_CHECK_NAME":                "SELECT 1 FROM characters WHERE UPPER(name) = UPPER(?)",
}

func StatementSQL(id StatementID, backend Backend) (string, error) {
	for _, definition := range AllStatements() {
		if definition.ID != id {
			continue
		}
		if backend == BackendSQLite {
			if override, ok := sqliteStatementOverrides[id]; ok {
				return override, nil
			}
		}
		return definition.SQL, nil
	}
	return "", fmt.Errorf("unknown statement %s", id)
}

func (s *Store) QueryRowStatement(ctx context.Context, id StatementID, args ...any) (*sql.Row, error) {
	query, err := StatementSQL(id, s.Backend)
	if err != nil {
		s.recordDatabaseEvent("query_row", string(id), len(args), err)
		return nil, err
	}
	s.recordDatabaseEvent("query_row", string(id), len(args), nil)
	return s.DB.QueryRowContext(ctx, query, args...), nil
}

func (s *Store) QueryStatement(ctx context.Context, id StatementID, args ...any) (*sql.Rows, error) {
	query, err := StatementSQL(id, s.Backend)
	if err != nil {
		s.recordDatabaseEvent("query", string(id), len(args), err)
		return nil, err
	}
	rows, queryErr := s.DB.QueryContext(ctx, query, args...)
	s.recordDatabaseEvent("query", string(id), len(args), queryErr)
	return rows, queryErr
}

func (s *Store) ExecStatement(ctx context.Context, id StatementID, args ...any) (sql.Result, error) {
	query, err := StatementSQL(id, s.Backend)
	if err != nil {
		s.recordDatabaseEvent("exec", string(id), len(args), err)
		return nil, err
	}
	result, execErr := s.DB.ExecContext(ctx, query, args...)
	s.recordDatabaseEvent("exec", string(id), len(args), execErr)
	return result, execErr
}
