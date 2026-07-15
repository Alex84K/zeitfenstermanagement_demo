package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zeitfenster/internal/zeitfenster"
)

// BuchungStore ist die von diesem Paket benoetigte Persistenz-Schnittstelle
// fuer Buchungen. internal/sqlite.BuchungStore erfuellt sie strukturell —
// CONVENTIONS.md §7: der Konsument (dieses Paket) erklaert die Schnittstelle.
type BuchungStore interface {
	Create(ctx context.Context, b *zeitfenster.Buchung) error
	Get(ctx context.Context, id int64) (zeitfenster.Buchung, error)
	ListByStandortUndZeitraum(ctx context.Context, standortID int64, von, bis time.Time) ([]zeitfenster.Buchung, error)
	ListBySpediteur(ctx context.Context, spediteur string, ab time.Time) ([]zeitfenster.Buchung, error)
	Cancel(ctx context.Context, id int64, jetzt time.Time) error
}

// EreignisStore ist die von diesem Paket benoetigte Persistenz-Schnittstelle
// fuer das Ereignis-Journal.
type EreignisStore interface {
	Append(ctx context.Context, e *zeitfenster.BuchungEreignis) error
	ListByBuchung(ctx context.Context, buchungID int64) ([]zeitfenster.BuchungEreignis, error)
}

// --- DTOs: Buchung ---

// buchungResponse ist das Draht-Format einer Buchung. Anders als der Domain-Typ
// (buchung.go im Domain hat KEIN status-Feld) traegt die Antwort den
// vorberechneten Status — der Client soll nicht selbst aus Ereignissen
// herleiten muessen, was der Server schon weiss (CONVENTIONS.md §10).
type buchungResponse struct {
	ID                int64  `json:"id"`
	RampeID           int64  `json:"rampe_id"`
	Spediteur         string `json:"spediteur"`
	Kennzeichen       string `json:"kennzeichen"`
	SendungNr         string `json:"sendung_nr"`
	Temperaturbereich string `json:"temperaturbereich"`
	Beginn            string `json:"beginn"`
	Ende              string `json:"ende"`
	Status            string `json:"status"`
}

func toBuchungResponse(b zeitfenster.Buchung, status zeitfenster.Status) buchungResponse {
	return buchungResponse{
		ID:                b.ID,
		RampeID:           b.RampeID,
		Spediteur:         b.Spediteur,
		Kennzeichen:       b.Kennzeichen,
		SendungNr:         b.SendungNr,
		Temperaturbereich: string(b.Temperaturbereich),
		Beginn:            b.Beginn.UTC().Format(time.RFC3339),
		Ende:              b.Ende.UTC().Format(time.RFC3339),
		Status:            string(status),
	}
}

type createBuchungRequest struct {
	RampeID           int64  `json:"rampe_id"`
	Spediteur         string `json:"spediteur"`
	Kennzeichen       string `json:"kennzeichen"`
	SendungNr         string `json:"sendung_nr"`
	Temperaturbereich string `json:"temperaturbereich"`
	Beginn            string `json:"beginn"`
	Ende              string `json:"ende"`
}

// validate deckt nur den Transport ab: Pflichtfelder, Formate. Domain-Regeln
// (2-7) prueft zeitfenster.PruefeBuchung im Handler, nicht hier.
func (r createBuchungRequest) validate() error {
	if r.RampeID <= 0 {
		return errors.New("rampe_id is required")
	}
	if strings.TrimSpace(r.Spediteur) == "" {
		return errors.New("spediteur is required")
	}
	if strings.TrimSpace(r.Kennzeichen) == "" {
		return errors.New("kennzeichen is required")
	}
	return nil
}

