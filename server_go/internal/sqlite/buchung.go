package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"zeitfenster/internal/zeitfenster"
)

// BuchungStore ist die SQLite-Implementierung fuer Buchungen.
type BuchungStore struct {
	db *sql.DB
}

func NewBuchungStore(db *sql.DB) *BuchungStore {
	return &BuchungStore{db: db}
}

// Create legt eine Buchung samt ihrer belegten Slots in einer Transaktion an
// und setzt b.ID.
//
// Regel 1 (kein doppelt belegter Slot) wird hier NICHT geprueft — sie kann es
// gar nicht race-frei werden, ein vorheriger SELECT waere eine Race Condition.
// Stattdessen erzwingt der PRIMARY KEY auf buchung_slot(rampe_id, slot_beginn)
// die Regel; verletzt eine der Slot-Inserts ihn, brechen wir die Transaktion
// ab und melden zeitfenster.ErrRampeBelegt. Der Aufrufer (httpapi) muss vorher
// zeitfenster.PruefeBuchung fuer die Regeln 2-7 aufgerufen haben — dieser
// Store validiert sie nicht erneut, er persistiert nur.
func (s *BuchungStore) Create(ctx context.Context, b *zeitfenster.Buchung) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO buchung (rampe_id, spediteur, kennzeichen, sendung_nr, temperaturbereich, beginn, ende, erstellt_am, storniert_am)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		b.RampeID, b.Spediteur, b.Kennzeichen, b.SendungNr, string(b.Temperaturbereich),
		formatZeit(b.Beginn), formatZeit(b.Ende), formatZeit(b.ErstelltAm),
	)
	if err != nil {
		return fmt.Errorf("insert buchung: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("buchung last insert id: %w", err)
	}

	// zeitfenster.Slots liefert [beginn, ende) exklusiv als 30-Minuten-Raster —
	// eine Buchung 06:30-07:30 fuegt zwei Zeilen ein, nicht drei (MVP.md §5).
	for _, slot := range zeitfenster.Slots(b.Beginn, b.Ende) {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO buchung_slot (rampe_id, slot_beginn, buchung_id) VALUES (?, ?, ?)`,
			b.RampeID, formatZeit(slot), id,
		); err != nil {
			if istConstraintVerletzung(err, sqliteConstraintPrimaryKey) {
				return zeitfenster.ErrRampeBelegt
			}
			return fmt.Errorf("insert buchung_slot: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	b.ID = id
	return nil
}

// Get laedt eine Buchung per ID. zeitfenster.ErrBuchungNotFound, wenn sie
// nicht existiert.
func (s *BuchungStore) Get(ctx context.Context, id int64) (zeitfenster.Buchung, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, rampe_id, spediteur, kennzeichen, sendung_nr, temperaturbereich,
		       beginn, ende, erstellt_am, storniert_am
		FROM buchung WHERE id = ?`, id)
	return scanBuchung(row)
}

// ListByStandortUndZeitraum liefert alle Buchungen einer Fläche, die das
// Intervall [von, bis) ueberlappen — von/bis kommen typischerweise aus
// zeitfenster.Standort.TagesgrenzenUTC (MVP.md §7 "Semantik von datum").
func (s *BuchungStore) ListByStandortUndZeitraum(ctx context.Context, standortID int64, von, bis time.Time) ([]zeitfenster.Buchung, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, b.rampe_id, b.spediteur, b.kennzeichen, b.sendung_nr, b.temperaturbereich,
		       b.beginn, b.ende, b.erstellt_am, b.storniert_am
		FROM buchung b
		JOIN rampe r ON r.id = b.rampe_id
		WHERE r.standort_id = ? AND b.beginn < ? AND b.ende > ?
		ORDER BY b.beginn`,
		standortID, formatZeit(bis), formatZeit(von),
	)
	if err != nil {
		return nil, fmt.Errorf("list buchung by standort: %w", err)
	}
	return scanBuchungen(rows)
}

// ListBySpediteur liefert alle Buchungen eines Spediteurs, die nach ab enden —
// "meine Buchungen" fuer die Fahrer-Panel (MVP.md §7).
func (s *BuchungStore) ListBySpediteur(ctx context.Context, spediteur string, ab time.Time) ([]zeitfenster.Buchung, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, rampe_id, spediteur, kennzeichen, sendung_nr, temperaturbereich,
		       beginn, ende, erstellt_am, storniert_am
		FROM buchung
		WHERE spediteur = ? AND ende > ?
		ORDER BY beginn`,
		spediteur, formatZeit(ab),
	)
	if err != nil {
		return nil, fmt.Errorf("list buchung by spediteur: %w", err)
	}
	return scanBuchungen(rows)
}

