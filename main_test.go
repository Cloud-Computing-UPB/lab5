package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Cloud-Computing-UPB/lab5/internal/store"
)

func TestDatabaseURLPrefersExplicitValue(t *testing.T) {
	want := "postgres://explicit:secret@example:5432/db?sslmode=require"
	t.Setenv("DATABASE_URL", want)
	t.Setenv("DB_HOST", "ignored")

	got, err := databaseURL()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDatabaseURLFromComponents(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "postgres")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "tickets")
	t.Setenv("DB_USER", "tickets")
	t.Setenv("DB_PASSWORD", "lab secret")

	got, err := databaseURL()
	if err != nil {
		t.Fatal(err)
	}
	want := "postgres://tickets:lab%20secret@postgres:5432/tickets?sslmode=disable"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestDatabaseURLReportsMissingValues(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "DB_HOST", "DB_NAME", "DB_USER", "DB_PASSWORD"} {
		t.Setenv(key, "")
	}

	_, err := databaseURL()
	if err == nil || !strings.Contains(err.Error(), "DB_HOST") || !strings.Contains(err.Error(), "DB_PASSWORD") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHealthEndpointsWithMemoryStore(t *testing.T) {
	mux := http.NewServeMux()
	registerHealthHandlers(mux, store.NewMemory())

	for _, path := range []string{"/health", "/livez", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		mux.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Errorf("%s: got status %d", path, resp.Code)
		}
	}
}
