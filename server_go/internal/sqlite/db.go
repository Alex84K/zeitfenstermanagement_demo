// Package sqlite ist die persistente Implementierung der vom Domain
// (internal/zeitfenster) benoetigten Speicher-Schnittstellen. Sie importiert
// den Domain, nie umgekehrt — CONVENTIONS.md §3 (Abhaengigkeitsregel).
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registriert den Treiber "sqlite" per init()
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// dbTx fasst die von *sql.DB und *sql.Tx gemeinsam implementierten Methoden,
// die dieses Paket braucht. Hilfsfunktionen, die sowohl ausserhalb als auch
// innerhalb einer Transaktion laufen sollen, nehmen dbTx statt eines
// konkreten Typs entgegen — CONVENTIONS.md §7: der Konsument (die
// Hilfsfunktion) erklaert, was er braucht.
type dbTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open oeffnet die SQLite-Datenbank unter path und wendet ausstehende
// Migrationen an. Eine neue/leere Datei wird beim ersten Zugriff angelegt.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite erlaubt genau einen Schreiber gleichzeitig. Ein Pool mit mehr als
	// einer Verbindung wuerde SQLITE_BUSY provozieren, sobald zwei Anfragen
	// gleichzeitig schreiben wollen. Fuer den Umfang dieses MVP reicht ein
	// einzelner Pool statt getrennter Read/Write-Pools (CONVENTIONS.md §8).
	db.SetMaxOpenConns(1)

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// buildDSN traegt die Pragmas direkt in die DSN ein statt sie nach dem Oeffnen
// per Exec zu setzen: modernc.org/sqlite wendet DSN-Pragmas auf JEDE neue
// Verbindung an, die der Pool jemals oeffnet. Ein einmaliges Exec nach Open()
// traefe dagegen nur die erste Verbindung und liesse eine zweite (etwa nach
// einem Verbindungsfehler) stillschweigend bei den SQLite-Defaults.
func buildDSN(path string) string {
	pragmas := []string{
		"_pragma=journal_mode(WAL)",
		"_pragma=busy_timeout(5000)",
		"_pragma=foreign_keys(on)",
		"_pragma=synchronous(normal)",
		"_txlock=immediate",
	}
	return "file:" + path + "?" + strings.Join(pragmas, "&")
}

// migrate wendet alle ausstehenden Migrationen aus dem eingebetteten
// migrations/-Verzeichnis an. Eingebettet statt von der Festplatte gelesen:
// der Binary traegt sein Schema mit sich, kein separater Deploy-Schritt fuer
// Migrationsdateien noetig. go:embed erlaubt keine ".."-Pfade, deshalb liegt
// migrations/ hier unter internal/sqlite und nicht — wie urspruenglich in
// CONVENTIONS.md skizziert — als Geschwister von cmd/ und internal/.
func migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	defer goose.SetBaseFS(nil)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	return goose.Up(db, "migrations")
}
