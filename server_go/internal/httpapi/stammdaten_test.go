package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestStandort_CreateUndListe(t *testing.T) {
	srv := newTestServer(t)
	standort, _ := seedStandortUndRampe(t, srv.URL)

	resp := doJSON(t, http.MethodGet, srv.URL+"/api/v1/standorte", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	liste := decodeJSON[[]standortAntwort](t, resp)
	if len(liste) != 1 || liste[0].ID != standort.ID {
		t.Fatalf("liste = %+v, want genau den angelegten Standort", liste)
	}
	if liste[0].OeffnungVon != "05:00" || liste[0].OeffnungBis != "22:00" {
		t.Errorf("Oeffnungszeiten = %s-%s, want 05:00-22:00", liste[0].OeffnungVon, liste[0].OeffnungBis)
	}
}

func TestStandort_Create_UngueltigeZeitzone(t *testing.T) {
	srv := newTestServer(t)
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/v1/standorte", map[string]any{
		"name": "Nirgendwo", "zeitzone": "Nirgendwo/Erfunden",
		"oeffnung_von": "05:00", "oeffnung_bis": "22:00",
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "UNGUELTIGE_ZEITZONE" {
		t.Errorf("code = %q, want UNGUELTIGE_ZEITZONE", fehler.Code)
	}
}

// TestStandort_PatchTeilweise belegt, dass PATCH wirklich nur die
// mitgeschickten Felder aendert (CONVENTIONS.md §10 Optionalitaet ueber
// *string) — der wichtigste Test in dieser Datei.
func TestStandort_PatchTeilweise(t *testing.T) {
	srv := newTestServer(t)
	standort, _ := seedStandortUndRampe(t, srv.URL)

	resp := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/standorte/%d", srv.URL, standort.ID),
		map[string]any{"name": "Versmold Nord"}, // NUR name, alles andere fehlt im Body
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	geaendert := decodeJSON[standortAntwort](t, resp)

	if geaendert.Name != "Versmold Nord" {
		t.Errorf("Name = %q, want Versmold Nord", geaendert.Name)
	}
	// Unveraendert gebliebene Felder muessen exakt erhalten bleiben.
	if geaendert.Adresse != standort.Adresse {
		t.Errorf("Adresse = %q, want unveraendert %q", geaendert.Adresse, standort.Adresse)
	}
	if geaendert.Zeitzone != standort.Zeitzone {
		t.Errorf("Zeitzone = %q, want unveraendert %q", geaendert.Zeitzone, standort.Zeitzone)
	}
	if geaendert.OeffnungVon != standort.OeffnungVon || geaendert.OeffnungBis != standort.OeffnungBis {
		t.Errorf("Oeffnungszeiten = %s-%s, want unveraendert %s-%s",
			geaendert.OeffnungVon, geaendert.OeffnungBis, standort.OeffnungVon, standort.OeffnungBis)
	}
	if !geaendert.Aktiv {
		t.Error("Aktiv sollte unveraendert true bleiben")
	}
}

// TestStandort_PatchRegel17 belegt: ein Standort mit aktiven Rampen laesst
// sich nicht deaktivieren, bis die Rampe selbst deaktiviert ist.
func TestStandort_PatchRegel17(t *testing.T) {
	srv := newTestServer(t)
	standort, rampe := seedStandortUndRampe(t, srv.URL)

	resp := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/standorte/%d", srv.URL, standort.ID),
		map[string]any{"aktiv": false},
	)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "STANDORT_HAT_RAMPEN" {
		t.Errorf("code = %q, want STANDORT_HAT_RAMPEN", fehler.Code)
	}

	doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/rampen/%d", srv.URL, rampe.ID), map[string]any{"aktiv": false})

	resp = doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/standorte/%d", srv.URL, standort.ID),
		map[string]any{"aktiv": false},
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("nach Rampe-Deaktivierung: status = %d, want 200", resp.StatusCode)
	}
}

