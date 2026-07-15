// Package httpapi ist der HTTP-Transport: DTOs, Routing, Fehler-Mapping.
// Importiert den Domain (internal/zeitfenster), nie umgekehrt —
// CONVENTIONS.md §3 (Abhaengigkeitsregel).
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"zeitfenster/internal/zeitfenster"
)

// apiError ist die einheitliche Fehlerantwort — MVP.md §7 "Format der Fehler".
// code ist maschinenlesbar (der Client verzweigt darauf, siehe CLIENT_ADR.md
// ADR-005), message ist fuer Menschen und auf Deutsch (CONVENTIONS.md §4).
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// fehlerZuStatus mappt einen Domain-Fehler auf HTTP-Status und
// maschinenlesbaren Code — die Tabelle aus MVP.md §7, an genau einer Stelle.
//
// 409 bedeutet: der Zustand der Welt erlaubt das nicht ("versuch's anders").
// 422 bedeutet: die Anfrage verletzt eine Domain-Regel ("so grundsaetzlich nicht").
// Reine Transport-Validierungsfehler (400) laufen NICHT hier durch, sondern
// ueber writeValidierungsFehler — sie sind keine Domain-Sentinels.
func fehlerZuStatus(err error) (int, apiError) {
	switch {
	case errors.Is(err, zeitfenster.ErrRampeBelegt):
		return http.StatusConflict, apiError{"RAMPE_BELEGT", "Rampe ist zu dieser Zeit bereits belegt"}
	case errors.Is(err, zeitfenster.ErrUngueltigerStatuswechsel):
		return http.StatusConflict, apiError{"UNGUELTIGER_STATUSWECHSEL", "Ereignis ist in diesem Zustand nicht zulaessig"}
	case errors.Is(err, zeitfenster.ErrBuchungStorniert):
		return http.StatusConflict, apiError{"BUCHUNG_STORNIERT", "Buchung ist storniert"}
	case errors.Is(err, zeitfenster.ErrRampeHatBuchungen):
		return http.StatusConflict, apiError{"RAMPE_HAT_BUCHUNGEN", "Rampe hat zukuenftige aktive Buchungen"}
	case errors.Is(err, zeitfenster.ErrStandortHatRampen):
		return http.StatusConflict, apiError{"STANDORT_HAT_RAMPEN", "Standort hat aktive Rampen"}
	case errors.Is(err, zeitfenster.ErrBezeichnungBelegt):
		return http.StatusConflict, apiError{"BEZEICHNUNG_BELEGT", "Bezeichnung ist an diesem Standort bereits vergeben"}
	case errors.Is(err, zeitfenster.ErrTemperaturMismatch):
		return http.StatusUnprocessableEntity, apiError{"TEMPERATUR_MISMATCH", "Rampe unterstuetzt diesen Temperaturbereich nicht"}
	case errors.Is(err, zeitfenster.ErrAusserhalbOeffnungszeit):
		return http.StatusUnprocessableEntity, apiError{"AUSSERHALB_OEFFNUNGSZEIT", "Buchung liegt ausserhalb der Oeffnungszeiten"}
	case errors.Is(err, zeitfenster.ErrNichtImRaster):
		return http.StatusUnprocessableEntity, apiError{"NICHT_IM_RASTER", "Beginn/Ende muessen auf das 30-Minuten-Raster fallen"}
	case errors.Is(err, zeitfenster.ErrUngueltigeDauer):
		return http.StatusUnprocessableEntity, apiError{"UNGUELTIGE_DAUER", "Dauer muss zwischen 30 Minuten und 4 Stunden liegen"}
	case errors.Is(err, zeitfenster.ErrVergangenheit):
		return http.StatusUnprocessableEntity, apiError{"VERGANGENHEIT", "Buchung liegt in der Vergangenheit"}
	case errors.Is(err, zeitfenster.ErrRampeInaktiv):
		return http.StatusUnprocessableEntity, apiError{"RAMPE_INAKTIV", "Rampe oder Standort ist inaktiv"}
	case errors.Is(err, zeitfenster.ErrKeinTemperaturbereich):
		return http.StatusUnprocessableEntity, apiError{"KEIN_TEMPERATURBEREICH", "Rampe braucht mindestens einen Temperaturbereich"}
	case errors.Is(err, zeitfenster.ErrUngueltigeZeitzone):
		return http.StatusUnprocessableEntity, apiError{"UNGUELTIGE_ZEITZONE", "Zeitzone ist ungueltig"}
	case errors.Is(err, zeitfenster.ErrBuchungNotFound):
		return http.StatusNotFound, apiError{"BUCHUNG_NICHT_GEFUNDEN", "Buchung nicht gefunden"}
	case errors.Is(err, zeitfenster.ErrStandortNotFound):
		return http.StatusNotFound, apiError{"STANDORT_NICHT_GEFUNDEN", "Standort nicht gefunden"}
	case errors.Is(err, zeitfenster.ErrRampeNotFound):
		return http.StatusNotFound, apiError{"RAMPE_NICHT_GEFUNDEN", "Rampe nicht gefunden"}
	default:
		return http.StatusInternalServerError, apiError{"INTERNER_FEHLER", "internal server error"}
	}
}

// writeError schreibt einen Domain-Fehler als JSON-Antwort. Unbekannte Fehler
// (Default-Fall in fehlerZuStatus, 500) werden geloggt — alles andere ist ein
// erwarteter, dem Client bereits per code erklaerter Zustand und braucht
// keinen Log-Eintrag (CONVENTIONS.md §5: nicht doppelt loggen).
func writeError(w http.ResponseWriter, log *slog.Logger, err error) {
	status, body := fehlerZuStatus(err)
	if status == http.StatusInternalServerError {
		log.Error("unhandled error", "err", err)
	}
	writeJSON(w, status, body)
}

// writeValidierungsFehler meldet einen reinen Transport-Validierungsfehler
// (kaputtes JSON, fehlendes Pflichtfeld, falsches Zeitformat) — kein
// Domain-Sentinel, deshalb nicht ueber fehlerZuStatus.
func writeValidierungsFehler(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, apiError{"VALIDIERUNG", err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
