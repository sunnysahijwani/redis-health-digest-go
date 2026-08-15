package threshold_test

import (
	"strings"
	"testing"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/collector"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/state"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/testsupport"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/threshold"
)

func TestEvaluateOK(t *testing.T) {
	r := testsupport.Report(nil)
	threshold.Default().Evaluate(r)

	if r.Status != report.StatusOK {
		t.Fatalf("status = %v, want OK", r.Status)
	}
	if msgs := r.ReasonMessages(); len(msgs) != 1 || msgs[0] != "All thresholds within limits" {
		t.Errorf("reasons = %v, want [All thresholds within limits]", msgs)
	}
}

func TestEvaluateWarnOnHitRate(t *testing.T) {
	r := testsupport.Report(nil)
	th := threshold.Default()
	th.MinHitRate = 80
	th.Evaluate(r)

	if r.Status != report.StatusWarn {
		t.Fatalf("status = %v, want WARN", r.Status)
	}
	if !strings.Contains(r.ReasonMessages()[0], "Hit rate 71.89% below minimum 80.00%") {
		t.Errorf("unexpected reason: %q", r.ReasonMessages()[0])
	}
}

func TestEvaluateCriticalOnPersistence(t *testing.T) {
	col := testsupport.NewCollector()
	info := collector.ParseInfo(strings.Replace(testsupport.RedisInfo, "rdb_last_bgsave_status:ok", "rdb_last_bgsave_status:err", 1))
	r := col.Build(info, nil)
	threshold.Default().Evaluate(r)

	if r.Status != report.StatusCritical {
		t.Fatalf("status = %v, want CRITICAL", r.Status)
	}
	if !strings.Contains(r.ReasonMessages()[0], `RDB last bgsave status is "err"`) {
		t.Errorf("unexpected reason: %q", r.ReasonMessages()[0])
	}
}

func TestEvaluateEvictionDelta(t *testing.T) {
	col := testsupport.NewCollector()
	info := collector.ParseInfo("evicted_keys:20\n")
	r := col.Build(info, &state.Snapshot{EvictedKeys: 5})
	threshold.Default().Evaluate(r)

	if r.Status != report.StatusWarn {
		t.Fatalf("status = %v, want WARN", r.Status)
	}
	if !strings.Contains(r.ReasonMessages()[0], "15 keys evicted since last check") {
		t.Errorf("unexpected reason: %q", r.ReasonMessages()[0])
	}
}

func TestEvaluateWorstOf(t *testing.T) {
	col := testsupport.NewCollector()
	info := collector.ParseInfo(strings.Replace(testsupport.RedisInfo, "rdb_last_bgsave_status:ok", "rdb_last_bgsave_status:err", 1))
	r := col.Build(info, nil)
	th := threshold.Default()
	th.MinHitRate = 90 // also trips a WARN
	th.Evaluate(r)

	// WARN (hit rate) + CRITICAL (persistence) => overall CRITICAL, both reasons.
	if r.Status != report.StatusCritical {
		t.Fatalf("status = %v, want CRITICAL", r.Status)
	}
	if len(r.Reasons) != 2 {
		t.Errorf("reasons count = %d, want 2", len(r.Reasons))
	}
}
