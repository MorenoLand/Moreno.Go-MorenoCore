package main

import (
	"testing"
)

func TestParseStatements(t *testing.T) {
	sample := `
void LoginDatabaseConnection::DoPrepareStatements()
{
    PrepareStatement(LOGIN_SEL_REALMLIST, "SELECT id, name, address, localAddress, localSubnetMask, port, icon, flag, timezone, allowedSecurityLevel, population, gamebuild FROM realmlist WHERE flag <> 3 ORDER BY name", CONNECTION_SYNCH);
    PrepareStatement(LOGIN_INS_ACCOUNT, "INSERT INTO account(username, salt, verifier, email, reg_mail) VALUES(?, ?, ?, ?, ?)", CONNECTION_ASYNC);
}
`
	stmts, err := parseStatements(sample)
	if err != nil {
		t.Fatalf("parseStatements failed: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if stmts[0].ID != "LOGIN_SEL_REALMLIST" || stmts[0].Async {
		t.Errorf("unexpected stmt[0]: %+v", stmts[0])
	}
	if stmts[1].ID != "LOGIN_INS_ACCOUNT" || !stmts[1].Async {
		t.Errorf("unexpected stmt[1]: %+v", stmts[1])
	}
}

func TestParseStatementsInvalid(t *testing.T) {
	invalid := `PrepareStatement(LOGIN_SEL_REALMLIST)`
	_, err := parseStatements(invalid)
	if err == nil {
		t.Errorf("expected error for invalid PrepareStatement call")
	}
}
