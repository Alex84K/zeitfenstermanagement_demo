package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// handler buendelt die Store-Abhaengigkeiten, die die Endpunkt-Methoden in
// buchung.go und stammdaten.go brauchen.
type handler struct {
	buchungen  BuchungStore
	ereignisse EreignisStore
	standorte  StandortStore
	rampen     RampeStore
	log        *slog.Logger
}

// Deps sind die Abhaengigkeiten von NewServer. Die konkreten
// internal/sqlite-Typen erfuellen diese Schnittstellen strukturell, ohne
// dass dieses Paket sqlite kennt (CONVENTIONS.md §3, §7).
type Deps struct {
	Buchungen  BuchungStore
	Ereignisse EreignisStore
	Standorte  StandortStore
	Rampen     RampeStore
	Log        *slog.Logger
}

// NewServer baut den vollstaendigen HTTP-Handler: Routing plus Middleware.
// Go 1.22+-Routenmuster (Methode + Pfad, {id} als Wildcard) — kein Chi/Gin
// noetig (CONVENTIONS.md §9).
func NewServer(deps Deps) http.Handler {
	h := &handler{
		buchungen:  deps.Buchungen,
		ereignisse: deps.Ereignisse,
		standorte:  deps.Standorte,
		rampen:     deps.Rampen,
		log:        deps.Log,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/verfuegbarkeit", h.sucheVerfuegbarkeit)
	mux.HandleFunc("GET /api/v1/buchungen", h.listeBuchungen)
	mux.HandleFunc("GET /api/v1/buchungen/{id}", h.getBuchung)
	mux.HandleFunc("POST /api/v1/buchungen", h.erstelleBuchung)
	mux.HandleFunc("DELETE /api/v1/buchungen/{id}", h.storniereBuchung)
	mux.HandleFunc("POST /api/v1/buchungen/{id}/ereignisse", h.erfasseEreignis)
	mux.HandleFunc("GET /api/v1/kennzahlen", h.getKennzahlen)

	mux.HandleFunc("GET /api/v1/standorte", h.listeStandorte)
	mux.HandleFunc("POST /api/v1/standorte", h.erstelleStandort)
	mux.HandleFunc("PATCH /api/v1/standorte/{id}", h.aktualisiereStandort)
	mux.HandleFunc("GET /api/v1/standorte/{id}/rampen", h.listeRampen)
	mux.HandleFunc("POST /api/v1/standorte/{id}/rampen", h.erstelleRampe)
	mux.HandleFunc("PATCH /api/v1/rampen/{id}", h.aktualisiereRampe)

	return withMiddleware(mux, deps.Log)
}

// NewHTTPServer baut einen *http.Server mit Timeouts fuer addr — ein Server
// ohne sie ist anfaellig fuer langsame Clients (CONVENTIONS.md §9).
func NewHTTPServer(addr string, deps Deps) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      NewServer(deps),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
