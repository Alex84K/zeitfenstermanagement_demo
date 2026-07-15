package zeitfenster

import "time"

// Fensterkandidat ist ein moegliches Buchungsfenster: frei, in der
// gewuenschten Laenge, innerhalb der Oeffnungszeiten.
type Fensterkandidat struct {
	Beginn time.Time
	Ende   time.Time
}

// FreieFenster liefert alle moeglichen Buchungsfenster der Laenge dauer auf
// rampe am Kalendertag tag: innerhalb der Oeffnungszeiten von standort, nicht
// vor jetzt, ohne Ueberschneidung mit buchungenDesStandorts.
//
// Prueft NICHT, ob rampe den gewuenschten Temperaturbereich unterstuetzt
// (Regel 2) — das entscheidet der Aufrufer VOR dem Aufruf, indem er nur
// passende Rampen ueberhaupt hier hineinreicht (MVP.md §7 "Verfuegbarkeit").
// Ebenso wird dauer selbst nicht gegen Regel 5 geprueft — Sache der
// Transport-Validierung, nicht dieser reinen Suchfunktion.
func FreieFenster(standort Standort, rampe Rampe, tag time.Time, dauer time.Duration, buchungenDesStandorts []Buchung, jetzt time.Time) []Fensterkandidat {
	tagBeginn, _ := standort.TagesgrenzenUTC(tag)
	oeffnungBeginn := tagBeginn.Add(standort.OeffnungVon)
	oeffnungEnde := tagBeginn.Add(standort.OeffnungBis)

	belegt := belegteSlotsFuerRampe(buchungenDesStandorts, rampe.ID)

	// oeffnungBeginn ist rastergerecht: OeffnungVon ist ein Vielfaches von
	// SlotDauer, und tagBeginn ist lokale Mitternacht in einer Zeitzone mit
	// vollem Stunden-Offset (dieselbe Annahme wie IstImRaster). Jeder Schritt
	// um SlotDauer bleibt deshalb automatisch im Raster — keine gesonderte
	// IstImRaster-Pruefung noetig.
	var ergebnis []Fensterkandidat
	for beginn := oeffnungBeginn; !beginn.Add(dauer).After(oeffnungEnde); beginn = beginn.Add(SlotDauer) {
		if beginn.Before(jetzt) {
			continue
		}
		ende := beginn.Add(dauer)
		if ueberschneidetBelegung(beginn, ende, belegt) {
			continue
		}
		ergebnis = append(ergebnis, Fensterkandidat{Beginn: beginn, Ende: ende})
	}
	return ergebnis
}

// belegteSlotsFuerRampe baut die Menge belegter Slot-Startzeiten einer
// einzelnen Rampe aus allen Buchungen eines Standorts (die anderer Rampen
// werden herausgefiltert). int64 (UnixNano) statt time.Time als Map-Key:
// time.Time kann eine monotone Uhrzeit-Komponente tragen, mit der == laut
// time-Dokumentation unzuverlaessig ist — UnixNano umgeht das vollstaendig.
func belegteSlotsFuerRampe(buchungen []Buchung, rampeID int64) map[int64]bool {
	belegt := make(map[int64]bool)
	for _, b := range buchungen {
		if b.RampeID != rampeID || b.Storniert() {
			continue
		}
		for _, slot := range Slots(b.Beginn, b.Ende) {
			belegt[slot.UnixNano()] = true
		}
	}
	return belegt
}

func ueberschneidetBelegung(beginn, ende time.Time, belegt map[int64]bool) bool {
	for _, slot := range Slots(beginn, ende) {
		if belegt[slot.UnixNano()] {
			return true
		}
	}
	return false
}