func TestStandort_Patch_NichtGefunden(t *testing.T) {
	srv := newTestServer(t)
	resp := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/standorte/999", map[string]any{"name": "X"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRampe_CreateUndListe(t *testing.T) {
	srv := newTestServer(t)
	standort, rampe := seedStandortUndRampe(t, srv.URL, "TK", "FRISCH")

	resp := doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/standorte/%d/rampen", srv.URL, standort.ID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	liste := decodeJSON[[]rampeAntwort](t, resp)
	if len(liste) != 1 || liste[0].ID != rampe.ID {
		t.Fatalf("liste = %+v, want genau die angelegte Rampe", liste)
	}
	if len(liste[0].Temperaturbereiche) != 2 {
		t.Errorf("Temperaturbereiche = %v, want [FRISCH TK]", liste[0].Temperaturbereiche)
	}
}

func TestRampe_Create_BezeichnungBelegt(t *testing.T) {
	srv := newTestServer(t)
	standort, _ := seedStandortUndRampe(t, srv.URL) // legt bereits "Tor 12" an

	resp := doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/standorte/%d/rampen", srv.URL, standort.ID),
		map[string]any{"bezeichnung": "Tor 12", "temperaturbereiche": []string{"TROCKEN"}},
	)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "BEZEICHNUNG_BELEGT" {
		t.Errorf("code = %q, want BEZEICHNUNG_BELEGT", fehler.Code)
	}
}

func TestRampe_Create_KeinTemperaturbereich(t *testing.T) {
	srv := newTestServer(t)
	standort, _ := seedStandortUndRampe(t, srv.URL)

	resp := doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/standorte/%d/rampen", srv.URL, standort.ID),
		map[string]any{"bezeichnung": "Tor 13", "temperaturbereiche": []string{}},
	)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (Transport-Validierung greift vor dem Domain-Konstruktor)", resp.StatusCode)
	}
}

// TestRampe_PatchTeilweise: nur bezeichnung aendern darf Temperaturbereiche
// und aktiv nicht anfassen.
func TestRampe_PatchTeilweise(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL, "TK", "FRISCH")

	resp := doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/rampen/%d", srv.URL, rampe.ID),
		map[string]any{"bezeichnung": "Tor 99"},
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	geaendert := decodeJSON[rampeAntwort](t, resp)

	if geaendert.Bezeichnung != "Tor 99" {
		t.Errorf("Bezeichnung = %q, want Tor 99", geaendert.Bezeichnung)
	}
	if len(geaendert.Temperaturbereiche) != 2 {
		t.Errorf("Temperaturbereiche = %v, sollten unveraendert [FRISCH TK] bleiben", geaendert.Temperaturbereiche)
	}
	if !geaendert.Aktiv {
		t.Error("Aktiv sollte unveraendert true bleiben")
	}
}

// TestRampe_PatchRegel16 belegt: ein Temperaturbereich, den eine zukuenftige
// Buchung noch braucht, laesst sich nicht entfernen.
func TestRampe_PatchRegel16(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL, "TK", "FRISCH")

	beginn := naechsterBuchungsbeginn()
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/v1/buchungen", map[string]any{
		"rampe_id": rampe.ID, "spediteur": "Spedition Mueller", "kennzeichen": "GT-ML 1",
		"temperaturbereich": "TK",
		"beginn":            beginn.Format(time.RFC3339),
		"ende":              beginn.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Buchung anlegen: status = %d", resp.StatusCode)
	}

	// TK entfernen (wird gebraucht) -> 409.
	resp = doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/rampen/%d", srv.URL, rampe.ID),
		map[string]any{"temperaturbereiche": []string{"FRISCH"}},
	)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("TK entfernen: status = %d, want 409", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "RAMPE_HAT_BUCHUNGEN" {
		t.Errorf("code = %q, want RAMPE_HAT_BUCHUNGEN", fehler.Code)
	}

	// FRISCH entfernen (ungenutzt) -> erlaubt.
	resp = doJSON(t, http.MethodPatch, fmt.Sprintf("%s/api/v1/rampen/%d", srv.URL, rampe.ID),
		map[string]any{"temperaturbereiche": []string{"TK"}},
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("FRISCH entfernen: status = %d, want 200", resp.StatusCode)
	}
}

func TestRampe_Patch_NichtGefunden(t *testing.T) {
	srv := newTestServer(t)
	resp := doJSON(t, http.MethodPatch, srv.URL+"/api/v1/rampen/999", map[string]any{"bezeichnung": "X"})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
