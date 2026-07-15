package sqlite

import (
	"errors"

	driver "modernc.org/sqlite"
)

// SQLite-Erweiterungscodes fuer Constraint-Verletzungen. Feste, oeffentlich
// dokumentierte Werte aus SQLite selbst (https://sqlite.org/rescode.html) —
// keine Interna von modernc.org/sqlite. Deshalb hier als eigene Konstanten
// statt eines Imports von modernc.org/sqlite/lib, einem generierten,
// nicht fuer den oeffentlichen Gebrauch gedachten Paket.
const (
	sqliteConstraintPrimaryKey = 1555 // SQLITE_CONSTRAINT_PRIMARYKEY
	sqliteConstraintUnique     = 2067 // SQLITE_CONSTRAINT_UNIQUE
)

// istConstraintVerletzung meldet, ob err eine SQLite-Constraint-Verletzung
// des angegebenen erweiterten Fehlercodes ist. modernc.org/sqlite aktiviert
// erweiterte Fehlercodes fuer jede Verbindung selbst (siehe conn.go
// newConn -> extendedResultCodes(true)), Error.Code() liefert also bereits
// den erweiterten und nicht nur den primaeren SQLITE_CONSTRAINT-Code.
func istConstraintVerletzung(err error, code int) bool {
	var sqliteErr *driver.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == code
}
