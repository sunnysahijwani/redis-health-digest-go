// Package output renders a report.Report as a console table, JSON, or email.
package output

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
)

// Console writes a human-readable table with a colored status banner.
func Console(w io.Writer, r *report.Report) {
	fmt.Fprintf(w, "\n  %s Redis Health Digest — %s\n\n", banner(r.Status), r.Environment)

	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, row := range rows(r) {
		fmt.Fprintf(tw, "  %s\t%s\n", row[0], row[1])
	}
	_ = tw.Flush()

	for _, msg := range r.ReasonMessages() {
		fmt.Fprintf(w, "  • %s\n", msg)
	}
	fmt.Fprintln(w)
}

func rows(r *report.Report) [][2]string {
	out := [][2]string{
		{"Sampled at", r.SampledAt.Format("2006-01-02T15:04:05Z07:00")},
		{"Redis", fmt.Sprintf("%s (%s)", orNA(r.RedisVersion), orNA(r.Role))},
		{fmt.Sprintf("DB %d keys", r.DBIndex), strconv.FormatInt(r.DBSize, 10)},
		{"Used memory", memoryLabel(r)},
		{"Fragmentation", floatLabel(r.MemFragmentationRatio)},
		{"Hits / Misses", fmt.Sprintf("%d / %d", r.KeyspaceHits, r.KeyspaceMisses)},
		{"Hit rate", hitRateLabel(r.HitRate)},
		{"Evicted (Δ)", fmt.Sprintf("%d (%d)", r.EvictedKeys, r.EvictedKeysDelta)},
		{"Rejected conns (Δ)", fmt.Sprintf("%d (%d)", r.RejectedConnections, r.RejectedConnectionsDelta)},
		{"Clients", fmt.Sprintf("%d connected, %d blocked", r.ConnectedClients, r.BlockedClients)},
		{"Ops/sec", strconv.FormatInt(r.OpsPerSec, 10)},
		{"Persistence", fmt.Sprintf("RDB %s / AOF %s", orNA(r.RDBLastBgsaveStatus), orNA(r.AOFLastWriteStatus))},
	}
	for _, name := range sortedKeys(r.CustomGauges) {
		out = append(out, [2]string{name, strconv.FormatFloat(r.CustomGauges[name], 'f', -1, 64)})
	}
	return out
}

// banner returns an ANSI-colored status chip.
func banner(s report.Status) string {
	color := "42" // green background
	switch s {
	case report.StatusWarn:
		color = "43" // yellow
	case report.StatusCritical:
		color = "41" // red
	}
	return fmt.Sprintf("\033[%s;30m %s \033[0m", color, s)
}

func memoryLabel(r *report.Report) string {
	label := fmt.Sprintf("%.2f MB", r.UsedMemoryMB())
	if pct := r.MemoryUsagePercent(); pct != nil {
		label += fmt.Sprintf(" (%.2f%% of %.2f MB max)", *pct, *r.MaxMemoryMB())
	}
	return label
}

func hitRateLabel(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.2f %%", *v)
}

func floatLabel(v *float64) string {
	if v == nil {
		return "n/a"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func orNA(s string) string {
	if s == "" {
		return "n/a"
	}
	return s
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
