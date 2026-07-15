package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"zeitfenster/internal/zeitfenster"
)

// StandortStore ist die von diesem Paket benoetigte Persistenz-Schnittstelle
// fuer Standorte.
type StandortStore interface {
	Create(ctx context.Context, s *zeitfenster.Standort) error
	Get(ctx context.Context, id int64) (zeitfenster.Standort, error)
	List(ctx context.Context) ([]zeitfenster.Standort, error)
	Update(ctx context.Context, s zeitfenster.Standort) error
}

// RampeStore ist die von diesem Paket benoetigte Persistenz-Schnittstelle
// fuer Rampen.
type RampeStore interface {
	Create(ctx context.Context, r *zeitfenster.Rampe) error
	Get(ctx context.Context, id int64) (zeitfenster.Rampe, error)
	ListByStandort(ctx context.Context, standortID int64) ([]zeitfenster.Rampe, error)
	Update(ctx context.Context, r zeitfenster.Rampe, jetzt time.Time) error
}

// --- DTOs: Standort ---

type standortResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Adresse     string `json:"adresse"`
	Zeitzone    string `json:"zeitzone"`
	OeffnungVon string `json:"oeffnung_von"`
	OeffnungBis string `json:"oeffnung_bis"`
	Aktiv       bool   `json:"aktiv"`
}

func toStandortResponse(s zeitfenster.Standort) standortResponse {
	return standortResponse{
		ID:          s.ID,
		Name:        s.Name,
		Adresse:     s.Adresse,
		Zeitzone:    s.Zeitzone.String(),
		OeffnungVon: formatUhrzeit(s.OeffnungVon),
		OeffnungBis: formatUhrzeit(s.OeffnungBis),
		Aktiv:       s.Aktiv,
	}
}

type createStandortRequest struct {
	Name        string `json:"name"`
	Adresse     string `json:"adresse"`
	Zeitzone    string `json:"zeitzone"`
	OeffnungVon string `json:"oeffnung_von"`
	OeffnungBis string `json:"oeffnung_bis"`
}

// validate deckt Pflichtfelder und Zeitformate ab — inklusive der Uhrzeiten,
// damit toDomain() sie gefahrlos ohne Fehlerbehandlung erneut parsen kann.
func (r createStandortRequest) validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(r.Zeitzone) == "" {
		return errors.New("zeitzone is required")
	}
	if _, err := parseUhrzeit(r.OeffnungVon); err != nil {
		return fmt.Errorf("oeffnung_von: %w", err)
	}
	if _, err := parseUhrzeit(r.OeffnungBis); err != nil {
		return fmt.Errorf("oeffnung_bis: %w", err)
	}
	return nil
}

// toDomain darf erst nach validate() aufgerufen werden. Anders als bei
// createBuchungRequest ist ein Fehler hier ausschliesslich
// zeitfenster.ErrUngueltigeZeitzone (Regel 19) — ein Domain-Fehler, kein
// Transportfehler, deshalb behandelt der Aufrufer ihn ueber writeError (422)
// statt writeValidierungsFehler (400).
func (r createStandortRequest) toDomain() (zeitfenster.Standort, error) {
	von, _ := parseUhrzeit(r.OeffnungVon) // Format bereits durch validate() geprueft
	bis, _ := parseUhrzeit(r.OeffnungBis)
	return zeitfenster.NewStandort(0, strings.TrimSpace(r.Name), r.Adresse, r.Zeitzone, von, bis, true)
}

// patchStandortRequest: nil bedeutet "nicht mitgeschickt", nicht "leer" —
// CONVENTIONS.md §10. Der Handler laedt den bestehenden Standort und
// ueberschreibt nur die Felder, die der Client tatsaechlich gesendet hat.
type patchStandortRequest struct {
	Name        *string `json:"name"`
	Adresse     *string `json:"adresse"`
	Zeitzone    *string `json:"zeitzone"`
	OeffnungVon *string `json:"oeffnung_von"`
	OeffnungBis *string `json:"oeffnung_bis"`
	Aktiv       *bool   `json:"aktiv"`
}

// --- DTOs: Rampe ---

type rampeResponse struct {
	ID                 int64    `json:"id"`
	StandortID         int64    `json:"standort_id"`
	Bezeichnung        string   `json:"bezeichnung"`
	Temperaturbereiche []string `json:"temperaturbereiche"`
	Aktiv              bool     `json:"aktiv"`
}

func toRampeResponse(r zeitfenster.Rampe) rampeResponse {
	bereiche := make([]string, 0, len(r.Temperaturbereiche))
	for t := range r.Temperaturbereiche {
		bereiche = append(bereiche, string(t))
	}
	sort.Strings(bereiche) // deterministische Reihenfolge statt Go-Map-Zufall
	return rampeResponse{
		ID:                 r.ID,
		StandortID:         r.StandortID,
		Bezeichnung:        r.Bezeichnung,
		Temperaturbereiche: bereiche,
		Aktiv:              r.Aktiv,
	}
}

type createRampeRequest struct {
	Bezeichnung        string   `json:"bezeichnung"`
	Temperaturbereiche []string `json:"temperaturbereiche"`
}

func (r createRampeRequest) validate() error {
	if strings.TrimSpace(r.Bezeichnung) == "" {
		return errors.New("bezeichnung is required")
	}
	if len(r.Temperaturbereiche) == 0 {
		return errors.New("temperaturbereiche must not be empty")
	}
	return nil
}

