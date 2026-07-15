package zeitfenster

import (
	"testing"
	"time"
)

// Testszenario: Standort mit engem Oeffnungsfenster 05:00-07:00 lokal
// (Europe/Berlin, Sommerzeit UTC+2 => 03:00-05:00 UTC), damit sich alle
// Kandidaten von Hand nachrechnen lassen. tag = 2026-07-20.
func testStandortEngeOeffnung(t *testing.T) Standort {
	t.Helper()
	standort, err := NewStandort(1, "Versmold", "", "Europe/Berlin", 5*time.Hour, 7*time.Hour, true)
	if err != nil {
		t.Fatalf("NewStandort: %v", err)
	}
	return standort
}

func TestFreieFenster_LeererTag(t *testing.T) {
	standort := testStandortEngeOeffnung(t)
	rampe := Rampe{ID: 12}
	tag := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	jetzt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC) // weit vor tag

	// Oeffnung UTC: 03:00-05:00. Mit dauer=30min sind alle vier Rasterpunkte
	// gueltige Kandidaten: 03:00, 03:30, 04:00, 04:30.
	kandidaten := FreieFenster(standort, rampe, tag, SlotDauer, nil, jetzt)

	want := []time.Time{utc(3, 0), utc(3, 30), utc(4, 0), utc(4, 30)}
	if len(kandidaten) != len(want) {
		t.Fatalf("got %d Kandidaten, want %d: %+v", len(kandidaten), len(want), kandidaten)
	}
	for i, w := range want {
		if !kandidaten[i].Beginn.Equal(w) {
			t.Errorf("kandidaten[%d].Beginn = %v, want %v", i, kandidaten[i].Beginn, w)
		}
		if !kandidaten[i].Ende.Equal(w.Add(SlotDauer)) {
			t.Errorf("kandidaten[%d].Ende = %v, want %v", i, kandidaten[i].Ende, w.Add(SlotDauer))
		}
	}
}

func TestFreieFenster_DauerSchliesstSpaeteKandidatenAus(t *testing.T) {
	standort := testStandortEngeOeffnung(t)
	rampe := Rampe{ID: 12}
	tag := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	jetzt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// dauer=60min: 04:30+60min=05:30 > Schliessung 05:00 -> ausgeschlossen.
	// Uebrig: 03:00, 03:30, 04:00 (04:00+60min=05:00, genau an der Grenze -> erlaubt).
	kandidaten := FreieFenster(standort, rampe, tag, time.Hour, nil, jetzt)

	want := []time.Time{utc(3, 0), utc(3, 30), utc(4, 0)}
	if len(kandidaten) != len(want) {
		t.Fatalf("got %d Kandidaten, want %d: %+v", len(kandidaten), len(want), kandidaten)
	}
	for i, w := range want {
		if !kandidaten[i].Beginn.Equal(w) {
			t.Errorf("kandidaten[%d].Beginn = %v, want %v", i, kandidaten[i].Beginn, w)
		}
	}
}

func TestFreieFenster_BelegterSlotBlockiertNurUeberlappendeKandidaten(t *testing.T) {
	standort := testStandortEngeOeffnung(t)
	rampe := Rampe{ID: 12}
	tag := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	jetzt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// Buchung 03:30-04:00 UTC auf dieser Rampe belegt genau den Slot 03:30.
	buchungen := []Buchung{{
		RampeID: 12,
		Beginn:  utc(3, 30),
		Ende:    utc(4, 0),
	}}

	kandidaten := FreieFenster(standort, rampe, tag, SlotDauer, buchungen, jetzt)

	want := []time.Time{utc(3, 0), utc(4, 0), utc(4, 30)} // 03:30 fehlt
	if len(kandidaten) != len(want) {
		t.Fatalf("got %d Kandidaten, want %d: %+v", len(kandidaten), len(want), kandidaten)
	}
	for i, w := range want {
		if !kandidaten[i].Beginn.Equal(w) {
			t.Errorf("kandidaten[%d].Beginn = %v, want %v", i, kandidaten[i].Beginn, w)
		}
	}
}

func TestFreieFenster_AndereRampeBlockiertNicht(t *testing.T) {
	standort := testStandortEngeOeffnung(t)
	rampe := Rampe{ID: 12}
	tag := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	jetzt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	// Buchung auf einer ANDEREN Rampe (ID 13) zur selben Zeit darf Rampe 12
	// nicht beeinflussen.
	buchungen := []Buchung{{
		RampeID: 13,
		Beginn:  utc(3, 30),
		Ende:    utc(4, 0),
	}}

	kandidaten := FreieFenster(standort, rampe, tag, SlotDauer, buchungen, jetzt)
	if len(kandidaten) != 4 {
		t.Fatalf("got %d Kandidaten, want 4 (Buchung auf anderer Rampe darf nicht blockieren)", len(kandidaten))
	}
}

func TestFreieFenster_StornierteBuchungBlockiertNicht(t *testing.T) {
	standort := testStandortEngeOeffnung(t)
	rampe := Rampe{ID: 12}
	tag := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	jetzt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	storniertZeitpunkt := utc(1, 0)

	buchungen := []Buchung{{
		RampeID:     12,
		Beginn:      utc(3, 30),
		Ende:        utc(4, 0),
		StorniertAm: &storniertZeitpunkt,
	}}

	kandidaten := FreieFenster(standort, rampe, tag, SlotDauer, buchungen, jetzt)
	if len(kandidaten) != 4 {
		t.Fatalf("got %d Kandidaten, want 4 (stornierte Buchung darf nicht blockieren)", len(kandidaten))
	}
}

func TestFreieFenster_JetztFiltertVergangeneKandidaten(t *testing.T) {
	standort := testStandortEngeOeffnung(t)
	rampe := Rampe{ID: 12}
	tag := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	jetzt := utc(3, 45) // zwischen 03:30 und 04:00

	kandidaten := FreieFenster(standort, rampe, tag, SlotDauer, nil, jetzt)

	want := []time.Time{utc(4, 0), utc(4, 30)} // 03:00, 03:30 liegen vor jetzt
	if len(kandidaten) != len(want) {
		t.Fatalf("got %d Kandidaten, want %d: %+v", len(kandidaten), len(want), kandidaten)
	}
	for i, w := range want {
		if !kandidaten[i].Beginn.Equal(w) {
			t.Errorf("kandidaten[%d].Beginn = %v, want %v", i, kandidaten[i].Beginn, w)
		}
	}
}
