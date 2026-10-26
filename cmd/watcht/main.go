// Command watcht is the self-hosted film and TV tracker.
//
// Usage:
//
//	watcht [serve]        run the server (the default)
//	watcht backup [FILE]  write a consistent snapshot of the database
//	watcht healthcheck    exit 0 if the local server's /healthz responds OK
//	watcht version        print the version
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ezo-mas/watcht/internal/auth"
	"github.com/Ezo-mas/watcht/internal/config"
	"github.com/Ezo-mas/watcht/internal/db"
	"github.com/Ezo-mas/watcht/internal/httpapi"
	"github.com/Ezo-mas/watcht/internal/server"
	"github.com/Ezo-mas/watcht/internal/webui"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "serve":
		return serve(stderr)
	case "backup":
		return backup(args[1:], stdout, stderr)
	case "healthcheck":
		return healthcheck(stderr)
	case "version", "--version":
		_, _ = fmt.Fprintln(stdout, "watcht", version)
		return 0
	case "help", "-h", "--help":
		_, _ = fmt.Fprint(stdout, usage)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "watcht: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}

const usage = `Usage: watcht [command]

Commands:
  serve          run the server (default)
  backup [FILE]  write a consistent snapshot of the database to FILE, or to
                 a timestamped file in the data directory's backups/ folder;
                 safe while the server is running
  healthcheck    check that the local server is healthy (for container health checks)
  version        print the version
`

func serve(stderr io.Writer) int {
	logger := slog.New(slog.NewTextHandler(stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "err", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		// Restore default signal handling once shutdown starts, so a second
		// Ctrl-C kills the process instead of waiting for the drain.
		<-ctx.Done()
		stop()
	}()

	logger.Info("starting watcht", "version", version, "data_dir", cfg.DataDir)
	if len(cfg.TrustedProxies) > 0 && (cfg.BaseURL == nil || cfg.BaseURL.Scheme != "https") {
		// Behind a TLS-terminating proxy, the scheme is only known from its
		// X-Forwarded-Proto header; if it doesn't send one, cookies lose Secure.
		logger.Warn("WATCHT_TRUSTED_PROXIES is set but WATCHT_BASE_URL isn't an https URL; if the proxy serves HTTPS, set WATCHT_BASE_URL=https://<your host> so session cookies are always Secure")
	}
	database, err := openDatabase(ctx, cfg, logger)
	if err != nil {
		logger.Error("database unavailable", "err", err)
		return 1
	}

	handler, err := httpapi.NewRouter(httpapi.Deps{
		Logger:         logger,
		WebUI:          webui.Handler(webui.Dist()),
		Health:         database.Ping,
		Auth:           auth.New(database, auth.Options{Logger: logger}),
		BaseOrigin:     cfg.BaseOrigin(),
		TrustedProxies: cfg.TrustedProxies,
	})
	if err != nil {
		logger.Error("invalid configuration", "err", err)
		_ = database.Close()
		return 1
	}
	code := 0
	if err := server.Run(ctx, cfg.Listen, handler, logger); err != nil {
		logger.Error("server stopped", "err", err)
		code = 1
	}
	// After the HTTP server has drained, so no request still uses it.
	if err := database.Close(); err != nil {
		logger.Error("close database", "err", err)
		code = 1
	}
	return code
}

// openDatabase prepares the data directory, opens the database, and applies
// pending migrations (backing up first when there's existing data).
func openDatabase(ctx context.Context, cfg config.Config, logger *slog.Logger) (*db.DB, error) {
	if err := cfg.EnsureDataDir(); err != nil {
		return nil, err
	}
	database, err := db.Open(ctx, cfg.DatabasePath())
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(ctx, db.MigrateOptions{BackupDir: cfg.BackupDir(), Logger: logger}); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func healthcheck(stderr io.Writer) int {
	cfg, err := config.Load()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "watcht healthcheck:", err)
		return 1
	}
	if err := checkHealth(context.Background(), cfg.Listen); err != nil {
		_, _ = fmt.Fprintln(stderr, "watcht healthcheck:", err)
		return 1
	}
	return 0
}
