package zeitfenster

import (
	"errors"
	"testing"
	"time"
)

func mustLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("time.LoadLocation(%q): %v", name, err)
	}
	return loc
}

func TestParseTemperaturbereich(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Temperaturbereich
		wantErr bool
	}{
		{name: "TK gueltig", input: "TK", want: TK},
		{name: "FRISCH gueltig", input: "FRISCH", want: Frisch},
		{name: "TROCKEN gueltig", input: "TROCKEN", want: Trocken},
		{name: "unbekannt", input: "WARM", wantErr: true},
		{name: "leer", input: "", wantErr: true},
		{name: "falsche Gross-Kleinschreibung", input: "tk", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTemperaturbereich(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("erwartete Fehler, bekam nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unerwarteter Fehler: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTemperaturbereichSet_Enthaelt(t *testing.T) {
	set := NewTemperaturbereichSet(TK, Frisch)

	if !set.Enthaelt(TK) {
		t.Error("TK sollte enthalten sein")
	}
	if !set.Enthaelt(Frisch) {
		t.Error("FRISCH sollte enthalten sein")
	}
	if set.Enthaelt(Trocken) {
		t.Error("TROCKEN sollte nicht enthalten sein")
	}

	leer := NewTemperaturbereichSet()
	if leer.Enthaelt(TK) {
		t.Error("leere Menge sollte nichts enthalten")
	}
}

func TestNewStandort_UngueltigeZeitzone(t *testing.T) {
	_, err := NewStandort(1, "Versmold", "", "Nirgendwo/Erfunden", 5*time.Hour, 22*time.Hour, true)
	if !errors.Is(err, ErrUngueltigeZeitzone) {
		t.Fatalf("got %v, want ErrUngueltigeZeitzone", err)
	}
}

func TestNewStandort_GueltigeZeitzone(t *testing.T) {
	s, err := NewStandort(1, "Versmold", "Industriestrasse 1", "Europe/Berlin", 5*time.Hour, 22*time.Hour, true)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if s.Zeitzone.String() != "Europe/Berlin" {
		t.Errorf("Zeitzone = %q, want Europe/Berlin", s.Zeitzone.String())
	}
}

// TestStandort_TagesgrenzenUTC_Sommerzeit prueft den DST-Uebergang: Europe/Berlin
// wechselt Ende Maerz von UTC+1 auf UTC+2. Eine feste Offset-Konstante waere hier falsch.
func TestStandort_TagesgrenzenUTC_Sommerzeit(t *testing.T) {
	berlin := mustLocation(t, "Europe/Berlin")

	tests := []struct {
		name       string
		standort   Standort
		datum      time.Time
		wantBeginn time.Time
		wantEnde   time.Time
	}{
		{
			name:       "Winterzeit, UTC+1",
			standort:   Standort{Zeitzone: berlin},
			datum:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			wantBeginn: time.Date(2026, 1, 14, 23, 0, 0, 0, time.UTC),
			wantEnde:   time.Date(2026, 1, 15, 23, 0, 0, 0, time.UTC),
		},
		{
			name:       "Sommerzeit, UTC+2",
			standort:   Standort{Zeitzone: berlin},
			datum:      time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
			wantBeginn: time.Date(2026, 7, 19, 22, 0, 0, 0, time.UTC),
			wantEnde:   time.Date(2026, 7, 20, 22, 0, 0, 0, time.UTC),
		},
		{
			// Tag der Umstellung selbst: 2026-03-29 hat in Berlin nur 23 Stunden.
			// time.Date muss das korrekt aufloesen, keine feste 24h-Arithmetik.
			name:       "Umstellungstag Winter zu Sommer",
			standort:   Standort{Zeitzone: berlin},
			datum:      time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			wantBeginn: time.Date(2026, 3, 28, 23, 0, 0, 0, time.UTC),
			wantEnde:   time.Date(2026, 3, 29, 22, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBeginn, gotEnde := tt.standort.TagesgrenzenUTC(tt.datum)
			if !gotBeginn.Equal(tt.wantBeginn) {
				t.Errorf("beginn = %v, want %v", gotBeginn, tt.wantBeginn)
			}
			if !gotEnde.Equal(tt.wantEnde) {
				t.Errorf("ende = %v, want %v", gotEnde, tt.wantEnde)
			}
		})
	}
}

func TestStandort_IstInnerhalbOeffnungszeiten(t *testing.T) {
	berlin := mustLocation(t, "Europe/Berlin")
	standort := Standort{
		Zeitzone:    berlin,
		OeffnungVon: 5 * time.Hour,
		OeffnungBis: 22 * time.Hour,
	}

	// 2026-07-20 ist Sommerzeit (UTC+2): 06:30 lokal = 04:30 UTC.
	tests := []struct {
		name   string
		beginn time.Time
		ende   time.Time
		want   bool
	}{
		{
			name:   "innerhalb der Oeffnungszeiten",
			beginn: time.Date(2026, 7, 20, 4, 30, 0, 0, time.UTC),
			ende:   time.Date(2026, 7, 20, 5, 30, 0, 0, time.UTC),
			want:   true,
		},
		{
			name:   "beginnt vor Oeffnung (04:00 lokal)",
			beginn: time.Date(2026, 7, 20, 2, 0, 0, 0, time.UTC),
			ende:   time.Date(2026, 7, 20, 3, 0, 0, 0, time.UTC),
			want:   false,
		},
		{
			name:   "endet nach Schliessung (22:30 lokal)",
			beginn: time.Date(2026, 7, 20, 19, 30, 0, 0, time.UTC),
			ende:   time.Date(2026, 7, 20, 20, 30, 0, 0, time.UTC),
			want:   false,
		},
		{
			name:   "genau an der Oeffnungsgrenze (05:00-06:00 lokal)",
			beginn: time.Date(2026, 7, 20, 3, 0, 0, 0, time.UTC),
			ende:   time.Date(2026, 7, 20, 4, 0, 0, 0, time.UTC),
			want:   true,
		},
		{
			name:   "genau an der Schliessungsgrenze (21:00-22:00 lokal)",
			beginn: time.Date(2026, 7, 20, 19, 0, 0, 0, time.UTC),
			ende:   time.Date(2026, 7, 20, 20, 0, 0, 0, time.UTC),
			want:   true,
		},
		{
			name:   "ueber Mitternacht hinweg",
			beginn: time.Date(2026, 7, 20, 21, 30, 0, 0, time.UTC), // 23:30 lokal
			ende:   time.Date(2026, 7, 20, 22, 30, 0, 0, time.UTC), // 00:30 lokal naechster Tag
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := standort.IstInnerhalbOeffnungszeiten(tt.beginn, tt.ende)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewRampe_KeinTemperaturbereich(t *testing.T) {
	_, err := NewRampe(1, 1, "Tor 12", NewTemperaturbereichSet(), true)
	if !errors.Is(err, ErrKeinTemperaturbereich) {
		t.Fatalf("got %v, want ErrKeinTemperaturbereich", err)
	}
}

func TestRampe_UnterstuetztTemperaturbereich(t *testing.T) {
	rampe, err := NewRampe(1, 1, "Tor 12", NewTemperaturbereichSet(TK, Frisch), true)
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}

	if !rampe.UnterstuetztTemperaturbereich(TK) {
		t.Error("Tor 12 sollte TK unterstuetzen")
	}
	if rampe.UnterstuetztTemperaturbereich(Trocken) {
		t.Error("Tor 12 sollte TROCKEN nicht unterstuetzen")
	}
}
