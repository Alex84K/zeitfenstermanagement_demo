package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// withMiddleware verkettet recover (aeusserste Schicht, faengt Panics aus
// allen inneren Schichten ab), logging (protokolliert jede Anfrage samt
// Status und Dauer) und CORS (Vite-Dev-Server, MVP.md §7 Konfiguration).
func withMiddleware(next http.Handler, log *slog.Logger) http.Handler {
	return recoverMiddleware(log, loggingMiddleware(log, corsMiddleware(next)))
}

func recoverMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", "panic", rec, "path", r.URL.Path)
				writeJSON(w, http.StatusInternalServerError, apiError{"INTERNER_FEHLER", "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"dauer_ms", time.Since(start).Milliseconds(),
		)
	})
}

// statusWriter merkt sich den geschriebenen Statuscode fuer die
// Logging-Middleware — http.ResponseWriter selbst gibt ihn nicht her.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// corsMiddleware oeffnet CORS fuer den Vite-Dev-Server (MVP.md §7). Fest
// verdrahtet statt konfigurierbar — YAGNI, solange es nur eine Origin gibt.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
