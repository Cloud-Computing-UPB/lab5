package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand/v2"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// FaultConfig controls the deterministic failure scenarios used in the lab.
// All values are disabled by default and must be explicitly enabled through
// environment variables or Helm values.
type FaultConfig struct {
	ArtificialDelay time.Duration
	FailureRate     float64
	CPUBurn         time.Duration
}

// ConfigFromEnv parses and validates the lab-only fault-injection settings.
func ConfigFromEnv() (FaultConfig, error) {
	delay, err := durationFromMilliseconds("ARTIFICIAL_DELAY_MS", 0, 10_000)
	if err != nil {
		return FaultConfig{}, err
	}
	cpuBurn, err := durationFromMilliseconds("CPU_BURN_MS", 0, 2_000)
	if err != nil {
		return FaultConfig{}, err
	}
	failureRate, err := strconv.ParseFloat(getenv("FAILURE_RATE", "0"), 64)
	if err != nil || failureRate < 0 || failureRate > 1 {
		return FaultConfig{}, errors.New("FAILURE_RATE must be a number between 0 and 1")
	}
	return FaultConfig{
		ArtificialDelay: delay,
		FailureRate:     failureRate,
		CPUBurn:         cpuBurn,
	}, nil
}

// Metrics owns the application-level RED metrics exposed at /metrics.
type Metrics struct {
	requests     *prometheus.CounterVec
	duration     *prometheus.HistogramVec
	inProgress   *prometheus.GaugeVec
	tickets      prometheus.Counter
	gatherer     prometheus.Gatherer
	logger       *slog.Logger
	random       func() float64
	instanceName string
}

// New creates and registers an isolated set of application metrics.
func New(registerer prometheus.Registerer, gatherer prometheus.Gatherer, logger *slog.Logger) *Metrics {
	if logger == nil {
		logger = slog.Default()
	}
	instanceName, err := os.Hostname()
	if err != nil {
		instanceName = "unknown"
	}
	m := &Metrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests completed by the Ticketing API.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of Ticketing API HTTP requests in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"method", "route"}),
		inProgress: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "http_requests_in_progress",
			Help: "Current number of Ticketing API HTTP requests being served.",
		}, []string{"method", "route"}),
		tickets: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "tickets_created_total",
			Help: "Total number of tickets successfully created.",
		}),
		gatherer:     gatherer,
		logger:       logger,
		random:       mathrand.Float64,
		instanceName: instanceName,
	}
	registerer.MustRegister(m.requests, m.duration, m.inProgress, m.tickets)
	return m
}

// NewDefault registers the metrics in the process-wide Prometheus registry.
func NewDefault(logger *slog.Logger) *Metrics {
	return New(prometheus.DefaultRegisterer, prometheus.DefaultGatherer, logger)
}

// Handler returns the Prometheus exposition endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.gatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// TicketCreated records a successful POST /tickets operation.
func (m *Metrics) TicketCreated() {
	m.tickets.Inc()
}

// Middleware records RED metrics, emits structured request logs and applies
// the controlled delay, failure and CPU load used in the investigation step.
func (m *Metrics) Middleware(next http.Handler, faults FaultConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := routePattern(next, r)
		if route == "GET /metrics" {
			next.ServeHTTP(w, r)
			return
		}

		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)

		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		labels := []string{r.Method, route}
		m.inProgress.WithLabelValues(labels...).Inc()
		defer func() {
			m.inProgress.WithLabelValues(labels...).Dec()
			duration := time.Since(start)
			m.duration.WithLabelValues(labels...).Observe(duration.Seconds())
			m.requests.WithLabelValues(r.Method, route, strconv.Itoa(recorder.status)).Inc()
			m.logger.Info("request completed",
				"request_id", requestID,
				"method", r.Method,
				"route", route,
				"status", recorder.status,
				"duration_ms", duration.Milliseconds(),
				"instance", m.instanceName,
			)
		}()

		if shouldInjectFault(r.URL.Path) {
			burnCPU(faults.CPUBurn)
			if err := waitForDelay(r.Context(), faults.ArtificialDelay); err != nil {
				writeFailure(recorder, http.StatusRequestTimeout, "request cancelled during artificial delay")
				return
			}
			if faults.FailureRate > 0 && m.random() < faults.FailureRate {
				writeFailure(recorder, http.StatusServiceUnavailable, "simulated dependency failure")
				return
			}
		}

		next.ServeHTTP(recorder, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusRecorder) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusRecorder) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the original ResponseWriter.
func (w *statusRecorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }

type routeResolver interface {
	Handler(*http.Request) (http.Handler, string)
}

func routePattern(next http.Handler, r *http.Request) string {
	if resolver, ok := next.(routeResolver); ok {
		_, pattern := resolver.Handler(r)
		if pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}

func shouldInjectFault(path string) bool {
	switch path {
	case "/health", "/livez", "/readyz", "/metrics":
		return false
	default:
		return true
	}
}

func waitForDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func burnCPU(duration time.Duration) {
	if duration <= 0 {
		return
	}
	deadline := time.Now().Add(duration)
	value := uint64(1)
	for time.Now().Before(deadline) {
		value = value*1_664_525 + 1_013_904_223
	}
	runtime.KeepAlive(value)
}

func writeFailure(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func durationFromMilliseconds(key string, fallback, maximum int64) (time.Duration, error) {
	value, err := strconv.ParseInt(getenv(key, strconv.FormatInt(fallback, 10)), 10, 64)
	if err != nil || value < 0 || value > maximum {
		return 0, fmt.Errorf("%s must be an integer between 0 and %d", key, maximum)
	}
	return time.Duration(value) * time.Millisecond, nil
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func newRequestID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(value[:])
}
