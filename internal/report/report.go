// Package report defines the immutable snapshot of sampled Redis metrics and
// the overall health status derived from them.
package report

import (
	"encoding/json"
	"math"
	"time"
)

// Status is the severity of a health check, ordered so the numerically largest
// value is the most severe. This ordering lets Worst pick the worst of a set.
type Status int

const (
	StatusOK Status = iota
	StatusWarn
	StatusCritical
)

func (s Status) String() string {
	switch s {
	case StatusCritical:
		return "CRITICAL"
	case StatusWarn:
		return "WARN"
	default:
		return "OK"
	}
}

// Worst returns the most severe status from the arguments (StatusOK if none).
func Worst(statuses ...Status) Status {
	worst := StatusOK
	for _, s := range statuses {
		if s > worst {
			worst = s
		}
	}
	return worst
}

// Reason is a single human-readable explanation for a non-OK check.
type Reason struct {
	Level   Status
	Message string
}

// Report holds one sample of Redis health metrics plus the evaluated status.
// Nullable numeric fields use pointers so "not available" is distinct from 0.
type Report struct {
	Environment              string
	SampledAt                time.Time
	RedisVersion             string
	UptimeSeconds            int64
	Role                     string
	DBIndex                  int
	DBSize                   int64
	UsedMemoryBytes          int64
	MaxMemoryBytes           int64
	MemFragmentationRatio    *float64
	EvictedKeys              int64
	EvictedKeysDelta         int64
	RejectedConnections      int64
	RejectedConnectionsDelta int64
	ConnectedClients         int64
	BlockedClients           int64
	OpsPerSec                int64
	KeyspaceHits             int64
	KeyspaceMisses           int64
	HitRate                  *float64
	RDBLastBgsaveStatus      string
	AOFLastWriteStatus       string
	CustomGauges             map[string]float64

	Status  Status
	Reasons []Reason
}

func (r *Report) UsedMemoryMB() float64 { return round2(float64(r.UsedMemoryBytes) / 1048576) }

func (r *Report) MaxMemoryMB() *float64 {
	if r.MaxMemoryBytes <= 0 {
		return nil
	}
	v := round2(float64(r.MaxMemoryBytes) / 1048576)
	return &v
}

func (r *Report) MemoryUsagePercent() *float64 {
	if r.MaxMemoryBytes <= 0 {
		return nil
	}
	v := round2(float64(r.UsedMemoryBytes) / float64(r.MaxMemoryBytes) * 100)
	return &v
}

// ReasonMessages returns just the message strings, in order.
func (r *Report) ReasonMessages() []string {
	msgs := make([]string, len(r.Reasons))
	for i, reason := range r.Reasons {
		msgs[i] = reason.Message
	}
	return msgs
}

// MarshalJSON emits a stable, nested, snake_cased document. Using a map keeps
// the encoder deterministic (keys are sorted) without a parallel DTO type.
func (r *Report) MarshalJSON() ([]byte, error) {
	obj := map[string]any{
		"status":      r.Status.String(),
		"environment": r.Environment,
		"sampled_at":  r.SampledAt.Format(time.RFC3339),
		"redis": map[string]any{
			"version":        r.RedisVersion,
			"role":           r.Role,
			"uptime_seconds": r.UptimeSeconds,
		},
		"keyspace": map[string]any{
			"db_index": r.DBIndex,
			"db_size":  r.DBSize,
		},
		"memory": map[string]any{
			"used_bytes":          r.UsedMemoryBytes,
			"used_mb":             r.UsedMemoryMB(),
			"max_bytes":           r.MaxMemoryBytes,
			"max_mb":              r.MaxMemoryMB(),
			"usage_percent":       r.MemoryUsagePercent(),
			"fragmentation_ratio": r.MemFragmentationRatio,
			"evicted_keys":        r.EvictedKeys,
			"evicted_keys_delta":  r.EvictedKeysDelta,
		},
		"cache": map[string]any{
			"hits":     r.KeyspaceHits,
			"misses":   r.KeyspaceMisses,
			"hit_rate": r.HitRate,
		},
		"clients": map[string]any{
			"connected":                  r.ConnectedClients,
			"blocked":                    r.BlockedClients,
			"rejected_connections":       r.RejectedConnections,
			"rejected_connections_delta": r.RejectedConnectionsDelta,
			"ops_per_sec":                r.OpsPerSec,
		},
		"persistence": map[string]any{
			"rdb_last_bgsave_status": r.RDBLastBgsaveStatus,
			"aof_last_write_status":  r.AOFLastWriteStatus,
		},
		"custom_gauges": r.customGaugesOrEmpty(),
		"reasons":       r.ReasonMessages(),
	}
	return json.Marshal(obj)
}

func (r *Report) customGaugesOrEmpty() map[string]float64 {
	if r.CustomGauges == nil {
		return map[string]float64{}
	}
	return r.CustomGauges
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
