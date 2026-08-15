package output_test

import (
	"strings"
	"testing"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/config"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/output"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/testsupport"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/threshold"
)

func TestBuildMessage(t *testing.T) {
	r := testsupport.Report(nil)
	threshold.Default().Evaluate(r)

	msg := string(output.BuildMessage(config.EmailConfig{
		From:    "monitor@example.com",
		To:      []string{"ops@example.com"},
		Subject: "Redis Health Digest",
	}, r))

	for _, want := range []string{
		"Subject: [OK] Redis Health Digest — testing",
		"To: ops@example.com",
		"Content-Type: text/html",
		"71.89",
		"All thresholds within limits",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q", want)
		}
	}
}
