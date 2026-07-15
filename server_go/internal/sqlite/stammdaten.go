package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"zeitfenster/internal/zeitfenster"
)

// StandortStore ist die SQLite-Implementierung fuer Standort-Stammdaten.
type StandortStore struct {
	db *sql.DB
}

func NewStandortStore(db *sql.DB) *StandortStore {
	return &StandortStore{db: db}
}

// Create legt einen neuen Standort an und setzt standort.ID.
func (s *StandortStore) Create(ctx context.Context, standort *zeitfenster.Standort) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO standort (name, adresse, zeitzone, oeffnung_von_min, oeffnung_bis_min, aktiv)
		VALUES (?, ?, ?, ?, ?, ?)`,
		standort.Name, standort.Adresse, standort.Zeitzone.String(),
		int(standort.OeffnungVon/time.Minute), int(standort.OeffnungBis/time.Minute),
		boolToInt(standort.Aktiv),
	)
	if err != nil {
		return fmt.Errorf("insert standort: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("standort last insert id: %w", err)
	}
	standort.ID = id
	return nil
}

// Get laedt einen Standort per ID. zeitfenster.ErrStandortNotFound, wenn er
// nicht existiert.
func (s *StandortStore) Get(ctx context.Context, id int64) (zeitfenster.Standort, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, adresse, zeitzone, oeffnung_von_min, oeffnung_bis_min, aktiv
		FROM standort WHERE id = ?`, id)
	return scanStandort(row)
}

// List liefert alle Standorte, alphabetisch nach Name.
func (s *StandortStore) List(ctx context.Context) ([]zeitfenster.Standort, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, adresse, zeitzone, oeffnung_von_min, oeffnung_bis_min, aktiv
		FROM standort ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list standort: %w", err)
	}
	defer rows.Close()

	var ergebnis []zeitfenster.Standort
	for rows.Next() {
		standort, err := scanStandort(rows)
		if err != nil {
			return nil, err
		}
		ergebnis = append(ergebnis, standort)
	}
	return ergebnis, rows.Err()
}

