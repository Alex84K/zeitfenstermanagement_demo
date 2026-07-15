package zeitfenster

import (
	"fmt"
	"time"
)

// Karenzzeit ist die Toleranz fuer Verspaetung: wie lange nach beginn eine
// Buchung noch als puenktlich gilt, bevor sie als NO_SHOW zaehlt. MVP.md §6
// legt sie als Domainkonstante fest — Konfigurierbarkeit pro Standort ist
// YAGNI, solange niemand danach gefragt hat.
const Karenzzeit = 15 * time.Minute

// EreignisTyp ist ein am Tor physisch beobachtetes Ereignis.
//
// NO_SHOW ist absichtlich KEIN Ereignistyp: eine Nichtankunft beobachtet
// niemand, sie ergibt sich aus dem FEHLEN eines Ereignisses zu einem
// bestimmten Zeitpunkt — siehe BerechneStatus.
type EreignisTyp string

const (
	Angekommen EreignisTyp = "ANGEKOMMEN"
	Angedockt  EreignisTyp = "ANGEDOCKT"
	Abgefahren EreignisTyp = "ABGEFAHREN"
)

// ParseEreignisTyp wandelt einen Draht-String in einen EreignisTyp um.
// Ein Fehlschlag ist ein Transport-Validierungsfehler (400), kein
// Domain-Sentinel — siehe CONVENTIONS.md §10.
func ParseEreignisTyp(s string) (EreignisTyp, error) {
	t := EreignisTyp(s)
	if !t.Valid() {
		return "", fmt.Errorf("unknown ereignis typ %q", s)
	}
	return t, nil
}

// Valid meldet, ob t einer der drei am Tor beobachtbaren Ereignistypen ist.
func (t EreignisTyp) Valid() bool {
	switch t {
	case Angekommen, Angedockt, Abgefahren:
		return true
	default:
		return false
	}
}

// BuchungEreignis ist ein Eintrag im Append-only-Journal einer Buchung.
type BuchungEreignis struct {
	ID         int64
	BuchungID  int64
	Typ        EreignisTyp
	Zeitpunkt  time.Time
	ErfasstVon string
}

// Status ist der aus dem Ereignis-Journal berechnete Zustand einer Buchung.
// Er wird nirgends gespeichert — siehe Buchung in buchung.go und MVP.md §5:
// Denormalisierung wuerde Status und Ereignisse auseinanderlaufen lassen.
type Status string

const (
	StatusGebucht    Status = "GEBUCHT"
	StatusAngekommen Status = "ANGEKOMMEN"
	StatusAngedockt  Status = "ANGEDOCKT"
	StatusAbgefahren Status = "ABGEFAHREN"
	StatusNoShow     Status = "NO_SHOW"
	StatusStorniert  Status = "STORNIERT"
)

// BerechneStatus leitet den aktuellen Status einer Buchung aus ihrem
// Ereignis-Journal her. now wird als Parameter uebergeben statt time.Now()
// intern aufzurufen — sonst liesse sich NO_SHOW nicht deterministisch testen
// (CONVENTIONS.md §13).
func BerechneStatus(ereignisse []BuchungEreignis, storniertAm *time.Time, beginn, now time.Time) Status {
	if storniertAm != nil {
		return StatusStorniert
	}
	switch {
	case hatEreignis(ereignisse, Abgefahren):
		return StatusAbgefahren
	case hatEreignis(ereignisse, Angedockt):
		return StatusAngedockt
	case hatEreignis(ereignisse, Angekommen):
		return StatusAngekommen
	case now.After(beginn.Add(Karenzzeit)):
		return StatusNoShow
	default:
		return StatusGebucht
	}
}

// PruefeEreignis validiert, ob neu als naechstes Ereignis fuer eine Buchung mit
// bisherigem Journal ereignisse zulaessig ist (Regeln 8-12).
//
// Die Reihenfolge-Regeln (9, 10) setzen voraus, dass ANGEDOCKT niemals ohne ein
// vorheriges ANGEKOMMEN im Journal landet. Das garantiert diese Funktion selbst,
// da jeder Eintrag vor dem Anhaengen hier geprueft wird — ein inkonsistentes
// Journal kann so gar nicht erst entstehen. Eine zusaetzliche Pruefung
// "ANGEKOMMEN nur aus GEBUCHT" waere Verteidigung gegen einen Zustand, den es
// nie geben kann (Regel 8 ist damit durch den Duplikat-Check bereits erfuellt).
func PruefeEreignis(ereignisse []BuchungEreignis, storniertAm *time.Time, neu EreignisTyp) error {
	if storniertAm != nil {
		return ErrBuchungStorniert
	}
	if hatEreignis(ereignisse, neu) {
		return fmt.Errorf("%w: %s bereits erfasst", ErrUngueltigerStatuswechsel, neu)
	}

	switch neu {
	case Angekommen:
		return nil
	case Angedockt:
		if !hatEreignis(ereignisse, Angekommen) {
			return fmt.Errorf("%w: ANGEDOCKT setzt ANGEKOMMEN voraus", ErrUngueltigerStatuswechsel)
		}
		return nil
	case Abgefahren:
		if !hatEreignis(ereignisse, Angedockt) {
			return fmt.Errorf("%w: ABGEFAHREN setzt ANGEDOCKT voraus", ErrUngueltigerStatuswechsel)
		}
		return nil
	default:
		return fmt.Errorf("unknown ereignis typ %q", neu)
	}
}

func hatEreignis(ereignisse []BuchungEreignis, typ EreignisTyp) bool {
	for _, e := range ereignisse {
		if e.Typ == typ {
			return true
		}
	}
	return false
}
