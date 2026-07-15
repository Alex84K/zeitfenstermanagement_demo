package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"zeitfenster/internal/zeitfenster"
)

// EreignisStore ist die SQLite-Implementierung fuer das Ereignis-Journal
// (BuchungEreignis) — Append-only, siehe MVP.md §5.
type EreignisStore struct {
	db *sql.DB
}

func NewEreignisStore(db *sql.DB) *EreignisStore {
	return &EreignisStore{db: db}
}

// Append haengt ein Ereignis an das Journal einer Buchung an und setzt e.ID.
//
// Regel 12 (jedes Ereignis hoechstens einmal) prueft der Aufrufer bereits vor
// dem Schreiben ueber zeitfenster.PruefeEreignis gegen das geladene Journal —
// der UNIQUE(buchung_id, typ)-Constraint hier ist der Race-Guard fuer zwei
// gleichzeitige Anfragen, die dasselbe Ereignis erfassen wollen, genau wie der
// PRIMARY KEY in buchung_slot fuer Regel 1 (siehe buchung.go Create).
func (s *EreignisStore) Append(ctx context.Context, e *zeitfenster.BuchungEreignis) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO buchung_ereignis (buchung_id, typ, zeitpunkt, erfasst_von)
		VALUES (?, ?, ?, ?)`,
		e.BuchungID, string(e.Typ), formatZeit(e.Zeitpunkt), e.ErfasstVon,
	)
	if err != nil {
		if istConstraintVerletzung(err, sqliteConstraintUnique) {
			return fmt.Errorf("%w: %s bereits erfasst", zeitfenster.ErrUngueltigerStatuswechsel, e.Typ)
		}
		return fmt.Errorf("insert buchung_ereignis: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("ereignis last insert id: %w", err)
	}
	e.ID = id
	return nil
}

// ListByBuchung laedt das vollstaendige Ereignis-Journal einer Buchung,
// chronologisch sortiert — Grundlage fuer zeitfenster.BerechneStatus und die
// Kennzahlen-Funktionen (Standzeit, Puenktlich, IstNoShow).
//
// ORDER BY zeitpunkt, id: zeitpunkt hat nur Sekundenaufloesung (formatZeit,
// CONVENTIONS.md §8). Erfasst jemand — oder ein schneller Testlauf — zwei
// Ereignisse innerhalb derselben Sekunde, waere die Reihenfolge bei
// gleichstehendem zeitpunkt sonst undefiniert. id als Tiebreaker spiegelt die
// tatsaechliche Einfuegereihenfolge, unabhaengig von der Zeitstempel-Aufloesung.
func (s *EreignisStore) ListByBuchung(ctx context.Context, buchungID int64) ([]zeitfenster.BuchungEreignis, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, buchung_id, typ, zeitpunkt, erfasst_von
		FROM buchung_ereignis
		WHERE buchung_id = ?
		ORDER BY zeitpunkt, id`,
		buchungID,
	)
	if err != nil {
		return nil, fmt.Errorf("list buchung_ereignis: %w", err)
	}
	defer rows.Close()

	var ergebnis []zeitfenster.BuchungEreignis
	for rows.Next() {
		var (
			id, bID      int64
			typ          string
			zeitpunktStr string
			erfasstVon   string
		)
		if err := rows.Scan(&id, &bID, &typ, &zeitpunktStr, &erfasstVon); err != nil {
			return nil, fmt.Errorf("scan buchung_ereignis: %w", err)
		}
		zeitpunkt, err := parseZeit(zeitpunktStr)
		if err != nil {
			return nil, fmt.Errorf("parse zeitpunkt: %w", err)
		}
		ergebnis = append(ergebnis, zeitfenster.BuchungEreignis{
			ID:         id,
			BuchungID:  bID,
			Typ:        zeitfenster.EreignisTyp(typ),
			Zeitpunkt:  zeitpunkt,
			ErfasstVon: erfasstVon,
		})
	}
	return ergebnis, rows.Err()
}
