package collector_test

import (
	"testing"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/collector"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/state"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/testsupport"
)

func TestParseInfo(t *testing.T) {
	info := collector.ParseInfo(testsupport.RedisInfo)

	if got := info.Str("redis_version"); got != "7.2.4" {
		t.Errorf("redis_version = %q, want 7.2.4", got)
	}
	if got := info.Int("keyspace_hits"); got != 1043712 {
		t.Errorf("keyspace_hits = %d, want 1043712", got)
	}
	if got := info.Keys(5); got != 104 {
		t.Errorf("db5 keys = %d, want 104", got)
	}
	if got := info.Keys(0); got != 1204 {
		t.Errorf("db0 keys = %d, want 1204", got)
	}
}

func TestParseInfoIgnoresJunk(t *testing.T) {
	info := collector.ParseInfo("# Server\r\ngarbage_without_colon\r\nredis_version:7.0.0\r\n\r\n")
	if got := info.Str("redis_version"); got != "7.0.0" {
		t.Errorf("redis_version = %q, want 7.0.0", got)
	}
	if _, ok := info.Fields["garbage_without_colon"]; ok {
		t.Error("malformed line should have been skipped")
	}
}

func TestBuildComputesDerivedMetrics(t *testing.T) {
	r := testsupport.Report(nil)

	if r.HitRate == nil || *r.HitRate != 71.89 {
		t.Errorf("hit rate = %v, want 71.89", r.HitRate)
	}
	if r.DBSize != 104 {
		t.Errorf("db size = %d, want 104", r.DBSize)
	}
	if got := r.UsedMemoryMB(); got != 147.74 {
		t.Errorf("used memory MB = %v, want 147.74", got)
	}
	if r.Role != "master" {
		t.Errorf("role = %q, want master", r.Role)
	}
}

func TestBuildNoDivideByZero(t *testing.T) {
	col := testsupport.NewCollector()
	r := col.Build(collector.ParseInfo("keyspace_hits:0\nkeyspace_misses:0\n"), nil)
	if r.HitRate != nil {
		t.Errorf("hit rate = %v, want nil when there is no traffic", *r.HitRate)
	}
}

func TestBuildDeltas(t *testing.T) {
	col := testsupport.NewCollector()
	info := collector.ParseInfo("evicted_keys:12\nrejected_connections:3\n")
	prev := &state.Snapshot{EvictedKeys: 5, RejectedConnections: 3}

	r := col.Build(info, prev)
	if r.EvictedKeysDelta != 7 {
		t.Errorf("evicted delta = %d, want 7", r.EvictedKeysDelta)
	}
	if r.RejectedConnectionsDelta != 0 {
		t.Errorf("rejected delta = %d, want 0", r.RejectedConnectionsDelta)
	}
}
