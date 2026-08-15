// Package metrics exposes a report.Report as Prometheus metrics. It implements
// prometheus.Collector directly (the exporter pattern): each scrape emits const
// metrics from the latest report, so /metrics always reflects current state and
// is safe for concurrent scrapes.
package metrics

import (
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
)

const namespace = "redis_health"

type Collector struct {
	mu        sync.RWMutex
	report    *report.Report
	scrapeErr bool

	up               *prometheus.Desc
	status           *prometheus.Desc
	hitRate          *prometheus.Desc
	usedMemory       *prometheus.Desc
	maxMemory        *prometheus.Desc
	fragmentation    *prometheus.Desc
	connectedClients *prometheus.Desc
	blockedClients   *prometheus.Desc
	opsPerSec        *prometheus.Desc
	lastScrape       *prometheus.Desc
	hitsTotal        *prometheus.Desc
	missesTotal      *prometheus.Desc
	evictedTotal     *prometheus.Desc
	rejectedTotal    *prometheus.Desc
	dbKeys           *prometheus.Desc
	persistenceOK    *prometheus.Desc
}

func New() *Collector {
	desc := func(name, help string, labels ...string) *prometheus.Desc {
		return prometheus.NewDesc(namespace+"_"+name, help, labels, nil)
	}
	return &Collector{
		up:               desc("up", "1 if the last scrape succeeded, else 0"),
		status:           desc("status", "Overall status: 0=OK, 1=WARN, 2=CRITICAL"),
		hitRate:          desc("hit_rate", "Cache hit rate in percent"),
		usedMemory:       desc("used_memory_bytes", "Used memory in bytes"),
		maxMemory:        desc("max_memory_bytes", "Configured maxmemory in bytes (0 = unlimited)"),
		fragmentation:    desc("mem_fragmentation_ratio", "Memory fragmentation ratio"),
		connectedClients: desc("connected_clients", "Number of connected clients"),
		blockedClients:   desc("blocked_clients", "Number of blocked clients"),
		opsPerSec:        desc("ops_per_sec", "Instantaneous operations per second"),
		lastScrape:       desc("last_scrape_timestamp_seconds", "Unix timestamp of the last successful scrape"),
		hitsTotal:        desc("keyspace_hits_total", "Total keyspace hits"),
		missesTotal:      desc("keyspace_misses_total", "Total keyspace misses"),
		evictedTotal:     desc("evicted_keys_total", "Total evicted keys"),
		rejectedTotal:    desc("rejected_connections_total", "Total rejected connections"),
		dbKeys:           desc("db_keys", "Number of keys in the sampled DB", "db"),
		persistenceOK:    desc("persistence_ok", "1 if the persistence status is ok, else 0", "type"),
	}
}

// Set replaces the report that subsequent scrapes will expose.
func (c *Collector) Set(r *report.Report, scrapeErr bool) {
	c.mu.Lock()
	c.report = r
	c.scrapeErr = scrapeErr
	c.mu.Unlock()
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.status
	ch <- c.hitRate
	ch <- c.usedMemory
	ch <- c.maxMemory
	ch <- c.fragmentation
	ch <- c.connectedClients
	ch <- c.blockedClients
	ch <- c.opsPerSec
	ch <- c.lastScrape
	ch <- c.hitsTotal
	ch <- c.missesTotal
	ch <- c.evictedTotal
	ch <- c.rejectedTotal
	ch <- c.dbKeys
	ch <- c.persistenceOK
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	r := c.report
	scrapeErr := c.scrapeErr
	c.mu.RUnlock()

	gauge := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}
	counter := func(d *prometheus.Desc, v float64) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.CounterValue, v)
	}

	up := 1.0
	if scrapeErr || r == nil {
		up = 0
	}
	gauge(c.up, up)
	if r == nil {
		return
	}

	gauge(c.status, float64(r.Status))
	if r.HitRate != nil {
		gauge(c.hitRate, *r.HitRate)
	}
	gauge(c.usedMemory, float64(r.UsedMemoryBytes))
	gauge(c.maxMemory, float64(r.MaxMemoryBytes))
	if r.MemFragmentationRatio != nil {
		gauge(c.fragmentation, *r.MemFragmentationRatio)
	}
	gauge(c.connectedClients, float64(r.ConnectedClients))
	gauge(c.blockedClients, float64(r.BlockedClients))
	gauge(c.opsPerSec, float64(r.OpsPerSec))
	gauge(c.lastScrape, float64(r.SampledAt.Unix()))

	counter(c.hitsTotal, float64(r.KeyspaceHits))
	counter(c.missesTotal, float64(r.KeyspaceMisses))
	counter(c.evictedTotal, float64(r.EvictedKeys))
	counter(c.rejectedTotal, float64(r.RejectedConnections))

	gauge(c.dbKeys, float64(r.DBSize), strconv.Itoa(r.DBIndex))
	gauge(c.persistenceOK, boolToFloat(strings.EqualFold(r.RDBLastBgsaveStatus, "ok")), "rdb")
	gauge(c.persistenceOK, boolToFloat(strings.EqualFold(r.AOFLastWriteStatus, "ok")), "aof")
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
