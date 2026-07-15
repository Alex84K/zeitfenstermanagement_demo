package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"zeitfenster/internal/zeitfenster"
)

// newTestDB oeffnet eine frische SQLite-Datenbank in einer temporaeren Datei
// mit angewendeten Migrationen (CONVENTIONS.md §13). Kein Docker, kein
// testcontainers noetig — genau der Vorteil von SQLite fuer dieses Projekt.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seedStandortUndRampe legt einen aktiven Standort (Europe/Berlin, 05:00-22:00)
// und eine aktive Rampe mit den gegebenen Temperaturbereichen an. Rueckgabe
// sind die vollstaendigen, aus der DB zurueckgelesenen Werte (inkl. ID).
func seedStandortUndRampe(t *testing.T, db *sql.DB, bereiche ...zeitfenster.Temperaturbereich) (zeitfenster.Standort, zeitfenster.Rampe) {
	t.Helper()
	ctx := context.Background()

	standort, err := zeitfenster.NewStandort(0, "Versmold", "Industriestrasse 1", "Europe/Berlin", 5*time.Hour, 22*time.Hour, true)
	if err != nil {
		t.Fatalf("NewStandort: %v", err)
	}
	if err := NewStandortStore(db).Create(ctx, &standort); err != nil {
		t.Fatalf("StandortStore.Create: %v", err)
	}

	if len(bereiche) == 0 {
		bereiche = []zeitfenster.Temperaturbereich{zeitfenster.TK, zeitfenster.Frisch}
	}
	rampe, err := zeitfenster.NewRampe(0, standort.ID, "Tor 12", zeitfenster.NewTemperaturbereichSet(bereiche...), true)
	if err != nil {
		t.Fatalf("NewRampe: %v", err)
	}
	if err := NewRampeStore(db).Create(ctx, &rampe); err != nil {
		t.Fatalf("RampeStore.Create: %v", err)
	}

	return standort, rampe
}

// gueltigeBuchung liefert eine gueltige Buchung fuer rampe: 2026-07-20
// 06:30-07:30 UTC (60 Minuten, im Raster, innerhalb der Oeffnungszeiten aus
// seedStandortUndRampe im Sommer). ErstelltAm liegt eine Stunde davor.
func gueltigeBuchung(rampeID int64, temperaturbereich zeitfenster.Temperaturbereich, spediteur, kennzeichen string) zeitfenster.Buchung {
	beginn := time.Date(2026, 7, 20, 6, 30, 0, 0, time.UTC)
	return zeitfenster.Buchung{
		RampeID:           rampeID,
		Spediteur:         spediteur,
		Kennzeichen:       kennzeichen,
		SendungNr:         "SND-1",
		Temperaturbereich: temperaturbereich,
		Beginn:            beginn,
		Ende:              beginn.Add(time.Hour),
		ErstelltAm:        beginn.Add(-time.Hour),
	}
}
