package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"zeitfenster/internal/httpapi"
	"zeitfenster/internal/sqlite"
)

func main() {
	log := newLogger(getenv("ZFM_LOG_LEVEL", "info"))

	dbPath := getenv("ZFM_DB_PATH", "./zeitfenster.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		log.Error("open db", "err", err, "path", dbPath)
		os.Exit(1)
	}
	defer db.Close()

	addr := getenv("ZFM_ADDR", ":8080")
	srv := httpapi.NewHTTPServer(addr, httpapi.Deps{
		Buchungen:  sqlite.NewBuchungStore(db),
		Ereignisse: sqlite.NewEreignisStore(db),
		Standorte:  sqlite.NewStandortStore(db),
		Rampen:     sqlite.NewRampeStore(db),
		Log:        log,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("server starting", "addr", addr, "db", dbPath)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen and serve", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
