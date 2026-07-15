package zeitfenster

import "time"

// Standzeit berechnet die Zeit von ANGEKOMMEN bis ABGEFAHREN. ok ist false,
// solange eines der beiden Ereignisse fehlt — Standzeit ist erst berechenbar,
// nachdem die Maschine abgefahren ist.
func Standzeit(ereignisse []BuchungEreignis) (dauer time.Duration, ok bool) {
	ankunft, hatAnkunft := zeitpunktVon(ereignisse, Angekommen)
	abfahrt, hatAbfahrt := zeitpunktVon(ereignisse, Abgefahren)
	if !hatAnkunft || !hatAbfahrt {
		return 0, false
	}
	return abfahrt.Sub(ankunft), true
}

// Puenktlich meldet, ob die Ankunft innerhalb der Karenzzeit nach beginn lag.
//
// Fruehere Ankunft ist NICHT unpuenktlich — sie belastet zwar den Hof, ist aber
// kein Regelverstoss (bewusste Entscheidung, siehe MVP.md §6). ok ist false,
// solange noch keine Ankunft erfasst wurde.
func Puenktlich(ereignisse []BuchungEreignis, beginn time.Time) (puenktlich, ok bool) {
	ankunft, hatAnkunft := zeitpunktVon(ereignisse, Angekommen)
	if !hatAnkunft {
		return false, false
	}
	return !ankunft.After(beginn.Add(Karenzzeit)), true
}

// IstNoShow meldet, ob eine Buchung als Nichtankunft gilt. Deckungsgleich mit
// BerechneStatus == StatusNoShow — als eigene Funktion, weil Kennzahlen ueber
// viele Buchungen aggregieren und ein sprechender Name pro Buchung lesbarer ist
// als der Umweg ueber Status.
func IstNoShow(ereignisse []BuchungEreignis, storniertAm *time.Time, beginn, now time.Time) bool {
	return BerechneStatus(ereignisse, storniertAm, beginn, now) == StatusNoShow
}

func zeitpunktVon(ereignisse []BuchungEreignis, typ EreignisTyp) (time.Time, bool) {
	for _, e := range ereignisse {
		if e.Typ == typ {
			return e.Zeitpunkt, true
		}
	}
	return time.Time{}, false
}