// toDomain ist die einzige Stelle, an der ein ungeprueftes Wire-Format zum
// Domain-Typ wird (CONVENTIONS.md §10). Alle Fehler hier sind Format-/
// Transportfehler (400), keine Domain-Sentinels — der Aufrufer behandelt sie
// entsprechend ueber writeValidierungsFehler.
func (r createBuchungRequest) toDomain() (zeitfenster.Buchung, error) {
	tb, err := zeitfenster.ParseTemperaturbereich(r.Temperaturbereich)
	if err != nil {
		return zeitfenster.Buchung{}, fmt.Errorf("temperaturbereich: %w", err)
	}
	beginn, err := time.Parse(time.RFC3339, r.Beginn)
	if err != nil {
		return zeitfenster.Buchung{}, fmt.Errorf("beginn: %w", err)
	}
	ende, err := time.Parse(time.RFC3339, r.Ende)
	if err != nil {
		return zeitfenster.Buchung{}, fmt.Errorf("ende: %w", err)
	}
	return zeitfenster.Buchung{
		RampeID:           r.RampeID,
		Spediteur:         strings.TrimSpace(r.Spediteur),
		Kennzeichen:       strings.TrimSpace(r.Kennzeichen),
		SendungNr:         r.SendungNr,
		Temperaturbereich: tb,
		Beginn:            beginn.UTC(),
		Ende:              ende.UTC(),
	}, nil
}

type ereignisResponse struct {
	Typ       string `json:"typ"`
	Zeitpunkt string `json:"zeitpunkt"`
}

func toEreignisResponse(e zeitfenster.BuchungEreignis) ereignisResponse {
	return ereignisResponse{Typ: string(e.Typ), Zeitpunkt: e.Zeitpunkt.UTC().Format(time.RFC3339)}
}

// buchungDetailResponse bettet buchungResponse ohne JSON-Tag ein, damit ihre
// Felder auf oberster Ebene neben ereignisse erscheinen (Go flacht
// unbenannte eingebettete Structs beim JSON-Encoding automatisch ab).
type buchungDetailResponse struct {
	buchungResponse
	Ereignisse []ereignisResponse `json:"ereignisse"`
}

type erfasseEreignisRequest struct {
	Typ        string `json:"typ"`
	ErfasstVon string `json:"erfasst_von"`
}

func (r erfasseEreignisRequest) validate() error {
	if strings.TrimSpace(r.Typ) == "" {
		return errors.New("typ is required")
	}
	return nil
}

// --- Handler ---