// Update ersetzt die veraenderlichen Felder eines Standorts vollstaendig.
// Partielles PATCH-Merging (welche Felder der Client ueberhaupt mitgeschickt
// hat) ist Aufgabe von internal/httpapi, das mit *string-Feldern gegen den
// per Get geladenen Stand mischt (CONVENTIONS.md §10) — hier kommt bereits
// ein vollstaendiger Zielzustand an.
func (s *StandortStore) Update(ctx context.Context, standort zeitfenster.Standort) error {
	if !standort.Aktiv {
		hatRampen, err := standortHatAktiveRampen(ctx, s.db, standort.ID)
		if err != nil {
			return err
		}
		if hatRampen {
			return zeitfenster.ErrStandortHatRampen
		}
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE standort
		SET name = ?, adresse = ?, zeitzone = ?, oeffnung_von_min = ?, oeffnung_bis_min = ?, aktiv = ?
		WHERE id = ?`,
		standort.Name, standort.Adresse, standort.Zeitzone.String(),
		int(standort.OeffnungVon/time.Minute), int(standort.OeffnungBis/time.Minute),
		boolToInt(standort.Aktiv), standort.ID,
	)
	if err != nil {
		return fmt.Errorf("update standort: %w", err)
	}
	return requireRowAffected(res, zeitfenster.ErrStandortNotFound)
}

// RampeStore ist die SQLite-Implementierung fuer Rampe-Stammdaten.
type RampeStore struct {
	db *sql.DB
}

func NewRampeStore(db *sql.DB) *RampeStore {
	return &RampeStore{db: db}
}

// Create legt eine neue Rampe samt ihrer Temperaturbereiche in einer
// Transaktion an und setzt rampe.ID.
func (s *RampeStore) Create(ctx context.Context, rampe *zeitfenster.Rampe) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO rampe (standort_id, bezeichnung, aktiv) VALUES (?, ?, ?)`,
		rampe.StandortID, rampe.Bezeichnung, boolToInt(rampe.Aktiv),
	)
	if err != nil {
		if istConstraintVerletzung(err, sqliteConstraintUnique) {
			return zeitfenster.ErrBezeichnungBelegt
		}
		return fmt.Errorf("insert rampe: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("rampe last insert id: %w", err)
	}

	if err := insertTemperaturbereiche(ctx, tx, id, rampe.Temperaturbereiche); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	rampe.ID = id
	return nil
}

// Get laedt eine Rampe samt ihrer Temperaturbereiche.
// zeitfenster.ErrRampeNotFound, wenn sie nicht existiert.
func (s *RampeStore) Get(ctx context.Context, id int64) (zeitfenster.Rampe, error) {
	var (
		standortID  int64
		bezeichnung string
		aktivInt    int
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT standort_id, bezeichnung, aktiv FROM rampe WHERE id = ?`, id,
	).Scan(&standortID, &bezeichnung, &aktivInt)
	if errors.Is(err, sql.ErrNoRows) {
		return zeitfenster.Rampe{}, zeitfenster.ErrRampeNotFound
	}
	if err != nil {
		return zeitfenster.Rampe{}, fmt.Errorf("scan rampe: %w", err)
	}

	bereiche, err := ladeTemperaturbereiche(ctx, s.db, id)
	if err != nil {
		return zeitfenster.Rampe{}, err
	}
	return zeitfenster.NewRampe(id, standortID, bezeichnung, bereiche, aktivInt != 0)
}

// ListByStandort liefert alle Rampen einer Flaeche, alphabetisch nach
// Bezeichnung. Laedt Temperaturbereiche pro Rampe nach (N+1) statt per JOIN —
// bei MVP-Groessenordnungen (Dutzende Rampen, nicht Tausende) ist das
// einfacher zu lesen und schnell genug; ein JOIN waere verfruehte Optimierung.
func (s *RampeStore) ListByStandort(ctx context.Context, standortID int64) ([]zeitfenster.Rampe, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, bezeichnung, aktiv FROM rampe WHERE standort_id = ? ORDER BY bezeichnung`,
		standortID,
	)
	if err != nil {
		return nil, fmt.Errorf("list rampe: %w", err)
	}
	defer rows.Close()

	type roh struct {
		id          int64
		bezeichnung string
		aktivInt    int
	}
	var rohe []roh
	for rows.Next() {
		var r roh
		if err := rows.Scan(&r.id, &r.bezeichnung, &r.aktivInt); err != nil {
			return nil, fmt.Errorf("scan rampe: %w", err)
		}
		rohe = append(rohe, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ergebnis := make([]zeitfenster.Rampe, 0, len(rohe))
	for _, r := range rohe {
		bereiche, err := ladeTemperaturbereiche(ctx, s.db, r.id)
		if err != nil {
			return nil, err
		}
		rampe, err := zeitfenster.NewRampe(r.id, standortID, r.bezeichnung, bereiche, r.aktivInt != 0)
		if err != nil {
			return nil, err
		}
		ergebnis = append(ergebnis, rampe)
	}
	return ergebnis, nil
}

// Update ersetzt bezeichnung, aktiv und die Menge der Temperaturbereiche einer
// Rampe vollstaendig (gleiches PATCH-Merging-Prinzip wie StandortStore.Update).
//
// jetzt wird als Parameter uebergeben statt time.Now() intern aufzurufen —
// dieselbe Testbarkeits-Begruendung wie im Domain (CONVENTIONS.md §13):
// die Regeln 15/16 vergleichen Buchungen gegen "jetzt".
func (s *RampeStore) Update(ctx context.Context, rampe zeitfenster.Rampe, jetzt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	alteBereiche, err := ladeTemperaturbereiche(ctx, tx, rampe.ID)
	if err != nil {
		return err
	}

	if !rampe.Aktiv {
		hat, err := hatZukuenftigeBuchungen(ctx, tx, rampe.ID, nil, jetzt)
		if err != nil {
			return err
		}
		if hat {
			return zeitfenster.ErrRampeHatBuchungen
		}
	}

	entfernt := ohneTemperaturbereiche(alteBereiche, rampe.Temperaturbereiche)
	if len(entfernt) > 0 {
		hat, err := hatZukuenftigeBuchungen(ctx, tx, rampe.ID, entfernt, jetzt)
		if err != nil {
			return err
		}
		if hat {
			return zeitfenster.ErrRampeHatBuchungen
		}
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE rampe SET bezeichnung = ?, aktiv = ? WHERE id = ?`,
		rampe.Bezeichnung, boolToInt(rampe.Aktiv), rampe.ID,
	)
	if err != nil {
		if istConstraintVerletzung(err, sqliteConstraintUnique) {
			return zeitfenster.ErrBezeichnungBelegt
		}
		return fmt.Errorf("update rampe: %w", err)
	}
	if err := requireRowAffected(res, zeitfenster.ErrRampeNotFound); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM rampe_temperaturbereich WHERE rampe_id = ?`, rampe.ID,
	); err != nil {
		return fmt.Errorf("delete alte temperaturbereiche: %w", err)
	}
	if err := insertTemperaturbereiche(ctx, tx, rampe.ID, rampe.Temperaturbereiche); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// --- gemeinsame Hilfsfunktionen ---

// rowScanner passt sowohl auf *sql.Row (Get) als auch *sql.Rows (List), damit
// scanStandort in beiden Faellen wiederverwendet werden kann.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanStandort(row rowScanner) (zeitfenster.Standort, error) {
	var (
		id                      int64
		name, adresse, zeitzone string
		vonMin, bisMin          int
		aktivInt                int
	)
	if err := row.Scan(&id, &name, &adresse, &zeitzone, &vonMin, &bisMin, &aktivInt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return zeitfenster.Standort{}, zeitfenster.ErrStandortNotFound
		}
		return zeitfenster.Standort{}, fmt.Errorf("scan standort: %w", err)
	}
	standort, err := zeitfenster.NewStandort(id, name, adresse, zeitzone,
		time.Duration(vonMin)*time.Minute, time.Duration(bisMin)*time.Minute, aktivInt != 0)
	if err != nil {
		return zeitfenster.Standort{}, fmt.Errorf("rekonstruiere standort %d: %w", id, err)
	}
	return standort, nil
}

func standortHatAktiveRampen(ctx context.Context, db dbTx, standortID int64) (bool, error) {
	var existiert bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM rampe WHERE standort_id = ? AND aktiv = 1)`, standortID,
	).Scan(&existiert)
	if err != nil {
		return false, fmt.Errorf("pruefe aktive rampen: %w", err)
	}
	return existiert, nil
}

func ladeTemperaturbereiche(ctx context.Context, q dbTx, rampeID int64) (zeitfenster.TemperaturbereichSet, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT temperaturbereich FROM rampe_temperaturbereich WHERE rampe_id = ?`, rampeID,
	)
	if err != nil {
		return nil, fmt.Errorf("lade temperaturbereiche: %w", err)
	}
	defer rows.Close()

	var werte []zeitfenster.Temperaturbereich
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan temperaturbereich: %w", err)
		}
		werte = append(werte, zeitfenster.Temperaturbereich(t))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return zeitfenster.NewTemperaturbereichSet(werte...), nil
}

