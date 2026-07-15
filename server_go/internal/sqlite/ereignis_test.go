package sqlite

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"zeitfenster/internal/zeitfenster"
)

func TestEreignisStore_AppendListByBuchung(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition A", "GT-A 1")
	if err := NewBuchungStore(db).Create(ctx, &buchung); err != nil {
		t.Fatalf("BuchungStore.Create: %v", err)
	}

	store := NewEreignisStore(db)
	ereignisse := []zeitfenster.BuchungEreignis{
		{BuchungID: buchung.ID, Typ: zeitfenster.Angekommen, Zeitpunkt: buchung.Beginn.Add(-2 * time.Minute), ErfasstVon: "Pfoertner"},
		{BuchungID: buchung.ID, Typ: zeitfenster.Angedockt, Zeitpunkt: buchung.Beginn.Add(3 * time.Minute), ErfasstVon: "Pfoertner"},
		{BuchungID: buchung.ID, Typ: zeitfenster.Abgefahren, Zeitpunkt: buchung.Beginn.Add(40 * time.Minute), ErfasstVon: "Pfoertner"},
	}
	for i := range ereignisse {
		if err := store.Append(ctx, &ereignisse[i]); err != nil {
			t.Fatalf("Append(%s): %v", ereignisse[i].Typ, err)
		}
		if ereignisse[i].ID == 0 {
			t.Errorf("Append(%s) sollte eine ID setzen", ereignisse[i].Typ)
		}
	}

	geladen, err := store.ListByBuchung(ctx, buchung.ID)
	if err != nil {
		t.Fatalf("ListByBuchung: %v", err)
	}
	if len(geladen) != 3 {
		t.Fatalf("geladen hat %d Eintraege, want 3", len(geladen))
	}
	// chronologisch: ANGEKOMMEN, ANGEDOCKT, ABGEFAHREN
	wantReihenfolge := []zeitfenster.EreignisTyp{zeitfenster.Angekommen, zeitfenster.Angedockt, zeitfenster.Abgefahren}
	for i, want := range wantReihenfolge {
		if geladen[i].Typ != want {
			t.Errorf("geladen[%d].Typ = %s, want %s", i, geladen[i].Typ, want)
		}
	}

	status := zeitfenster.BerechneStatus(geladen, buchung.StorniertAm, buchung.Beginn, buchung.Beginn.Add(time.Hour))
	if status != zeitfenster.StatusAbgefahren {
		t.Errorf("BerechneStatus = %s, want ABGEFAHREN", status)
	}
}

func TestEreignisStore_Append_DuplikatRaceGuard(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition A", "GT-A 1")
	if err := NewBuchungStore(db).Create(ctx, &buchung); err != nil {
		t.Fatalf("BuchungStore.Create: %v", err)
	}

	store := NewEreignisStore(db)
	erstes := zeitfenster.BuchungEreignis{BuchungID: buchung.ID, Typ: zeitfenster.Angekommen, Zeitpunkt: buchung.Beginn}
	if err := store.Append(ctx, &erstes); err != nil {
		t.Fatalf("erstes Append: %v", err)
	}

	zweites := zeitfenster.BuchungEreignis{BuchungID: buchung.ID, Typ: zeitfenster.Angekommen, Zeitpunkt: buchung.Beginn.Add(time.Minute)}
	err := store.Append(ctx, &zweites)
	if !errors.Is(err, zeitfenster.ErrUngueltigerStatuswechsel) {
		t.Fatalf("zweites Append: got %v, want ErrUngueltigerStatuswechsel", err)
	}
}

// TestEreignisStore_Append_KonkurrenzKonflikt ist das Ereignis-Pendant zu
// TestBuchungStore_Create_KonkurrenzKonflikt: mehrere Goroutinen erfassen
// gleichzeitig dasselbe Ereignis fuer dieselbe Buchung. Der
// UNIQUE(buchung_id, typ)-Constraint muss race-frei genau einen Gewinner
// zulassen. Mit -race laufen lassen.
func TestEreignisStore_Append_KonkurrenzKonflikt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition A", "GT-A 1")
	if err := NewBuchungStore(db).Create(ctx, &buchung); err != nil {
		t.Fatalf("BuchungStore.Create: %v", err)
	}

	store := NewEreignisStore(db)
	const n = 10
	var start sync.WaitGroup
	start.Add(1)
	var alle sync.WaitGroup
	fehler := make([]error, n)

	for i := 0; i < n; i++ {
		alle.Add(1)
		go func(i int) {
			defer alle.Done()
			start.Wait()
			e := zeitfenster.BuchungEreignis{
				BuchungID: buchung.ID,
				Typ:       zeitfenster.Angekommen,
				Zeitpunkt: buchung.Beginn,
			}
			fehler[i] = store.Append(ctx, &e)
		}(i)
	}
	start.Done()
	alle.Wait()

	var erfolge, konflikte int
	for _, err := range fehler {
		switch {
		case err == nil:
			erfolge++
		case errors.Is(err, zeitfenster.ErrUngueltigerStatuswechsel):
			konflikte++
		default:
			t.Errorf("unerwarteter Fehler: %v", err)
		}
	}

	if erfolge != 1 {
		t.Errorf("erfolge = %d, want 1", erfolge)
	}
	if konflikte != n-1 {
		t.Errorf("konflikte = %d, want %d", konflikte, n-1)
	}
}
