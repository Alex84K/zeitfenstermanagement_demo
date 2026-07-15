package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"zeitfenster/internal/httpapi"
	"zeitfenster/internal/sqlite"
)

// newTestServer verdrahtet echte SQLite-Stores (temporaere Datei, CONVENTIONS.md
// §13) mit httpapi.NewServer und startet einen echten HTTP-Server. Das ist
// der Moment, in dem der Compiler beweist, dass sqlite.BuchungStore & Co.
// die von httpapi deklarierten Schnittstellen tatsaechlich erfuellen
// (CONVENTIONS.md §7) — kein Mock, echte Verdrahtung wie in cmd/zeitfenster.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	deps := httpapi.Deps{
		Buchungen:  sqlite.NewBuchungStore(db),
		Ereignisse: sqlite.NewEreignisStore(db),
		Standorte:  sqlite.NewStandortStore(db),
		Rampen:     sqlite.NewRampeStore(db),
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv := httptest.NewServer(httpapi.NewServer(deps))
	t.Cleanup(srv.Close)
	return srv
}

// doJSON schickt eine Anfrage mit optionalem JSON-Body und liefert die rohe
// Antwort — die Tests entscheiden selbst, in welche Struktur sie dekodieren.
func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

// decodeJSON dekodiert den Antwort-Body. Die Ziel-Struktur gehoert dem Test,
// nicht httpapi — dessen DTOs sind bewusst unexportiert (CONVENTIONS.md §10),
// ein Black-Box-Test kennt nur den Draht, genau wie der echte Vue-Client.
func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return v
}

// --- Antwort-Formen, wie ein Client sie sieht (Spiegel der DTOs in buchung.go/stammdaten.go) ---

type buchungAntwort struct {
	ID                int64  `json:"id"`
	RampeID           int64  `json:"rampe_id"`
	Spediteur         string `json:"spediteur"`
	Kennzeichen       string `json:"kennzeichen"`
	Temperaturbereich string `json:"temperaturbereich"`
	Beginn            string `json:"beginn"`
	Ende              string `json:"ende"`
	Status            string `json:"status"`
}

type buchungDetailAntwort struct {
	buchungAntwort
	Ereignisse []ereignisAntwort `json:"ereignisse"`
}

type ereignisAntwort struct {
	Typ       string `json:"typ"`
	Zeitpunkt string `json:"zeitpunkt"`
}

type fensterKandidatAntwort struct {
	RampeID     int64  `json:"rampe_id"`
	Bezeichnung string `json:"bezeichnung"`
	Beginn      string `json:"beginn"`
	Ende        string `json:"ende"`
}

type kennzahlenAntwort struct {
	BuchungenGesamt       int      `json:"buchungen_gesamt"`
	StandzeitSchnittMin   *float64 `json:"standzeit_schnitt_min"`
	PuenktlichkeitProzent *float64 `json:"puenktlichkeit_prozent"`
	NoShows               int      `json:"no_shows"`
}

type standortAntwort struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Adresse     string `json:"adresse"`
	Zeitzone    string `json:"zeitzone"`
	OeffnungVon string `json:"oeffnung_von"`
	OeffnungBis string `json:"oeffnung_bis"`
	Aktiv       bool   `json:"aktiv"`
}

type rampeAntwort struct {
	ID                 int64    `json:"id"`
	StandortID         int64    `json:"standort_id"`
	Bezeichnung        string   `json:"bezeichnung"`
	Temperaturbereiche []string `json:"temperaturbereiche"`
	Aktiv              bool     `json:"aktiv"`
}

type apiFehlerAntwort struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- Fixtures ---

// seedStandortUndRampe legt einen Standort und eine Rampe ueber die echte API
// an (POST) — das uebt gleichzeitig erstelleStandort/erstelleRampe mit ab,
// statt an ihnen vorbei direkt in die Stores zu schreiben.
func seedStandortUndRampe(t *testing.T, baseURL string, bereiche ...string) (standortAntwort, rampeAntwort) {
	t.Helper()
	if len(bereiche) == 0 {
		bereiche = []string{"TK", "FRISCH"}
	}

	resp := doJSON(t, http.MethodPost, baseURL+"/api/v1/standorte", map[string]any{
		"name": "Versmold", "adresse": "Industriestrasse 1", "zeitzone": "Europe/Berlin",
		"oeffnung_von": "05:00", "oeffnung_bis": "22:00",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create standort: status = %d", resp.StatusCode)
	}
	standort := decodeJSON[standortAntwort](t, resp)

	resp = doJSON(t, http.MethodPost, fmt.Sprintf("%s/api/v1/standorte/%d/rampen", baseURL, standort.ID),
		map[string]any{"bezeichnung": "Tor 12", "temperaturbereiche": bereiche},
	)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create rampe: status = %d", resp.StatusCode)
	}
	rampe := decodeJSON[rampeAntwort](t, resp)

	return standort, rampe
}

// naechsterBuchungsbeginn liefert einen sicher in der Zukunft liegenden,
// rastergerechten Zeitpunkt: morgen 09:00 UTC. "Morgen" relativ zu time.Now()
// statt eines festen Datums — ein hartcodiertes Zukunftsdatum waere
// irgendwann in der Vergangenheit und liesse den Test verrotten. 09:00 UTC
// liegt sowohl im Sommer (11:00 Berlin) als auch im Winter (10:00 Berlin)
// sicher innerhalb 05:00-22:00 lokal.
func naechsterBuchungsbeginn() time.Time {
	morgen := time.Now().UTC().AddDate(0, 0, 1)
	return time.Date(morgen.Year(), morgen.Month(), morgen.Day(), 9, 0, 0, 0, time.UTC)
}

// datumFuer liefert den lokalen Kalendertag (Europe/Berlin) von t im Format
// YYYY-MM-DD, wie ihn der datum-Query-Parameter erwartet (MVP.md §7).
func datumFuer(t time.Time) string {
	berlin, _ := time.LoadLocation("Europe/Berlin")
	return t.In(berlin).Format("2006-01-02")
}
