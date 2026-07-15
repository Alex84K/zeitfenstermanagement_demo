package zeitfenster

import "time"

// SlotDauer ist die Groesse eines Zeitfensters auf dem Buchungsraster — 30 Minuten.
// MVP.md §3: Zeit ist in der Lebensmittellogistik knapper als bei Trockenware,
// daher ein feineres Raster, als es fuer eine Meetingraum-Buchung ueblich waere.
const SlotDauer = 30 * time.Minute

// MinDauer und MaxDauer begrenzen die Laenge einer Buchung (Regel 5): 1 bis 8 Slots.
const (
	MinDauer = SlotDauer
	MaxDauer = 8 * SlotDauer // 4 Stunden
)

// IstImRaster meldet, ob t exakt auf eine Slot-Grenze faellt (volle oder halbe
// Stunde, ohne Sekunden/Nanosekunden) — Regel 4.
//
// Das prueft die Uhrzeit von t direkt, ohne Umrechnung in eine Zeitzone. Das ist
// korrekt, solange alle relevanten Zeitzonen einen vollstuendigen Stunden-Offset
// zu UTC haben — bei Nagel gilt das fuer DE/PL/CZ/NL. Ein Standort mit
// Halbstunden-Offset (z.B. Indien) wuerde diese Annahme brechen.
func IstImRaster(t time.Time) bool {
	return t.Second() == 0 && t.Nanosecond() == 0 && t.Minute()%30 == 0
}

// AnzahlSlots liefert die Anzahl der 30-Minuten-Slots in [beginn, ende).
//
// Sind beginn und ende beide im Raster (IstImRaster), ist das Ergebnis exakt —
// der Abstand zweier Rasterpunkte ist immer ein Vielfaches von 30 Minuten.
// Andernfalls (z.B. 06:15-07:00) wird abgeschnitten; das ist dann bereits ueber
// IstImRaster als Regel-4-Verstoss erkannt, bevor die Slot-Anzahl ueberhaupt zaehlt.
func AnzahlSlots(beginn, ende time.Time) int {
	dauer := ende.Sub(beginn)
	if dauer <= 0 {
		return 0
	}
	return int(dauer / SlotDauer)
}

// Slots liefert die Startzeitpunkte aller 30-Minuten-Slots, die [beginn, ende)
// belegt. ende ist EXKLUSIV: eine Buchung 06:30-07:30 belegt die Slots 06:30
// und 07:00, nicht 07:30 — MVP.md §5 begruendet, warum das kein Off-by-one ist,
// sondern die Slot-Grenze fuer die naechste Buchung frei laesst.
func Slots(beginn, ende time.Time) []time.Time {
	n := AnzahlSlots(beginn, ende)
	if n <= 0 {
		return nil
	}
	slots := make([]time.Time, n)
	for i := range slots {
		slots[i] = beginn.Add(time.Duration(i) * SlotDauer)
	}
	return slots
}
