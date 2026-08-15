// Package threshold evaluates a report against configurable limits and stamps
// it with an overall status (the worst of every breached check) plus reasons.
package threshold

import (
	"fmt"
	"strings"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
)

// Thresholds are the limits that decide OK / WARN / CRITICAL.
type Thresholds struct {
	MinHitRate            float64 `yaml:"min_hit_rate"`
	MaxUsedMemoryMB       float64 `yaml:"max_used_memory_mb"`
	MaxFragmentationRatio float64 `yaml:"max_fragmentation_ratio"`
	MaxEvictedKeysDelta   int64   `yaml:"max_evicted_keys_delta"`
	MaxRejectedConnDelta  int64   `yaml:"max_rejected_conn_delta"`
	RequirePersistenceOK  bool    `yaml:"require_persistence_ok"`
}

// Default returns sensible starting limits.
func Default() Thresholds {
	return Thresholds{
		MinHitRate:            50,
		MaxUsedMemoryMB:       512,
		MaxFragmentationRatio: 1.5,
		MaxEvictedKeysDelta:   0,
		MaxRejectedConnDelta:  0,
		RequirePersistenceOK:  true,
	}
}

// Evaluate applies the thresholds to r, setting r.Status and r.Reasons.
// A clean report reads "All thresholds within limits".
func (t Thresholds) Evaluate(r *report.Report) {
	var reasons []report.Reason

	if r.HitRate != nil && *r.HitRate < t.MinHitRate {
		reasons = append(reasons, warn(fmt.Sprintf(
			"Hit rate %.2f%% below minimum %.2f%%", *r.HitRate, t.MinHitRate)))
	}

	if used := r.UsedMemoryMB(); used > t.MaxUsedMemoryMB {
		reasons = append(reasons, warn(fmt.Sprintf(
			"Used memory %.2f MB exceeds limit %.0f MB", used, t.MaxUsedMemoryMB)))
	}

	if r.MemFragmentationRatio != nil && *r.MemFragmentationRatio > t.MaxFragmentationRatio {
		reasons = append(reasons, warn(fmt.Sprintf(
			"Memory fragmentation ratio %.2f exceeds %.2f", *r.MemFragmentationRatio, t.MaxFragmentationRatio)))
	}

	if r.EvictedKeysDelta > t.MaxEvictedKeysDelta {
		reasons = append(reasons, warn(fmt.Sprintf(
			"%d keys evicted since last check (limit %d)", r.EvictedKeysDelta, t.MaxEvictedKeysDelta)))
	}

	if r.RejectedConnectionsDelta > t.MaxRejectedConnDelta {
		reasons = append(reasons, warn(fmt.Sprintf(
			"%d connections rejected since last check (limit %d)", r.RejectedConnectionsDelta, t.MaxRejectedConnDelta)))
	}

	if t.RequirePersistenceOK {
		for _, check := range []struct{ label, value string }{
			{"RDB last bgsave", r.RDBLastBgsaveStatus},
			{"AOF last write", r.AOFLastWriteStatus},
		} {
			if check.value != "" && !strings.EqualFold(check.value, "ok") {
				reasons = append(reasons, report.Reason{
					Level:   report.StatusCritical,
					Message: fmt.Sprintf("%s status is %q", check.label, check.value),
				})
			}
		}
	}

	if len(reasons) == 0 {
		r.Status = report.StatusOK
		r.Reasons = []report.Reason{{Level: report.StatusOK, Message: "All thresholds within limits"}}
		return
	}

	levels := make([]report.Status, len(reasons))
	for i, reason := range reasons {
		levels[i] = reason.Level
	}
	r.Status = report.Worst(levels...)
	r.Reasons = reasons
}

func warn(message string) report.Reason {
	return report.Reason{Level: report.StatusWarn, Message: message}
}
