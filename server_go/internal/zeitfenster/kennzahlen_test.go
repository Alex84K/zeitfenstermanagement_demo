package zeitfenster

import (
	"testing"
	"time"
)

func TestStandzeit(t *testing.T) {
	tests := []struct {
		name       string
		ereignisse []BuchungEreignis
		wantDauer  time.Duration
		wantOk     bool
	}{
		{
			name: "beide Ereignisse vorhanden",
			ereignisse: []BuchungEreignis{
				ereignis(Angekommen, utc(6, 25)),
				ereignis(Angedockt, utc(6, 30)),
				ereignis(Abgefahren, utc(7, 7)),
			},
			wantDauer: 42 * time.Minute,
			wantOk:    true,
		},
		{
			name:       "ABGEFAHREN fehlt -> nicht berechenbar",
			ereignisse: []BuchungEreignis{ereignis(Angekommen, utc(6, 25))},
			wantOk:     false,
		},
		{
			name:       "keine Ereignisse -> nicht berechenbar",
			ereignisse: nil,
			wantOk:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDauer, gotOk := Standzeit(tt.ereignisse)
			if gotOk != tt.wantOk {
				t.Fatalf("ok = %v, want %v", gotOk, tt.wantOk)
			}
			if gotOk && gotDauer != tt.wantDauer {
				t.Errorf("dauer = %v, want %v", gotDauer, tt.wantDauer)
			}
		})
	}
}

func TestPuenktlich(t *testing.T) {
	beginn := utc(6, 30)

	tests := []struct {
		name           string
		ereignisse     []BuchungEreignis
		wantPuenktlich bool
		wantOk         bool
	}{
		{
			name:           "Ankunft genau zur beginn-Zeit",
			ereignisse:     []BuchungEreignis{ereignis(Angekommen, beginn)},
			wantPuenktlich: true,
			wantOk:         true,
		},
		{
			name:           "Ankunft 10 Minuten frueher -> puenktlich, kein Verstoss",
			ereignisse:     []BuchungEreignis{ereignis(Angekommen, beginn.Add(-10*time.Minute))},
			wantPuenktlich: true,
			wantOk:         true,
		},
		{
			name:           "Ankunft genau an der Karenzgrenze (+15min) -> noch puenktlich",
			ereignisse:     []BuchungEreignis{ereignis(Angekommen, beginn.Add(Karenzzeit))},
			wantPuenktlich: true,
			wantOk:         true,
		},
		{
			name:           "Ankunft 1 Minute nach der Karenzgrenze -> unpuenktlich",
			ereignisse:     []BuchungEreignis{ereignis(Angekommen, beginn.Add(Karenzzeit+time.Minute))},
			wantPuenktlich: false,
			wantOk:         true,
		},
		{
			name:   "keine Ankunft -> nicht berechenbar",
			wantOk: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPuenktlich, gotOk := Puenktlich(tt.ereignisse, beginn)
			if gotOk != tt.wantOk {
				t.Fatalf("ok = %v, want %v", gotOk, tt.wantOk)
			}
			if gotOk && gotPuenktlich != tt.wantPuenktlich {
				t.Errorf("puenktlich = %v, want %v", gotPuenktlich, tt.wantPuenktlich)
			}
		})
	}
}

func TestIstNoShow(t *testing.T) {
	beginn := utc(6, 30)

	tests := []struct {
		name        string
		ereignisse  []BuchungEreignis
		storniertAm *time.Time
		now         time.Time
		want        bool
	}{
		{
			name: "keine Ereignisse, Karenzzeit ueberschritten -> NO_SHOW",
			now:  beginn.Add(Karenzzeit + time.Minute),
			want: true,
		},
		{
			name: "keine Ereignisse, noch innerhalb Karenzzeit -> kein NO_SHOW",
			now:  beginn.Add(5 * time.Minute),
			want: false,
		},
		{
			name:       "ANGEKOMMEN vorhanden -> kein NO_SHOW, auch spaet",
			ereignisse: []BuchungEreignis{ereignis(Angekommen, beginn.Add(2*time.Minute))},
			now:        beginn.Add(3 * time.Hour),
			want:       false,
		},
		{
			name:        "storniert -> kein NO_SHOW",
			storniertAm: timePtr(beginn.Add(-time.Hour)),
			now:         beginn.Add(Karenzzeit + time.Minute),
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IstNoShow(tt.ereignisse, tt.storniertAm, beginn, tt.now)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
