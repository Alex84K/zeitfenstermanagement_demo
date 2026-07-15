package zeitfenster

import (
	"errors"
	"testing"
	"time"
)

// gueltigeBuchungsszenario liefert eine vollstaendig gueltige Kombination aus
// Buchung, Rampe, Standort und jetzt. Jeder Testfall veraendert genau ein
// Element davon, um genau eine Regel zu verletzen — die anderen bleiben gueltig.
//
// Datum: 2026-07-20, Sommerzeit (Europe/Berlin = UTC+2). beginn=06:30 UTC
// entspricht 08:30 lokal, deutlich innerhalb 05:00-22:00.
func gueltigesBuchungsszenario(t *testing.T) (Buchung, Rampe, Standort, time.Time) {
	t.Helper()

	rampe, err := NewRampe(12, 1, "Tor 12", NewTemperaturbereichSet(TK, Frisch), true)
	if err != nil {
		t.Fatalf("NewRampe: %v", err)
	}

	standort, err := NewStandort(1, "Versmold", "", "Europe/Berlin", 5*time.Hour, 22*time.Hour, true)
	if err != nil {
		t.Fatalf("NewStandort: %v", err)
	}

	buchung := Buchung{
		RampeID:           rampe.ID,
		Spediteur:         "Spedition Mueller",
		Kennzeichen:       "GT-ML 1234",
		Temperaturbereich: TK,
		Beginn:            utc(6, 30),
		Ende:              utc(7, 30), // 60 Minuten = 2 Slots
	}

	jetzt := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC) // Vortag — klar in der Vergangenheit relativ zu beginn

	return buchung, rampe, standort, jetzt
}

func TestPruefeBuchung(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time)
		wantErr error // nil bedeutet: keine Verletzung erwartet
	}{
		{
			name:   "gueltiges Basisszenario",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {},
		},
		{
			name: "Regel 4: beginn nicht im Raster",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				b.Beginn = time.Date(2026, 7, 20, 6, 15, 0, 0, time.UTC)
			},
			wantErr: ErrNichtImRaster,
		},
		{
			name: "Regel 4: ende nicht im Raster",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				b.Ende = time.Date(2026, 7, 20, 7, 45, 0, 0, time.UTC)
			},
			wantErr: ErrNichtImRaster,
		},
		{
			name: "Regel 5: Dauer null (ende == beginn)",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				b.Ende = b.Beginn
			},
			wantErr: ErrUngueltigeDauer,
		},
		{
			name: "Regel 5: Dauer ueber 4 Stunden",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				b.Ende = utc(11, 0) // 4,5 Stunden ab 06:30
			},
			wantErr: ErrUngueltigeDauer,
		},
		{
			name: "Regel 6: Buchung liegt in der Vergangenheit",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				*jetzt = utc(7, 0) // nach beginn (06:30)
			},
			wantErr: ErrVergangenheit,
		},
		{
			name: "Regel 7: Rampe inaktiv",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				r.Aktiv = false
			},
			wantErr: ErrRampeInaktiv,
		},
		{
			name: "Regel 7: Standort inaktiv",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				s.Aktiv = false
			},
			wantErr: ErrRampeInaktiv,
		},
		{
			name: "Regel 2: Temperaturbereich nicht unterstuetzt",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				b.Temperaturbereich = Trocken // Rampe unterstuetzt nur TK, FRISCH
			},
			wantErr: ErrTemperaturMismatch,
		},
		{
			name: "Regel 3: vor Oeffnung (04:00 lokal)",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				b.Beginn = utc(2, 0) // 04:00 lokal, Oeffnung erst ab 05:00
				b.Ende = utc(2, 30)
			},
			wantErr: ErrAusserhalbOeffnungszeit,
		},
		{
			name: "Regel 3: nach Schliessung (22:30 lokal)",
			mutate: func(b *Buchung, r *Rampe, s *Standort, jetzt *time.Time) {
				b.Beginn = utc(20, 30) // 22:30 lokal
				b.Ende = utc(21, 0)    // 23:00 lokal, Schliessung um 22:00
			},
			wantErr: ErrAusserhalbOeffnungszeit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buchung, rampe, standort, jetzt := gueltigesBuchungsszenario(t)
			tt.mutate(&buchung, &rampe, &standort, &jetzt)

			err := PruefeBuchung(buchung, rampe, standort, jetzt)

			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unerwarteter Fehler: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuchung_Storniert(t *testing.T) {
	aktiv := Buchung{}
	if aktiv.Storniert() {
		t.Error("frische Buchung sollte nicht storniert sein")
	}

	zeitpunkt := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	storniert := Buchung{StorniertAm: &zeitpunkt}
	if !storniert.Storniert() {
		t.Error("Buchung mit StorniertAm sollte storniert sein")
	}
}