func parseTemperaturbereiche(werte []string) (zeitfenster.TemperaturbereichSet, error) {
	geparst := make([]zeitfenster.Temperaturbereich, 0, len(werte))
	for _, s := range werte {
		t, err := zeitfenster.ParseTemperaturbereich(s)
		if err != nil {
			return nil, err
		}
		geparst = append(geparst, t)
	}
	return zeitfenster.NewTemperaturbereichSet(geparst...), nil
}

type patchRampeRequest struct {
	Bezeichnung        *string   `json:"bezeichnung"`
	Temperaturbereiche *[]string `json:"temperaturbereiche"`
	Aktiv              *bool     `json:"aktiv"`
}

// --- Handler: Standort ---

// GET /api/v1/standorte
func (h *handler) listeStandorte(w http.ResponseWriter, r *http.Request) {
	standorte, err := h.standorte.List(r.Context())
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	antwort := make([]standortResponse, len(standorte))
	for i, s := range standorte {
		antwort[i] = toStandortResponse(s)
	}
	writeJSON(w, http.StatusOK, antwort)
}

// POST /api/v1/standorte
func (h *handler) erstelleStandort(w http.ResponseWriter, r *http.Request) {
	var req createStandortRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidierungsFehler(w, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := req.validate(); err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	standort, err := req.toDomain()
	if err != nil {
		writeError(w, h.log, err) // ErrUngueltigeZeitzone -> 422
		return
	}
	if err := h.standorte.Create(r.Context(), &standort); err != nil {
		writeError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toStandortResponse(standort))
}

// PATCH /api/v1/standorte/{id} — laedt den bestehenden Standort, ueberschreibt
// nur mitgeschickte Felder, validiert das Ergebnis erneut ueber den
// Domain-Konstruktor (CONVENTIONS.md §10), persistiert den Volltreffer.
// StandortStore.Update prueft Regel 17 (aktive Rampen) vor der Deaktivierung.
func (h *handler) aktualisiereStandort(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	var patch patchStandortRequest
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeValidierungsFehler(w, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	ctx := r.Context()
	bestehend, err := h.standorte.Get(ctx, id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	name := bestehend.Name
	if patch.Name != nil {
		name = *patch.Name
	}
	adresse := bestehend.Adresse
	if patch.Adresse != nil {
		adresse = *patch.Adresse
	}
	zeitzone := bestehend.Zeitzone.String()
	if patch.Zeitzone != nil {
		zeitzone = *patch.Zeitzone
	}
	von := bestehend.OeffnungVon
	if patch.OeffnungVon != nil {
		von, err = parseUhrzeit(*patch.OeffnungVon)
		if err != nil {
			writeValidierungsFehler(w, fmt.Errorf("oeffnung_von: %w", err))
			return
		}
	}
	bis := bestehend.OeffnungBis
	if patch.OeffnungBis != nil {
		bis, err = parseUhrzeit(*patch.OeffnungBis)
		if err != nil {
			writeValidierungsFehler(w, fmt.Errorf("oeffnung_bis: %w", err))
			return
		}
	}
	aktiv := bestehend.Aktiv
	if patch.Aktiv != nil {
		aktiv = *patch.Aktiv
	}

	merged, err := zeitfenster.NewStandort(id, name, adresse, zeitzone, von, bis, aktiv)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	if err := h.standorte.Update(ctx, merged); err != nil {
		writeError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toStandortResponse(merged))
}

// --- Handler: Rampe ---

// GET /api/v1/standorte/{id}/rampen
func (h *handler) listeRampen(w http.ResponseWriter, r *http.Request) {
	standortID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	rampen, err := h.rampen.ListByStandort(r.Context(), standortID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	antwort := make([]rampeResponse, len(rampen))
	for i, rampe := range rampen {
		antwort[i] = toRampeResponse(rampe)
	}
	writeJSON(w, http.StatusOK, antwort)
}

// POST /api/v1/standorte/{id}/rampen
func (h *handler) erstelleRampe(w http.ResponseWriter, r *http.Request) {
	standortID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	var req createRampeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidierungsFehler(w, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := req.validate(); err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	bereiche, err := parseTemperaturbereiche(req.Temperaturbereiche)
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}

	rampe, err := zeitfenster.NewRampe(0, standortID, strings.TrimSpace(req.Bezeichnung), bereiche, true)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	if err := h.rampen.Create(r.Context(), &rampe); err != nil {
		writeError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRampeResponse(rampe))
}

// PATCH /api/v1/rampen/{id} — gleiches Load-Merge-Validate-Muster wie
// aktualisiereStandort. RampeStore.Update prueft Regeln 15/16 (zukuenftige
// Buchungen) vor Deaktivierung bzw. Entfernen eines Temperaturbereichs.
func (h *handler) aktualisiereRampe(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	var patch patchRampeRequest
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeValidierungsFehler(w, fmt.Errorf("invalid JSON: %w", err))
		return
	}

	ctx := r.Context()
	bestehend, err := h.rampen.Get(ctx, id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	bezeichnung := bestehend.Bezeichnung
	if patch.Bezeichnung != nil {
		bezeichnung = *patch.Bezeichnung
	}
	bereiche := bestehend.Temperaturbereiche
	if patch.Temperaturbereiche != nil {
		bereiche, err = parseTemperaturbereiche(*patch.Temperaturbereiche)
		if err != nil {
			writeValidierungsFehler(w, err)
			return
		}
	}
	aktiv := bestehend.Aktiv
	if patch.Aktiv != nil {
		aktiv = *patch.Aktiv
	}

	merged, err := zeitfenster.NewRampe(id, bestehend.StandortID, bezeichnung, bereiche, aktiv)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	if err := h.rampen.Update(ctx, merged, time.Now().UTC()); err != nil {
		writeError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toRampeResponse(merged))
}
