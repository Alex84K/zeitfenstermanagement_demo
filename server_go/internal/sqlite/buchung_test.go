package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"zeitfenster/internal/zeitfenster"
)

func TestBuchungStore_CreateGet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)

	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition Mueller", "GT-ML 1234")
	if err := NewBuchungStore(db).Create(ctx, &buchung); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if buchung.ID == 0 {
		t.Fatal("Create sollte eine ID setzen")
	}

	geladen, err := NewBuchungStore(db).Get(ctx, buchung.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if geladen.Spediteur != "Spedition Mueller" || geladen.Kennzeichen != "GT-ML 1234" {
		t.Errorf("geladen = %+v", geladen)
	}
	if !geladen.Beginn.Equal(buchung.Beginn) || !geladen.Ende.Equal(buchung.Ende) {
		t.Errorf("Zeiten stimmen nicht: got %v-%v, want %v-%v", geladen.Beginn, geladen.Ende, buchung.Beginn, buchung.Ende)
	}
	if geladen.StorniertAm != nil {
		t.Error("frische Buchung sollte nicht storniert sein")
	}
}

func TestBuchungStore_Get_NotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := NewBuchungStore(db).Get(context.Background(), 999)
	if !errors.Is(err, zeitfenster.ErrBuchungNotFound) {
		t.Fatalf("got %v, want ErrBuchungNotFound", err)
	}
}

// TestBuchungStore_Create_SlotKonflikt belegt Regel 1 sequenziell: eine zweite,
// ueberlappende Buchung auf derselben Rampe scheitert; eine nicht
// ueberlappende gelingt.
func TestBuchungStore_Create_SlotKonflikt(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	store := NewBuchungStore(db)

	erste := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition A", "GT-A 1")
	if err := store.Create(ctx, &erste); err != nil {
		t.Fatalf("erste Buchung: %v", err)
	}

	// ueberlappt erste (06:30-07:30) um 30 Minuten.
	zweite := erste
	zweite.ID = 0
	zweite.Spediteur = "Spedition B"
	zweite.Kennzeichen = "GT-B 2"
	zweite.Beginn = erste.Beginn.Add(30 * time.Minute)
	zweite.Ende = erste.Ende.Add(30 * time.Minute)
	err := store.Create(ctx, &zweite)
	if !errors.Is(err, zeitfenster.ErrRampeBelegt) {
		t.Fatalf("ueberlappende Buchung: got %v, want ErrRampeBelegt", err)
	}

	// direkt anschliessend (07:30-08:30) darf nicht kollidieren — 07:30 muss frei bleiben.
	dritte := erste
	dritte.ID = 0
	dritte.Spediteur = "Spedition C"
	dritte.Kennzeichen = "GT-C 3"
	dritte.Beginn = erste.Ende
	dritte.Ende = erste.Ende.Add(time.Hour)
	if err := store.Create(ctx, &dritte); err != nil {
		t.Fatalf("direkt anschliessende Buchung sollte gelingen: %v", err)
	}
}

