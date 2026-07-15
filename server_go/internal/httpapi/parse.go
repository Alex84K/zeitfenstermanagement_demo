package httpapi

import (
	"fmt"
	"strconv"
	"time"
)

// parseID liest eine positive Ganzzahl-ID aus einem Pfad- oder Query-Segment.
func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid id %q", s)
	}
	return id, nil
}

// parseStandortID wie parseID, aber mit eigener Fehlermeldung fuer den
// haeufigen Fall eines fehlenden Query-Parameters.
func parseStandortID(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("standort_id is required")
	}
	return parseID(s)
}

// parseDatum liest einen Kalendertag im Format YYYY-MM-DD. Der Query-Parameter
// datum meint den lokalen Kalendertag am Standort, nicht UTC — die Umrechnung
// in UTC-Tagesgrenzen macht standort.TagesgrenzenUTC (MVP.md §7).
func parseDatum(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("datum is required")
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid datum %q, want YYYY-MM-DD", s)
	}
	return t, nil
}

// parseAb liest einen praezisen Zeitpunkt (RFC3339) fuer den ab-Parameter von
// "meine Buchungen" — anders als datum ist das ein Zeitpunkt, kein Kalendertag:
// der Client fragt "ab jetzt", nicht "an diesem Tag".
func parseAb(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("ab is required")
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid ab %q, want RFC3339", s)
	}
	return t.UTC(), nil
}

// parsePositiveInt liest eine positive Ganzzahl aus einem Query-Parameter,
// z.B. dauer (in Minuten) fuer die Verfuegbarkeitssuche.
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid positive integer %q", s)
	}
	return n, nil
}

// formatUhrzeit wandelt eine Tagesoffset-Dauer (z.B. Standort.OeffnungVon) in
// "HH:MM" — das Textformat, in dem Oeffnungszeiten auf dem Draht erscheinen.
func formatUhrzeit(d time.Duration) string {
	return fmt.Sprintf("%02d:%02d", int(d.Hours()), int(d.Minutes())%60)
}

// parseUhrzeit ist die Umkehrung von formatUhrzeit.
func parseUhrzeit(s string) (time.Duration, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("invalid uhrzeit %q, want HH:MM", s)
	}
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute, nil
}
