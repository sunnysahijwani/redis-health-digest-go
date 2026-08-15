// Package collector samples a Redis instance via a single read-only INFO
// command and assembles an immutable report.Report.
package collector

import (
	"context"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/state"
)

// InfoProvider returns the raw Redis INFO payload. Abstracting it keeps metric
// assembly independent of the client and trivially fakeable in tests.
type InfoProvider interface {
	Info(ctx context.Context) (string, error)
}

// RedisProvider is the production InfoProvider backed by go-redis.
type RedisProvider struct {
	Client *redis.Client
}

func (p RedisProvider) Info(ctx context.Context) (string, error) {
	return p.Client.Info(ctx).Result()
}

// Gauge is an optional user-defined metric derived from the parsed INFO map.
type Gauge struct {
	Name string
	Fn   func(Info) float64
}

// Collector turns Redis INFO into a report.Report.
type Collector struct {
	Provider    InfoProvider
	Environment string
	Location    *time.Location
	DBIndex     int
	Gauges      []Gauge

	// Now is injectable so tests get a deterministic SampledAt.
	Now func() time.Time
}

func (c *Collector) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Collect fetches a fresh sample. prev (may be nil) supplies the baseline for
// "since last check" delta counters.
func (c *Collector) Collect(ctx context.Context, prev *state.Snapshot) (*report.Report, error) {
	raw, err := c.Provider.Info(ctx)
	if err != nil {
		return nil, err
	}
	return c.Build(ParseInfo(raw), prev), nil
}

// Build assembles a report from an already-parsed Info. It is pure (no I/O) so
// it can be unit-tested against fixtures.
func (c *Collector) Build(info Info, prev *state.Snapshot) *report.Report {
	hits := info.Int("keyspace_hits")
	misses := info.Int("keyspace_misses")
	total := hits + misses
	var hitRate *float64
	if total > 0 {
		v := round2(float64(hits) / float64(total) * 100)
		hitRate = &v
	}

	evicted := info.Int("evicted_keys")
	rejected := info.Int("rejected_connections")
	var evictedDelta, rejectedDelta int64
	if prev != nil {
		evictedDelta = max(int64(0), evicted-prev.EvictedKeys)
		rejectedDelta = max(int64(0), rejected-prev.RejectedConnections)
	}

	var frag *float64
	if f, ok := info.Float("mem_fragmentation_ratio"); ok {
		frag = &f
	}

	loc := c.Location
	if loc == nil {
		loc = time.UTC
	}

	env := c.Environment
	if env == "" {
		env = "production"
	}

	return &report.Report{
		Environment:              env,
		SampledAt:                c.now().In(loc),
		RedisVersion:             info.Str("redis_version"),
		UptimeSeconds:            info.Int("uptime_in_seconds"),
		Role:                     info.Str("role"),
		DBIndex:                  c.DBIndex,
		DBSize:                   info.Keys(c.DBIndex),
		UsedMemoryBytes:          info.Int("used_memory"),
		MaxMemoryBytes:           info.Int("maxmemory"),
		MemFragmentationRatio:    frag,
		EvictedKeys:              evicted,
		EvictedKeysDelta:         evictedDelta,
		RejectedConnections:      rejected,
		RejectedConnectionsDelta: rejectedDelta,
		ConnectedClients:         info.Int("connected_clients"),
		BlockedClients:           info.Int("blocked_clients"),
		OpsPerSec:                info.Int("instantaneous_ops_per_sec"),
		KeyspaceHits:             hits,
		KeyspaceMisses:           misses,
		HitRate:                  hitRate,
		RDBLastBgsaveStatus:      info.Str("rdb_last_bgsave_status"),
		AOFLastWriteStatus:       info.Str("aof_last_write_status"),
		CustomGauges:             c.resolveGauges(info),
	}
}

func (c *Collector) resolveGauges(info Info) map[string]float64 {
	if len(c.Gauges) == 0 {
		return nil
	}
	out := make(map[string]float64, len(c.Gauges))
	for _, g := range c.Gauges {
		out[g.Name] = safeGauge(g.Fn, info)
	}
	return out
}

// safeGauge isolates a user closure so a panic doesn't crash a scrape.
func safeGauge(fn func(Info) float64, info Info) (v float64) {
	defer func() { _ = recover() }()
	return fn(info)
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
