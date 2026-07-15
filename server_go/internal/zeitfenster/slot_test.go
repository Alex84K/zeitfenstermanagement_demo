package zeitfenster

import (
	"testing"
	"time"
)

func utc(hour, min int) time.Time {
	return time.Date(2026, 7, 20, hour, min, 0, 0, time.UTC)
}

func TestIstImRaster(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want bool
	}{
		{name: "volle Stunde", in: utc(6, 0), want: true},
		{name: "halbe Stunde", in: utc(6, 30), want: true},
		{name: "Mitternacht", in: utc(0, 0), want: true},
		{name: "15 Minuten daneben", in: utc(6, 15), want: false},
		{name: "1 Minute daneben", in: utc(6, 1), want: false},
		{name: "45 Minuten", in: utc(6, 45), want: false},
		{
			name: "volle Stunde, aber mit Sekunden",
			in:   time.Date(2026, 7, 20, 6, 0, 1, 0, time.UTC),
			want: false,
		},
		{
			name: "volle Stunde, aber mit Nanosekunden",
			in:   time.Date(2026, 7, 20, 6, 0, 0, 1, time.UTC),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IstImRaster(tt.in); got != tt.want {
				t.Errorf("IstImRaster(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestAnzahlSlots(t *testing.T) {
	tests := []struct {
		name         string
		beginn, ende time.Time
		want         int
	}{
		{name: "30 Minuten = 1 Slot", beginn: utc(6, 30), ende: utc(7, 0), want: 1},
		{name: "60 Minuten = 2 Slots", beginn: utc(6, 30), ende: utc(7, 30), want: 2},
		{name: "4 Stunden = 8 Slots", beginn: utc(6, 0), ende: utc(10, 0), want: 8},
		{name: "ende vor beginn", beginn: utc(7, 0), ende: utc(6, 0), want: 0},
		{name: "ende gleich beginn", beginn: utc(6, 0), ende: utc(6, 0), want: 0},
		{name: "nicht durch 30min teilbar wird abgeschnitten", beginn: utc(6, 0), ende: utc(6, 45), want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AnzahlSlots(tt.beginn, tt.ende); got != tt.want {
				t.Errorf("AnzahlSlots(%v, %v) = %d, want %d", tt.beginn, tt.ende, got, tt.want)
			}
		})
	}
}

// TestSlots_Exklusiv ist der wichtigste Test in dieser Datei: er belegt, dass
// eine Buchung 06:30-07:30 zwei Slots liefert (06:30, 07:00) und NICHT drei —
// der off-by-one, den MVP.md §5 als zentrale Falle benennt.
func TestSlots_Exklusiv(t *testing.T) {
	tests := []struct {
		name         string
		beginn, ende time.Time
		want         []time.Time
	}{
		{
			name:   "90 Minuten -> zwei Slots, 07:30 bleibt frei",
			beginn: utc(6, 30),
			ende:   utc(7, 30),
			want:   []time.Time{utc(6, 30), utc(7, 0)},
		},
		{
			name:   "30 Minuten -> ein Slot",
			beginn: utc(6, 0),
			ende:   utc(6, 30),
			want:   []time.Time{utc(6, 0)},
		},
		{
			name:   "120 Minuten -> drei Slots",
			beginn: utc(6, 30),
			ende:   utc(8, 0),
			want:   []time.Time{utc(6, 30), utc(7, 0), utc(7, 30)},
		},
		{
			name:   "leeres Intervall -> keine Slots",
			beginn: utc(6, 0),
			ende:   utc(6, 0),
			want:   nil,
		},
		{
			name:   "negatives Intervall -> keine Slots",
			beginn: utc(7, 0),
			ende:   utc(6, 0),
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slots(tt.beginn, tt.ende)
			if len(got) != len(tt.want) {
				t.Fatalf("Slots(%v, %v) = %v (len %d), want %v (len %d)",
					tt.beginn, tt.ende, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if !got[i].Equal(tt.want[i]) {
					t.Errorf("Slots()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
			// 07:30 darf in keinem "90 Minuten"-Fall auftauchen — das waere der
			// off-by-one, der den naechsten Slot faelschlich blockieren wuerde.
			if tt.name == "90 Minuten -> zwei Slots, 07:30 bleibt frei" {
				for _, s := range got {
					if s.Equal(utc(7, 30)) {
						t.Error("Slots() enthaelt 07:30 — ende muss exklusiv sein")
					}
				}
			}
		})
	}
}
