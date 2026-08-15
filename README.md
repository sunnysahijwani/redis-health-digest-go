# Redis Health Digest (Go)

[![test](https://github.com/sunnysahijwani/redis-health-digest-go/actions/workflows/test.yml/badge.svg)](https://github.com/sunnysahijwani/redis-health-digest-go/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/sunnysahijwani/redis-health-digest-go.svg)](https://pkg.go.dev/github.com/sunnysahijwani/redis-health-digest-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

A single static binary that samples a Redis instance with one read-only `INFO`
command, evaluates it against configurable thresholds, and reports a
**thresholded `OK` / `WARN` / `CRITICAL` digest** — as a **cron one-shot**
(console / JSON / email) or as a **long-running daemon** exposing `/healthz`
and Prometheus `/metrics`.

> One `INFO` call per sample. **No keys written, no `SELECT`, nothing flushed** — safe against production.

> 🐘 Prefer PHP/Laravel? See the sister package: **[redis-health-digest](https://github.com/sunnysahijwani/redis-health-digest)**.

---

## Why a Go version

The PHP/Laravel package is perfect *inside* a Laravel app. This one is for
**operating Redis anywhere**:

- **Zero runtime dependencies** — one static binary, `scp` it to any box and run.
- **Cron *or* daemon** — the same tool does a one-shot digest for cron, or runs
  resident and exposes a Prometheus endpoint for Grafana/Alertmanager.
- **Kubernetes-friendly** — `/healthz` returns `503` when Redis is CRITICAL, so
  it works as a liveness/readiness probe or a sidecar.
- **Cross-compiles** — build for `linux/amd64`, `linux/arm64`, `darwin/*` from
  one machine.

## Install

```bash
go install github.com/sunnysahijwani/redis-health-digest-go/cmd/redis-health-digest@latest
```

Or build from source:

```bash
git clone https://github.com/sunnysahijwani/redis-health-digest-go
cd redis-health-digest-go
go build -o redis-health-digest ./cmd/redis-health-digest
```

Requires Go 1.23+ to build. The resulting binary has no runtime dependencies.

## Usage

### One-shot (cron)

```bash
redis-health-digest digest --redis 127.0.0.1:6379            # console table
redis-health-digest digest --redis 127.0.0.1:6379 --format json
redis-health-digest digest --config config.yaml              # email per config
```

Exit code is **1** when the status is `CRITICAL`, so cron/CI can alert:

```bash
redis-health-digest digest || notify-my-pager "Redis unhealthy"
```

Schedule it:

```cron
0 9 * * *  /usr/local/bin/redis-health-digest digest --config /etc/redis-health.yaml
```

### Daemon (Prometheus + health probe)

```bash
redis-health-digest serve --redis 127.0.0.1:6379 --addr :9099 --interval 30s
# GET /healthz  -> 200 (OK/WARN) or 503 (CRITICAL)
# GET /metrics  -> Prometheus exposition
```

### Sample console output

```
  OK  Redis Health Digest — production

  Sampled at          2026-08-12T09:01:55+02:00
  Redis               7.2.4 (master)
  DB 5 keys           104
  Used memory         147.74 MB
  Fragmentation       1.12
  Hits / Misses       1043712 / 408168
  Hit rate            71.89 %
  Evicted (Δ)         0 (0)
  Rejected conns (Δ)  0 (0)
  Clients             12 connected, 0 blocked
  Ops/sec             35
  Persistence         RDB ok / AOF ok

  • All thresholds within limits
```

### Prometheus metrics

All metrics are namespaced `redis_health_`:

| Metric | Type | Notes |
| --- | --- | --- |
| `redis_health_up` | gauge | 1 if the last scrape succeeded |
| `redis_health_status` | gauge | 0=OK, 1=WARN, 2=CRITICAL |
| `redis_health_hit_rate` | gauge | cache hit rate % |
| `redis_health_used_memory_bytes` / `redis_health_max_memory_bytes` | gauge | |
| `redis_health_mem_fragmentation_ratio` | gauge | |
| `redis_health_keyspace_hits_total` / `redis_health_keyspace_misses_total` | counter | |
| `redis_health_evicted_keys_total` / `redis_health_rejected_connections_total` | counter | |
| `redis_health_connected_clients` / `redis_health_blocked_clients` | gauge | |
| `redis_health_db_keys{db="N"}` | gauge | keys in the sampled DB |
| `redis_health_persistence_ok{type="rdb\|aof"}` | gauge | 1 if status is ok |
| `redis_health_last_scrape_timestamp_seconds` | gauge | |

## Configuration

Settings resolve from **defaults → YAML file → environment → flags** (later wins).
See [`config.example.yaml`](config.example.yaml) for the full file. Key env vars:

| Env | Default | Purpose |
| --- | --- | --- |
| `REDIS_HEALTH_ADDR` | `127.0.0.1:6379` | Redis address |
| `REDIS_HEALTH_PASSWORD` / `REDIS_HEALTH_DB` / `REDIS_HEALTH_TLS` | — | Redis auth/DB/TLS |
| `REDIS_HEALTH_MIN_HIT_RATE` | `50` | WARN below this % |
| `REDIS_HEALTH_MAX_MEMORY_MB` | `512` | WARN above this |
| `REDIS_HEALTH_MAIL_TO` | — | comma-separated recipients |
| `REDIS_HEALTH_SERVER_ADDR` / `REDIS_HEALTH_SERVER_INTERVAL` | `:9099` / `30s` | daemon |

> **Deltas** (`evicted_keys`, `rejected_connections`) are cumulative counters.
> In one-shot mode they're compared against a small JSON **state file**
> (`--state`); in daemon mode the previous sample is held in memory.

## Architecture

```
INFO (read-only) ─▶ collector.ParseInfo ─▶ collector.Build ─▶ report.Report
                                                                   │
                                            threshold.Evaluate ◀───┘
                                                    │
          ┌──────────────┬────────────────┬────────┴───────┐
      output.Console   output.JSON    output.Send      metrics.Collector
                                        (SMTP)         (Prometheus, daemon)
```

Redis access sits behind a one-method `InfoProvider` interface, which keeps the
whole pipeline pure and testable — the entire suite runs against a fixture
`INFO` string with no live Redis.

```bash
go test -race ./...
```

## License

MIT © [Sunny Sahijwani](https://github.com/sunnysahijwani)
