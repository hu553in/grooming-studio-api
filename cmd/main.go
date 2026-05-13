package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hu553in/grooming-studio-api/internal/avito"
	"github.com/hu553in/grooming-studio-api/internal/config"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg := config.Load(log)
	log.Info("config is loaded", "config", cfg)

	avitoClient := avito.NewClient(
		cfg.AvitoUserID,
		cfg.CacheTTL,
		&http.Client{Timeout: cfg.RequestTimeout},
	)

	mux := http.NewServeMux()
	mux.Handle("/api/reviews/avito", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		if !cfg.AvitoEnabled {
			writeJSON(r, w, http.StatusOK, map[string]string{"error": "Avito is disabled"}, log)

			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		reviews, err := avitoClient.FetchReviews(ctx, log)
		if err != nil {
			log.ErrorContext(ctx, "failed to fetch reviews", "error", err)
			writeJSON(
				r,
				w,
				http.StatusServiceUnavailable,
				map[string]string{"error": "Failed to fetch reviews"},
				log,
			)

			return
		}

		writeJSON(r, w, http.StatusOK, reviews, log)
	}))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)

			return
		}

		writeJSON(r, w, http.StatusOK, map[string]string{"status": "OK"}, log)
	})

	handler := loggingMiddleware(corsMiddleware(mux), log)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.InfoContext(ctx, "starting server", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.ErrorContext(ctx, "server is terminated unexpectedly", "error", err)
			stop()
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.ErrorContext(shutdownCtx, "graceful shutdown is failed", "error", err)
	}

	log.InfoContext(shutdownCtx, "server is stopped")
}

func writeJSON(r *http.Request, w http.ResponseWriter, status int, payload any, log *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.ErrorContext(r.Context(), "failed to encode response", "error", err, "payload", payload)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Vary", "Origin")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.InfoContext(
			r.Context(),
			"request is completed",
			"method", r.Method,
			"path", r.URL.Path,
			"elapsed", time.Since(start),
		)
	})
}
