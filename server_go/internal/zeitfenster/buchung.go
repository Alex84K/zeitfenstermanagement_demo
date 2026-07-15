package zeitfenster

import (
	"fmt"
	"time"
)

// Buchung ist eine Reservierung eines Zeitfensters auf einer Rampe.
//
// Es gibt bewusst kein Statusfeld: der Status wird aus BuchungEreignis berechnet
// (siehe ereignis.go) — MVP.md §5 begruendet, warum Denormalisierung hier
// vermieden wird: Status und Ereignisse koennten auseinanderlaufen.
type Buchung struct {
	ID                int64
	RampeID           int64
	Spediteur         string
	Kennzeichen       string
	SendungNr         string
	Temperaturbereich Temperaturbereich
	Beginn            time.Time
	Ende              time.Time
	ErstelltAm        time.Time
	StorniertAm       *time.Time
}

// Storniert meldet, ob die Buchung storniert wurde.
func (b Buchung) Storniert() bool {
	return b.StorniertAm != nil
}

// PruefeBuchung validiert die Regeln 2-7 einer neuen Buchung gegen ihre Rampe
// und deren Standort. jetzt wird als Parameter uebergeben statt time.Now()
// intern aufzurufen — sonst liesse sich Regel 6 (Vergangenheit) nicht
// deterministisch testen (CONVENTIONS.md §13).
//
// Regel 1 (Slot-Konflikt) ist absichtlich NICHT hier: die einzige race-freie
// Durchsetzung ist der Constraint in der Datenbank — ein vorheriger SELECT
// "ist der Slot frei?" waere eine Race Condition. Siehe MVP.md §6, CONVENTIONS.md §8.
//
// Reihenfolge der Pruefungen: erst die strukturelle Gueltigkeit der Zeitwerte
// selbst (Raster, Dauer, Vergangenheit) — das sind Fehler, die nichts mit Rampe
// oder Standort zu tun haben. Dann, ob Rampe/Standort ueberhaupt nutzbar sind.
// Erst danach die Regeln, die beide Seiten gemeinsam brauchen (Temperatur,
// Oeffnungszeiten). MVP.md nummeriert die Regeln 2-7 nur als Bezeichner, nicht
// als vorgeschriebene Pruefreihenfolge.
func PruefeBuchung(b Buchung, rampe Rampe, standort Standort, jetzt time.Time) error {
	if !IstImRaster(b.Beginn) || !IstImRaster(b.Ende) {
		return fmt.Errorf("%w: beginn=%s ende=%s", ErrNichtImRaster, b.Beginn, b.Ende)
	}

	dauer := b.Ende.Sub(b.Beginn)
	if dauer < MinDauer || dauer > MaxDauer {
		return fmt.Errorf("%w: %s", ErrUngueltigeDauer, dauer)
	}

	if b.Beginn.Before(jetzt) {
		return fmt.Errorf("%w: beginn=%s jetzt=%s", ErrVergangenheit, b.Beginn, jetzt)
	}

	if !rampe.Aktiv || !standort.Aktiv {
		return fmt.Errorf("%w: rampe %d", ErrRampeInaktiv, rampe.ID)
	}

	if !rampe.UnterstuetztTemperaturbereich(b.Temperaturbereich) {
		return fmt.Errorf("%w: rampe %d unterstuetzt kein %s", ErrTemperaturMismatch, rampe.ID, b.Temperaturbereich)
	}

	if !standort.IstInnerhalbOeffnungszeiten(b.Beginn, b.Ende) {
		return fmt.Errorf("%w: beginn=%s ende=%s", ErrAusserhalbOeffnungszeit, b.Beginn, b.Ende)
	}

	return nil
}