// GET /api/v1/buchungen — entweder ?standort_id=&datum= (Tagesplan fuer
// Disponent/Pfoertner) oder ?spediteur=&ab= ("meine Buchungen" fuer Fahrer).
func (h *handler) listeBuchungen(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx := r.Context()

	if spediteur := q.Get("spediteur"); spediteur != "" {
		ab, err := parseAb(q.Get("ab"))
		if err != nil {
			writeValidierungsFehler(w, err)
			return
		}
		buchungen, err := h.buchungen.ListBySpediteur(ctx, spediteur, ab)
		if err != nil {
			writeError(w, h.log, err)
			return
		}
		h.antwortMitBuchungen(w, r, buchungen)
		return
	}

	standortID, err := parseStandortID(q.Get("standort_id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	tag, err := parseDatum(q.Get("datum"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	standort, err := h.standorte.Get(ctx, standortID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	von, bis := standort.TagesgrenzenUTC(tag)
	buchungen, err := h.buchungen.ListByStandortUndZeitraum(ctx, standortID, von, bis)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	h.antwortMitBuchungen(w, r, buchungen)
}

// antwortMitBuchungen berechnet fuer jede Buchung ihren Status aus dem
// Ereignis-Journal (N+1 Anfragen — bei MVP-Groessenordnungen unproblematisch,
// siehe RampeStore.ListByStandort in internal/sqlite fuer denselben Kompromiss)
// und schreibt die Antwort.
func (h *handler) antwortMitBuchungen(w http.ResponseWriter, r *http.Request, buchungen []zeitfenster.Buchung) {
	jetzt := time.Now().UTC()
	antwort := make([]buchungResponse, len(buchungen))
	for i, b := range buchungen {
		ereignisse, err := h.ereignisse.ListByBuchung(r.Context(), b.ID)
		if err != nil {
			writeError(w, h.log, err)
			return
		}
		status := zeitfenster.BerechneStatus(ereignisse, b.StorniertAm, b.Beginn, jetzt)
		antwort[i] = toBuchungResponse(b, status)
	}
	writeJSON(w, http.StatusOK, antwort)
}

// GET /api/v1/buchungen/{id} — Buchung plus vollstaendiges Ereignis-Journal.
func (h *handler) getBuchung(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	ctx := r.Context()

	buchung, err := h.buchungen.Get(ctx, id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	ereignisse, err := h.ereignisse.ListByBuchung(ctx, id)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	status := zeitfenster.BerechneStatus(ereignisse, buchung.StorniertAm, buchung.Beginn, time.Now().UTC())
	antwort := buchungDetailResponse{
		buchungResponse: toBuchungResponse(buchung, status),
		Ereignisse:      make([]ereignisResponse, len(ereignisse)),
	}
	for i, e := range ereignisse {
		antwort.Ereignisse[i] = toEreignisResponse(e)
	}
	writeJSON(w, http.StatusOK, antwort)
}

// POST /api/v1/buchungen — laedt Rampe+Standort, prueft Regeln 2-7, persistiert.
// Regel 1 (Slot-Konflikt) meldet der Store ueber zeitfenster.ErrRampeBelegt.
func (h *handler) erstelleBuchung(w http.ResponseWriter, r *http.Request) {
	var req createBuchungRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidierungsFehler(w, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := req.validate(); err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	buchung, err := req.toDomain()
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}

	ctx := r.Context()
	rampe, err := h.rampen.Get(ctx, buchung.RampeID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	standort, err := h.standorte.Get(ctx, rampe.StandortID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	jetzt := time.Now().UTC()
	buchung.ErstelltAm = jetzt
	if err := zeitfenster.PruefeBuchung(buchung, rampe, standort, jetzt); err != nil {
		writeError(w, h.log, err)
		return
	}

	if err := h.buchungen.Create(ctx, &buchung); err != nil {
		writeError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toBuchungResponse(buchung, zeitfenster.StatusGebucht))
}

// DELETE /api/v1/buchungen/{id} — Storno (soft delete + Slots freigeben,
// beides im Store in einer Transaktion, siehe internal/sqlite/buchung.go).
func (h *handler) storniereBuchung(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	if err := h.buchungen.Cancel(r.Context(), id, time.Now().UTC()); err != nil {
		writeError(w, h.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/buchungen/{id}/ereignisse — ein Ereignis (Ankunft/Andockung/
// Abfahrt) erfassen. zeitfenster.PruefeEreignis prueft Regeln 8-12 gegen das
// geladene Journal; der UNIQUE-Constraint im Store ist der Race-Guard fuer
// zwei gleichzeitige Anfragen (siehe internal/sqlite/ereignis.go).
func (h *handler) erfasseEreignis(w http.ResponseWriter, r *http.Request) {
	buchungID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}

	var req erfasseEreignisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeValidierungsFehler(w, fmt.Errorf("invalid JSON: %w", err))
		return
	}
	if err := req.validate(); err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	typ, err := zeitfenster.ParseEreignisTyp(req.Typ)
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}

	ctx := r.Context()
	buchung, err := h.buchungen.Get(ctx, buchungID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	bisherige, err := h.ereignisse.ListByBuchung(ctx, buchungID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	if err := zeitfenster.PruefeEreignis(bisherige, buchung.StorniertAm, typ); err != nil {
		writeError(w, h.log, err)
		return
	}

	neu := zeitfenster.BuchungEreignis{
		BuchungID:  buchungID,
		Typ:        typ,
		Zeitpunkt:  time.Now().UTC(),
		ErfasstVon: req.ErfasstVon,
	}
	if err := h.ereignisse.Append(ctx, &neu); err != nil {
		writeError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEreignisResponse(neu))
}

// --- Verfuegbarkeit ---

type fensterKandidatResponse struct {
	RampeID     int64  `json:"rampe_id"`
	Bezeichnung string `json:"bezeichnung"`
	Beginn      string `json:"beginn"`
	Ende        string `json:"ende"`
}

// GET /api/v1/verfuegbarkeit — freie Fenster ueber alle passenden Rampen
// eines Standorts. zeitfenster.FreieFenster prueft NICHT den
// Temperaturbereich (Regel 2) — das entscheidet dieser Handler, indem er nur
// passende, aktive Rampen ueberhaupt hineinreicht.
func (h *handler) sucheVerfuegbarkeit(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx := r.Context()

	standortID, err := parseStandortID(q.Get("standort_id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	tag, err := parseDatum(q.Get("datum"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	bereich, err := zeitfenster.ParseTemperaturbereich(q.Get("temperaturbereich"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	dauerMin, err := parsePositiveInt(q.Get("dauer"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	dauer := time.Duration(dauerMin) * time.Minute
	if dauer < zeitfenster.MinDauer || dauer > zeitfenster.MaxDauer {
		writeValidierungsFehler(w, fmt.Errorf("dauer must be between %s and %s", zeitfenster.MinDauer, zeitfenster.MaxDauer))
		return
	}

	standort, err := h.standorte.Get(ctx, standortID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}
	rampen, err := h.rampen.ListByStandort(ctx, standortID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	von, bis := standort.TagesgrenzenUTC(tag)
	buchungen, err := h.buchungen.ListByStandortUndZeitraum(ctx, standortID, von, bis)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	jetzt := time.Now().UTC()
	antwort := make([]fensterKandidatResponse, 0)
	for _, rampe := range rampen {
		if !rampe.Aktiv || !rampe.UnterstuetztTemperaturbereich(bereich) {
			continue
		}
		for _, kandidat := range zeitfenster.FreieFenster(standort, rampe, tag, dauer, buchungen, jetzt) {
			antwort = append(antwort, fensterKandidatResponse{
				RampeID:     rampe.ID,
				Bezeichnung: rampe.Bezeichnung,
				Beginn:      kandidat.Beginn.Format(time.RFC3339),
				Ende:        kandidat.Ende.Format(time.RFC3339),
			})
		}
	}
	writeJSON(w, http.StatusOK, antwort)
}

// --- Kennzahlen ---

// kennzahlenResponse verwendet *float64 statt float64 fuer die Mittelwerte:
// "noch nichts zu berechnen" (nil) und "Mittelwert ist null" sind
// unterschiedliche Zustaende und muessen im UI unterschiedlich aussehen
// (MVP.md §7, CONVENTIONS.md §10 Optionalitaet).
type kennzahlenResponse struct {
	BuchungenGesamt       int      `json:"buchungen_gesamt"`
	StandzeitSchnittMin   *float64 `json:"standzeit_schnitt_min"`
	PuenktlichkeitProzent *float64 `json:"puenktlichkeit_prozent"`
	NoShows               int      `json:"no_shows"`
}

// GET /api/v1/kennzahlen — Tages-KPIs: Ø Standzeit, Puenktlichkeit, No-Shows.
func (h *handler) getKennzahlen(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ctx := r.Context()

	standortID, err := parseStandortID(q.Get("standort_id"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	tag, err := parseDatum(q.Get("datum"))
	if err != nil {
		writeValidierungsFehler(w, err)
		return
	}
	standort, err := h.standorte.Get(ctx, standortID)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	von, bis := standort.TagesgrenzenUTC(tag)
	buchungen, err := h.buchungen.ListByStandortUndZeitraum(ctx, standortID, von, bis)
	if err != nil {
		writeError(w, h.log, err)
		return
	}

	jetzt := time.Now().UTC()
	var (
		standzeitSumme   time.Duration
		standzeitCount   int
		puenktlichCount  int
		puenktlichGesamt int
		noShows          int
	)
	for _, b := range buchungen {
		if b.Storniert() {
			continue
		}
		ereignisse, err := h.ereignisse.ListByBuchung(ctx, b.ID)
		if err != nil {
			writeError(w, h.log, err)
			return
		}

		if dauer, ok := zeitfenster.Standzeit(ereignisse); ok {
			standzeitSumme += dauer
			standzeitCount++
		}
		if puenktlich, ok := zeitfenster.Puenktlich(ereignisse, b.Beginn); ok {
			puenktlichGesamt++
			if puenktlich {
				puenktlichCount++
			}
		}
		if zeitfenster.IstNoShow(ereignisse, b.StorniertAm, b.Beginn, jetzt) {
			noShows++
		}
	}

	antwort := kennzahlenResponse{BuchungenGesamt: len(buchungen), NoShows: noShows}
	if standzeitCount > 0 {
		schnitt := standzeitSumme.Minutes() / float64(standzeitCount)
		antwort.StandzeitSchnittMin = &schnitt
	}
	if puenktlichGesamt > 0 {
		prozent := 100 * float64(puenktlichCount) / float64(puenktlichGesamt)
		antwort.PuenktlichkeitProzent = &prozent
	}
	writeJSON(w, http.StatusOK, antwort)
}
