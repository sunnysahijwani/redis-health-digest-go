// Command redis-health-digest samples a Redis instance and emits a thresholded
// health digest — either once (the `digest` subcommand, for cron) or as a
// long-running daemon exposing /healthz and /metrics (the `serve` subcommand).
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/collector"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/config"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/metrics"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/output"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/report"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/server"
	"github.com/sunnysahijwani/redis-health-digest-go/internal/state"
)

// version is set at release build time via -ldflags "-X main.version=v1.0.0".
// When unset (e.g. `go install ...@v1.2.3`), resolveVersion falls back to the
// version Go embeds in the binary's build info.
var version = "dev"

// resolveVersion reports the binary's version. An explicit ldflags value wins;
// otherwise it uses the module version recorded in the build info (the tag or
// pseudo-version for `go install`), falling back to "dev" for local builds.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "digest":
		err = runDigest(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Println("redis-health-digest", resolveVersion())
		return
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		var ec exitCode
		if errorsAs(err, &ec) {
			os.Exit(int(ec))
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `redis-health-digest — thresholded Redis health digest

Usage:
  redis-health-digest digest [flags]   Sample once and print/email a digest (cron).
  redis-health-digest serve  [flags]   Run a daemon exposing /healthz and /metrics.
  redis-health-digest version          Print the version.

Common flags:
  -config PATH   YAML config file (or REDIS_HEALTH_CONFIG)
  -redis ADDR    Redis address host:port (default 127.0.0.1:6379)
  -password S    Redis password
  -db N          Redis DB index
  -tls           Connect using TLS

digest flags:
  -format console|json   Output format (default console)
  -state PATH            State file for "since last check" deltas
  -no-mail               Do not send email even if configured

serve flags:
  -addr ADDR             HTTP listen address (default :9099)
  -interval DURATION     Scrape interval (default 30s)
`)
}

// bindRedis registers the flags shared by both subcommands.
func bindRedis(fs *flag.FlagSet, c *config.Config) {
	fs.StringVar(&c.Redis.Addr, "redis", c.Redis.Addr, "Redis address host:port")
	fs.StringVar(&c.Redis.Username, "username", c.Redis.Username, "Redis username")
	fs.StringVar(&c.Redis.Password, "password", c.Redis.Password, "Redis password")
	fs.IntVar(&c.Redis.DB, "db", c.Redis.DB, "Redis DB index")
	fs.BoolVar(&c.Redis.TLS, "tls", c.Redis.TLS, "connect using TLS")
	fs.StringVar(&c.Environment, "env", c.Environment, "environment label")
}

func runDigest(args []string) error {
	var (
		format string
		noMail bool
	)
	cfg, _, err := config.Load("digest", args, func(fs *flag.FlagSet, c *config.Config) {
		bindRedis(fs, c)
		fs.StringVar(&c.StatePath, "state", c.StatePath, "state file path for deltas")
		fs.StringVar(&format, "format", "console", "output format: console|json")
		fs.BoolVar(&noMail, "no-mail", false, "do not send email")
	})
	if err != nil {
		return err
	}

	client := newRedisClient(cfg.Redis)
	defer client.Close()

	col := &collector.Collector{
		Provider:    collector.RedisProvider{Client: client},
		Environment: cfg.Environment,
		Location:    cfg.Location(),
		DBIndex:     cfg.Redis.DB,
	}

	store := state.Store{Path: cfg.StatePath}
	prev, err := store.Load()
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	r, err := col.Collect(ctx, prev)
	if err != nil {
		return fmt.Errorf("collect: %w", err)
	}
	cfg.Thresholds.Evaluate(r)

	if format == "json" {
		if err := output.JSON(os.Stdout, r); err != nil {
			return err
		}
	} else {
		output.Console(os.Stdout, r)
	}

	if err := deliverEmail(cfg.Email, r, noMail); err != nil {
		return err
	}

	if err := store.Save(state.Snapshot{
		EvictedKeys:         r.EvictedKeys,
		RejectedConnections: r.RejectedConnections,
		SampledAt:           r.SampledAt,
	}); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	if r.Status == report.StatusCritical {
		return exitCode(1)
	}
	return nil
}

func runServe(args []string) error {
	cfg, _, err := config.Load("serve", args, func(fs *flag.FlagSet, c *config.Config) {
		bindRedis(fs, c)
		fs.StringVar(&c.Server.Addr, "addr", c.Server.Addr, "HTTP listen address")
		fs.DurationVar(&c.Server.Interval, "interval", c.Server.Interval, "scrape interval")
	})
	if err != nil {
		return err
	}

	client := newRedisClient(cfg.Redis)
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	col := &collector.Collector{
		Provider:    collector.RedisProvider{Client: client},
		Environment: cfg.Environment,
		Location:    cfg.Location(),
		DBIndex:     cfg.Redis.DB,
	}

	srv := server.New(server.Deps{
		Collector:  col,
		Thresholds: cfg.Thresholds,
		Metrics:    metrics.New(),
		Interval:   cfg.Server.Interval,
		Logger:     logger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return srv.Run(ctx, cfg.Server.Addr)
}

func deliverEmail(cfg config.EmailConfig, r *report.Report, noMail bool) error {
	if !cfg.Enabled || noMail {
		return nil
	}
	if cfg.OnlyWhenNotOK && r.Status == report.StatusOK {
		return nil
	}
	if len(cfg.To) == 0 {
		fmt.Fprintln(os.Stderr, "warning: email enabled but no recipients configured")
		return nil
	}
	if err := output.Send(cfg, r); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	fmt.Fprintln(os.Stderr, "digest emailed to", cfg.To)
	return nil
}

func newRedisClient(cfg config.RedisConfig) *redis.Client {
	opts := &redis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
	if cfg.TLS {
		opts.TLSConfig = &tls.Config{ServerName: hostOnly(cfg.Addr)}
	}
	return redis.NewClient(opts)
}

func hostOnly(addr string) string {
	for i := 0; i < len(addr); i++ {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// exitCode is an error carrying a process exit status.
type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// errorsAs is a tiny wrapper so main doesn't import errors just for one call.
func errorsAs(err error, target *exitCode) bool {
	if ec, ok := err.(exitCode); ok {
		*target = ec
		return true
	}
	return false
}
