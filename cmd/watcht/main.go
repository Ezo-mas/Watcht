// Command watcht is the self-hosted film and TV tracker.
//
// Usage:
//
//	watcht [serve]      run the server (the default)
//	watcht healthcheck  exit 0 if the local server's /healthz responds OK
//	watcht version      print the version
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ezo-mas/watcht/internal/config"
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
  serve        run the server (default)
  healthcheck  check that the local server is healthy (for container health checks)
  version      print the version
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

	handler := httpapi.NewRouter(httpapi.Deps{
		Logger: logger,
		WebUI:  webui.Handler(webui.Dist()),
	})
	logger.Info("starting watcht", "version", version)
	if err := server.Run(ctx, cfg.Listen, handler, logger); err != nil {
		logger.Error("server stopped", "err", err)
		return 1
	}
	return 0
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
