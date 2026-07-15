package zeitfenster

import (
	"errors"
	"testing"
	"time"
)

func ereignis(typ EreignisTyp, zeitpunkt time.Time) BuchungEreignis {
	return BuchungEreignis{BuchungID: 1, Typ: typ, Zeitpunkt: zeitpunkt}
}

func TestParseEreignisTyp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    EreignisTyp
		wantErr bool
	}{
		{name: "ANGEKOMMEN gueltig", input: "ANGEKOMMEN", want: Angekommen},
		{name: "ANGEDOCKT gueltig", input: "ANGEDOCKT", want: Angedockt},
		{name: "ABGEFAHREN gueltig", input: "ABGEFAHREN", want: Abgefahren},
		{name: "NO_SHOW ist kein Ereignistyp", input: "NO_SHOW", wantErr: true},
		{name: "unbekannt", input: "IRGENDWAS", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEreignisTyp(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("erwartete Fehler, bekam nil")
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

func TestBerechneStatus(t *testing.T) {
	beginn := utc(6, 30)

	tests := []struct {
		name        string
		ereignisse  []BuchungEreignis
		storniertAm *time.Time
		now         time.Time
		want        Status
	}{
		{
			name: "keine Ereignisse, innerhalb Karenzzeit -> GEBUCHT",
			now:  beginn.Add(5 * time.Minute),
			want: StatusGebucht,
		},
		{
			name: "keine Ereignisse, genau an der Karenzgrenze -> GEBUCHT",
			now:  beginn.Add(Karenzzeit), // nicht "danach", also noch kein NO_SHOW
			want: StatusGebucht,
		},
		{
			name: "keine Ereignisse, Karenzzeit ueberschritten -> NO_SHOW",
			now:  beginn.Add(Karenzzeit + time.Minute),
			want: StatusNoShow,
		},
		{
			name: "nur ANGEKOMMEN, auch lange nach Karenzzeit -> ANGEKOMMEN, kein NO_SHOW",
			ereignisse: []BuchungEreignis{
				ereignis(Angekommen, beginn.Add(2*time.Minute)),
			},
			now:  beginn.Add(3 * time.Hour),
			want: StatusAngekommen,
		},
		{
			name: "ANGEKOMMEN + ANGEDOCKT -> ANGEDOCKT",
			ereignisse: []BuchungEreignis{
				ereignis(Angekommen, beginn.Add(2*time.Minute)),
				ereignis(Angedockt, beginn.Add(5*time.Minute)),
			},
			now:  beginn.Add(10 * time.Minute),
			want: StatusAngedockt,
		},
		{
			name: "ANGEKOMMEN + ANGEDOCKT + ABGEFAHREN -> ABGEFAHREN",
			ereignisse: []BuchungEreignis{
				ereignis(Angekommen, beginn.Add(2*time.Minute)),
				ereignis(Angedockt, beginn.Add(5*time.Minute)),
				ereignis(Abgefahren, beginn.Add(40*time.Minute)),
			},
			now:  beginn.Add(time.Hour),
			want: StatusAbgefahren,
		},
		{
			name: "storniert gewinnt, auch mit Ereignissen im Journal",
			ereignisse: []BuchungEreignis{
				ereignis(Angekommen, beginn.Add(2*time.Minute)),
			},
			storniertAm: timePtr(beginn.Add(time.Minute)),
			now:         beginn.Add(10 * time.Minute),
			want:        StatusStorniert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BerechneStatus(tt.ereignisse, tt.storniertAm, beginn, tt.now)
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPruefeEreignis(t *testing.T) {
	tests := []struct {
		name        string
		ereignisse  []BuchungEreignis
		storniertAm *time.Time
		neu         EreignisTyp
		wantErr     error // nil bedeutet: zulaessig
	}{
		{
			name: "ANGEKOMMEN auf leerem Journal ist zulaessig",
			neu:  Angekommen,
		},
		{
			name:       "ANGEKOMMEN doppelt ist unzulaessig (Regel 12)",
			ereignisse: []BuchungEreignis{ereignis(Angekommen, utc(6, 30))},
			neu:        Angekommen,
			wantErr:    ErrUngueltigerStatuswechsel,
		},
		{
			name:    "ANGEDOCKT ohne vorheriges ANGEKOMMEN ist unzulaessig (Regel 9)",
			neu:     Angedockt,
			wantErr: ErrUngueltigerStatuswechsel,
		},
		{
			name:       "ANGEDOCKT nach ANGEKOMMEN ist zulaessig",
			ereignisse: []BuchungEreignis{ereignis(Angekommen, utc(6, 30))},
			neu:        Angedockt,
		},
		{
			name: "ANGEDOCKT doppelt ist unzulaessig (Regel 12)",
			ereignisse: []BuchungEreignis{
				ereignis(Angekommen, utc(6, 30)),
				ereignis(Angedockt, utc(6, 35)),
			},
			neu:     Angedockt,
			wantErr: ErrUngueltigerStatuswechsel,
		},
		{
			name:       "ABGEFAHREN ohne vorheriges ANGEDOCKT ist unzulaessig (Regel 10)",
			ereignisse: []BuchungEreignis{ereignis(Angekommen, utc(6, 30))},
			neu:        Abgefahren,
			wantErr:    ErrUngueltigerStatuswechsel,
		},
		{
			name: "ABGEFAHREN nach ANGEDOCKT ist zulaessig",
			ereignisse: []BuchungEreignis{
				ereignis(Angekommen, utc(6, 30)),
				ereignis(Angedockt, utc(6, 35)),
			},
			neu: Abgefahren,
		},
		{
			name:        "storniert -> jedes Ereignis unzulaessig (Regel 11)",
			storniertAm: timePtr(utc(6, 0)),
			neu:         Angekommen,
			wantErr:     ErrBuchungStorniert,
		},
		{
			name:        "storniert gewinnt auch bei sonst gueltiger Reihenfolge",
			ereignisse:  []BuchungEreignis{ereignis(Angekommen, utc(6, 30))},
			storniertAm: timePtr(utc(6, 40)),
			neu:         Angedockt,
			wantErr:     ErrBuchungStorniert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := PruefeEreignis(tt.ereignisse, tt.storniertAm, tt.neu)
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

func timePtr(t time.Time) *time.Time { return &t }
