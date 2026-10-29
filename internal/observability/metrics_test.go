package observability

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestConfigFromEnvRejectsInvalidFailureRate(t *testing.T) {
	t.Setenv("FAILURE_RATE", "1.2")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected invalid FAILURE_RATE to be rejected")
	}
}

func TestMiddlewareRecordsInjectedFailure(t *testing.T) {
	registry := prometheus.NewRegistry()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	metrics := New(registry, registry, logger)
	metrics.random = func() float64 { return 0 }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /tickets", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := metrics.Middleware(mux, FaultConfig{FailureRate: 1})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/tickets", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "http_requests_total" && len(family.Metric) == 1 {
			if got := family.Metric[0].Counter.GetValue(); got != 1 {
				t.Fatalf("got counter %v, want 1", got)
			}
			return
		}
	}
	t.Fatal("http_requests_total was not gathered")
}

func TestTicketCreatedMetric(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := New(registry, registry, slog.New(slog.NewTextHandler(io.Discard, nil)))
	metrics.TicketCreated()

	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "tickets_created_total" {
			if got := family.Metric[0].Counter.GetValue(); got != 1 {
				t.Fatalf("got counter %v, want 1", got)
			}
			return
		}
	}
	t.Fatal("tickets_created_total was not gathered")
}
