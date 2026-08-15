package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/collector"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/metrics"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/testsupport"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/threshold"
)

func quietDeps(col *collector.Collector) Deps {
	return Deps{
		Collector:  col,
		Thresholds: threshold.Default(),
		Metrics:    metrics.New(),
		Interval:   time.Hour,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func TestHealthzOKAndMetrics(t *testing.T) {
	s := New(quietDeps(testsupport.NewCollector()))
	s.scrape(context.Background())

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "OK") {
		t.Errorf("/healthz body = %q, want to contain OK", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), "redis_health_up") {
		t.Error("/metrics did not expose redis_health_up")
	}
}

func TestHealthzCriticalReturns503(t *testing.T) {
	broken := strings.Replace(testsupport.RedisInfo, "rdb_last_bgsave_status:ok", "rdb_last_bgsave_status:err", 1)
	col := &collector.Collector{
		Provider: testsupport.FakeProvider{Payload: broken},
		Location: time.UTC,
		DBIndex:  5,
	}
	s := New(quietDeps(col))
	s.scrape(context.Background())

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/healthz = %d, want 503 when CRITICAL", rec.Code)
	}
}

func TestHealthzUnavailableOnScrapeError(t *testing.T) {
	col := &collector.Collector{
		Provider: testsupport.FakeProvider{Err: context.DeadlineExceeded},
		Location: time.UTC,
	}
	s := New(quietDeps(col))
	s.scrape(context.Background())

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("/healthz = %d, want 503 when scrape fails", rec.Code)
	}
}
