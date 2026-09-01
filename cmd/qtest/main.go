package main

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "bin/world.db")
	if err != nil { fmt.Println("open:", err); return }
	defer db.Close()
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
	rows, err := db.Query(full, 0, -8915.0, -8715.0, 550.0, 750.0, true, true)
	if err != nil { fmt.Println("full err:", err); return }
	n := 0
	for rows.Next() { n++ }
	rows.Close()
	fmt.Println("full count:", n)

	fullNpc := `SELECT c.guid FROM creature AS c JOIN creature_template AS t ON t.entry = c.id
		LEFT JOIN game_event_creature AS gec ON gec.guid = c.guid
		WHERE c.map = ? AND c.position_x BETWEEN ? AND ? AND c.position_y BETWEEN ? AND ?
		AND (? OR c.phaseMask = 0 OR (c.phaseMask & 1) <> 0)
		AND (? OR (COALESCE(t.flags_extra, 0) & 1) = 0)
		AND (gec.eventEntry IS NULL OR gec.eventEntry = 0)
		ORDER BY c.guid`
	rows2, err2 := db.Query(fullNpc, 0, -8915.0, -8715.0, 550.0, 750.0, true, true)
	if err2 != nil { fmt.Println("npcflag err:", err2); return }
	n2 := 0
	for rows2.Next() { n2++ }
	rows2.Close()
	fmt.Println("plain count:", n2)
}
