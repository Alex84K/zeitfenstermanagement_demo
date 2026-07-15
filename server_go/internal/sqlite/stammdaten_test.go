package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"zeitfenster/internal/zeitfenster"
)

func TestStandortStore_CreateGetList(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	store := NewStandortStore(db)

	standort, err := zeitfenster.NewStandort(0, "Versmold", "Industriestrasse 1", "Europe/Berlin", 5*time.Hour, 22*time.Hour, true)
	if err != nil {
		t.Fatalf("NewStandort: %v", err)
	}
	if err := store.Create(ctx, &standort); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if standort.ID == 0 {
		t.Fatal("Create sollte eine ID setzen")
	}

	geladen, err := store.Get(ctx, standort.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if geladen.Name != "Versmold" || geladen.Adresse != "Industriestrasse 1" {
		t.Errorf("geladen = %+v, name/adresse stimmen nicht", geladen)
	}
	if geladen.Zeitzone.String() != "Europe/Berlin" {
		t.Errorf("Zeitzone = %q, want Europe/Berlin", geladen.Zeitzone.String())
	}
	if geladen.OeffnungVon != 5*time.Hour || geladen.OeffnungBis != 22*time.Hour {
		t.Errorf("Oeffnungszeiten = %v-%v, want 5h-22h", geladen.OeffnungVon, geladen.OeffnungBis)
	}
	if !geladen.Aktiv {
		t.Error("sollte aktiv sein")
	}

	liste, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(liste) != 1 {
		t.Fatalf("List() hat %d Eintraege, want 1", len(liste))
	}
}

func TestStandortStore_Get_NotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := NewStandortStore(db).Get(context.Background(), 999)
	if !errors.Is(err, zeitfenster.ErrStandortNotFound) {
		t.Fatalf("got %v, want ErrStandortNotFound", err)
	}
}

// TestStandortStore_Update_Regel17 belegt, dass ein Standort mit aktiven
// Rampen nicht deaktiviert werden kann — erst muss die Rampe weg.
func TestStandortStore_Update_Regel17(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	standort, rampe := seedStandortUndRampe(t, db)

	standort.Aktiv = false
	err := NewStandortStore(db).Update(ctx, standort)
	if !errors.Is(err, zeitfenster.ErrStandortHatRampen) {
		t.Fatalf("got %v, want ErrStandortHatRampen", err)
	}

	// Rampe zuerst deaktivieren, dann sollte der Standort deaktivierbar sein.
	rampe.Aktiv = false
	if err := NewRampeStore(db).Update(ctx, rampe, time.Now()); err != nil {
		t.Fatalf("RampeStore.Update: %v", err)
	}
	if err := NewStandortStore(db).Update(ctx, standort); err != nil {
		t.Fatalf("StandortStore.Update nach Rampe-Deaktivierung: %v", err)
	}

	geladen, err := NewStandortStore(db).Get(ctx, standort.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if geladen.Aktiv {
		t.Error("Standort sollte jetzt inaktiv sein")
	}
}

func TestRampeStore_CreateGetListByStandort(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	standort, rampe := seedStandortUndRampe(t, db, zeitfenster.TK, zeitfenster.Frisch)

	geladen, err := NewRampeStore(db).Get(ctx, rampe.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if geladen.Bezeichnung != "Tor 12" {
		t.Errorf("Bezeichnung = %q, want Tor 12", geladen.Bezeichnung)
	}
	if !geladen.UnterstuetztTemperaturbereich(zeitfenster.TK) || !geladen.UnterstuetztTemperaturbereich(zeitfenster.Frisch) {
		t.Errorf("Temperaturbereiche = %v, want TK+FRISCH", geladen.Temperaturbereiche)
	}
	if geladen.UnterstuetztTemperaturbereich(zeitfenster.Trocken) {
		t.Error("sollte TROCKEN nicht unterstuetzen")
	}

	liste, err := NewRampeStore(db).ListByStandort(ctx, standort.ID)
	if err != nil {
		t.Fatalf("ListByStandort: %v", err)
	}
	if len(liste) != 1 {
		t.Fatalf("ListByStandort() hat %d Eintraege, want 1", len(liste))
	}
}

func TestRampeStore_Get_NotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := NewRampeStore(db).Get(context.Background(), 999)
	if !errors.Is(err, zeitfenster.ErrRampeNotFound) {
		t.Fatalf("got %v, want ErrRampeNotFound", err)
	}
}

// TestRampeStore_Create_Regel18 belegt die UNIQUE(standort_id, bezeichnung) —
// zwei Rampen mit demselben Namen an derselben Flaeche sind nicht erlaubt.
func TestRampeStore_Create_Regel18(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	standort, _ := seedStandortUndRampe(t, db) // legt bereits "Tor 12" an

	duplikat, err := zeitfenster.NewRampe(0, standort.ID, "Tor 12", zeitfenster.NewTemperaturbereichSet(zeitfenster.Trocken), true)
	if err != nil {
		t.Fatalf("NewRampe: %v", err)
	}
	err = NewRampeStore(db).Create(ctx, &duplikat)
	if !errors.Is(err, zeitfenster.ErrBezeichnungBelegt) {
		t.Fatalf("got %v, want ErrBezeichnungBelegt", err)
	}
}

// TestRampeStore_Update_Regel15 belegt: eine Rampe mit einer zukuenftigen
// aktiven Buchung kann nicht deaktiviert werden.
func TestRampeStore_Update_Regel15(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK)

	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition Mueller", "GT-ML 1")
	if err := NewBuchungStore(db).Create(ctx, &buchung); err != nil {
		t.Fatalf("BuchungStore.Create: %v", err)
	}

	jetzt := buchung.Beginn.Add(-2 * time.Hour) // vor der Buchung — sie liegt in der Zukunft
	rampe.Aktiv = false
	err := NewRampeStore(db).Update(ctx, rampe, jetzt)
	if !errors.Is(err, zeitfenster.ErrRampeHatBuchungen) {
		t.Fatalf("got %v, want ErrRampeHatBuchungen", err)
	}
}

