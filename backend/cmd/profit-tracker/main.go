// Command profit-tracker runs the tool: the HTTP API and the embedded frontend on one port.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/httpapi"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := start(logger); err != nil {
		logger.Error("profit-tracker stopped", "error", err)
		os.Exit(1)
	}
}

// start reads the configuration and runs the server until an interrupt or SIGTERM.
func start(logger *slog.Logger) error {
	cfg, err := parseConfig(os.Args[1:], os.Getenv, os.UserConfigDir)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, cfg, logger)
}

// run opens the database, starts the server and stops it when ctx ends.
func run(ctx context.Context, cfg config, logger *slog.Logger) error {
	db, err := store.Open(ctx, cfg.dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	frontend, err := web.Dist()
	if err != nil {
		return err
	}

	// Bind the port first. Thus a busy port fails at once, and the log tells the truth.
	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.addr, err)
	}

	srv := &http.Server{
		Handler: httpapi.NewHandler(httpapi.Options{
			Store:    db,
			Frontend: frontend,
			Logger:   logger,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	logger.Info("profit-tracker started", "url", "http://"+ln.Addr().String(), "db", cfg.dbPath)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve on %s: %w", cfg.addr, err)
	case <-ctx.Done():
	}

	logger.Info("profit-tracker stops")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down server: %w", err)
	}
	return nil
}
