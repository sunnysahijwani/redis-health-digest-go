package report_test

import (
	"encoding/json"
	"testing"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/testsupport"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/threshold"
)

func TestStatusString(t *testing.T) {
	cases := map[report.Status]string{
		report.StatusOK:       "OK",
		report.StatusWarn:     "WARN",
		report.StatusCritical: "CRITICAL",
	}
	for status, want := range cases {
		if got := status.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", status, got, want)
		}
	}
}

func TestWorst(t *testing.T) {
	if got := report.Worst(report.StatusOK, report.StatusCritical, report.StatusWarn); got != report.StatusCritical {
		t.Errorf("Worst = %v, want CRITICAL", got)
	}
	if got := report.Worst(); got != report.StatusOK {
		t.Errorf("Worst() = %v, want OK", got)
	}
}

func TestMarshalJSONShape(t *testing.T) {
	r := testsupport.Report(nil)
	threshold.Default().Evaluate(r)

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"status", "environment", "sampled_at", "redis", "memory", "cache", "persistence", "reasons"} {
		if _, ok := doc[key]; !ok {
			t.Errorf("json missing key %q", key)
		}
	}
	if doc["status"] != "OK" {
		t.Errorf("status = %v, want OK", doc["status"])
	}
	cache := doc["cache"].(map[string]any)
	if cache["hit_rate"].(float64) != 71.89 {
		t.Errorf("cache.hit_rate = %v, want 71.89", cache["hit_rate"])
	}
}