func insertTemperaturbereiche(ctx context.Context, q dbTx, rampeID int64, bereiche zeitfenster.TemperaturbereichSet) error {
	for t := range bereiche {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO rampe_temperaturbereich (rampe_id, temperaturbereich) VALUES (?, ?)`,
			rampeID, string(t),
		); err != nil {
			return fmt.Errorf("insert temperaturbereich %s: %w", t, err)
		}
	}
	return nil
}

// ohneTemperaturbereiche liefert die Elemente aus alt, die in neu fehlen —
// die Regel-16-Pruefung braucht genau diese Differenz, nicht die volle Menge.
func ohneTemperaturbereiche(alt, neu zeitfenster.TemperaturbereichSet) []zeitfenster.Temperaturbereich {
	var entfernt []zeitfenster.Temperaturbereich
	for t := range alt {
		if !neu.Enthaelt(t) {
			entfernt = append(entfernt, t)
		}
	}
	return entfernt
}

// hatZukuenftigeBuchungen meldet, ob es fuer rampeID eine nicht stornierte
// Buchung gibt, deren ende nach jetzt liegt. Ist bereiche nicht leer, wird
// zusaetzlich auf diese Temperaturbereiche eingeschraenkt (Regel 16);
// bereiche == nil prueft ueber alle Bereiche hinweg (Regel 15).
func hatZukuenftigeBuchungen(ctx context.Context, q dbTx, rampeID int64, bereiche []zeitfenster.Temperaturbereich, jetzt time.Time) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM buchung WHERE rampe_id = ? AND storniert_am IS NULL AND ende > ?`
	args := []any{rampeID, formatZeit(jetzt)}

	if len(bereiche) > 0 {
		platzhalter := make([]string, len(bereiche))
		for i, b := range bereiche {
			platzhalter[i] = "?"
			args = append(args, string(b))
		}
		query += ` AND temperaturbereich IN (` + strings.Join(platzhalter, ", ") + `)`
	}
	query += `)`

	var existiert bool
	if err := q.QueryRowContext(ctx, query, args...).Scan(&existiert); err != nil {
		return false, fmt.Errorf("pruefe zukuenftige buchungen: %w", err)
	}
	return existiert, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func requireRowAffected(res sql.Result, notFound error) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return notFound
	}
	return nil
}
