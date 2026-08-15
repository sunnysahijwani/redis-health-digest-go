package metrics_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/metrics"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/testsupport"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/threshold"
)

func TestCollectorExposesReport(t *testing.T) {
	r := testsupport.Report(nil)
	threshold.Default().Evaluate(r)

	c := metrics.New()
	c.Set(r, false)

	reg := prometheus.NewRegistry()
	reg.MustRegister(c)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	values := map[string]float64{}
	for _, mf := range families {
		for _, m := range mf.GetMetric() {
			key := mf.GetName()
			for _, label := range m.GetLabel() {
				key += "{" + label.GetName() + "=" + label.GetValue() + "}"
			}
			if g := m.GetGauge(); g != nil {
				values[key] = g.GetValue()
			}
			if ct := m.GetCounter(); ct != nil {
				values[key] = ct.GetValue()
			}
		}
	}

	want := map[string]float64{
		"redis_health_up":                       1,
		"redis_health_status":                   0,
		"redis_health_hit_rate":                 71.89,
		"redis_health_used_memory_bytes":        154920960,
		"redis_health_keyspace_hits_total":      1043712,
		"redis_health_db_keys{db=5}":            104,
		"redis_health_persistence_ok{type=rdb}": 1,
	}
	for name, wantVal := range want {
		got, ok := values[name]
		if !ok {
			t.Errorf("metric %s missing", name)
			continue
		}
		if got != wantVal {
			t.Errorf("metric %s = %v, want %v", name, got, wantVal)
		}
	}
}

func TestCollectorScrapeError(t *testing.T) {
	c := metrics.New()
	c.Set(nil, true)

	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	for _, mf := range families {
		if mf.GetName() == "redis_health_up" {
			if v := mf.GetMetric()[0].GetGauge().GetValue(); v != 0 {
				t.Errorf("up = %v on scrape error, want 0", v)
			}
			return
		}
	}
	t.Error("redis_health_up not exposed")
}
