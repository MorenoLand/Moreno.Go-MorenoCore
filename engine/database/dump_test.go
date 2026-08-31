package database

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

func TestNormalizeSQLiteSchemaAndInsert(t *testing.T) {
	schema := "CREATE TABLE `foo` (`id` int unsigned NOT NULL AUTO_INCREMENT, `name` varchar(32) NOT NULL DEFAULT '', PRIMARY KEY (`id`), KEY `name` (`name`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb3"
	parts, err := NormalizeSQLiteCreateTable(schema)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, part := range parts {
		if _, err := db.ExecContext(context.Background(), part); err != nil {
			t.Fatalf("%s: %v", part, err)
		}
	}
	insert, err := NormalizeSQLiteInsert("INSERT INTO `foo` VALUES (1,'a\\'b')")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), insert); err != nil {
		t.Fatalf("%s: %v", insert, err)
	}
	var value string
	if err := db.QueryRow("SELECT name FROM foo WHERE id = 1").Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "a'b" {
		t.Fatalf("value %q", value)
	}
	if strings.Contains(parts[0], "unsigned") {
		t.Fatalf("SQLite schema retained unsigned modifier: %s", parts[0])
	}
}
