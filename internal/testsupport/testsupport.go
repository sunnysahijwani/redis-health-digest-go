// Package testsupport provides a shared Redis INFO fixture and helpers so every
// package's tests build reports the same way, without a live Redis.
package testsupport

import (
	"context"
	"time"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/collector"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/state"
)

// RedisInfo is a realistic INFO payload (hits/misses give a 71.89% hit rate,
// used_memory is 147.74 MB, db5 has 104 keys).
const RedisInfo = `# Server
redis_version:7.2.4
redis_mode:standalone
uptime_in_seconds:864000
# Clients
connected_clients:12
blocked_clients:0
# Memory
used_memory:154920960
used_memory_human:147.74M
maxmemory:0
mem_fragmentation_ratio:1.12
# Persistence
rdb_last_bgsave_status:ok
aof_last_write_status:ok
# Stats
instantaneous_ops_per_sec:35
rejected_connections:0
evicted_keys:0
expired_keys:88214
keyspace_hits:1043712
keyspace_misses:408168
# Replication
role:master
connected_slaves:0
# Keyspace
db0:keys=1204,expires=880,avg_ttl=0
db5:keys=104,expires=0,avg_ttl=0
`

// FixedTime is a deterministic SampledAt for reproducible assertions.
var FixedTime = time.Date(2026, 8, 12, 9, 1, 55, 0, time.UTC)

// NewCollector returns a collector wired to the fixture with a fixed clock,
// reporting on DB 5.
func NewCollector() *collector.Collector {
	return &collector.Collector{
		Provider:    FakeProvider{Payload: RedisInfo},
		Environment: "testing",
		Location:    time.UTC,
		DBIndex:     5,
		Now:         func() time.Time { return FixedTime },
	}
}

// Report builds an evaluated report from the fixture (prev may be nil).
func Report(prev *state.Snapshot) *report.Report {
	return NewCollector().Build(collector.ParseInfo(RedisInfo), prev)
}

// FakeProvider is an InfoProvider that returns a canned payload (or error).
type FakeProvider struct {
	Payload string
	Err     error
}

func (f FakeProvider) Info(context.Context) (string, error) {
	return f.Payload, f.Err
}
