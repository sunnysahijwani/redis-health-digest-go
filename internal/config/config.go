// Package config resolves settings from defaults, an optional YAML file,
// environment variables, and command-line flags — in that precedence order
// (later sources win).
package config

import (
	"flag"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sunnysahijwani/redis-health-digest-go/internal/threshold"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Redis       RedisConfig          `yaml:"redis"`
	Environment string               `yaml:"environment"`
	Timezone    string               `yaml:"timezone"`
	StatePath   string               `yaml:"state_path"`
	Thresholds  threshold.Thresholds `yaml:"thresholds"`
	Email       EmailConfig          `yaml:"email"`
	Server      ServerConfig         `yaml:"server"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	TLS      bool   `yaml:"tls"`
}

type EmailConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Host          string   `yaml:"host"`
	Port          int      `yaml:"port"`
	Username      string   `yaml:"username"`
	Password      string   `yaml:"password"`
	From          string   `yaml:"from"`
	To            []string `yaml:"to"`
	Subject       string   `yaml:"subject"`
	OnlyWhenNotOK bool     `yaml:"only_when_not_ok"`
}

type ServerConfig struct {
	Addr     string        `yaml:"addr"`
	Interval time.Duration `yaml:"interval"`
}

// Default returns the baseline configuration.
func Default() Config {
	return Config{
		Redis:       RedisConfig{Addr: "127.0.0.1:6379", DB: 0},
		Environment: "production",
		Timezone:    "UTC",
		Thresholds:  threshold.Default(),
		Email:       EmailConfig{Port: 587, Subject: "Redis Health Digest"},
		Server:      ServerConfig{Addr: ":9099", Interval: 30 * time.Second},
	}
}

// Location resolves the configured timezone (falling back to UTC).
func (c Config) Location() *time.Location {
	if loc, err := time.LoadLocation(c.Timezone); err == nil {
		return loc
	}
	return time.UTC
}

// Load builds a Config from defaults, then a YAML file (via -config or
// REDIS_HEALTH_CONFIG), then environment variables, then flags. The bind
// callback lets each subcommand register its own flags whose default values are
// the post-env config, so an explicitly-set flag always wins.
func Load(name string, args []string, bind func(*flag.FlagSet, *Config)) (Config, *flag.FlagSet, error) {
	cfg := Default()

	path := extractFlagValue(args, "config")
	if path == "" {
		path = os.Getenv("REDIS_HEALTH_CONFIG")
	}
	if path != "" {
		if err := cfg.mergeYAML(path); err != nil {
			return cfg, nil, err
		}
	}

	cfg.applyEnv()

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var configPath string
	fs.StringVar(&configPath, "config", path, "path to a YAML config file")
	bind(fs, &cfg)

	if err := fs.Parse(args); err != nil {
		return cfg, fs, err
	}
	return cfg, fs, nil
}

func (c *Config) mergeYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Unmarshalling onto the existing struct overlays only the keys present in
	// the file, leaving defaults intact for the rest.
	return yaml.Unmarshal(data, c)
}

func (c *Config) applyEnv() {
	envStr("REDIS_HEALTH_ADDR", &c.Redis.Addr)
	envStr("REDIS_HEALTH_USERNAME", &c.Redis.Username)
	envStr("REDIS_HEALTH_PASSWORD", &c.Redis.Password)
	envInt("REDIS_HEALTH_DB", &c.Redis.DB)
	envBool("REDIS_HEALTH_TLS", &c.Redis.TLS)

	envStr("REDIS_HEALTH_ENV", &c.Environment)
	envStr("REDIS_HEALTH_TZ", &c.Timezone)
	envStr("REDIS_HEALTH_STATE_PATH", &c.StatePath)

	envFloat("REDIS_HEALTH_MIN_HIT_RATE", &c.Thresholds.MinHitRate)
	envFloat("REDIS_HEALTH_MAX_MEMORY_MB", &c.Thresholds.MaxUsedMemoryMB)
	envFloat("REDIS_HEALTH_MAX_FRAGMENTATION", &c.Thresholds.MaxFragmentationRatio)
	envInt64("REDIS_HEALTH_MAX_EVICTED_DELTA", &c.Thresholds.MaxEvictedKeysDelta)
	envInt64("REDIS_HEALTH_MAX_REJECTED_DELTA", &c.Thresholds.MaxRejectedConnDelta)
	envBool("REDIS_HEALTH_REQUIRE_PERSISTENCE_OK", &c.Thresholds.RequirePersistenceOK)

	envBool("REDIS_HEALTH_MAIL_ENABLED", &c.Email.Enabled)
	envStr("REDIS_HEALTH_MAIL_HOST", &c.Email.Host)
	envInt("REDIS_HEALTH_MAIL_PORT", &c.Email.Port)
	envStr("REDIS_HEALTH_MAIL_USERNAME", &c.Email.Username)
	envStr("REDIS_HEALTH_MAIL_PASSWORD", &c.Email.Password)
	envStr("REDIS_HEALTH_MAIL_FROM", &c.Email.From)
	envStr("REDIS_HEALTH_MAIL_SUBJECT", &c.Email.Subject)
	envBool("REDIS_HEALTH_MAIL_ONLY_ALERTS", &c.Email.OnlyWhenNotOK)
	if v, ok := os.LookupEnv("REDIS_HEALTH_MAIL_TO"); ok {
		c.Email.To = splitList(v)
	}

	envStr("REDIS_HEALTH_SERVER_ADDR", &c.Server.Addr)
	if v, ok := os.LookupEnv("REDIS_HEALTH_SERVER_INTERVAL"); ok {
		if d, err := time.ParseDuration(v); err == nil {
			c.Server.Interval = d
		}
	}
}

// extractFlagValue scans args for -name/--name (space- or =-separated) so the
// config file can be loaded before the main flag set is defined.
func extractFlagValue(args []string, name string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-"+name || arg == "--"+name {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		for _, prefix := range []string{"-" + name + "=", "--" + name + "="} {
			if strings.HasPrefix(arg, prefix) {
				return strings.TrimPrefix(arg, prefix)
			}
		}
	}
	return ""
}

func splitList(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envStr(key string, dst *string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = v
	}
}

func envInt(key string, dst *int) {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			*dst = n
		}
	}
}

func envInt64(key string, dst *int64) {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			*dst = n
		}
	}
}

func envFloat(key string, dst *float64) {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			*dst = f
		}
	}
}

func envBool(key string, dst *bool) {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			*dst = b
		}
	}
}
