package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestQtestQuerySyntax(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	// Verify SQLite can prepare the table structure and queries
	_, err = db.Exec(`
		CREATE TABLE creature (
			guid INTEGER PRIMARY KEY,
			id INTEGER,
			equipment_id INTEGER,
			map INTEGER,
			position_x REAL,
			position_y REAL,
			phaseMask INTEGER
		);
		CREATE TABLE creature_template (
			entry INTEGER PRIMARY KEY,
			flags_extra INTEGER
		);
		CREATE TABLE creature_addon (guid INTEGER PRIMARY KEY);
		CREATE TABLE creature_template_addon (entry INTEGER PRIMARY KEY);
		CREATE TABLE creature_equip_template (CreatureID INTEGER, ID INTEGER);
		CREATE TABLE game_event_creature (guid INTEGER, eventEntry INTEGER);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	full := `SELECT c.guid, c.id FROM creature AS c
		JOIN creature_template AS t ON t.entry = c.id
		LEFT JOIN creature_addon AS ca ON ca.guid = c.guid
		LEFT JOIN creature_template_addon AS cta ON cta.entry = c.id
		LEFT JOIN creature_equip_template AS eq ON eq.CreatureID = c.id AND eq.ID = COALESCE(NULLIF(c.equipment_id, 0), 1)
		LEFT JOIN game_event_creature AS gec ON gec.guid = c.guid
		WHERE c.map = ? AND c.position_x BETWEEN ? AND ? AND c.position_y BETWEEN ? AND ?
		AND (? OR c.phaseMask = 0 OR (c.phaseMask & 1) <> 0)
		AND (? OR (COALESCE(t.flags_extra, 0) & 1) = 0)
		AND (gec.eventEntry IS NULL OR gec.eventEntry = 0)
		ORDER BY c.guid`
	stmt, err := db.Prepare(full)
	if err != nil {
		t.Fatalf("failed to prepare full query: %v", err)
	}
	_ = stmt.Close()
}
