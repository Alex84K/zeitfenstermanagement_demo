package sqlite

import "time"

// formatZeit wandelt einen Zeitpunkt in das ISO-8601-UTC-Textformat, in dem
// alle Zeiten dieser Datenbank gespeichert werden (CONVENTIONS.md §8) —
// lexikographische Sortierung entspricht dabei der chronologischen, BETWEEN
// und < funktionieren direkt auf dem TEXT-Wert.
func formatZeit(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// parseZeit ist die Umkehrung von formatZeit.
func parseZeit(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
