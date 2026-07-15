package httpapi_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestErstelleBuchung_ErfolgUndSlotKonflikt(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL, "TK", "FRISCH")

	beginn := naechsterBuchungsbeginn()
	ende := beginn.Add(time.Hour)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/v1/buchungen", map[string]any{
		"rampe_id":          rampe.ID,
		"spediteur":         "Spedition Mueller",
		"kennzeichen":       "GT-ML 1234",
		"temperaturbereich": "TK",
		"beginn":            beginn.Format(time.RFC3339),
		"ende":              ende.Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("erste Buchung: status = %d", resp.StatusCode)
	}
	erste := decodeJSON[buchungAntwort](t, resp)
	if erste.ID == 0 {
		t.Error("Antwort sollte eine ID enthalten")
	}
	if erste.Status != "GEBUCHT" {
		t.Errorf("Status = %q, want GEBUCHT", erste.Status)
	}

	// Ueberlappende Buchung auf derselben Rampe -> 409 RAMPE_BELEGT.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/v1/buchungen", map[string]any{
		"rampe_id":          rampe.ID,
		"spediteur":         "Spedition Schmidt",
		"kennzeichen":       "GT-SM 5678",
		"temperaturbereich": "TK",
		"beginn":            beginn.Add(30 * time.Minute).Format(time.RFC3339),
		"ende":              ende.Add(30 * time.Minute).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("ueberlappende Buchung: status = %d, want 409", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "RAMPE_BELEGT" {
		t.Errorf("code = %q, want RAMPE_BELEGT", fehler.Code)
	}
}

func TestErstelleBuchung_TemperaturMismatch(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL, "FRISCH") // kein TK

	beginn := naechsterBuchungsbeginn()
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/v1/buchungen", map[string]any{
		"rampe_id":          rampe.ID,
		"spediteur":         "Spedition Mueller",
		"kennzeichen":       "GT-ML 1234",
		"temperaturbereich": "TK",
		"beginn":            beginn.Format(time.RFC3339),
		"ende":              beginn.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "TEMPERATUR_MISMATCH" {
		t.Errorf("code = %q, want TEMPERATUR_MISMATCH", fehler.Code)
	}
}

func TestErstelleBuchung_ValidierungsFehler(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/v1/buchungen", map[string]any{
		"rampe_id":          rampe.ID,
		"spediteur":         "", // fehlt
		"kennzeichen":       "GT-ML 1234",
		"temperaturbereich": "TK",
		"beginn":            naechsterBuchungsbeginn().Format(time.RFC3339),
		"ende":              naechsterBuchungsbeginn().Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "VALIDIERUNG" {
		t.Errorf("code = %q, want VALIDIERUNG", fehler.Code)
	}
}

func TestGetBuchung_MitJournal(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL, "TK")
	buchung := erstelleTestBuchung(t, srv.URL, rampe.ID, "TK")

	resp := doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/buchungen/%d", srv.URL, buchung.ID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	detail := decodeJSON[buchungDetailAntwort](t, resp)
	if detail.ID != buchung.ID {
		t.Errorf("ID = %d, want %d", detail.ID, buchung.ID)
	}
	if len(detail.Ereignisse) != 0 {
		t.Errorf("frische Buchung sollte kein Journal haben, hat %d Eintraege", len(detail.Ereignisse))
	}
}

func TestGetBuchung_NichtGefunden(t *testing.T) {
	srv := newTestServer(t)
	resp := doJSON(t, http.MethodGet, srv.URL+"/api/v1/buchungen/999", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "BUCHUNG_NICHT_GEFUNDEN" {
		t.Errorf("code = %q, want BUCHUNG_NICHT_GEFUNDEN", fehler.Code)
	}
}

// TestStorniereBuchung belegt MVP.md §5 ueber die volle Kette: Storno gibt
// den Slot frei, danach ist er wieder buchbar.
func TestStorniereBuchung(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL, "TK")
	buchung := erstelleTestBuchung(t, srv.URL, rampe.ID, "TK")

	resp := doJSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/buchungen/%d", srv.URL, buchung.ID), nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("Cancel: status = %d, want 204", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/buchungen/%d", srv.URL, buchung.ID), nil)
	detail := decodeJSON[buchungDetailAntwort](t, resp)
	if detail.Status != "STORNIERT" {
		t.Errorf("Status = %q, want STORNIERT", detail.Status)
	}

	// Slot ist frei: dieselbe Zeit erneut buchen sollte gelingen.
	beginn := naechsterBuchungsbeginn()
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/v1/buchungen", map[string]any{
		"rampe_id": rampe.ID, "spediteur": "Spedition Neu", "kennzeichen": "GT-NEU 1",
		"temperaturbereich": "TK",
		"beginn":            beginn.Format(time.RFC3339),
		"ende":              beginn.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("erneutes Buchen nach Storno: status = %d, want 201", resp.StatusCode)
	}
}

