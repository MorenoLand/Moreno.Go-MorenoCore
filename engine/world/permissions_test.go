package world

import (
	"context"
	"database/sql"
	"testing"
)

func TestAccountHasPermissionExpandsDefaultAndLinkedPermissions(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE rbac_account_permissions (accountId INTEGER, permissionId INTEGER, granted INTEGER, realmId INTEGER); CREATE TABLE rbac_default_permissions (secId INTEGER, permissionId INTEGER, realmId INTEGER); CREATE TABLE rbac_linked_permissions (id INTEGER, linkedId INTEGER);`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO rbac_default_permissions VALUES (3, 192, -1); INSERT INTO rbac_linked_permissions VALUES (192, 193), (193, 194), (194, 198), (198, 372);`)
	if err != nil {
		t.Fatal(err)
	}
	allowed, err := accountHasPermission(context.Background(), db, 15, 1, 3, permissionCommandGMChat)
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	if _, err := db.Exec("INSERT INTO rbac_account_permissions VALUES (15, 372, 0, -1)"); err != nil {
		t.Fatal(err)
	}
	allowed, err = accountHasPermission(context.Background(), db, 15, 1, 3, permissionCommandGMChat)
	if err != nil || allowed {
		t.Fatalf("denied allowed=%v err=%v", allowed, err)
	}
}

