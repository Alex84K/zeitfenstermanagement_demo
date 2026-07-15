package sqlite

import "testing"

func TestOpen_AppliesMigrations(t *testing.T) {
	db := newTestDB(t)

	tabellen := []string{"standort", "rampe", "rampe_temperaturbereich", "buchung", "buchung_slot", "buchung_ereignis"}
	for _, tabelle := range tabellen {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, tabelle).Scan(&name)
		if err != nil {
			t.Errorf("Tabelle %q fehlt nach Migration: %v", tabelle, err)
		}
	}
}

// TestOpen_PragmasAngewendet prueft empirisch, dass die DSN-Pragmas aus
// buildDSN tatsaechlich wirken — nicht nur, dass die DSN-Syntax akzeptiert
// wird. foreign_keys ist per SQLite-Default AUS; ohne die Pragma waere das
// hier false.
func TestOpen_PragmasAngewendet(t *testing.T) {
	db := newTestDB(t)

	var foreignKeys int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}

	var journalMode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want \"wal\"", journalMode)
	}
}
