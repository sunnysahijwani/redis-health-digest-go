// Package server runs the daemon mode: it scrapes Redis on a ticker and serves
// /healthz (liveness) and /metrics (Prometheus) over HTTP.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/collector"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/metrics"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/state"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/threshold"
)

// Deps are the collaborators the server needs.
type Deps struct {
	Collector     *collector.Collector
	Thresholds    threshold.Thresholds
	Metrics       *metrics.Collector
	Interval      time.Duration
	ScrapeTimeout time.Duration
	Logger        *slog.Logger
}

// Server holds the runtime state shared between the scrape loop and the HTTP
// handlers.
type Server struct {
	deps   Deps
	reg    *prometheus.Registry
	health atomic.Int32 // latest report.Status
	prev   *state.Snapshot
}

// New wires a Server and registers its metrics. Health starts CRITICAL until
// the first successful scrape completes.
func New(deps Deps) *Server {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.ScrapeTimeout == 0 {
		deps.ScrapeTimeout = 5 * time.Second
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(deps.Metrics)

	s := &Server{deps: deps, reg: reg}
	s.health.Store(int32(report.StatusCritical))
	return s
}

// scrape samples once, evaluates thresholds, and updates metrics + health.
func (s *Server) scrape(ctx context.Context) {
	sctx, cancel := context.WithTimeout(ctx, s.deps.ScrapeTimeout)
	defer cancel()

	r, err := s.deps.Collector.Collect(sctx, s.prev)
	if err != nil {
		s.deps.Metrics.Set(nil, true)
		s.health.Store(int32(report.StatusCritical))
		s.deps.Logger.Error("redis scrape failed", "error", err)
		return
	}

	s.deps.Thresholds.Evaluate(r)
	s.deps.Metrics.Set(r, false)
	s.health.Store(int32(r.Status))
	s.prev = &state.Snapshot{
		EvictedKeys:         r.EvictedKeys,
		RejectedConnections: r.RejectedConnections,
		SampledAt:           r.SampledAt,
	}
	s.deps.Logger.Info("scrape complete", "status", r.Status.String(), "hit_rate", hitRate(r))
}

// Handler returns the HTTP mux (exposed for testing).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "redis-health-digest — see /healthz and /metrics")
	})
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	status := report.Status(s.health.Load())
	if status == report.StatusCritical {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	fmt.Fprintln(w, status.String())
}

// Run scrapes immediately, then on Interval, and serves HTTP until ctx is
// cancelled, at which point it shuts the server down gracefully.
func (s *Server) Run(ctx context.Context, addr string) error {
	s.scrape(ctx) // prime state before the first request arrives

	go func() {
		ticker := time.NewTicker(s.deps.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.scrape(ctx)
			}
		}
	}()

	srv := &http.Server{Addr: addr, Handler: s.Handler()}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	s.deps.Logger.Info("listening", "addr", addr, "interval", s.deps.Interval.String())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func hitRate(r *report.Report) float64 {
	if r.HitRate == nil {
		return 0
	}
	return *r.HitRate
}