// TestErfasseEreignis_MaschineDerZustaende deckt die Torprozess-Kette ab:
// falscher Uebergang wird abgelehnt, die richtige Reihenfolge gelingt, und
// der Status im Detail-Endpunkt aktualisiert sich entsprechend.
func TestErfasseEreignis_MaschineDerZustaende(t *testing.T) {
	srv := newTestServer(t)
	_, rampe := seedStandortUndRampe(t, srv.URL, "TK")
	buchung := erstelleTestBuchung(t, srv.URL, rampe.ID, "TK")
	ereignisURL := fmt.Sprintf("%s/api/v1/buchungen/%d/ereignisse", srv.URL, buchung.ID)

	// ANGEDOCKT ohne vorheriges ANGEKOMMEN -> 409.
	resp := doJSON(t, http.MethodPost, ereignisURL, map[string]any{"typ": "ANGEDOCKT"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("ANGEDOCKT zuerst: status = %d, want 409", resp.StatusCode)
	}
	fehler := decodeJSON[apiFehlerAntwort](t, resp)
	if fehler.Code != "UNGUELTIGER_STATUSWECHSEL" {
		t.Errorf("code = %q, want UNGUELTIGER_STATUSWECHSEL", fehler.Code)
	}

	// Richtige Reihenfolge: ANGEKOMMEN -> ANGEDOCKT -> ABGEFAHREN.
	for _, typ := range []string{"ANGEKOMMEN", "ANGEDOCKT", "ABGEFAHREN"} {
		resp := doJSON(t, http.MethodPost, ereignisURL, map[string]any{"typ": typ, "erfasst_von": "Pfoertner"})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s: status = %d, want 201", typ, resp.StatusCode)
		}
	}

	// ANGEKOMMEN erneut (jetzt ein Duplikat) -> 409.
	resp = doJSON(t, http.MethodPost, ereignisURL, map[string]any{"typ": "ANGEKOMMEN"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("Duplikat ANGEKOMMEN: status = %d, want 409", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, fmt.Sprintf("%s/api/v1/buchungen/%d", srv.URL, buchung.ID), nil)
	detail := decodeJSON[buchungDetailAntwort](t, resp)
	if detail.Status != "ABGEFAHREN" {
		t.Errorf("Status = %q, want ABGEFAHREN", detail.Status)
	}
	if len(detail.Ereignisse) != 3 {
		t.Fatalf("Journal hat %d Eintraege, want 3", len(detail.Ereignisse))
	}
	if detail.Ereignisse[0].Typ != "ANGEKOMMEN" || detail.Ereignisse[2].Typ != "ABGEFAHREN" {
		t.Errorf("Journal nicht chronologisch: %+v", detail.Ereignisse)
	}
}

func TestListeBuchungen_TagUndMeineBuchungen(t *testing.T) {
	srv := newTestServer(t)
	standort, rampe := seedStandortUndRampe(t, srv.URL, "TK")
	buchung := erstelleTestBuchung(t, srv.URL, rampe.ID, "TK")

	datum := datumFuer(naechsterBuchungsbeginn())
	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/v1/buchungen?standort_id=%d&datum=%s", srv.URL, standort.ID, datum), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	tagesplan := decodeJSON[[]buchungAntwort](t, resp)
	if len(tagesplan) != 1 || tagesplan[0].ID != buchung.ID {
		t.Fatalf("Tagesplan = %+v, want genau eine Buchung mit ID %d", tagesplan, buchung.ID)
	}

	ab := naechsterBuchungsbeginn().Add(-24 * time.Hour).Format(time.RFC3339)
	resp = doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/v1/buchungen?spediteur=%s&ab=%s", srv.URL, "SpeditionTest", url.QueryEscape(ab)), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("meine Buchungen: status = %d", resp.StatusCode)
	}
	meine := decodeJSON[[]buchungAntwort](t, resp)
	if len(meine) != 1 || meine[0].ID != buchung.ID {
		t.Fatalf("meine Buchungen = %+v, want genau eine Buchung mit ID %d", meine, buchung.ID)
	}
}

func TestSucheVerfuegbarkeit(t *testing.T) {
	srv := newTestServer(t)
	standort, rampe := seedStandortUndRampe(t, srv.URL, "TK")

	datum := datumFuer(naechsterBuchungsbeginn())
	url := fmt.Sprintf("%s/api/v1/verfuegbarkeit?standort_id=%d&datum=%s&temperaturbereich=TK&dauer=60",
		srv.URL, standort.ID, datum)

	resp := doJSON(t, http.MethodGet, url, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	vorher := decodeJSON[[]fensterKandidatAntwort](t, resp)
	if len(vorher) == 0 {
		t.Fatal("erwartete freie Fenster an einem leeren Tag")
	}
	fuerRampe := 0
	for _, k := range vorher {
		if k.RampeID == rampe.ID {
			fuerRampe++
		}
	}
	if fuerRampe == 0 {
		t.Fatal("keiner der Kandidaten gehoert zur gesuchten Rampe")
	}

	// Buche eines der gefundenen Fenster. Bei dauer=60min (2 Slots) blockiert
	// das i.d.R. ZWEI benachbarte 30-Minuten-gestufte Kandidaten (den
	// gebuchten selbst und den 30 Minuten spaeteren, der den zweiten
	// belegten Slot mitnutzt) — keine feste Differenz erwarten, sondern die
	// eigentliche Invariante pruefen: der gebuchte Kandidat ist weg, und die
	// Gesamtzahl ist echt kleiner geworden.
	erste := vorher[0]
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/v1/buchungen", map[string]any{
		"rampe_id": erste.RampeID, "spediteur": "Spedition Test", "kennzeichen": "GT-TS 1",
		"temperaturbereich": "TK", "beginn": erste.Beginn, "ende": erste.Ende,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Buchung des gefundenen Fensters: status = %d", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, url, nil)
	nachher := decodeJSON[[]fensterKandidatAntwort](t, resp)
	if len(nachher) >= len(vorher) {
		t.Errorf("nach Buchung: %d Kandidaten, want weniger als vorher (%d)", len(nachher), len(vorher))
	}
	for _, k := range nachher {
		if k.RampeID == erste.RampeID && k.Beginn == erste.Beginn {
			t.Errorf("gebuchtes Fenster %+v erscheint noch in der Suche", erste)
		}
	}
}

func TestGetKennzahlen(t *testing.T) {
	srv := newTestServer(t)
	standort, rampe := seedStandortUndRampe(t, srv.URL, "TK")
	buchung := erstelleTestBuchung(t, srv.URL, rampe.ID, "TK")
	ereignisURL := fmt.Sprintf("%s/api/v1/buchungen/%d/ereignisse", srv.URL, buchung.ID)

	for _, typ := range []string{"ANGEKOMMEN", "ANGEDOCKT", "ABGEFAHREN"} {
		resp := doJSON(t, http.MethodPost, ereignisURL, map[string]any{"typ": typ})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("%s: status = %d", typ, resp.StatusCode)
		}
	}

	datum := datumFuer(naechsterBuchungsbeginn())
	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/v1/kennzahlen?standort_id=%d&datum=%s", srv.URL, standort.ID, datum), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	kennzahlen := decodeJSON[kennzahlenAntwort](t, resp)

	if kennzahlen.BuchungenGesamt != 1 {
		t.Errorf("buchungen_gesamt = %d, want 1", kennzahlen.BuchungenGesamt)
	}
	if kennzahlen.StandzeitSchnittMin == nil {
		t.Fatal("standzeit_schnitt_min sollte gesetzt sein — ANGEKOMMEN und ABGEFAHREN sind erfasst")
	}
	if kennzahlen.PuenktlichkeitProzent == nil {
		t.Fatal("puenktlichkeit_prozent sollte gesetzt sein")
	}
	if kennzahlen.NoShows != 0 {
		t.Errorf("no_shows = %d, want 0", kennzahlen.NoShows)
	}
}

func TestGetKennzahlen_LeererTagLiefertNull(t *testing.T) {
	srv := newTestServer(t)
	standort, _ := seedStandortUndRampe(t, srv.URL, "TK")

	datum := datumFuer(naechsterBuchungsbeginn())
	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/v1/kennzahlen?standort_id=%d&datum=%s", srv.URL, standort.ID, datum), nil)
	kennzahlen := decodeJSON[kennzahlenAntwort](t, resp)

	// "noch nichts zu berechnen" muss null sein, nicht 0 — CONVENTIONS.md §10.
	if kennzahlen.StandzeitSchnittMin != nil {
		t.Errorf("standzeit_schnitt_min = %v, want nil an einem leeren Tag", *kennzahlen.StandzeitSchnittMin)
	}
	if kennzahlen.PuenktlichkeitProzent != nil {
		t.Errorf("puenktlichkeit_prozent = %v, want nil an einem leeren Tag", *kennzahlen.PuenktlichkeitProzent)
	}
}

// erstelleTestBuchung ist der gemeinsame Weg, ueber die echte API eine
// gueltige Buchung fuer "morgen 09:00-10:00 UTC" anzulegen.
func erstelleTestBuchung(t *testing.T, baseURL string, rampeID int64, temperaturbereich string) buchungAntwort {
	t.Helper()
	beginn := naechsterBuchungsbeginn()
	resp := doJSON(t, http.MethodPost, baseURL+"/api/v1/buchungen", map[string]any{
		"rampe_id":          rampeID,
		"spediteur":         "SpeditionTest",
		"kennzeichen":       "GT-TS 1",
		"temperaturbereich": temperaturbereich,
		"beginn":            beginn.Format(time.RFC3339),
		"ende":              beginn.Add(time.Hour).Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("erstelleTestBuchung: status = %d", resp.StatusCode)
	}
	return decodeJSON[buchungAntwort](t, resp)
}
