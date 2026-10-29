package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Cloud-Computing-UPB/lab5/internal/observability"
	"github.com/Cloud-Computing-UPB/lab5/internal/store"
	"github.com/Cloud-Computing-UPB/lab5/internal/ticket"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	usePostgres, err := strconv.ParseBool(getenv("USE_POSTGRES", "false"))
	if err != nil {
		log.Fatalf("invalid USE_POSTGRES value: %v", err)
	}

	var s ticket.Store
	if usePostgres {
		dsn, err := databaseURL()
		if err != nil {
			log.Fatalf("database configuration: %v", err)
		}
		pg, err := store.NewPostgres(ctx, dsn)
		if err != nil {
			log.Fatalf("postgres: %v", err)
		}
		s = pg
		log.Println("using PostgreSQL store")
	} else {
		s = store.NewMemory()
		log.Println("using in-memory store")
	}
	defer s.Close()
	faults, err := observability.ConfigFromEnv()
	if err != nil {
		log.Fatalf("observability configuration: %v", err)
	}
	metrics := observability.NewDefault(logger)

	mux := http.NewServeMux()
	registerHealthHandlers(mux, s)
	mux.Handle("GET /metrics", metrics.Handler())
	mux.HandleFunc("GET /info", func(w http.ResponseWriter, _ *http.Request) {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{
			"service":  "ticketing-api",
			"version":  getenv("APP_VERSION", "dev"),
			"instance": hostname,
		}); err != nil {
			log.Printf("encode info response: %v", err)
		}
	})
	ticket.NewHandler(s, ticket.WithTicketCreated(metrics.TicketCreated)).Register(mux)

	srv := &http.Server{
		Addr:              ":" + getenv("PORT", "8080"),
		Handler:           metrics.Middleware(mux, faults),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

func registerHealthHandlers(mux *http.ServeMux, s ticket.Store) {
	live := func(w http.ResponseWriter, _ *http.Request) {
		writeHealthStatus(w, http.StatusOK, "ok")
	}

	// /health is kept for compatibility with the previous laboratories.
	mux.HandleFunc("GET /health", live)
	mux.HandleFunc("GET /livez", live)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := s.Ping(ctx); err != nil {
			log.Printf("readiness check failed: %v", err)
			writeHealthStatus(w, http.StatusServiceUnavailable, "not ready")
			return
		}
		writeHealthStatus(w, http.StatusOK, "ready")
	})
}

func writeHealthStatus(w http.ResponseWriter, status int, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": value}); err != nil {
		log.Printf("encode health response: %v", err)
	}
}

// databaseURL keeps DATABASE_URL compatible with the previous laboratories.
// If it is not present, a DSN is assembled from ConfigMap and Secret values.
func databaseURL() (string, error) {
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		return dsn, nil
	}

	required := map[string]string{
		"DB_HOST":     strings.TrimSpace(os.Getenv("DB_HOST")),
		"DB_NAME":     strings.TrimSpace(os.Getenv("DB_NAME")),
		"DB_USER":     strings.TrimSpace(os.Getenv("DB_USER")),
		"DB_PASSWORD": os.Getenv("DB_PASSWORD"),
	}
	missing := make([]string, 0, len(required))
	for _, key := range []string{"DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD"} {
		if required[key] == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return "", errors.New("missing " + strings.Join(missing, ", "))
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(required["DB_USER"], required["DB_PASSWORD"]),
		Host:   net.JoinHostPort(required["DB_HOST"], getenv("DB_PORT", "5432")),
		Path:   required["DB_NAME"],
	}
	query := u.Query()
	query.Set("sslmode", "disable")
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