// TestRampeStore_Update_Regel16 belegt: ein Temperaturbereich, der noch von
// einer zukuenftigen Buchung gebraucht wird, kann nicht entfernt werden — ein
// ungenutzter Bereich dagegen schon.
func TestRampeStore_Update_Regel16(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_, rampe := seedStandortUndRampe(t, db, zeitfenster.TK, zeitfenster.Frisch)

	buchung := gueltigeBuchung(rampe.ID, zeitfenster.TK, "Spedition Mueller", "GT-ML 1")
	if err := NewBuchungStore(db).Create(ctx, &buchung); err != nil {
		t.Fatalf("BuchungStore.Create: %v", err)
	}
	jetzt := buchung.Beginn.Add(-2 * time.Hour)

	// TK entfernen: verletzt Regel 16, da die Buchung TK braucht.
	rampeOhneTK := rampe
	rampeOhneTK.Temperaturbereiche = zeitfenster.NewTemperaturbereichSet(zeitfenster.Frisch)
	err := NewRampeStore(db).Update(ctx, rampeOhneTK, jetzt)
	if !errors.Is(err, zeitfenster.ErrRampeHatBuchungen) {
		t.Fatalf("TK entfernen: got %v, want ErrRampeHatBuchungen", err)
	}

	// FRISCH entfernen (ungenutzt): sollte erlaubt sein.
	rampeOhneFrisch := rampe
	rampeOhneFrisch.Temperaturbereiche = zeitfenster.NewTemperaturbereichSet(zeitfenster.TK)
	if err := NewRampeStore(db).Update(ctx, rampeOhneFrisch, jetzt); err != nil {
		t.Fatalf("FRISCH entfernen sollte erlaubt sein: %v", err)
	}

	geladen, err := NewRampeStore(db).Get(ctx, rampe.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if geladen.UnterstuetztTemperaturbereich(zeitfenster.Frisch) {
		t.Error("FRISCH sollte entfernt sein")
	}
	if !geladen.UnterstuetztTemperaturbereich(zeitfenster.TK) {
		t.Error("TK sollte erhalten geblieben sein")
	}
}
