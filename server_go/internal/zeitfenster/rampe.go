// Package zeitfenster enthaelt die Domaenenlogik von Zeitfenstermanagement:
// Typen, Geschaeftsregeln und Fehler. Importiert ausschliesslich die Standardbibliothek —
// siehe CONVENTIONS.md §3 (Abhaengigkeitsregel).
package zeitfenster

import (
	"fmt"
	"time"
)

// Temperaturbereich ist die Temperaturzone einer Sendung oder einer Rampe.
type Temperaturbereich string

const (
	TK      Temperaturbereich = "TK"
	Frisch  Temperaturbereich = "FRISCH"
	Trocken Temperaturbereich = "TROCKEN"
)

// ParseTemperaturbereich wandelt einen Draht-String in einen Temperaturbereich um.
// Ein Fehlschlag ist ein Transport-Validierungsfehler (400), kein Domain-Sentinel —
// siehe CONVENTIONS.md §10.
func ParseTemperaturbereich(s string) (Temperaturbereich, error) {
	t := Temperaturbereich(s)
	if !t.Valid() {
		return "", fmt.Errorf("unknown temperaturbereich %q", s)
	}
	return t, nil
}

// Valid meldet, ob t einer der drei bekannten Temperaturbereiche ist.
func (t Temperaturbereich) Valid() bool {
	switch t {
	case TK, Frisch, Trocken:
		return true
	default:
		return false
	}
}

// TemperaturbereichSet ist eine Menge unterstuetzter Temperaturbereiche einer Rampe.
type TemperaturbereichSet map[Temperaturbereich]struct{}

// NewTemperaturbereichSet baut eine Menge aus einzelnen Werten.
func NewTemperaturbereichSet(ts ...Temperaturbereich) TemperaturbereichSet {
	set := make(TemperaturbereichSet, len(ts))
	for _, t := range ts {
		set[t] = struct{}{}
	}
	return set
}

// Enthaelt meldet, ob t Teil der Menge ist.
func (s TemperaturbereichSet) Enthaelt(t Temperaturbereich) bool {
	_, ok := s[t]
	return ok
}

// Standort ist eine Lager-Niederlassung mit eigener Zeitzone und Oeffnungszeiten.
//
// OeffnungVon/OeffnungBis sind Uhrzeiten als Offset ab lokaler Mitternacht
// (z.B. 5*time.Hour fuer 05:00) — MVP.md §5 begruendet, warum die Zeitzone
// ein eigenes Feld ist: ohne sie liesse sich Regel 3 nicht pruefen.
type Standort struct {
	ID          int64
	Name        string
	Adresse     string
	Zeitzone    *time.Location
	OeffnungVon time.Duration
	OeffnungBis time.Duration
	Aktiv       bool
}

// NewStandort validiert die IANA-Zeitzone (Regel 19) und liefert einen Standort.
// Eine ungueltige Zone kann so gar nicht erst zu einem Standort-Wert werden —
// invalid state unrepresentable statt einer separaten Pruef-Funktion.
func NewStandort(id int64, name, adresse, zeitzone string, oeffnungVon, oeffnungBis time.Duration, aktiv bool) (Standort, error) {
	loc, err := time.LoadLocation(zeitzone)
	if err != nil {
		return Standort{}, fmt.Errorf("%w: %q", ErrUngueltigeZeitzone, zeitzone)
	}
	return Standort{
		ID:          id,
		Name:        name,
		Adresse:     adresse,
		Zeitzone:    loc,
		OeffnungVon: oeffnungVon,
		OeffnungBis: oeffnungBis,
		Aktiv:       aktiv,
	}, nil
}

// TagesgrenzenUTC liefert Anfang und Ende (exklusiv) des Kalendertags von datum
// in der Zeitzone des Standorts, umgerechnet nach UTC.
//
// Das ist die Umsetzung von MVP.md §7 "Semantik von datum": der Query-Parameter
// datum=2026-07-20 meint den lokalen Kalendertag am Standort, nicht UTC. Fuer
// Versmold (Europe/Berlin) beginnt dieser Tag um 2026-07-19T22:00Z, nicht um
// Mitternacht UTC — sonst rutschen morgendliche Buchungen in den Vortag.
//
// time.Date normalisiert den Tag-Ueberlauf und loest die Zeitzone inklusive DST
// korrekt auf; eine feste "minus zwei Stunden"-Konstante waere im Sommer falsch.
func (s Standort) TagesgrenzenUTC(datum time.Time) (beginn, ende time.Time) {
	jahr, monat, tag := datum.Date()
	beginn = time.Date(jahr, monat, tag, 0, 0, 0, 0, s.Zeitzone)
	ende = time.Date(jahr, monat, tag+1, 0, 0, 0, 0, s.Zeitzone)
	return beginn.UTC(), ende.UTC()
}

// IstInnerhalbOeffnungszeiten prueft Regel 3: liegt [beginn, ende) vollstaendig
// innerhalb der Oeffnungszeiten des Standorts? beginn/ende sind UTC-Zeitpunkte.
func (s Standort) IstInnerhalbOeffnungszeiten(beginn, ende time.Time) bool {
	von := zeitAbMitternacht(beginn.In(s.Zeitzone))
	bis := zeitAbMitternacht(ende.In(s.Zeitzone))
	return von >= s.OeffnungVon && bis <= s.OeffnungBis && von < bis
}

// zeitAbMitternacht liefert die lokale Uhrzeit von t als Offset ab Mitternacht.
// Faellt ende auf den naechsten Kalendertag (z.B. Buchung ueber Mitternacht),
// wird bis kleiner als von berechnet — IstInnerhalbOeffnungszeiten lehnt das
// dadurch bereits ueber von < bis korrekt ab, ohne einen separaten Tagesvergleich.
func zeitAbMitternacht(t time.Time) time.Duration {
	return time.Duration(t.Hour())*time.Hour + time.Duration(t.Minute())*time.Minute
}

// Rampe ist ein Verladetor an einem Standort.
type Rampe struct {
	ID                 int64
	StandortID         int64
	Bezeichnung        string
	Temperaturbereiche TemperaturbereichSet
	Aktiv              bool
}

// NewRampe erzwingt Regel 20: mindestens ein Temperaturbereich.
func NewRampe(id, standortID int64, bezeichnung string, bereiche TemperaturbereichSet, aktiv bool) (Rampe, error) {
	if len(bereiche) == 0 {
		return Rampe{}, ErrKeinTemperaturbereich
	}
	return Rampe{
		ID:                 id,
		StandortID:         standortID,
		Bezeichnung:        bezeichnung,
		Temperaturbereiche: bereiche,
		Aktiv:              aktiv,
	}, nil
}

// UnterstuetztTemperaturbereich prueft, ob diese Rampe fuer t ausgeruestet ist —
// die zentrale Domainregel 2 aus MVP.md §3: Tiefkuehlware darf nur auf eine
// Rampe mit TK-Ausruestung gebucht werden.
func (r Rampe) UnterstuetztTemperaturbereich(t Temperaturbereich) bool {
	return r.Temperaturbereiche.Enthaelt(t)
}