// TestBuchungStore_Create_KonkurrenzKonflikt ist der wichtigste Test in diesem
// Paket: mehrere Goroutinen buchen gleichzeitig denselben Slot derselben
// Rampe. Genau eine darf gewinnen — der PRIMARY KEY auf buchung_slot erzwingt
// das race-frei, nicht ein vorheriger SELECT (MVP.md §6, CONVENTIONS.md §8).
// Mit -race laufen lassen.
func TestBuchungStore_Create_KonkurrenzKonflikt(t *testing.T) {
	db := newTestDB(t)
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	store := NewBuchungStore(db)

	const n = 10
	var start sync.WaitGroup
	start.Add(1)
	var alle sync.WaitGroup
	fehler := make([]error, n)

	for i := 0; i < n; i++ {
		alle.Add(1)
		go func(i int) {
			defer alle.Done()
			start.Wait() // alle Goroutinen so gleichzeitig wie moeglich starten
			buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK,
				fmt.Sprintf("Spedition %d", i), fmt.Sprintf("GT-ML %04d", i))
			fehler[i] = store.Create(context.Background(), &buchung)
		}(i)
	}
	start.Done()
	alle.Wait()

	var erfolge, konflikte int
	for _, err := range fehler {
		switch {
		case err == nil:
			erfolge++
		case errors.Is(err, zeitfenster.ErrRampeBelegt):
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

func TestBuchungStore_ListByStandortUndZeitraum(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	standort, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	store := NewBuchungStore(db)

	imTag := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition A", "GT-A 1")
	if err := store.Create(ctx, &imTag); err != nil {
		t.Fatalf("Create imTag: %v", err)
	}

	andererTag := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition B", "GT-B 2")
	andererTag.Beginn = andererTag.Beginn.AddDate(0, 0, 1)
	andererTag.Ende = andererTag.Ende.AddDate(0, 0, 1)
	if err := store.Create(ctx, &andererTag); err != nil {
		t.Fatalf("Create andererTag: %v", err)
	}

	von, bis := standort.TagesgrenzenUTC(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC))
	liste, err := store.ListByStandortUndZeitraum(ctx, standort.ID, von, bis)
	if err != nil {
		t.Fatalf("ListByStandortUndZeitraum: %v", err)
	}
	if len(liste) != 1 {
		t.Fatalf("liste hat %d Eintraege, want 1 (nur imTag)", len(liste))
	}
	if liste[0].ID != imTag.ID {
		t.Errorf("liste[0].ID = %d, want %d", liste[0].ID, imTag.ID)
	}
}

func TestBuchungStore_ListBySpediteur(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	store := NewBuchungStore(db)

	meine := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition Mueller", "GT-ML 1")
	if err := store.Create(ctx, &meine); err != nil {
		t.Fatalf("Create meine: %v", err)
	}
	fremde := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition Fremd", "GT-FR 2")
	fremde.Beginn = fremde.Beginn.Add(2 * time.Hour)
	fremde.Ende = fremde.Ende.Add(2 * time.Hour)
	if err := store.Create(ctx, &fremde); err != nil {
		t.Fatalf("Create fremde: %v", err)
	}

	liste, err := store.ListBySpediteur(ctx, "Spedition Mueller", meine.Beginn.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("ListBySpediteur: %v", err)
	}
	if len(liste) != 1 || liste[0].ID != meine.ID {
		t.Fatalf("liste = %+v, want genau meine Buchung", liste)
	}
}

// TestBuchungStore_Cancel belegt MVP.md §5: Stornieren setzt storniert_am UND
// gibt die Slots frei — danach muss derselbe Slot erneut buchbar sein.
func TestBuchungStore_Cancel(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	store := NewBuchungStore(db)

	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition A", "GT-A 1")
	if err := store.Create(ctx, &buchung); err != nil {
		t.Fatalf("Create: %v", err)
	}

	stornoZeit := buchung.Beginn.Add(-30 * time.Minute)
	if err := store.Cancel(ctx, buchung.ID, stornoZeit); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	geladen, err := store.Get(ctx, buchung.ID)
	if err != nil {
		t.Fatalf("Get nach Cancel: %v", err)
	}
	if geladen.StorniertAm == nil {
		t.Fatal("storniert_am sollte gesetzt sein")
	}
	if !geladen.StorniertAm.Equal(stornoZeit) {
		t.Errorf("storniert_am = %v, want %v", geladen.StorniertAm, stornoZeit)
	}

	// Slots wurden freigegeben: derselbe Zeitraum ist jetzt wieder buchbar.
	neu := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition B", "GT-B 2")
	if err := store.Create(ctx, &neu); err != nil {
		t.Fatalf("erneutes Buchen nach Storno sollte gelingen: %v", err)
	}
}

func TestBuchungStore_Cancel_NichtGefunden(t *testing.T) {
	db := newTestDB(t)
	err := NewBuchungStore(db).Cancel(context.Background(), 999, time.Now())
	if !errors.Is(err, zeitfenster.ErrBuchungNotFound) {
		t.Fatalf("got %v, want ErrBuchungNotFound", err)
	}
}

func TestBuchungStore_Cancel_Zweimal(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)
	store := NewBuchungStore(db)

	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition A", "GT-A 1")
	if err := store.Create(ctx, &buchung); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.Cancel(ctx, buchung.ID, time.Now()); err != nil {
		t.Fatalf("erstes Cancel: %v", err)
	}

	err := store.Cancel(ctx, buchung.ID, time.Now())
	if !errors.Is(err, zeitfenster.ErrBuchungNotFound) {
		t.Fatalf("zweites Cancel: got %v, want ErrBuchungNotFound (siehe buchung.go)", err)
	}
}