// Cancel storniert eine Buchung: setzt storniert_am und entfernt ihre
// belegten Slots — in einer Transaktion. MVP.md §5: laesst man die
// Slot-Zeilen stehen, bleibt die Rampe fuer eine stornierte Buchung
// faelschlich belegt.
//
// Existiert die Buchung nicht oder ist sie bereits storniert, liefert Cancel
// in beiden Faellen zeitfenster.ErrBuchungNotFound — der Aufrufer (httpapi)
// hat die Buchung bereits per Get geladen und kennt den Unterschied dort
// bereits; eine zweite Fallunterscheidung hier waere doppelte Arbeit.
func (s *BuchungStore) Cancel(ctx context.Context, id int64, jetzt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE buchung SET storniert_am = ? WHERE id = ? AND storniert_am IS NULL`,
		formatZeit(jetzt), id,
	)
	if err != nil {
		return fmt.Errorf("update buchung storniert_am: %w", err)
	}
	if err := requireRowAffected(res, zeitfenster.ErrBuchungNotFound); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM buchung_slot WHERE buchung_id = ?`, id); err != nil {
		return fmt.Errorf("delete buchung_slot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func scanBuchung(row rowScanner) (zeitfenster.Buchung, error) {
	var (
		id, rampeID                       int64
		spediteur, kennzeichen, sendungNr string
		temperaturbereich                 string
		beginnStr, endeStr, erstelltAmStr string
		storniertAmStr                    sql.NullString
	)
	err := row.Scan(&id, &rampeID, &spediteur, &kennzeichen, &sendungNr, &temperaturbereich,
		&beginnStr, &endeStr, &erstelltAmStr, &storniertAmStr)
	if errors.Is(err, sql.ErrNoRows) {
		return zeitfenster.Buchung{}, zeitfenster.ErrBuchungNotFound
	}
	if err != nil {
		return zeitfenster.Buchung{}, fmt.Errorf("scan buchung: %w", err)
	}

	beginn, err := parseZeit(beginnStr)
	if err != nil {
		return zeitfenster.Buchung{}, fmt.Errorf("parse beginn: %w", err)
	}
	ende, err := parseZeit(endeStr)
	if err != nil {
		return zeitfenster.Buchung{}, fmt.Errorf("parse ende: %w", err)
	}
	erstelltAm, err := parseZeit(erstelltAmStr)
	if err != nil {
		return zeitfenster.Buchung{}, fmt.Errorf("parse erstellt_am: %w", err)
	}

	var storniertAm *time.Time
	if storniertAmStr.Valid {
		t, err := parseZeit(storniertAmStr.String)
		if err != nil {
			return zeitfenster.Buchung{}, fmt.Errorf("parse storniert_am: %w", err)
		}
		storniertAm = &t
	}

	return zeitfenster.Buchung{
		ID:                id,
		RampeID:           rampeID,
		Spediteur:         spediteur,
		Kennzeichen:       kennzeichen,
		SendungNr:         sendungNr,
		Temperaturbereich: zeitfenster.Temperaturbereich(temperaturbereich),
		Beginn:            beginn,
		Ende:              ende,
		ErstelltAm:        erstelltAm,
		StorniertAm:       storniertAm,
	}, nil
}

func scanBuchungen(rows *sql.Rows) ([]zeitfenster.Buchung, error) {
	defer rows.Close()

	var ergebnis []zeitfenster.Buchung
	for rows.Next() {
		b, err := scanBuchung(rows)
		if err != nil {
			return nil, err
		}
		ergebnis = append(ergebnis, b)
	}
	return ergebnis, rows.Err()
}
