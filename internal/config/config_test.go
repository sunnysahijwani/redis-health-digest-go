package config_test

import (
	"flag"
	"testing"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/config"
)

func TestDefaults(t *testing.T) {
	cfg := config.Default()
	if cfg.Redis.Addr != "127.0.0.1:6379" {
		t.Errorf("default addr = %q", cfg.Redis.Addr)
	}
	if cfg.Thresholds.MinHitRate != 50 {
		t.Errorf("default min hit rate = %v, want 50", cfg.Thresholds.MinHitRate)
	}
}

func TestPrecedenceFlagBeatsEnvBeatsDefault(t *testing.T) {
	t.Setenv("REDIS_HEALTH_ADDR", "env-host:6379")
	t.Setenv("REDIS_HEALTH_MIN_HIT_RATE", "70")

	cfg, _, err := config.Load("test", []string{"-redis", "flag-host:6379"}, func(fs *flag.FlagSet, c *config.Config) {
		fs.StringVar(&c.Redis.Addr, "redis", c.Redis.Addr, "")
	})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Flag was set explicitly -> wins over the env value.
	if cfg.Redis.Addr != "flag-host:6379" {
		t.Errorf("addr = %q, want flag-host:6379 (flag overrides env)", cfg.Redis.Addr)
	}
	// No flag for this one -> env value applied over the default.
	if cfg.Thresholds.MinHitRate != 70 {
		t.Errorf("min hit rate = %v, want 70 (env overrides default)", cfg.Thresholds.MinHitRate)
	}
}

func TestEnvListParsing(t *testing.T) {
	t.Setenv("REDIS_HEALTH_MAIL_TO", "a@x.com, b@y.com ")
	cfg, _, err := config.Load("test", nil, func(*flag.FlagSet, *config.Config) {})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Email.To) != 2 || cfg.Email.To[0] != "a@x.com" || cfg.Email.To[1] != "b@y.com" {
		t.Errorf("mail to = %v, want [a@x.com b@y.com]", cfg.Email.To)
	}
}
